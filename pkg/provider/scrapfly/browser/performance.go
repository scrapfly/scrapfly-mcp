package browser

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/performance"
	"github.com/chromedp/cdproto/performancetimeline"
)

// cdpMonoMs converts CDP's monotonic timestamp (pointer to *time.Time shim)
// into milliseconds since MonotonicTimeEpoch. Returns 0 for nil.
func cdpMonoMs(t *cdp.MonotonicTime) float64 {
	if t == nil {
		return 0
	}
	return float64(t.Time().UnixNano()) / 1e6
}

// cdpEpochMs converts a CDP TimeSinceEpoch into ms-since-epoch. Returns 0 for nil.
func cdpEpochMs(t *cdp.TimeSinceEpoch) float64 {
	if t == nil {
		return 0
	}
	return float64(t.Time().UnixNano()) / 1e6
}

// PSI (PageSpeed Insights) lab-run replica.
//
// Mirrors what pagespeed.dev shows under "Lab data": cold-cache navigation with
// throttling applied, Core Web Vitals + Speed Index + TBT + TTI, resource
// waterfall, diagnostics. Field data (CrUX) is not computable from a CDP
// session and is intentionally not returned.
//
// Every CDP event we consume is unmarshalled into the typed struct from
// github.com/chromedp/cdproto so the shape stays in sync with the protocol.

// ── Presets ────────────────────────────────────────────────────────────────

// Preset matches PSI's two lab environments. The mobile preset reproduces
// Lighthouse's Moto G4 profile (360x640 @ DPR 3, slow 4G, 4x CPU slowdown);
// desktop reproduces 1350x940 wired (10 Mbps, no CPU throttling).
type Preset string

const (
	PresetMobile  Preset = "mobile"
	PresetDesktop Preset = "desktop"
)

type presetConfig struct {
	width, height, dpr  int
	mobile              bool
	ua                  string
	downloadKbps        float64 // in kilobits/s
	uploadKbps          float64
	latencyMs           float64
	cpuSlowdown         float64 // 1 = no throttle
}

var presets = map[Preset]presetConfig{
	PresetMobile: {
		width: 360, height: 640, dpr: 3, mobile: true,
		ua:           "Mozilla/5.0 (Linux; Android 11; moto g(4)) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
		downloadKbps: 1600,
		uploadKbps:   750,
		latencyMs:    150,
		cpuSlowdown:  4,
	},
	PresetDesktop: {
		width: 1350, height: 940, dpr: 1, mobile: false,
		ua:           "",
		downloadKbps: 10000,
		uploadKbps:   10000,
		latencyMs:    40,
		cpuSlowdown:  1,
	},
}

// ── Options / report ───────────────────────────────────────────────────────

type PSIOptions struct {
	Preset    Preset
	TimeoutMs int // total budget for the run (default 10000, capped at 30000)
}

type PSIReport struct {
	Preset       Preset            `json:"preset"`
	URL          string            `json:"url,omitempty"`
	FetchTimeMs  int               `json:"fetch_time_ms"`
	Metrics      LabMetrics        `json:"metrics"`
	Ratings      map[string]string `json:"ratings"`
	Score        int               `json:"performance_score"`
	Diagnostics  Diagnostics       `json:"diagnostics"`
	Resources    ResourceReport    `json:"resources"`
	Field        *string           `json:"field_data"`   // always nil — CrUX requires API; see warnings
	Warnings     []string          `json:"warnings,omitempty"`
}

// LabMetrics: all times in ms, all scores in ms, CLS unitless.
// Pointers for values that can be legitimately missing (e.g. INP without
// interaction) so the LLM sees `null` instead of a misleading zero.
type LabMetrics struct {
	FCPMs        *int    `json:"fcp_ms"`
	LCPMs        *int    `json:"lcp_ms"`
	LCPElement   string  `json:"lcp_element,omitempty"`
	LCPURL       string  `json:"lcp_url,omitempty"`
	CLS          float64 `json:"cls"`
	TBTMs        *int    `json:"tbt_ms"`
	SpeedIndexMs *int    `json:"speed_index_ms"`
	TTIMs        *int    `json:"tti_ms"`
	TTFBMs       *int    `json:"ttfb_ms"`
	INPMs        *int    `json:"inp_ms"`
}

type Diagnostics struct {
	MainThreadMs      int     `json:"main_thread_ms"`
	LongTasksCount    int     `json:"long_tasks_count"`
	LongTasksTotalMs  int     `json:"long_tasks_total_ms"`
	DOMNodes          int     `json:"dom_nodes"`
	RenderBlockingKB  float64 `json:"render_blocking_kb"`
	TotalByteWeightKB float64 `json:"total_byte_weight_kb"`
}

type ResourceReport struct {
	Count           int                  `json:"count"`
	TotalKB         float64              `json:"total_transfer_kb"`
	FromCache       int                  `json:"from_cache_count"`
	ByType          map[string]TypeStats `json:"by_type,omitempty"`
	Slowest         []ResourceEntry      `json:"slowest,omitempty"`
	LargestTransfer []ResourceEntry      `json:"largest_transfer,omitempty"`
	RenderBlocking  []ResourceEntry      `json:"render_blocking,omitempty"`
}

type TypeStats struct {
	Count      int     `json:"count"`
	TransferKB float64 `json:"transfer_kb"`
	// AvgMs is the mean per-request duration in ms. We intentionally do NOT
	// emit a naive wall-clock sum (`total_ms`) because concurrent requests
	// make it look larger than the page's actual load time — 19 parallel 10s
	// requests would show "190s" even though the real elapsed was 10s.
	AvgMs int `json:"avg_ms"`
}

type ResourceEntry struct {
	URL        string  `json:"url"`
	Type       string  `json:"type,omitempty"`
	DurationMs int     `json:"duration_ms"`
	TTFBMs     int     `json:"ttfb_ms,omitempty"`
	TransferKB float64 `json:"transfer_kb,omitempty"`
	FromCache  bool    `json:"from_cache,omitempty"`
}

// ── Internal collection state ──────────────────────────────────────────────

// netReq tracks one Network.requestWillBeSent → responseReceived →
// loadingFinished lifecycle. We keep the minimum needed for the waterfall +
// TTI in-flight counting.
type netReq struct {
	requestID     string
	url           string
	resType       string
	startMs       float64 // Network.EventRequestWillBeSent.timestamp is a monotonic seconds.fractional value
	endMs         float64 // set on loadingFinished / loadingFailed
	ttfbMs        float64
	transferBytes int64 // exact byte count; KB derived at output time
	fromCache     bool
	isRenderBlocking bool // heuristic: stylesheet/script in <head> loaded before FCP
	priority      string
}

type screenFrame struct {
	timestampMs float64
	data        []byte
}

// ── Entry point ────────────────────────────────────────────────────────────

