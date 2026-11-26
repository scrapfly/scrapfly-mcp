---
name: scrapfly-crawler
description: Crawl entire websites using the Scrapfly Crawler API with the Python SDK
---

# Scrapfly Crawler

Use the Scrapfly Crawler API via the Python SDK to schedule and manage site-wide crawl jobs with automatic link discovery, depth control, and structured data extraction.

## When to use

- Crawling entire websites or specific sections
- Discovering all pages/URLs on a site
- Collecting content from multiple pages in bulk
- Extracting structured data at scale across a site
- Downloading site archives (WARC/HAR format)
- Building LLM.txt files from crawled documentation

## Setup

```bash
pip install scrapfly-sdk
```

The API key must be provided via environment variable `SCRAPFLY_API_KEY` or passed directly to the client.

## SDK Imports

```python
from scrapfly import ScrapflyClient, CrawlerConfig, Crawl
```

Additional imports for webhooks:

```python
from scrapfly import (
    webhook_from_payload,
    CrawlerLifecycleWebhook,
    CrawlerUrlDiscoveredWebhook,
    CrawlerUrlFailedWebhook,
)
```

## CrawlerConfig Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `url` | str | **required** | Starting URL for the crawl |
| `page_limit` | int | None | Maximum number of pages to crawl |
| `max_depth` | int | None | Maximum link traversal depth from start URL |
| `max_duration` | int | None | Maximum crawl duration in seconds |
| `exclude_paths` | list[str] | None | URL path patterns to skip (supports `*`, max 100). Mutually exclusive with `include_only_paths` |
| `include_only_paths` | list[str] | None | Only crawl URLs matching these path patterns (supports `*`, max 100). Mutually exclusive with `exclude_paths` |
| `ignore_base_path_restriction` | bool | False | Allow crawling any path on the same domain, not just the seed URL's base path |
| `follow_external_links` | bool | False | Follow links to external domains |
| `allowed_external_domains` | list[str] | None | Whitelist of external domains when `follow_external_links=True` (supports `*`, max 250) |
| `headers` | dict | None | Custom HTTP headers as JSON object |
| `delay` | int | 100 ms (API) | Pause after each page for each worker. Use 1–15000 ms for a custom pause. Omit it or pass `0` for the API default. Workers can still send requests in parallel. |
| `user_agent` | str | None | Custom User-Agent string (ignored when `unblocker=True`) |
| `max_concurrency` | int | None | Maximum concurrent crawl requests |
| `rendering_delay` | int | None | Wait time in ms after page load before extraction (0-25000). `0` disables browser rendering |
| `use_sitemaps` | bool | False | Discover URLs from `sitemap.xml` when available |
| `respect_robots_txt` | bool | None (API default `true`) | Respect `robots.txt` rules and `Disallow` directives. The SDK omits the parameter unless explicitly set |
| `ignore_no_follow` | bool | False | Ignore `rel="nofollow"` attributes on links |
| `cache` | bool | False | Enable cache layer for crawled pages |
| `cache_ttl` | int | None | Cache time-to-live in seconds (0-604800) |
| `cache_clear` | bool | False | Force refresh of cached pages |
| `content_formats` | list[str] | None | Content formats to extract: `html`, `clean_html`, `markdown`, `text` |
| `extraction_rules` | dict | None | Custom extraction rules for structured data |
| `unblocker` | bool | False | Enable the Unblocker anti-bot bypass with browser rendering. `asp` is the permanently supported deprecated alias; when both are supplied, `asp` wins |
| `proxy_pool` | str | None | Proxy pool to use (e.g., `public_residential_pool`) |
| `country` | str | None | Proxy country selection (ISO code) |
| `webhook_name` | str | None | Name of webhook configured in dashboard |
| `webhook_events` | list[str] | None | Events to subscribe to (e.g., `crawler_started`, `crawler_url_visited`, `crawler_finished`) |
| `max_api_credit` | int | None | Maximum API credits to spend on this crawl. A crawl that reaches it ends successfully (`state.stop_reason` is `max_api_credit`) and keeps the pages it visited |

### Crawl pace and page retries

For a small server, start with `max_concurrency=1` and `delay=1000` (one second after each page). Lower concurrency or increase the delay if the site slows down. When the crawler respects `robots.txt`, a `Crawl-delay` directive for the seed URL takes precedence over the configured pause. The crawler also reduces concurrency automatically when enough recent page attempts end with website timeouts, temporary connection refusals, HTTP 429, or HTTP 5xx.

