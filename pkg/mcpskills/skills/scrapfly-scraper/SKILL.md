---
name: scrapfly-scraper
description: Web scraping using the Scrapfly Scraper API with the Python SDK
---

# Scrapfly Scraper

Use the Scrapfly Scraper API to collect web page data with proxy rotation, anti-bot bypass, JavaScript rendering, and JavaScript scenarios for browser control.

## When to use

- Scraping web pages (HTML, JSON, text, markdown)
- Bypassing anti-bot protections (Cloudflare, DataDome, PerimeterX, Kasada, and more)
- Collecting data through rotating proxies with geo-targeting
- Rendering JavaScript-heavy pages with headless browsers
- Control the browser using JavaScript scenario for common actions (waiting for selectors, clicking, filling elements, etc.)
- Session-reuse for session-presited scraping
- Capturing browser XHR call data

## Setup

```bash
pip install scrapfly-sdk
```

The API key must be provided via environment variable `SCRAPFLY_API_KEY` or passed directly to the client.

## API Reference

**Endpoint:** `https://api.scrapfly.io/scrape`
The HTTP method is forwarded to the upstream URL.
To retrieve data from a web page or an API, use the GET method, which is the default method.
If the API resource like an API requries other methods, like POST. You can use it via the `method` parameter of the `ScrapeConfig`

### ScrapflyClient

```python
from scrapfly import ScrapflyClient, ScrapeConfig, ScrapeApiResponse
import os

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])
```

### ScrapeConfig Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `url` | str | **required** | Target URL to scrape |
| `method` | str | `"GET"` | HTTP method: GET, POST, PUT, PATCH, HEAD, OPTIONS |
| `headers` | dict | None | Custom HTTP headers |
| `cookies` | dict | None | Custom cookies (merged into `headers["cookie"]`) |
| `body` | str | None | Raw request body for POST/PUT/PATCH; used when you pre-encode payload |
| `data` | dict | None | Form/JSON data for POST/PUT/PATCH; encoded according to `Content-Type` when `body` is not provided |
| `timeout` | int | None | Request timeout in ms (default ~150,000ms) |
| `retry` | bool | True | Auto-retry on network failures |
| `country` | str | None | Proxy country: an ISO 3166-1 alpha-2 code (`"us"`), a comma-separated list (`"us,ca"`) or exclusions (`"-us"`: any country of the pool but those) |
| `proxy_pool` | str | `"public_datacenter_pool"` | Proxy pool: `"public_datacenter_pool"` or `"public_residential_pool"` |
| `session` | str | None | Session ID to persist cookies/fingerprint across requests |
| `session_sticky_proxy` | bool | None (API default `true` when `session` is set) | Keep the same proxy IP for a given `session`. The SDK omits the parameter unless explicitly set |
| `unblocker` | bool | False | Enable the Unblocker anti-bot bypass. `asp` is the permanently supported deprecated alias; when both are supplied, `asp` wins |
| `render_js` | bool | False | Enable headless browser JavaScript rendering (+5 credits) |
| `rendering_wait` | int | None | Wait time in ms after page load (requires `render_js=True`) |
| `rendering_stage` | str | `"complete"` | Browser readiness stage: `"complete"` or `"domcontentloaded"` (requires `render_js=True`) |
| `wait_for_selector` | str | None | CSS/XPath selector to wait for (requires `render_js=True`) |
| `js` | str | None | JavaScript code to execute in the browser (auto-encoded) |
| `js_scenario` | list | None | List of browser actions (click, fill, scroll, wait, etc.) |
| `auto_scroll` | bool | None | Automatically scroll page during rendering for lazy-loaded content (requires `render_js=True`) |
| `screenshots` | dict | None | Capture screenshots: `{"fullpage": "png"}` or `{"selector": ".element"}` (requires `render_js=True`) |
| `screenshot_flags` | list[str] | None | Screenshot options: `"load_images"`, `"dark_mode"`, `"block_banners"`, `"high_quality"`, `"print_media_format"` |
| `format` | str | `"raw"` | Output format: `"raw"`, `"clean_html"`, `"json"`, `"markdown"`, `"text"` |
| `format_options` | list[str] | None | Format modifiers (markdown only): `"no_images"`, `"no_links"`, `"only_content"` |
| `extract` | dict | None | Raw extraction spec to apply on the response (encoded and sent as `extract`) |
| `extraction_template` | str | None | Name of a saved server-side extraction template |
| `extraction_ephemeral_template` | dict | None | Inline JSON extraction template used once (`ephemeral:`) |
| `extraction_prompt` | str | None | Natural language instructions for Extraction API |
| `extraction_model` | str | None | LLM model name to use for Extraction API |
| `cache` | bool | False | Enable response caching |
| `cache_ttl` | int | None | Cache time-to-live in seconds |
| `cache_clear` | bool | False | Clear any existing cached response when `cache=True` |
| `dns` | bool | False | Collect DNS records and timings in `result.dns` (slower) |
| `ssl` | bool | False | Collect SSL certificate details in `result.ssl` (slower) |
| `debug` | bool | False | Enable debug recording and extra metadata |
| `raise_on_upstream_error` | bool | True | Raise exceptions for upstream 4xx/5xx HTTP responses |
| `correlation_id` | str | None | Custom ID for request tracking across systems |
| `tags` | list[str] | None | Custom tags for request organization and analytics |
| `lang` | list[str] | None | Accept-Language values, e.g. `["en-US", "en"]` |
| `os` | str | None | Choose a browser operating system such as `"win"`, `"mac"`, or `"linux"` |
| `webhook` | str | None | Named webhook for async scrape completion callbacks |
| `cost_budget` | int | None | Max credits to spend on unblocker retries and extra features |