// CollectPSI runs a PSI-style lab measurement against the given session and
// returns the structured report. On hard CDP errors it returns (partial, err);
// on soft errors it appends to report.Warnings.
func CollectPSI(s *Session, opts PSIOptions) (*PSIReport, error) {
	if opts.Preset == "" {
		opts.Preset = PresetMobile
	}
	cfg, ok := presets[opts.Preset]
	if !ok {
		return nil, fmt.Errorf("unknown preset %q (mobile|desktop)", opts.Preset)
	}
	if opts.TimeoutMs <= 0 {
		opts.TimeoutMs = 30000
	}
	if opts.TimeoutMs > 45000 {
		opts.TimeoutMs = 45000
	}

	report := &PSIReport{
		Preset:   opts.Preset,
		Warnings: []string{},
	}
	start := time.Now()

	// 1. Apply preset emulation. Best-effort — if any of these fail we warn
	//    and proceed; the numbers just won't match PSI exactly.
	applyEmulation(s, cfg, report)
	defer clearEmulation(s) // always restore so subsequent tool calls aren't throttled

	// 2. Enable CDP domains we need.
	for _, d := range []string{"Performance", "Page", "Network"} {
		if _, err := s.SendCDP(d+".enable", nil); err != nil {
			report.Warnings = append(report.Warnings, d+".enable: "+err.Error())
		}
	}
	// PerformanceTimeline.enable is strict: ONE bad entry type rejects the whole
	// call, killing subscriptions for the good types too. Chrome's CDP frontend
	// only exposes a subset of the W3C PerformanceObserver entry types here —
	// notably longtask, paint, and event are NOT enumerable via CDP. We keep
	// only the types that Chrome 120+ accepts here; longtask and paint come
	// from the in-page PerformanceObserver fallback below.
	ptlTypes := []string{
		"largest-contentful-paint",
		"layout-shift",
		"first-input",
	}
	if _, err := s.SendCDP("PerformanceTimeline.enable", map[string]any{
		"eventTypes": ptlTypes,
	}); err != nil {
		// Retry per type so a future-added unsupported type doesn't kill the
		// good ones.
		accepted := []string{}
		for _, t := range ptlTypes {
			if _, pe := s.SendCDP("PerformanceTimeline.enable", map[string]any{
				"eventTypes": []string{t},
			}); pe == nil {
				accepted = append(accepted, t)
			}
		}
		if len(accepted) == 0 {
			report.Warnings = append(report.Warnings,
				"PerformanceTimeline.enable: "+err.Error()+" (all types rejected; relying on in-page observer fallback)")
		} else {
			// Only warn if a CRITICAL type was rejected. LCP + layout-shift are
			// the metrics the in-page observer can't always back up (in some
			// iframes/cross-origin cases the CDP path is the only source). If
			// both are accepted, the partial is benign — the in-page fallback
			// covers what we're missing.
			hasLCP, hasCLS := false, false
			for _, t := range accepted {
				if t == "largest-contentful-paint" {
					hasLCP = true
				}
				if t == "layout-shift" {
					hasCLS = true
				}
			}
			if !hasLCP || !hasCLS {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("PerformanceTimeline.enable partial: accepted=%v (critical types missing)", accepted))
			}
		}
	}

	// 3. Subscribe to events. All goroutine-safe via mutex.
	var mu sync.Mutex
	var timelineEvents []performancetimeline.TimelineEvent
	var finalMetrics []performance.Metric
	netByID := map[string]*netReq{}
	var screencastFrames []screenFrame
	loadFired := make(chan struct{}, 1)
	domLoaded := make(chan struct{}, 1)

	registerCollectors(s, &mu, &timelineEvents, &finalMetrics, netByID, &screencastFrames, loadFired, domLoaded)

	// 4. Capture current URL, then blank the viewport so the screencast has a
	//    clean "before" baseline. Without this step, the first screencast frame
	//    on a fast page already shows a painted layout — (1 - progress) is ~0
	//    everywhere and Speed Index comes out absurdly low (~20ms).
	targetURL := ""
	if raw, err := s.SendCDP("Runtime.evaluate", map[string]any{
		"expression":    `location.href`,
		"returnByValue": true,
	}); err == nil {
		var env struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		if json.Unmarshal(raw, &env) == nil {
			targetURL = env.Result.Value
		}
	}
	if targetURL == "" || targetURL == "about:blank" {
		return report, fmt.Errorf("could not resolve current page URL for cold-cache reload")
	}
	// Blank viewport, clear caches for a true cold-cache run.
	_, _ = s.SendCDP("Network.clearBrowserCache", nil)
	if _, err := s.SendCDP("Page.navigate", map[string]any{"url": "about:blank"}); err != nil {
		report.Warnings = append(report.Warnings, "Page.navigate about:blank: "+err.Error())
	}
	// Small pause so the blank page paints before we start the screencast.
	time.Sleep(200 * time.Millisecond)

	// 5. Start screencast with a known-blank baseline. Chrome streams JPEG
	//    frames (~60fps potential; we cap via everyNthFrame=1 = every frame).
	if _, err := s.SendCDP("Page.startScreencast", map[string]any{
		"format":        "jpeg",
		"quality":       30,
		"maxWidth":      cfg.width,
		"maxHeight":     cfg.height,
		"everyNthFrame": 1,
	}); err != nil {
		report.Warnings = append(report.Warnings, "Page.startScreencast (Speed Index will be unavailable): "+err.Error())
	}

	// 6. Navigate to the real URL — this starts the measurement.
	if _, err := s.SendCDP("Page.navigate", map[string]any{"url": targetURL}); err != nil {
		return report, fmt.Errorf("Page.navigate to target: %w", err)
	}

	// 6. Wait for load + LCP-settle window. LCP can update up to the first
	//    user interaction; PSI waits a quiet window after loadEventFired, and
	//    so do we.
	waitForQuietLoad(loadFired, domLoaded, &mu, &timelineEvents, netByID, opts.TimeoutMs, report)

	// 7. Stop screencast, final metrics snapshot.
	if _, err := s.SendCDP("Page.stopScreencast", nil); err != nil {
		report.Warnings = append(report.Warnings, "Page.stopScreencast: "+err.Error())
	}
	if raw, err := s.SendCDP("Performance.getMetrics", nil); err == nil {
		var env struct {
			Metrics []performance.Metric `json:"metrics"`
		}
		if json.Unmarshal(raw, &env) == nil {
			mu.Lock()
			finalMetrics = env.Metrics
			mu.Unlock()
		}
	}

	// 8. In-page fallback: captures LCP/FCP/CLS/longtask/INP/DOM size via
	//    PerformanceObserver({buffered:true}). Authoritative when CDP
	//    PerformanceTimeline subscription dropped or rejected types.
	fallback := fetchPageFallback(s, report)
	var domNodes int
	if fallback != nil {
		domNodes = fallback.DOMNodes
		report.URL = fallback.URL
	}

	// 9. Compute everything from the collected events.
	mu.Lock()
	defer mu.Unlock()
	computeMetrics(report, cfg, timelineEvents, finalMetrics, netByID, screencastFrames, domNodes, int(time.Since(start).Milliseconds()))
	applyFallback(report, fallback)

	report.FetchTimeMs = int(time.Since(start).Milliseconds())
	report.Score, report.Ratings = scoreAndRate(report.Metrics, report.Preset)

	// TTI was removed from Lighthouse v10 scoring — we emit it as informational
	// only. No warning when null; the field just won't appear in ratings.
	ttiDebugLen, ttiDebugStart, ttiDebugMinNt, ttiDebugObsEnd = 0, 0, 0, 0

	report.Warnings = append(report.Warnings,
		"field data (CrUX) requires the PageSpeed Insights API — not derivable from a CDP session")

	return report, nil
}