- Website timeouts get up to five retries after 2, 4, 8, 16, and 32 seconds.
- Temporary website connection refusals get up to three retries after 5, 15, and 45 seconds. A refusal because the port has no service ends the page.
- A final HTTP 5xx marks the page failed. Some 5xx responses can be retried within the individual scrape before it reports the result to the crawler.

The retry limit counts earlier retries for the same URL across failure types. The crawler may stop retrying early if the crawl ends or if enough pages fail without a success. See [Crawler troubleshooting](https://scrapfly.io/docs/crawler-api/troubleshoot#high-failure-rate) for details.

### Webhook Events

- `"crawler_started"` - Crawler execution began
- `"crawler_url_visited"` - A URL was successfully crawled
- `"crawler_url_skipped"` - URLs were skipped
- `"crawler_url_discovered"` - New URLs were discovered
- `"crawler_url_failed"` - A URL failed to crawl
- `"crawler_stopped"` - Crawler ended without succeeding (no page visited, an archive that could not be saved, an unusable `remote_url_list`, a crawl that could not start), or was paused: a graceful restart of its worker (`state.stop_reason` `worker_interrupted`) or an internal error (`internal_error`) paused it. The payload does not say whether the crawl is finished, its status does (`links.status`, `GET /crawl/{uuid}/status`): `is_finished` is false for a pause, and the crawl resumes by itself and sends `crawler_finished` or `crawler_stopped` again when it ends
- `"crawler_cancelled"` - Crawl was manually cancelled
- `"crawler_finished"` - Crawler ended successfully, a stop on a limit (`page_limit`, `max_duration_reached`, `max_api_credit`) included: `state.stop_reason` says why it stopped

## Examples

### Quick start: crawl and get results

```python
import os
from scrapfly import ScrapflyClient, CrawlerConfig, Crawl

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

# Create, start, and wait for crawl in one chain
# 2. Create and run crawler
crawl = Crawl(
    client,
    CrawlerConfig(url='https://web-scraping.dev/products', page_limit=5)
).crawl().wait()

# 3. Get results
pages = crawl.warc().get_pages()

# 4. Process results
print(f"Crawled {len(pages)} pages:")
for page in pages:
    print(f"  • {page['url']} ({page['status_code']})")

# 5. Access specific URLs
html = crawl.read('https://web-scraping.dev/products')
if html:
    print(f"\nMain page is {len(html):,} bytes")
```

### Low-level workflow: start, poll, download

```python
import os
import time
from scrapfly import ScrapflyClient, CrawlerConfig

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

config = CrawlerConfig(
    url='https://web-scraping.dev',
    page_limit=10,
    max_depth=2,
    content_formats=['html', 'markdown']
)

# 1. Start crawler
start_response = client.start_crawl(config)
print(f"Crawler started: {start_response.uuid}")

# 2. Poll for status
while True:
    status = client.get_crawl_status(start_response.uuid)
    print(f"Status: {status.status} - {status.progress_pct:.1f}%")
    print(f"Crawled: {status.state.urls_visited}/{status.state.urls_extracted} pages")

    if status.is_complete:
        break
    elif status.is_failed:
        print("Crawl failed!")
        break

    time.sleep(5)

# 3. Download and process results
if status.is_complete:
    artifact = client.get_crawl_artifact(start_response.uuid)
    pages = artifact.get_pages()
    print(f"Downloaded {len(pages)} pages")

    for page in pages:
        print(f"  {page['url']}: {page['status_code']} ({len(page['content'])} bytes)")

    # Save to file
    artifact.save('crawl_results.warc.gz')
```

### Crawl with path filtering

```python
config = CrawlerConfig(
    url='https://web-scraping.dev/',
    page_limit=50,
    max_depth=5,
    include_only_paths=['*/product/*'],
    content_formats=['markdown', 'text'],
)
crawl = Crawl(client, config).crawl().wait()
```

### Crawl with anti-bot bypass

```python
config = CrawlerConfig(
    url='https://web-scraping.dev',
    page_limit=50,
    unblocker=True,
    content_formats=['html'],
)
crawl = Crawl(client, config).crawl().wait()
```

### Crawl with extraction rules

```python
config = CrawlerConfig(
    url='https://web-scraping.dev/products',
    page_limit=100,
    max_depth=3,
    include_only_paths=['/products/*'],
    extraction_rules={
        "/products/*": {
            "type": "model",
            "value": "product",
        },
    },
)
crawl = Crawl(client, config).crawl().wait()
```

### Read content for specific URLs

```python
# Read HTML (from WARC - fast)
content = crawl.read('https://web-scraping.dev/product/1')
if content:
    print(f"URL: {content.url}")
    print(f"Status: {content.status_code}")
    print(f"Duration: {content.duration}s")
    print(content.content)

# Read markdown (from contents API)
content = crawl.read('https://web-scraping.dev/product/1', format='markdown')
if content:
    print(content.content)
```

### Iterate URLs matching a pattern

```python
for content in crawl.read_iter(pattern="*/products?page=*", format="markdown"):
    print(f"{content.url}: {len(content.content)} chars")
```

### Batch content retrieval

```python
# Get markdown for multiple URLs in a single API call (max 100 per request)
urls = ['https://web-scraping.dev/product/1', 'https://web-scraping.dev/product/2']
contents = crawl.read_batch(urls, formats=['markdown'])

for url, formats_dict in contents.items():
    markdown = formats_dict.get('markdown', '')
    print(f"{url}: {len(markdown)} chars")
```

### Download WARC/HAR archive

```python
# WARC format (default)
artifact = crawl.warc()
artifact.save('crawl_results.warc.gz')

# HAR format
artifact = crawl.har()
artifact.save('crawl_results.har')

# Iterate through records
for record in artifact.iter_responses():
    print(f"{record.url}: {record.status_code}")
```

### Build LLM.txt from crawled documentation

```python
import os
from scrapfly import ScrapflyClient, CrawlerConfig, Crawl

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

config = CrawlerConfig(
    url='https://web-scraping.dev/',
    include_only_paths=['/docs', '/docs/*'], # only allow doc resources
    page_limit=50,
    max_depth=5,
    follow_external_links=False,
    content_formats=['markdown'],
)

crawl = Crawl(client, config).crawl().wait(poll_interval=5, verbose=True)

# Get URLs from WARC
warc_artifact = crawl.warc()
all_urls = [
    record.url for record in warc_artifact.iter_responses()
    if record.status_code == 200
]

# Retrieve markdown content in batches
all_contents = {}
for i in range(0, len(all_urls), 100):
    batch_urls = all_urls[i:i + 100]
    batch_contents = crawl.read_batch(batch_urls, formats=['markdown'])
    all_contents.update(batch_contents)

# Build llms-full.txt
lines = ["# Documentation\n"]
for url, formats_dict in all_contents.items():
    markdown = formats_dict.get('markdown', '').strip()
    if markdown:
        lines.append(f"---\n\n### {url}\n\n{markdown}\n")

with open("llms-full.txt", "w", encoding="utf-8") as f:
    f.write("\n".join(lines))
```

### Crawl statistics

```python
stats = crawl.stats()
print(f"URLs extracted: {stats['urls_extracted']}")
print(f"URLs visited: {stats['urls_visited']}")
print(f"Progress: {stats['progress_pct']:.1f}%")
```

## Key SDK Classes

| Class | Description |
|-------|-------------|
| `ScrapflyClient` | Main client - handles auth and API calls |
| `CrawlerConfig` | Crawl job configuration |
| `Crawl` | High-level crawl lifecycle manager (start, wait, read) |
| `CrawlerStartResponse` | Response from `client.start_crawl()` |
| `CrawlerStatusResponse` | Response from `client.get_crawl_status()` - has `is_complete`, `is_failed`, `progress_pct` |
| `CrawlerArtifactResponse` | Response from `client.get_crawl_artifact()` - has `get_pages()`, `iter_responses()`, `save()` |
| `CrawlContent` | Content for a single URL - has `url`, `content`, `status_code`, `duration` |

## Important Notes

- Use `page_limit` to control credit spend on large sites
- Use `include_only_paths` and `exclude_paths` to focus crawls on relevant sections
- `exclude_paths` and `include_only_paths` are mutually exclusive
- Crawl jobs are asynchronous - use `Crawl.crawl().wait()` for simple workflows or poll with `client.get_crawl_status()` for more control
- WARC/HAR artifacts are available for offline processing via `crawl.warc()` / `crawl.har()`
- `read_batch()` supports up to 100 URLs per request for efficient bulk content retrieval
- Webhook signatures should be verified for security in production
- `follow_external_links` should be used carefully to avoid crawling the entire web