### ScrapeApiResponse

```python
response = client.scrape(ScrapeConfig(url="https://httpbin.dev"))

response.content                      # Page content (HTML/JSON/text)
response.scrape_result                # Full result dict
response.status_code                  # Scrapfly API HTTP status code
response.scrape_result["status_code"] # Upsteam HTTP status code
response.headers                      # Response headers
response.context                      # Metadata about features used
```

## Examples

### Basic scrape

```python
from scrapfly import ScrapflyClient, ScrapeConfig
import os

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

result = client.scrape(ScrapeConfig(
    url="https://httpbin.dev",
))
print(result.content)
```

### Scrape with anti-bot bypass and geo-targeting

```python
result = client.scrape(ScrapeConfig(
    url="https://httpbin.dev",
    unblocker=True, # enable the unblocker to bypass antibots
    country="us", # match the proxy with the domain country
    proxy_pool="public_residential_pool", # use the residential proxy pool to match real ISP IPs
))
```

### Scrape with JavaScript rendering

```python
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/products",
    render_js=True, # enable JavaScript rendering using a cloud browser
    rendering_wait=5000, # rendering wait for wait for
    wait_for_selector="div.product", # wait a for a selector to be present on the page
))
```

### Scrape as markdown

```python
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/products",
    format="markdown", # other supported formats are: json, text, clean_html
))
print(result.content)
```

### POST request with body

```python
result = client.scrape(ScrapeConfig(
    url="https://httpbin.dev/anything",
    method="POST",
    headers={"Content-Type": "application/json"},
    body='{"query": "search term"}',
))
print(result.content)
```

### JavaScript scenario (browser actions)

```python
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/login",
    render_js=True,
    # browser control
    js_scenario=[
        {
            "fill":{
                "selector":"input[name='username']","clear":True,"value":"user123"
                }
            },{
            "fill":{
                "selector":"input[name='password']","clear":True,"value":"password"
                }
            },{
            "click":{
                "selector":"form > button[type='submit']"
                }
            },{
            "wait_for_navigation":{
                "timeout":5000
                }
            }
    ],
    # request headers
    headers={
        "cookie":"cookiesAccepted=true"
    }
))

# access the element under login
print(result.selector.css("div#secret-message::text").get())
```

# Built-in Parsel selector
```python
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/products",
    render_js=True
))

# access the built-in Parsel selector
selector = result.selector
for product in selector.css("div.product"):
    print("product name:", product.xpath(".//a/text()").get()) # using xpath
    print("product price:", product.css("div.price::text").get()) # using css
```

### Concurrent scraping

```python
import asyncio
from scrapfly import ScrapflyClient, ScrapeConfig

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

urls = [
    "https://web-scraping.dev/products",
    "https://web-scraping.dev/products?page=2",
    "https://web-scraping.dev/products?page=3",
]

configs = [ScrapeConfig(url=url) for url in urls]

async def concurrent_scraping():
    results = []
    async for result in client.concurrent_scrape(configs):
        results.append(result)

    for result in results:
        print(result.content[:100])
        print("===========")

asyncio.run(concurrent_scraping())
```

### Using sessions for stateful scraping

```python
# First request: login
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/login",
    render_js=True,
    # browser control
    js_scenario=[
        {
            "fill":{
                "selector":"input[name='username']","clear":True,"value":"user123"
                }
            },{
            "fill":{
                "selector":"input[name='password']","clear":True,"value":"password"
                }
            },{
            "click":{
                "selector":"form > button[type='submit']"
                }
            },{
            "wait_for_navigation":{
                "timeout":5000
                }
            }
    ],
    # request headers
    headers={
        "cookie":"cookiesAccepted=true"
    },
    session="logged-in-session"
))

# Second request: access protected page with same session
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/login",
    session="logged-in-session"
))

# access the built-in selector
print(result.selector.css("div#secret-message::text").get())
```

### Caching responses

```python
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev",
    cache=True,
    cache_ttl=3600,  # Cache for 1 hour
))
```

## Error Handling

```python
from scrapfly import ScrapflyClient, ScrapeConfig
from scrapfly.errors import (
    ScrapflyError,
    UpstreamHttpClientError,
    UpstreamHttpServerError,
    ScrapflyProxyError,
    ScrapflyThrottleError,
)

try:
    result = client.scrape(ScrapeConfig(url="https://httpbin.dev", unblocker=True))
except ScrapflyThrottleError as e:
    print(f"Rate limited, retry after {e.retry_delay}s")
except UpstreamHttpClientError as e:
    print(f"Target returned 4xx: {e.message}")
except UpstreamHttpServerError as e:
    print(f"Target returned 5xx: {e.message}")
except ScrapflyProxyError as e:
    print(f"Proxy error: {e.message}")
except ScrapflyError as e:
    print(f"Scrapfly error: {e.message}")
```

## Important Notes
- `render_js=True` is required for `rendering_wait`, `wait_for_selector`, `js`, `js_scenario`, and `screenshots`
- `proxy_pool="public_residential_pool"` is recommended when using `unblocker=True`
- `unblocker` is the current name for the anti-bot bypass; `asp` is the deprecated alias and keeps working forever. Only the name changed; the SDK sends `asp` on the wire either way. Do not pass both
- Use `format="markdown"` for clean content accessible for LLMs
- Use `session` parameter to maintain state across multiple requests
- The `concurrent_scrape` method handles rate limiting automatically