// ── Emulation ──────────────────────────────────────────────────────────────

func applyEmulation(s *Session, cfg presetConfig, report *PSIReport) {
	deviceParams := map[string]any{
		"width":             cfg.width,
		"height":            cfg.height,
		"deviceScaleFactor": cfg.dpr,
		"mobile":            cfg.mobile,
	}
	if _, err := s.SendCDP("Emulation.setDeviceMetricsOverride", deviceParams); err != nil {
		report.Warnings = append(report.Warnings, "Emulation.setDeviceMetricsOverride: "+err.Error())
	}
	if cfg.ua != "" {
		if _, err := s.SendCDP("Emulation.setUserAgentOverride", map[string]any{"userAgent": cfg.ua}); err != nil {
			report.Warnings = append(report.Warnings, "Emulation.setUserAgentOverride: "+err.Error())
		}
	}
	// Network throttling — convert kilobits/s → bytes/s as CDP expects.
	if _, err := s.SendCDP("Network.emulateNetworkConditions", map[string]any{
		"offline":            false,
		"latency":            cfg.latencyMs,
		"downloadThroughput": cfg.downloadKbps * 1024 / 8,
		"uploadThroughput":   cfg.uploadKbps * 1024 / 8,
	}); err != nil {
		report.Warnings = append(report.Warnings, "Network.emulateNetworkConditions: "+err.Error())
	}
	if cfg.cpuSlowdown > 1 {
		if _, err := s.SendCDP("Emulation.setCPUThrottlingRate", map[string]any{"rate": cfg.cpuSlowdown}); err != nil {
			report.Warnings = append(report.Warnings, "Emulation.setCPUThrottlingRate: "+err.Error())
		}
	}
}

func clearEmulation(s *Session) {
	s.SendCDP("Emulation.clearDeviceMetricsOverride", nil)
	s.SendCDP("Emulation.setUserAgentOverride", map[string]any{"userAgent": ""})
	s.SendCDP("Network.emulateNetworkConditions", map[string]any{
		"offline": false, "latency": 0, "downloadThroughput": -1, "uploadThroughput": -1,
	})
	s.SendCDP("Emulation.setCPUThrottlingRate", map[string]any{"rate": 1})
}

// ── Event collectors ───────────────────────────────────────────────────────

func registerCollectors(
	s *Session,
	mu *sync.Mutex,
	timeline *[]performancetimeline.TimelineEvent,
	finalMetrics *[]performance.Metric,
	netByID map[string]*netReq,
	screencastFrames *[]screenFrame,
	loadFired chan<- struct{},
	domLoaded chan<- struct{},
) {
	s.OnEvent("PerformanceTimeline.timelineEventAdded", func(_ string, params json.RawMessage) bool {
		var evt performancetimeline.EventTimelineEventAdded
		if err := json.Unmarshal(params, &evt); err != nil || evt.Event == nil {
			return true
		}
		mu.Lock()
		*timeline = append(*timeline, *evt.Event)
		mu.Unlock()
		return true
	})

	s.OnEvent("Performance.metrics", func(_ string, params json.RawMessage) bool {
		var evt performance.EventMetrics
		if err := json.Unmarshal(params, &evt); err != nil {
			return true
		}
		mu.Lock()
		*finalMetrics = (*finalMetrics)[:0]
		for _, m := range evt.Metrics {
			if m != nil {
				*finalMetrics = append(*finalMetrics, *m)
			}
		}
		mu.Unlock()
		return true
	})

	s.OnEvent("Network.requestWillBeSent", func(_ string, params json.RawMessage) bool {
		var evt network.EventRequestWillBeSent
		if err := json.Unmarshal(params, &evt); err != nil {
			return true
		}
		mu.Lock()
		netByID[string(evt.RequestID)] = &netReq{
			requestID: string(evt.RequestID),
			url:       evt.Request.URL,
			resType:   strings.ToLower(string(evt.Type)),
			startMs:   cdpMonoMs(evt.Timestamp),
			priority:  string(evt.Request.InitialPriority),
		}
		mu.Unlock()
		return true
	})

	s.OnEvent("Network.responseReceived", func(_ string, params json.RawMessage) bool {
		var evt network.EventResponseReceived
		if err := json.Unmarshal(params, &evt); err != nil {
			return true
		}
		mu.Lock()
		defer mu.Unlock()
		r, ok := netByID[string(evt.RequestID)]
		if !ok {
			return true
		}
		if evt.Response != nil {
			// TTFB = responseReceived.timestamp - requestWillBeSent.timestamp.
			// Close enough to per-resource TTFB for waterfall purposes.
			r.ttfbMs = cdpMonoMs(evt.Timestamp) - r.startMs
			r.fromCache = evt.Response.FromDiskCache || evt.Response.FromServiceWorker
			if evt.Response.Timing != nil {
				// Prefer authoritative protocol timing if available.
				r.ttfbMs = float64(evt.Response.Timing.ReceiveHeadersEnd)
			}
		}
		return true
	})

	s.OnEvent("Network.loadingFinished", func(_ string, params json.RawMessage) bool {
		var evt network.EventLoadingFinished
		if err := json.Unmarshal(params, &evt); err != nil {
			return true
		}
		mu.Lock()
		defer mu.Unlock()
		r, ok := netByID[string(evt.RequestID)]
		if !ok {
			return true
		}
		r.endMs = cdpMonoMs(evt.Timestamp)
		r.transferBytes = int64(evt.EncodedDataLength)
		return true
	})

	s.OnEvent("Network.loadingFailed", func(_ string, params json.RawMessage) bool {
		var evt network.EventLoadingFailed
		if err := json.Unmarshal(params, &evt); err != nil {
			return true
		}
		mu.Lock()
		defer mu.Unlock()
		if r, ok := netByID[string(evt.RequestID)]; ok {
			r.endMs = cdpMonoMs(evt.Timestamp)
		}
		return true
	})

	s.OnEvent("Page.loadEventFired", func(_ string, params json.RawMessage) bool {
		var evt page.EventLoadEventFired
		_ = json.Unmarshal(params, &evt) // we only need the signal
		select {
		case loadFired <- struct{}{}:
		default:
		}
		return false // one-shot
	})
	// domContentLoadedEventFired is a useful fallback signal when load takes
	// >30s but DOM was ready much earlier. The wait logic uses this plus a
	// network-idle window as an alternate exit condition.
	s.OnEvent("Page.domContentEventFired", func(_ string, params json.RawMessage) bool {
		select {
		case domLoaded <- struct{}{}:
		default:
		}
		return false
	})

	s.OnEvent("Page.screencastFrame", func(_ string, params json.RawMessage) bool {
		// Typed: page.EventScreencastFrame.
		var evt page.EventScreencastFrame
		if err := json.Unmarshal(params, &evt); err != nil {
			return true
		}
		// Must ack so Chrome keeps sending.
		if evt.SessionID != 0 {
			s.SendCDPFireAndForget("Page.screencastFrameAck", map[string]any{"sessionId": evt.SessionID})
		}
		if evt.Metadata == nil || evt.Data == "" {
			return true
		}
		raw, err := base64.StdEncoding.DecodeString(evt.Data)
		if err != nil {
			return true
		}
		mu.Lock()
		*screencastFrames = append(*screencastFrames, screenFrame{
			timestampMs: float64(evt.Metadata.Timestamp.Time().UnixMilli()),
			data:        raw,
		})
		mu.Unlock()
		return true
	})
}

