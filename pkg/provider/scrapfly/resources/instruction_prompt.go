package resources

// Served as tool output, so it reaches the client as untrusted content. Keep
// it descriptive documentation: imperative text addressed to the model reads
// as prompt injection and gets the tools refused.
//
// Both scraping schemas are closed (additionalProperties: false), so a
// parameter named here that the tool does not declare is a hard call
// rejection — keep the per-tool split below in sync with the input structs.
const InstructionPromptString = `# Scrapfly scraping options — cheat sheet

` + "`url`" + ` is the only required parameter on either scraping tool; everything else
is optional and defaulted by the service.

## Which tool
* ` + "`web_get_page`" + ` — quick fetch. Anti-scraping protection, browser rendering and
  markdown output are always on; it takes only ` + "`url`, `country`, `format`, `format_options`" + `,
  ` + "`proxy_pool`, `rendering_wait`, `capture_page`, `capture_flags`, `extraction_model`" + `.
* ` + "`web_scrape`" + ` — full control. Everything below applies to this tool.

## Defaults worth setting on web_scrape
* ` + "`asp: true`" + ` — anti-scraping-protection solver. Resolves most blocking
  (WAF challenges, bot checks, CAPTCHAs).
* ` + "`render_js: true`" + ` — headless-browser rendering. Needed for pages whose
  content is built client-side.
* ` + "`format: markdown`" + ` — cheapest content shape to read and to reason over.
  ` + "`clean_html`" + ` is for when selectors matter, ` + "`raw`" + ` returns unprocessed bytes
  and is rarely the right choice.

## Timing (web_scrape, except rendering_wait which both accept)
* ` + "`rendering_wait`" + ` (milliseconds) covers pages that fetch their content
  after load. Preferred over ` + "`timeout`" + `.
* ` + "`timeout`" + ` is best left unset — the service sizes it from the request.
* ` + "`wait_for_selector`" + ` is only reliable once the page's real markup is known,
  i.e. after it has been scraped at least once.

## Blocking (both tools)
* ` + "`asp: true`" + ` covers most cases, and is already on for ` + "`web_get_page`" + `.
* Residential exit (` + "`proxy_pool: public_residential_pool`" + `) helps when a target
  rejects datacenter ranges, or when the response is a VPN/consent interstitial.
* ` + "`country`" + ` (ISO 3166-1 alpha-2) is what geo-gated and CMP-gated pages key on.

## URL discovery
Guessed URLs (a product's pricing page, a category listing) are usually wrong.
Scraping a search engine result page returns the real ones.

## Multi-step flows (web_scrape)
` + "`js_scenario`" + ` drives clicks, form fills and waits inside one request:
https://scrapfly.io/docs/scrapfly-js/js-scenario/
Selectors used there should come from a page already scraped with
` + "`clean_html`" + `, not from guesswork.

## Data extraction (both tools)
` + "`extraction_model`" + ` names a pre-trained extractor (product, article, review_list,
job_posting and others). ` + "`extraction_prompt`" + ` (` + "`web_scrape`" + ` only) adds a billed
LLM extraction pass server-side; it earns its cost only when the calling model
cannot process the returned content itself, and markdown output is normally
enough.
`
