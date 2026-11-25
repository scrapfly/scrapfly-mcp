---
name: scrapfly-browser
description: >-
  Automate cloud browsers using the Scrapfly Cloud Browser API with Python
  Playwright. Covers connection options, sessions, anti-bot fingerprinting,
  and file downloads including streaming large files (up to
  500 MB) out of the session with downloads_stream=1 and the CDP IO domain.
---

# Scrapfly Cloud Browser

Use the Scrapfly Cloud Browser API with Python Playwright to automate remote cloud browsers with built-in proxy rotation, anti-bot fingerprinting, and geo-targeting.

## When to use

- Automating browser interactions on remote cloud browsers (login flows, form filling, navigation)
- Scraping JavaScript-heavy sites that require full browser automation
- Bypassing anti-bot protections with managed browser fingerprinting
- Running Playwright scripts through geo-targeted proxies
- Maintaining persistent browser sessions across multiple connections
- Downloading files through browser interactions

## Setup

```bash
pip install playwright
```

The API key must be provided via environment variable `SCRAPFLY_API_KEY` or passed directly in the connection URL.

## API Reference

**Connection:** WebSocket CDP (Chrome DevTools Protocol)

```
wss://browser.scrapfly.io?api_key=YOUR_API_KEY
```

### Connection Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `api_key` | str | **required** | Scrapfly API key |
| `proxy_pool` | str | `"datacenter"` | Proxy type: `"datacenter"` or `"residential"` |
| `os` | str | random | OS fingerprint: `"linux"`, `"windows"`, or `"macos"` |
| `country` | str | None | Proxy country (ISO 3166-1 alpha-2, e.g. `"us"`, `"de"`) |
| `session` | str | None | Session ID for browser state persistence |
| `auto_close` | str | `"true"` | Close session on disconnect. Set `"false"` to keep alive |
| `timeout` | int | 900 | Max session duration in seconds. Any positive integer is accepted and values above 1800 are capped at 1800 |
| `debug` | str | `"false"` | Enable session video recording |
| `downloads_stream` | str | None | `"1"` returns large downloads as IO stream handles instead of inline base64 |

### Building the Connection URL

```python
import os
from urllib.parse import urlencode

params = {
    "api_key": os.environ["SCRAPFLY_API_KEY"],
    "proxy_pool": "datacenter",
    "os": "linux",
}
BROWSER_WS = f"wss://browser.scrapfly.io?{urlencode(params)}"
```

## Examples

### Basic page navigation

```python
from playwright.sync_api import sync_playwright
import os

API_KEY = os.environ["SCRAPFLY_API_KEY"]
BROWSER_WS = f"wss://browser.scrapfly.io?api_key={API_KEY}&proxy_pool=datacenter&os=linux"

with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto("https://web-scraping.dev")
        print("Title:", page.title())
        print("Content:", page.content()[:500])
    finally:
        browser.close()
```

### Async Playwright

```python
from playwright.async_api import async_playwright
import asyncio
import os

API_KEY = os.environ["SCRAPFLY_API_KEY"]
BROWSER_WS = f"wss://browser.scrapfly.io?api_key={API_KEY}&proxy_pool=datacenter&os=linux"

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.connect_over_cdp(BROWSER_WS)
        try:
            context = browser.contexts[0]
            page = context.pages[0] if context.pages else await context.new_page()

            await page.goto("https://web-scraping.dev")
            print("Title:", await page.title())
        finally:
            await browser.close()

asyncio.run(main())
```

### Login flow with form filling

```python
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto("https://web-scraping.dev/login")
        page.fill("input[name=username]", "user123")
        page.fill("input[name=password]", "password")
        page.click("button[type=submit]")
        page.wait_for_selector("div#secret-message")

        print("Logged in:", page.url)
    finally:
        browser.close()
```

### Take a screenshot

```python
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto("https://web-scraping.dev")
        page.screenshot(path="screenshot.png", full_page=True)
    finally:
        browser.close()
```

### Scrape data with selectors

```python
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto("https://web-scraping.dev/products")
        page.wait_for_selector("div.product")

        products = page.query_selector_all("div.product")
        for product in products:
            name = product.query_selector("a").inner_text()
            price = product.query_selector(".price").inner_text()
            print(f"{name}: {price}")
    finally:
        browser.close()
```

### Geo-targeted browsing with residential proxy

```python
BROWSER_WS = (
    f"wss://browser.scrapfly.io?"
    f"api_key={API_KEY}"
    f"&proxy_pool=residential"
    f"&country=de"
    f"&os=windows"
)

with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto("https://web-scraping.dev")
        print("Page content from DE proxy:", page.title())
    finally:
        browser.close()
```

### Persistent sessions (multi-step workflows)