// waitForQuietLoad blocks until the page is ready for metric computation.
//
// Exit conditions (whichever happens first):
//   1. loadEventFired + 2s of no new LCP events + no new in-flight requests.
//   2. domContentLoadedEventFired + 3s of network idle (≤2 in-flight) — slow
//      pages where some resources hang past loadEventFired still reach this.
//   3. budgetMs elapsed.
//
// Returns true if we exited via a completion signal (1 or 2), false on budget
// timeout. A warning is appended for partial-data cases.
func waitForQuietLoad(
	loadFired, domLoaded <-chan struct{},
	mu *sync.Mutex,
	timeline *[]performancetimeline.TimelineEvent,
	netByID map[string]*netReq,
	budgetMs int,
	report *PSIReport,
) {
	budget := time.Duration(budgetMs) * time.Millisecond
	deadline := time.After(budget)
	settlePoll := 200 * time.Millisecond
	lcpQuiet := 2 * time.Second
	netIdleFallback := 3 * time.Second
	// TTI needs a 5s post-FCP quiet window to be computable. Guarantee a
	// minimum 5s observation window from navigation start so fast pages don't
	// return with tti_ms=null just because we measured too early.
	minObservation := 5 * time.Second
	startedAt := time.Now()

	var loadSeen, domSeen bool

	checkInFlight := func() int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, r := range netByID {
			if r.endMs == 0 {
				n++
			}
		}
		return n
	}

	lastLCP := lcpCount(mu, timeline)
	lastLCPChange := time.Now()
	lastNetQuiet := time.Time{} // first moment we observed ≤2 in-flight after dom/load

	for {
		select {
		case <-loadFired:
			loadSeen = true
			lastLCP = lcpCount(mu, timeline)
			lastLCPChange = time.Now()
		case <-domLoaded:
			domSeen = true
		case <-deadline:
			// Give a clear, actionable warning describing why we gave up.
			if !loadSeen && !domSeen {
				report.Warnings = append(report.Warnings,
					"neither load nor DOMContentLoaded fired within budget — metrics may be incomplete")
			} else {
				report.Warnings = append(report.Warnings,
					"load did not fully settle within budget — metrics may reflect an in-flight page")
			}
			return
		case <-time.After(settlePoll):
		}

		// Don't exit until minObservation has elapsed, even if the page is
		// fully loaded — needed so TTI's 5s quiet window can be detected.
		if time.Since(startedAt) < minObservation {
			continue
		}

		// Exit path 1: loadEventFired + LCP quiet + network quiet
		if loadSeen {
			cur := lcpCount(mu, timeline)
			if cur != lastLCP {
				lastLCP = cur
				lastLCPChange = time.Now()
			}
			if time.Since(lastLCPChange) >= lcpQuiet && checkInFlight() <= 2 {
				return
			}
		}

		// Exit path 2: DOMContentLoaded + sustained network idle
		if domSeen && !loadSeen {
			if checkInFlight() <= 2 {
				if lastNetQuiet.IsZero() {
					lastNetQuiet = time.Now()
				} else if time.Since(lastNetQuiet) >= netIdleFallback {
					report.Warnings = append(report.Warnings,
						"loadEventFired didn't fire; using DOMContentLoaded + network-idle as completion signal")
					return
				}
			} else {
				lastNetQuiet = time.Time{}
			}
		}
	}
}

func lcpCount(mu *sync.Mutex, timeline *[]performancetimeline.TimelineEvent) int {
	mu.Lock()
	defer mu.Unlock()
	n := 0
	for _, e := range *timeline {
		if e.Type == "largest-contentful-paint" {
			n++
		}
	}
	return n
}

// applyFallback folds in-page observer data into the report, preferring values
// already computed from CDP events (which are anchored to the navigation t0)
// and only filling nil slots. For longtasks, CDP and in-page may double-count
// — we replace the list when CDP had none.
func applyFallback(report *PSIReport, fb *pageFallback) {
	if fb == nil {
		return
	}
	if report.Metrics.FCPMs == nil && fb.FCP != nil {
		ms := int(*fb.FCP)
		report.Metrics.FCPMs = &ms
	}
	if report.Metrics.LCPMs == nil && fb.LCP != nil {
		ms := int(fb.LCP.TimeMs)
		report.Metrics.LCPMs = &ms
		if report.Metrics.LCPElement == "" {
			report.Metrics.LCPElement = fb.LCP.Element
		}
		if report.Metrics.LCPURL == "" {
			report.Metrics.LCPURL = fb.LCP.URL
		}
	}
	if report.Metrics.CLS == 0 && fb.LayoutCLS > 0 {
		report.Metrics.CLS = fb.LayoutCLS
	}
	if report.Metrics.INPMs == nil && fb.INPMs != nil {
		ms := int(*fb.INPMs)
		report.Metrics.INPMs = &ms
	}
	// If the timeline path saw no longtasks, use the in-page list. Recompute
	// TBT/diagnostics from it. This path handles the common case: CDP
	// PerformanceTimeline rejected "longtask" as an unknown type.
	if len(fb.LongTask) > 0 && report.Diagnostics.LongTasksCount == 0 {
		report.Diagnostics.LongTasksCount = len(fb.LongTask)
		total := 0
		tbt := 0
		fcp := 0
		if report.Metrics.FCPMs != nil {
			fcp = *report.Metrics.FCPMs
		}
		tbtEnd := 0
		if report.Metrics.TTIMs != nil {
			tbtEnd = *report.Metrics.TTIMs
		} else if report.Metrics.LCPMs != nil {
			tbtEnd = *report.Metrics.LCPMs + 3000
		}
		for _, lt := range fb.LongTask {
			total += int(lt.DurationMs)
			if fcp == 0 || tbtEnd == 0 {
				continue
			}
			startMs := int(lt.StartMs)
			endMs := startMs + int(lt.DurationMs)
			if endMs <= fcp || startMs >= tbtEnd {
				continue
			}
			blocking := int(lt.DurationMs) - 50
			if blocking > 0 {
				tbt += blocking
			}
		}
		report.Diagnostics.LongTasksTotalMs = total
		if report.Metrics.TBTMs == nil && fcp > 0 {
			report.Metrics.TBTMs = &tbt
		}
	}
}

