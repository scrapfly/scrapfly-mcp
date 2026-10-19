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
* ` + "`web_get_page`" + ` — quick fetch. Browser rendering and markdown output
  are always on; it takes only ` + "`url`, `country`, `format`, `format_options`" + `,
  ` + "`proxy_pool`, `rendering_wait`, `capture_page`, `capture_flags`, `extraction_model`" + `.
* ` + "`web_scrape`" + ` — full control. Everything below applies to this tool.

## Proxy pool (both tools)
* ` + "`proxy_pool: basic`" + ` runs datacenter proxies with the Unblocker off.
* ` + "`proxy_pool: unblocker`" + ` runs residential proxies with the Unblocker,
  the anti-bot bypass for WAF challenges, bot checks and CAPTCHAs.
* ` + "`proxy_pool: unblocker_ultra`" + ` runs residential proxies with the
  Unblocker and is the only pool accepted by a target or an anti-bot protection
  with a price of its own (e.g. Cloudflare, Kasada): those refuse ` + "`basic`" + ` and
  ` + "`unblocker`" + ` with ` + "`ERR::PROXY::UNBLOCKER_ULTRA_REQUIRED`" + ` (422, not billed).
* ` + "`proxy_pool: auto`" + ` lets the API pick the pool before the request, from
  the target's profile: the Unblocker on datacenter proxies, or the pool the
  target needs. Accepted on every target, the ones that require
  ` + "`unblocker_ultra`" + ` included, where their price is added; priced as the
  pool it runs on.
* The right pool is the one a site needs: ` + "`basic`" + ` where datacenter proxies
  get through, ` + "`unblocker`" + ` or ` + "`unblocker_ultra`" + ` where it needs residential
  proxies and the Unblocker, ` + "`auto`" + ` to let the API pick. A call refused with
  ` + "`ERR::PROXY::UNBLOCKER_ULTRA_REQUIRED`" + ` goes through resent with
  ` + "`proxy_pool: unblocker_ultra`" + ` or ` + "`proxy_pool: auto`" + `.
* ` + "`proxy_pool: public_tor_pool`" + ` runs on the Tor network (.onion).
* Without a ` + "`proxy_pool`" + `, both tools send the API no pool and, by default,
  ` + "`asp=true`" + `. The ` + "`unblocker`" + ` and ` + "`asp`" + ` parameters of ` + "`web_scrape`" + ` are
  deprecated in favor of ` + "`proxy_pool`" + ` (` + "`unblocker_ultra`" + ` for the
  Unblocker). The migration guide covers requests written before the proxy
  pools, the pool names ` + "`public_datacenter_pool`" + ` and
  ` + "`public_residential_pool`" + ` included:
  https://scrapfly.io/new-pricing#legacy-spellings

## Defaults worth setting on web_scrape
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
* A blocked response (403/429, a challenge or CAPTCHA page), a target that
  rejects datacenter ranges, or a VPN/consent interstitial is what the
  Unblocker is for: ` + "`proxy_pool: unblocker`" + `,
  ` + "`proxy_pool: unblocker_ultra`" + ` where the API answers
  ` + "`ERR::PROXY::UNBLOCKER_ULTRA_REQUIRED`" + `, or ` + "`proxy_pool: auto`" + ` to let the
  API pick the pool.
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
