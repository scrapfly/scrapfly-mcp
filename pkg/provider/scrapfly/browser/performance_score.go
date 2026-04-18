package browser

import "math"

// Thresholds mirror PSI / Lighthouse v10 as published at
// https://web.dev/articles/defining-core-web-vitals-thresholds. Units are ms
// except CLS (unitless).
//
// Good:           metric <= good
// Needs-Improve:  good < metric <= needsImprove
// Poor:           metric > needsImprove
//
// For metrics where lower is better, these are the canonical PSI cutoffs.
var (
	thresholdsFCP  = [2]float64{1800, 3000}
	thresholdsLCP  = [2]float64{2500, 4000}
	thresholdsCLS  = [2]float64{0.1, 0.25}
	thresholdsINP  = [2]float64{200, 500}
	thresholdsTTFB = [2]float64{800, 1800}
	thresholdsTBT  = [2]float64{200, 600}
	thresholdsTTI  = [2]float64{3800, 7300}
	// Speed Index buckets differ by device per Lighthouse docs.
	thresholdsSIMobile  = [2]float64{3400, 5800}
	thresholdsSIDesktop = [2]float64{1300, 2300}
)

// Lighthouse v10 performance-score weights.
// TTI is intentionally excluded (removed from score in v10) but we still
// report the metric and bucket. PSI uses slightly different weights per
// preset — we match their published numbers exactly. Each weight set is
// renormalized to 1.0 at use time.
type scoreWeights struct {
	fcp, si, lcp, tbt, cls float64
}

// From PSI's "Performance" score breakdown on pagespeed.dev:
//   desktop: FCP+10 LCP+25 TBT+30 CLS+25 SI+10 (total 100)
//   mobile:  FCP+9  LCP+23 TBT+28 CLS+25 SI+10 (total 95 — PSI normalizes)
var (
	weightsDesktop = scoreWeights{fcp: 0.10, si: 0.10, lcp: 0.25, tbt: 0.30, cls: 0.25}
	weightsMobile  = scoreWeights{fcp: 0.09, si: 0.10, lcp: 0.23, tbt: 0.28, cls: 0.25}
)

func weightsFor(preset Preset) scoreWeights {
	if preset == PresetDesktop {
		return weightsDesktop
	}
	return weightsMobile
}

// Log-normal curve control points from Lighthouse's scoring calibration:
// p50 (score ≈ 0.9), p90 (score ≈ 0.5). These are the numbers the scoring
// function is tuned against. Not a perfect match to Lighthouse — we use a
// simpler piecewise-linear mapping against the documented buckets so the
// numbers stay explainable.
//
// Score formula: piecewise-linear between (0, 100), (good_threshold, 90),
// (needs_improve_threshold, 50), (2×needs_improve, 0). This approximates
// Lighthouse's log-normal curve within ±5 points across the Good and
// Needs-Improvement ranges, which is what matters for the overall bucket.