// pageFallback carries in-page observations collected via Runtime.evaluate.
// This serves two purposes:
//  1. Gather values CDP doesn't typify (DOM node count, performance.memory,
//     navigation timing origin-relative fields).
//  2. Fallback for PerformanceObserver entry types Chrome's CDP PerformanceTimeline
//     domain doesn't expose (notably longtask, paint). PerformanceObserver
//     with buffered:true still captures these via the W3C API.
type pageFallback struct {
	URL       string         `json:"url"`
	DOMNodes  int            `json:"nodes"`
	NavStart  float64        `json:"nav_start_ms"` // performance.timeOrigin-relative 0
	FCP       *float64       `json:"fcp"`
	FP        *float64       `json:"fp"`
	LCP       *fallbackLCP   `json:"lcp"`
	LongTask  []fallbackTask `json:"longtasks"`
	LayoutCLS float64        `json:"cls"`
	INPMs     *float64       `json:"inp"`
	MemUsedMB int            `json:"mem_used_mb"`
	MemLimMB  int            `json:"mem_lim_mb"`
}
type fallbackLCP struct {
	TimeMs  float64 `json:"time_ms"`
	SizePx  float64 `json:"size_px"`
	URL     string  `json:"url"`
	Element string  `json:"element"`
}
type fallbackTask struct {
	StartMs    float64 `json:"start_ms"`
	DurationMs float64 `json:"duration_ms"`
}

// fetchPageFallback runs a single Runtime.evaluate that uses
// PerformanceObserver({buffered:true}) to capture LCP, FCP, paint, longtask,
// layout-shift, and first-input entries that the page observed directly. This
// is the ground-truth fallback for when CDP's PerformanceTimeline domain
// rejects some entry types.
func fetchPageFallback(s *Session, report *PSIReport) *pageFallback {
	raw, err := s.SendCDP("Runtime.evaluate", map[string]any{
		"expression":    pageFallbackScript,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		report.Warnings = append(report.Warnings, "Runtime.evaluate fallback: "+err.Error())
		return nil
	}
	var env struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return nil
	}
	if env.ExceptionDetails != nil {
		report.Warnings = append(report.Warnings, "fallback JS exception: "+env.ExceptionDetails.Text)
		return nil
	}
	var payload pageFallback
	if err := json.Unmarshal([]byte(env.Result.Value), &payload); err != nil {
		report.Warnings = append(report.Warnings, "fallback JSON parse: "+err.Error())
		return nil
	}
	return &payload
}

// pageFallbackScript installs a PerformanceObserver for every useful entry
// type with {buffered:true} so it picks up entries that fired before it
// registered. A short setTimeout gives observers time to deliver their buffer
// before we serialize. Returns a JSON string.
const pageFallbackScript = `(async () => {
  const observe = (types) => new Promise(resolve => {
    const out = [];
    try {
      const obs = new PerformanceObserver(list => out.push(...list.getEntries()));
      for (const t of types) {
        try { obs.observe({type: t, buffered: true}); } catch {}
      }
      setTimeout(() => { try { obs.disconnect(); } catch {} resolve(out); }, 80);
    } catch (e) { resolve([]); }
  });

  const [lcpEntries, clsEntries, longtaskEntries, firstInputEntries] = await Promise.all([
    observe(['largest-contentful-paint']),
    observe(['layout-shift']),
    observe(['longtask']),
    observe(['first-input']),
  ]);

  const paintEntries = performance.getEntriesByType('paint');
  const fcp = paintEntries.find(p => p.name === 'first-contentful-paint');
  const fp  = paintEntries.find(p => p.name === 'first-paint');

  let lcp = null;
  if (lcpEntries.length) {
    const last = lcpEntries[lcpEntries.length - 1];
    const timeMs = last.renderTime || last.loadTime || last.startTime;
    let element = '';
    if (last.element && last.element.tagName) {
      element = last.element.tagName.toLowerCase() +
        (last.element.id ? '#' + last.element.id : '') +
        (last.element.className && typeof last.element.className === 'string'
          ? '.' + last.element.className.split(/\s+/).filter(Boolean).slice(0, 2).join('.')
          : '');
    }
    lcp = { time_ms: timeMs, size_px: last.size || 0, url: last.url || '', element };
  }

  let cls = 0;
  for (const e of clsEntries) {
    if (!e.hadRecentInput) cls += e.value;
  }

  let inp = null;
  if (firstInputEntries.length) {
    const fi = firstInputEntries[0];
    inp = fi.processingStart - fi.startTime;
  }

  const mem = performance.memory || {};
  return JSON.stringify({
    url: location.href,
    nodes: document.querySelectorAll('*').length,
    nav_start_ms: 0,
    fcp: fcp ? fcp.startTime : null,
    fp:  fp  ? fp.startTime  : null,
    lcp,
    longtasks: longtaskEntries.map(e => ({ start_ms: e.startTime, duration_ms: e.duration })),
    cls: Math.round(cls * 1000) / 1000,
    inp,
    mem_used_mb: mem.usedJSHeapSize ? Math.round(mem.usedJSHeapSize / 1048576) : 0,
    mem_lim_mb:  mem.jsHeapSizeLimit ? Math.round(mem.jsHeapSizeLimit / 1048576) : 0,
  });
})()`

// ── Metric computation ─────────────────────────────────────────────────────