Uses the same login credentials and selectors as the [Login flow with form filling](#login-flow-with-form-filling) example.

```python
LOGIN_URL = "https://web-scraping.dev/login"
USERNAME = "user123"
PASSWORD = "password"

# Step 1: Login and keep session alive
BROWSER_WS = (
    f"wss://browser.scrapfly.io?"
    f"api_key={API_KEY}"
    f"&session=my-session"
    f"&auto_close=false"
    f"&proxy_pool=datacenter"
)

with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto(LOGIN_URL)
        page.fill("input[name=username]", USERNAME)
        page.fill("input[name=password]", PASSWORD)
        page.click("button[type=submit]")
        page.wait_for_selector("div#secret-message")
        print("Login complete, session preserved")
    finally:
        browser.close()  # Disconnects but session stays alive

# Step 2: Reconnect to same session (still authenticated with same credentials)
BROWSER_WS = (
    f"wss://browser.scrapfly.io?"
    f"api_key={API_KEY}"
    f"&session=my-session"
    f"&proxy_pool=datacenter"
)

with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto(LOGIN_URL)
        page.wait_for_selector("div#secret-message")
        print("Accessed protected page (still logged in):", page.title())
    finally:
        browser.close()
```

### Infinite scroll scraping

```python
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto("https://web-scraping.dev/testimonials")
        page.wait_for_selector("span.rating")

        all_items = []
        for _ in range(10):  # Scroll 10 times
            items = page.query_selector_all("span.rating")
            all_items = items
            page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
            page.wait_for_timeout(2000)

        print(f"Collected {len(all_items)} reviews")
    finally:
        browser.close()
```

### Execute JavaScript in page

```python
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        page.goto("https://web-scraping.dev")

        # Extract data using JavaScript
        data = page.evaluate("""
            () => {
                return {
                    title: document.title,
                    links: Array.from(document.querySelectorAll('a')).map(a => ({
                        text: a.innerText,
                        href: a.href,
                    })),
                };
            }
        """)
        print(f"Found {len(data['links'])} links")
    finally:
        browser.close()
```

### Multi-page navigation

```python
with sync_playwright() as p:
    browser = p.chromium.connect_over_cdp(BROWSER_WS)
    try:
        context = browser.contexts[0]
        page = context.pages[0] if context.pages else context.new_page()

        urls = [
            "https://web-scraping.dev/products",
            "https://web-scraping.dev/products?page=2",
            "https://web-scraping.dev/products?page=3",
        ]

        results = []
        for url in urls:
            page.goto(url)
            results.append({"url": url, "content": content})

        print(f"Scraped {len(results)} pages")
    finally:
        browser.close()
```

## File Downloads

Use the standard CDP `Runtime` and `IO` methods to retrieve content. For large
files, stream the response in chunks instead of putting the whole body or its
base64 encoding in one message.

### Stream a URL out of the page

When the URL is known and the browser download machinery is not needed, fetch it
in the page and stream the Blob. Nothing large ever crosses the wire, and the
500 MB cap does not apply on this path.

```python
CHUNK = 10 * 1024 * 1024  # raw bytes here; the reply inflates ~1.34x

evaluated = cdp.send("Runtime.evaluate", {
    "expression": f"fetch({url!r}).then(r => r.blob())",
    "awaitPromise": True,
})
resolved = cdp.send("IO.resolveBlob", {"objectId": evaluated["result"]["objectId"]})
handle = "blob:" + resolved["uuid"]
# Drain with IO.read / IO.close, decoding each chunk when base64Encoded is true.
```

`Page.printToPDF` takes `transferMode: "ReturnAsStream"` and returns a handle the
same way.

**Do not use `Network.getResponseBody` for a large body.** Chromium evicts big
response bodies from the inspector cache before you can ask
(`Request content was evicted from inspector cache`), and for a text content type
it returns the body already decoded, which silently corrupts binary payloads.

## Anti-Bot Features

The Cloud Browser automatically manages browser fingerprinting:

- TLS/JA3 fingerprints matching real browsers
- User-Agent, device metrics, timezone, locale (based on `country`)
- WebGL and Canvas fingerprint emulation
- HTTP/2 fingerprints

## Important Notes

- **Always use `connect_over_cdp()`** (not `connect()`) to connect to the Cloud Browser
- **Always use `browser.contexts[0]`** to get the pre-configured context. Do NOT create new contexts - the Cloud Browser provides a context with proper fingerprinting
- **Always close the browser in a `try/finally` block** to stop billing. Forgetting to close means continuous charges
- **Do NOT set fingerprint overrides** (user agent, viewport, timezone, etc.) - they are managed automatically
- Sessions expire after 1 hour of inactivity
- Maximum session duration is 30 minutes (1800 seconds)
- Maximum file download size is 500 MB per file. For large responses, use the
  streaming approach in [File Downloads](#file-downloads) rather than reading the whole body at once
- Start with `proxy_pool=datacenter`, upgrade to `residential` only when needed for anti-bot bypass
- Use `debug=true` for session video recording when troubleshooting
