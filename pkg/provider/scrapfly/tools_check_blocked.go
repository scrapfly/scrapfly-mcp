package scrapflyprovider

import (
	"context"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CheckIfBlockedInput struct {
	Content         string            `json:"content" jsonschema:"Page content (HTML/text) from a scrape result. Use raw or clean_html format for best detection accuracy."`
	StatusCode      int               `json:"status_code,omitempty" jsonschema:"HTTP status code from the scrape result (e.g. 403, 429, 503). Improves detection accuracy."`
	ResponseHeaders map[string]string `json:"response_headers,omitempty" jsonschema:"Response headers from the scrape result. Enables header-based antibot detection."`
}

type CheckIfBlockedOutput struct {
	IsBlocked      bool   `json:"is_blocked"`
	Antibot        string `json:"antibot,omitempty"`
	BlockType      string `json:"block_type,omitempty"`
	Confidence     string `json:"confidence"`
	Details        string `json:"details"`
	Recommendation string `json:"recommendation"`
}

type antibotSignature struct {
	name           string
	blockType      string
	contentMatches []string
	contentRegexps []*regexp.Regexp
	headerKeys     []string
	headerMatches  map[string]string
	statusCodes    []int
	recommendation string
}

var antibotSignatures = []antibotSignature{
	{
		name:      "cloudflare",
		blockType: "challenge",
		contentMatches: []string{
			"cf-browser-verification",
			"cf_chl_opt",
			"cf-turnstile",
			"challenges.cloudflare.com",
			"Checking if the site connection is secure",
			"Enable JavaScript and cookies to continue",
			"cf-please-wait",
			"Just a moment...",
		},
		contentRegexps: []*regexp.Regexp{
			regexp.MustCompile(`(?i)attention required[!\s]*\|?\s*cloudflare`),
			regexp.MustCompile(`(?i)error 1020.*access denied`),
			regexp.MustCompile(`(?i)ray id:`),
		},
		headerKeys:    []string{"cf-mitigated", "cf-chl-bypass"},
		headerMatches: map[string]string{"server": "cloudflare"},
		statusCodes:   []int{403, 503},
		recommendation: "Enable asp=true with render_js=true. If still blocked, try residential proxy pool.",
	},
	{
		name:      "datadome",
		blockType: "captcha",
		contentMatches: []string{
			"datadome.co",
			"dd.datadome",
			"DataDome",
			"geo.captcha-delivery.com",
			"interstitial.datadome",
		},
		headerKeys:    []string{"x-datadome", "x-datadome-cid"},
		headerMatches: map[string]string{"server": "datadome"},
		statusCodes:   []int{403},
		recommendation: "Enable asp=true with render_js=true and residential proxy pool. DataDome requires browser-level bypass.",
	},
	{
		name:      "perimeterx",
		blockType: "challenge",
		contentMatches: []string{
			"px-captcha",
			"_pxhd",
			"perimeterx",
			"human-challenge",
			"_px2",
			"client.perimeterx",
		},
		headerKeys:  []string{"x-px-cd"},
		statusCodes: []int{403},
		recommendation: "Enable asp=true with render_js=true. PerimeterX often requires residential proxies.",
	},
	{
		name:      "akamai",
		blockType: "challenge",
		contentMatches: []string{
			"_abck",
			"akamai",
			"akam/",
			"ak_bmsc",
			"bm-verify",
		},
		headerKeys:    []string{"x-akamai-session-info"},
		headerMatches: map[string]string{"server": "akamaighost"},
		statusCodes:   []int{403},
		recommendation: "Enable asp=true with render_js=true. Akamai Bot Manager may need residential proxies.",
	},
	{
		name:      "kasada",
		blockType: "challenge",
		contentMatches: []string{
			"cd.kasada.io",
			"/ips.js",
			"KP_UIDz",
		},
		statusCodes:    []int{403, 429},
		recommendation: "Enable asp=true with render_js=true and residential proxy pool.",
	},
	{
		name:      "imperva",
		blockType: "challenge",
		contentMatches: []string{
			"incapsula",
			"imperva",
			"_incap_",
			"visid_incap",
			"incap_ses",
			"reese84",
		},
		headerKeys:    []string{"x-iinfo", "x-cdn"},
		headerMatches: map[string]string{"x-cdn": "incapsula"},
		statusCodes:   []int{403},
		recommendation: "Enable asp=true with render_js=true. Imperva/Incapsula may need session persistence.",
	},
	{
		name:      "aws_waf",
		blockType: "block",
		contentMatches: []string{
			"aws-waf-token",
			"captcha.awswaf",
			"awswaf",
		},
		headerKeys:  []string{"x-amzn-waf-action"},
		statusCodes: []int{403},
		recommendation: "Enable asp=true. AWS WAF blocks are often IP-based — try a different proxy pool or country.",
	},
	{
		name:      "vercel",
		blockType: "rate_limit",
		contentMatches: []string{
			"vercel.com/attack-mode",
			"attack-challenge",
		},
		headerKeys:  []string{"x-vercel-id"},
		statusCodes: []int{429},
		recommendation: "Reduce request rate. Enable asp=true if challenge page is shown.",
	},
	{
		name:      "anubis",
		blockType: "proof_of_work",
		contentMatches: []string{
			"anubis",
			"proof-of-work",
			"pow_challenge",
		},
		statusCodes:    []int{403, 503},
		recommendation: "Enable asp=true with render_js=true to solve proof-of-work challenges.",
	},
	{
		name:      "f5_shape",
		blockType: "challenge",
		contentMatches: []string{
			"shape.com",
			"f5.com",
			"shape-security",
			"_imp_apg_r_",
		},
		statusCodes:    []int{403},
		recommendation: "Enable asp=true with render_js=true and residential proxy pool.",
	},
}

func (p *ScrapflyToolProvider) CheckIfBlocked(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CheckIfBlockedInput,
) (*mcp.CallToolResult, *CheckIfBlockedOutput, error) {
	p.logger.Println("Executing tool: check_if_blocked (local heuristic, zero cost)")

	contentLower := strings.ToLower(input.Content)
	headersLower := make(map[string]string, len(input.ResponseHeaders))
	for k, v := range input.ResponseHeaders {
		headersLower[strings.ToLower(k)] = strings.ToLower(v)
	}

	var bestMatch *antibotSignature
	bestScore := 0

	for i := range antibotSignatures {
		sig := &antibotSignatures[i]
		score := 0

		// Check content matches
		for _, match := range sig.contentMatches {
			if strings.Contains(contentLower, strings.ToLower(match)) {
				score += 2
			}
		}

		// Check content regexps
		for _, re := range sig.contentRegexps {
			if re.MatchString(input.Content) {
				score += 3
			}
		}

		// Check header keys
		for _, key := range sig.headerKeys {
			if _, exists := headersLower[key]; exists {
				score += 3
			}
		}

		// Check header value matches
		for key, expectedVal := range sig.headerMatches {
			if val, exists := headersLower[key]; exists && strings.Contains(val, expectedVal) {
				score += 2
			}
		}

		// Check status codes
		if input.StatusCode > 0 {
			for _, code := range sig.statusCodes {
				if input.StatusCode == code {
					score++
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestMatch = sig
		}
	}

	if bestMatch != nil && bestScore >= 2 {
		confidence := "medium"
		if bestScore >= 5 {
			confidence = "high"
		}

		details := bestMatch.name + " " + bestMatch.blockType + " detected"
		if bestScore >= 5 {
			details += " with strong signal"
		}
		details += "."

		return nil, &CheckIfBlockedOutput{
			IsBlocked:      true,
			Antibot:        bestMatch.name,
			BlockType:      bestMatch.blockType,
			Confidence:     confidence,
			Details:        details,
			Recommendation: bestMatch.recommendation,
		}, nil
	}

	// Check for generic block signals
	genericBlock := false
	genericDetails := ""
	if input.StatusCode == 403 || input.StatusCode == 429 || input.StatusCode == 503 {
		if len(input.Content) < 1000 {
			genericBlock = true
			genericDetails = "Short response body with block-associated status code — possible generic block."
		}
	}

	if genericBlock {
		return nil, &CheckIfBlockedOutput{
			IsBlocked:      true,
			Antibot:        "unknown",
			BlockType:      "block",
			Confidence:     "low",
			Details:        genericDetails,
			Recommendation: "Try enabling asp=true with render_js=true. If still blocked, switch to residential proxies.",
		}, nil
	}

	return nil, &CheckIfBlockedOutput{
		IsBlocked:      false,
		Confidence:     "high",
		Details:        "No antibot blocking detected. The page content appears to be legitimate.",
		Recommendation: "No action needed — the page was fetched successfully.",
	}, nil
}