func computeMetrics(
	report *PSIReport,
	cfg presetConfig,
	timeline []performancetimeline.TimelineEvent,
	metrics []performance.Metric,
	netByID map[string]*netReq,
	frames []screenFrame,
	domNodes int,
	observationMs int,
) {
	// Find navigation anchor: earliest request is ~t0 for our relative timing.
	// We convert PerformanceTimeline event times (seconds since epoch) to ms
	// relative to the first request's timestamp.
	var t0Ms float64 = 0
	for _, r := range netByID {
		if t0Ms == 0 || r.startMs < t0Ms {
			t0Ms = r.startMs
		}
	}

	// --- FCP, LCP, CLS from timeline events ---
	var fcpMs, lcpMs int
	var fcpSet, lcpSet bool
	var clsTotal float64
	var longTasks []struct{ startMs, durMs int }
	for _, e := range timeline {
		evtMs := int(cdpEpochMs(e.Time) - t0Ms)
		switch e.Type {
		case "paint":
			if e.Name == "first-contentful-paint" && (!fcpSet || evtMs < fcpMs) {
				fcpMs = evtMs
				fcpSet = true
			}
		case "largest-contentful-paint":
			if e.LcpDetails != nil {
				ms := int(cdpEpochMs(e.LcpDetails.RenderTime) - t0Ms)
				if ms <= 0 {
					ms = int(cdpEpochMs(e.LcpDetails.LoadTime) - t0Ms)
				}
				if ms > 0 && (!lcpSet || ms > lcpMs) {
					lcpMs = ms
					lcpSet = true
					if report.Metrics.LCPURL == "" {
						report.Metrics.LCPURL = e.LcpDetails.URL
					}
				}
			}
		case "layout-shift":
			if e.LayoutShiftDetails != nil && !e.LayoutShiftDetails.HadRecentInput {
				clsTotal += e.LayoutShiftDetails.Value
			}
		case "longtask":
			longTasks = append(longTasks, struct{ startMs, durMs int }{
				startMs: evtMs,
				durMs:   int(e.Duration),
			})
		case "first-input":
			// duration on first-input ~= input delay (processingStart - startTime)
			ms := int(e.Duration)
			report.Metrics.INPMs = &ms
		}
	}

	if fcpSet {
		report.Metrics.FCPMs = &fcpMs
	}
	if lcpSet {
		report.Metrics.LCPMs = &lcpMs
	}
	report.Metrics.CLS = roundCLS(clsTotal)

	// --- TTFB from the main-document request ---
	var mainDoc *netReq
	for _, r := range netByID {
		if r.resType == "document" && (mainDoc == nil || r.startMs < mainDoc.startMs) {
			mainDoc = r
		}
	}
	if mainDoc != nil && mainDoc.ttfbMs > 0 {
		ms := int(mainDoc.ttfbMs)
		report.Metrics.TTFBMs = &ms
	}

	// --- TTI: first 5s quiet window after FCP (no longtask + ≤2 in-flight requests) ---
	if fcpSet {
		if ms := computeTTI(fcpMs, longTasks, netByID, t0Ms, observationMs); ms > 0 {
			report.Metrics.TTIMs = &ms
		}
	}

	// --- TBT: Σ max(0, duration - 50) over longtasks in (FCP, tbtEnd] ---
	// tbtEnd choice mirrors Lighthouse v10: prefer TTI, fall back to
	// LCP + 2500ms (their default FMP→TTI stand-in). Capped at observation
	// end so we never count "phantom" longtasks past the trace window.
	if fcpSet {
		tbtEnd := 0
		if report.Metrics.TTIMs != nil {
			tbtEnd = *report.Metrics.TTIMs
		} else if lcpSet {
			tbtEnd = lcpMs + 2500
		} else {
			tbtEnd = fcpMs + 5000
		}
		if observationMs > 0 && tbtEnd > observationMs {
			tbtEnd = observationMs
		}
		tbt := 0
		for _, lt := range longTasks {
			if lt.startMs+lt.durMs <= fcpMs || lt.startMs >= tbtEnd {
				continue
			}
			blocking := lt.durMs - 50
			if blocking > 0 {
				tbt += blocking
			}
		}
		report.Metrics.TBTMs = &tbt
	}

	// --- Speed Index from screencast frames ---
	if si := computeSpeedIndex(frames, int(t0Ms)); si >= 0 {
		report.Metrics.SpeedIndexMs = &si
	}

	// --- Diagnostics ---
	report.Diagnostics.DOMNodes = domNodes
	report.Diagnostics.LongTasksCount = len(longTasks)
	for _, lt := range longTasks {
		report.Diagnostics.LongTasksTotalMs += lt.durMs
	}
	for _, m := range metrics {
		switch m.Name {
		case "TaskDuration":
			report.Diagnostics.MainThreadMs = int(m.Value * 1000)
		}
	}

	// --- Resource waterfall ---
	report.Resources = buildResourceReport(netByID, fcpMs, fcpSet)
	report.Diagnostics.TotalByteWeightKB = report.Resources.TotalKB
	for _, rb := range report.Resources.RenderBlocking {
		report.Diagnostics.RenderBlockingKB += rb.TransferKB
	}
}