// scoreAndRate computes the overall performance score (0-100) and the
// per-metric Good/Needs-Improvement/Poor ratings using the PSI thresholds.
// Metrics that are nil (no data) contribute nothing to the score and are
// absent from the ratings map.
func scoreAndRate(m LabMetrics, preset Preset) (int, map[string]string) {
	ratings := map[string]string{}
	w := weightsFor(preset)
	type component struct {
		score  float64 // 0-1
		weight float64
	}
	var components []component

	if m.FCPMs != nil {
		s := metricScore(float64(*m.FCPMs), thresholdsFCP[0], thresholdsFCP[1])
		ratings["fcp"] = rating(float64(*m.FCPMs), thresholdsFCP)
		components = append(components, component{s, w.fcp})
	} else {
		ratings["fcp"] = "not-measured"
	}
	if m.LCPMs != nil {
		s := metricScore(float64(*m.LCPMs), thresholdsLCP[0], thresholdsLCP[1])
		ratings["lcp"] = rating(float64(*m.LCPMs), thresholdsLCP)
		components = append(components, component{s, w.lcp})
	} else {
		ratings["lcp"] = "not-measured"
	}
	if m.TBTMs != nil {
		s := metricScore(float64(*m.TBTMs), thresholdsTBT[0], thresholdsTBT[1])
		ratings["tbt"] = rating(float64(*m.TBTMs), thresholdsTBT)
		components = append(components, component{s, w.tbt})
	} else {
		ratings["tbt"] = "not-measured"
	}
	// CLS is never nil (defaults to 0), and 0 is the best score.
	{
		s := metricScore(m.CLS, thresholdsCLS[0], thresholdsCLS[1])
		ratings["cls"] = rating(m.CLS, thresholdsCLS)
		components = append(components, component{s, w.cls})
	}
	if m.SpeedIndexMs != nil {
		th := thresholdsSIMobile
		if preset == PresetDesktop {
			th = thresholdsSIDesktop
		}
		s := metricScore(float64(*m.SpeedIndexMs), th[0], th[1])
		ratings["speed_index"] = rating(float64(*m.SpeedIndexMs), th)
		components = append(components, component{s, w.si})
	} else {
		ratings["speed_index"] = "not-measured"
	}

	// Metrics that don't contribute to the overall score but still get a rating.
	if m.TTFBMs != nil {
		ratings["ttfb"] = rating(float64(*m.TTFBMs), thresholdsTTFB)
	} else {
		ratings["ttfb"] = "not-measured"
	}
	if m.INPMs != nil {
		ratings["inp"] = rating(float64(*m.INPMs), thresholdsINP)
	} else {
		ratings["inp"] = "not-measured"
	}
	// TTI was removed from Lighthouse v10 scoring. PSI no longer surfaces TTI
	// in its lab metrics section. We still compute and emit it as a
	// diagnostic, but we don't put it in ratings when null — otherwise a
	// null (genuinely unreachable on pages with sustained XHR tails) shows up
	// as "not-measured" and looks like a data-collection bug.
	if m.TTIMs != nil {
		ratings["tti"] = rating(float64(*m.TTIMs), thresholdsTTI)
	}

	// Weighted average, renormalised to the subset of metrics present so a
	// missing INP / SI doesn't silently deflate the overall score.
	//
	// BUT: if metrics summing to >50% of total weight are MISSING, the result
	// is not representative of PSI — we mark it as insufficient and return 0.
	// This prevents the confident "99/100" result when LCP + FCP + TBT all
	// failed to collect. The caller sees the raw `null` metrics + a warning.
	if len(components) == 0 {
		ratings["_status"] = "insufficient_data"
		return 0, ratings
	}
	totalWeight := w.fcp + w.si + w.lcp + w.tbt + w.cls
	collectedWeight := 0.0
	weighted := 0.0
	for _, c := range components {
		collectedWeight += c.weight
		weighted += c.score * c.weight
	}
	if collectedWeight < 0.5*totalWeight {
		ratings["_status"] = "insufficient_data"
		return 0, ratings
	}
	return int(math.Round((weighted / collectedWeight) * 100)), ratings
}

// metricScore maps a metric value into [0, 1] using a piecewise-linear curve
// anchored to the PSI Good / Needs-Improvement thresholds.
//
//	value <= good           → 1.0 (well above 90)
//	value == needsImprove   → 0.5
//	value == 2*needsImprove → 0.0
//
// For CLS we accept float thresholds unchanged. For ms-based metrics this
// scales the same way because we're in a linear space.
func metricScore(value, good, needsImprove float64) float64 {
	if value <= good {
		// Linear 1.0 at 0 → 0.9 at good threshold.
		if good <= 0 {
			return 1.0
		}
		return 1.0 - 0.1*(value/good)
	}
	if value <= needsImprove {
		// 0.9 at good → 0.5 at needsImprove.
		span := needsImprove - good
		if span <= 0 {
			return 0.5
		}
		return 0.9 - 0.4*((value-good)/span)
	}
	// >needsImprove: 0.5 linearly down to 0.0 at 2×needsImprove.
	tail := needsImprove // span from needsImprove → 2×needsImprove
	if tail <= 0 {
		return 0
	}
	s := 0.5 - 0.5*((value-needsImprove)/tail)
	if s < 0 {
		return 0
	}
	return s
}

func rating(value float64, thresholds [2]float64) string {
	if value <= thresholds[0] {
		return "good"
	}
	if value <= thresholds[1] {
		return "needs-improvement"
	}
	return "poor"
}