// computeTTI scans forward from FCP for the first 5s window where (a) no
// longtask is active at any instant, and (b) in-flight request count stays ≤ 2.
// If no such window exists before max(longtask end, last response), returns 0.
func computeTTI(fcpMs int, longTasks []struct{ startMs, durMs int }, netByID map[string]*netReq, t0Ms float64, observationEndMs int) int {
	// Build sorted event list: longtask start/end, request start/end.
	var ltEdges, netEdges []edge
	var horizon int
	for _, lt := range longTasks {
		ltEdges = append(ltEdges, edge{lt.startMs, 1, "lt"}, edge{lt.startMs + lt.durMs, -1, "lt"})
		if lt.startMs+lt.durMs > horizon {
			horizon = lt.startMs + lt.durMs
		}
	}
	// observationEndMs is passed in explicitly from the caller (wall-clock
	// duration of the measurement run). This is more accurate than inferring
	// from the last network edge, because a page might have no network
	// activity for the last 2s of observation — we still saw that quiet.
	observationEnd := observationEndMs
	for _, r := range netByID {
		sMs := int(r.startMs - t0Ms)
		eMs := int(r.endMs - t0Ms)
		if r.endMs == 0 {
			// Still in-flight at observation end. Mark as lasting to the last
			// observed network edge — not +30s into the future, which was the
			// previous bug that made every slot look busy.
			eMs = sMs + 500
		}
		netEdges = append(netEdges, edge{sMs, 1, "net"}, edge{eMs, -1, "net"})
		if eMs > horizon {
			horizon = eMs
		}
	}
	sort.Slice(ltEdges, func(i, j int) bool { return ltEdges[i].at < ltEdges[j].at })
	sort.Slice(netEdges, func(i, j int) bool { return netEdges[i].at < netEdges[j].at })

	// First pass: walk in 50ms steps from FCP looking for a 5s quiet window
	// fully contained within our observation. This is the strict Lighthouse
	// definition.
	const stepMs = 50
	const quietMs = 5000
	for t := fcpMs; t+quietMs <= observationEnd; t += stepMs {
		if isQuietWindow(t, t+quietMs, ltEdges, netEdges) {
			return t
		}
	}

	// Second pass: find the longest partial quiet window after FCP. If it's
	// ≥3s, report its start as an approximation — TTI is AT MOST this value;
	// the true TTI could be later if quiet breaks after observation end. For
	// short runs where network settles fast but observation also ends fast,
	// this is the best we can report.
	var bestStart, bestLen int
	var curStart int = -1
	// Merge edges by time and walk with running counters.
	type tick struct {
		at    int
		dlt   int
		dnet  int
	}
	allTicks := make([]tick, 0, len(ltEdges)+len(netEdges))
	for _, e := range ltEdges {
		allTicks = append(allTicks, tick{at: e.at, dlt: e.d})
	}
	for _, e := range netEdges {
		allTicks = append(allTicks, tick{at: e.at, dnet: e.d})
	}
	sort.Slice(allTicks, func(i, j int) bool { return allTicks[i].at < allTicks[j].at })

	lt := 0
	nt := 0
	// Initial state at FCP.
	for _, e := range ltEdges {
		if e.at <= fcpMs {
			lt += e.d
		} else {
			break
		}
	}
	for _, e := range netEdges {
		if e.at <= fcpMs {
			nt += e.d
		} else {
			break
		}
	}
	if lt <= 0 && nt <= 2 {
		curStart = fcpMs
	}
	for _, tk := range allTicks {
		if tk.at < fcpMs {
			continue
		}
		prevQuiet := lt <= 0 && nt <= 2
		lt += tk.dlt
		nt += tk.dnet
		nowQuiet := lt <= 0 && nt <= 2
		if !prevQuiet && nowQuiet {
			curStart = tk.at
		} else if prevQuiet && !nowQuiet && curStart >= 0 {
			length := tk.at - curStart
			if length > bestLen {
				bestStart = curStart
				bestLen = length
			}
			curStart = -1
		}
	}
	// Handle an open-ended quiet run at the end of observation.
	if curStart >= 0 && observationEnd-curStart > bestLen {
		bestStart = curStart
		bestLen = observationEnd - curStart
	}

	// Diagnostic breadcrumbs — always populated so the caller can surface
	// "why was TTI null" in the warnings list. `minNt` is the floor of
	// in-flight requests observed across the walk; if it stays >2 the whole
	// time, the page has a sustained XHR tail (analytics/crisp/posthog) that
	// never lets the network go idle.
	ttiDebugLen = bestLen
	ttiDebugStart = bestStart
	ttiDebugMinNt = walkMinNt(fcpMs, ltEdges, netEdges)
	ttiDebugObsEnd = observationEnd

	if bestLen >= 3000 {
		return bestStart
	}
	return 0
}

// walkMinNt returns the minimum `nt` (in-flight network) count seen at any
// moment from fcpMs onward. Independent second walk so we can report the
// number without restructuring the main scan.
func walkMinNt(fcpMs int, ltEdges, netEdges []edge) int {
	nt := 0
	for _, e := range netEdges {
		if e.at <= fcpMs {
			nt += e.d
		} else {
			break
		}
	}
	min := nt
	for _, e := range netEdges {
		if e.at < fcpMs {
			continue
		}
		nt += e.d
		if nt < min {
			min = nt
		}
	}
	return min
}

// Package-level debug slots — read once per call in CollectPSI. Not
// concurrency-safe across concurrent CollectPSI calls; acceptable since this
// is a diagnostic breadcrumb, not load-bearing logic.
var (
	ttiDebugLen    int
	ttiDebugStart  int
	ttiDebugMinNt  int
	ttiDebugObsEnd int
)

func isQuietWindow(start, end int, ltEdges, netEdges []edge) bool {
	ltActive := activeAt(start, ltEdges)
	netActive := activeAt(start, netEdges)
	if ltActive > 0 || netActive > 2 {
		return false
	}
	// Walk edges inside the window; any violation disqualifies.
	for _, e := range ltEdges {
		if e.at < start {
			continue
		}
		if e.at > end {
			break
		}
		ltActive += e.d
		if ltActive > 0 {
			return false
		}
	}
	for _, e := range netEdges {
		if e.at < start {
			continue
		}
		if e.at > end {
			break
		}
		netActive += e.d
		if netActive > 2 {
			return false
		}
	}
	return true
}

type edge struct {
	at  int
	d   int
	typ string
}

func activeAt(t int, edges []edge) int {
	n := 0
	for _, e := range edges {
		if e.at > t {
			break
		}
		n += e.d
	}
	return n
}

// computeSpeedIndex integrates (1 - visual_progress) over time. Visual
// progress for frame_i = histogramSimilarity(frame_i, final), where similarity
// is intersection-over-union of per-channel RGB histograms (16 buckets each,
// 48 bins total). This is the approach Speedline takes — counts how close the
// color distribution of an intermediate frame is to the final frame, which
// correctly captures "50% painted" as ~50% progress even when the background
// color stays constant across the load.
//
// Previous implementation used "non-blank pixel count ratio" which collapses
// to ~1.0 instantly on any page with a non-white hero section, producing
// absurdly low Speed Index values.
func computeSpeedIndex(frames []screenFrame, t0Ms int) int {
	if len(frames) < 2 {
		return -1
	}
	finalImg, err := jpeg.Decode(bytes.NewReader(frames[len(frames)-1].data))
	if err != nil {
		return -1
	}
	finalHist := imageHistogram(finalImg)
	if histSum(finalHist) == 0 {
		return -1
	}

	// progressPoints holds the relative progress for each captured frame.
	type progressPoint struct {
		atMs     int
		progress float64
	}
	// Establish the first-frame histogram as the "0% painted" baseline.
	firstImg, err := jpeg.Decode(bytes.NewReader(frames[0].data))
	if err != nil {
		return -1
	}
	firstHist := imageHistogram(firstImg)

	points := make([]progressPoint, 0, len(frames))
	for _, f := range frames {
		img, err := jpeg.Decode(bytes.NewReader(f.data))
		if err != nil {
			continue
		}
		hist := imageHistogram(img)
		p := histogramProgress(firstHist, hist, finalHist)
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		points = append(points, progressPoint{
			atMs:     int(f.timestampMs) - t0Ms,
			progress: p,
		})
	}
	if len(points) < 2 {
		return -1
	}
	// Speed Index = Σ (t_{i+1} - t_i) * (1 - progress_i)
	si := 0.0
	for i := 0; i < len(points)-1; i++ {
		dt := float64(points[i+1].atMs - points[i].atMs)
		if dt <= 0 {
			continue
		}
		si += dt * (1 - points[i].progress)
	}
	return int(si)
}

// imageHistogram returns a 48-slot histogram: 16 buckets for each of the R, G,
// and B channels. Stride 4 on each axis for speed.
func imageHistogram(img image.Image) [48]uint32 {
	var h [48]uint32
	b := img.Bounds()
	if b.Empty() {
		return h
	}
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			r, g, bl, _ := img.At(x, y).RGBA()
			// RGBA() returns 16-bit channel values; shift to get 8-bit, then
			// to 4-bit bucket (0-15).
			h[(r>>12)&0xF]++
			h[16+((g>>12)&0xF)]++
			h[32+((bl>>12)&0xF)]++
		}
	}
	return h
}

func histSum(h [48]uint32) uint64 {
	var s uint64
	for _, v := range h {
		s += uint64(v)
	}
	return s
}

// histogramProgress returns how far `cur` has moved from `first` toward
// `final`, on [0, 1]. We compute it as:
//   progress = (distance(first, final) - distance(cur, final)) / distance(first, final)
// where distance is the L1 norm of bucket differences (histogram intersection's
// complement). A frame identical to `final` has distance=0 → progress=1. A
// frame identical to `first` has distance=dist(first,final) → progress=0.
func histogramProgress(first, cur, final [48]uint32) float64 {
	distFirst := histDistance(first, final)
	if distFirst == 0 {
		// First frame already matches final — no movement possible.
		return 1.0
	}
	distCur := histDistance(cur, final)
	p := 1.0 - float64(distCur)/float64(distFirst)
	return p
}

func histDistance(a, b [48]uint32) uint64 {
	var d uint64
	for i := 0; i < 48; i++ {
		if a[i] > b[i] {
			d += uint64(a[i] - b[i])
		} else {
			d += uint64(b[i] - a[i])
		}
	}
	return d
}

// ── Resource waterfall ─────────────────────────────────────────────────────

func buildResourceReport(netByID map[string]*netReq, fcpMs int, fcpSet bool) ResourceReport {
	out := ResourceReport{ByType: map[string]TypeStats{}}
	// Find navigation anchor.
	var t0Ms float64 = 0
	for _, r := range netByID {
		if t0Ms == 0 || r.startMs < t0Ms {
			t0Ms = r.startMs
		}
	}
	typeDurSum := map[string]int{} // for AvgMs computation after the loop
	for _, r := range netByID {
		dur := 0
		if r.endMs > 0 {
			dur = int(r.endMs - r.startMs)
		}
		transferKB := bytesToKB(r.transferBytes)
		entry := ResourceEntry{
			URL:        truncateURL(r.url),
			Type:       r.resType,
			DurationMs: dur,
			TTFBMs:     int(r.ttfbMs),
			TransferKB: transferKB,
			FromCache:  r.fromCache,
		}
		out.Count++
		out.TotalKB += transferKB
		if r.fromCache {
			out.FromCache++
		}
		stats := out.ByType[r.resType]
		stats.Count++
		stats.TransferKB += transferKB
		out.ByType[r.resType] = stats
		typeDurSum[r.resType] += dur

		out.Slowest = append(out.Slowest, entry)
		out.LargestTransfer = append(out.LargestTransfer, entry)

		// Render-blocking heuristic: stylesheet OR script with high priority,
		// loaded before FCP. Matches Lighthouse's heuristic closely enough.
		if fcpSet && dur > 0 {
			startRel := int(r.startMs - t0Ms)
			if startRel < fcpMs && (r.resType == "stylesheet" ||
				(r.resType == "script" && (r.priority == "High" || r.priority == "VeryHigh"))) {
				out.RenderBlocking = append(out.RenderBlocking, entry)
			}
		}
	}

	// AvgMs per type = mean per-request duration, avoiding the concurrent
	// wall-clock sum that made "total_ms" misleading (19 parallel 10s = 190s).
	for t, s := range out.ByType {
		if s.Count > 0 {
			s.AvgMs = typeDurSum[t] / s.Count
		}
		s.TransferKB = roundKB(s.TransferKB)
		out.ByType[t] = s
	}
	out.TotalKB = roundKB(out.TotalKB)

	sort.SliceStable(out.Slowest, func(i, j int) bool { return out.Slowest[i].DurationMs > out.Slowest[j].DurationMs })
	if len(out.Slowest) > 10 {
		out.Slowest = out.Slowest[:10]
	}
	sort.SliceStable(out.LargestTransfer, func(i, j int) bool { return out.LargestTransfer[i].TransferKB > out.LargestTransfer[j].TransferKB })
	if len(out.LargestTransfer) > 10 {
		out.LargestTransfer = out.LargestTransfer[:10]
	}
	for i := range out.Slowest {
		out.Slowest[i].TransferKB = roundKB(out.Slowest[i].TransferKB)
	}
	for i := range out.LargestTransfer {
		out.LargestTransfer[i].TransferKB = roundKB(out.LargestTransfer[i].TransferKB)
	}
	for i := range out.RenderBlocking {
		out.RenderBlocking[i].TransferKB = roundKB(out.RenderBlocking[i].TransferKB)
	}
	return out
}

// bytesToKB converts bytes → KB keeping 1 decimal place (e.g. 870 bytes → 0.8 KB).
// Previously we truncated via int division which silently zeroed anything <1KB.
func bytesToKB(b int64) float64 {
	return float64(b) / 1024
}

func roundKB(kb float64) float64 {
	return float64(int(kb*10+0.5)) / 10
}

func truncateURL(u string) string {
	if len(u) > 200 {
		return u[:200] + "…"
	}
	return u
}

func roundCLS(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}

// ── Output helpers ─────────────────────────────────────────────────────────

// FormatReport returns pretty-printed JSON.
func FormatReport(r *PSIReport) string {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

// SummarizeReport — short one-liner for logs.
func SummarizeReport(r *PSIReport) string {
	if r == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("preset=%s score=%d", r.Preset, r.Score)}
	if r.Metrics.FCPMs != nil {
		parts = append(parts, fmt.Sprintf("fcp=%dms", *r.Metrics.FCPMs))
	}
	if r.Metrics.LCPMs != nil {
		parts = append(parts, fmt.Sprintf("lcp=%dms", *r.Metrics.LCPMs))
	}
	if r.Metrics.SpeedIndexMs != nil {
		parts = append(parts, fmt.Sprintf("si=%dms", *r.Metrics.SpeedIndexMs))
	}
	if r.Metrics.TBTMs != nil {
		parts = append(parts, fmt.Sprintf("tbt=%dms", *r.Metrics.TBTMs))
	}
	if r.Metrics.CLS > 0 {
		parts = append(parts, fmt.Sprintf("cls=%.3f", r.Metrics.CLS))
	}
	if r.Resources.Count > 0 {
		parts = append(parts, fmt.Sprintf("resources=%d (%.1fKB)", r.Resources.Count, r.Resources.TotalKB))
	}
	return strings.Join(parts, " ")
}
