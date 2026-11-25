---
name: scrapfly-screenshot
description: Capture web page screenshots using the Scrapfly Screenshot API with the Python SDK
---

# Scrapfly Screenshot

Use the Scrapfly Screenshot API to capture high-quality screenshots of web pages with JavaScript rendering and anti-bot bypass.

## When to use

- Capturing full-page or viewport screenshots of websites
- Taking screenshots of specific page elements via CSS selector/XPath
- Rendering pages behind anti-bot protections for visual capture
- Generating screenshots in different formats (JPG, PNG, WebP, GIF)
- Testing pages at different screen resolutions

## Setup

```bash
pip install scrapfly-sdk
```

The API key must be provided via environment variable `SCRAPFLY_API_KEY` or passed directly to the client.

## API Reference

**Endpoint:** `GET https://api.scrapfly.io/screenshot`

### ScrapflyClient

```python
from scrapfly import ScrapflyClient, ScreenshotConfig
import os

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])
```

### ScreenshotConfig Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `url` | str | **required** | Target URL to screenshot |
| `format` | str or `Format` | `"jpg"` | Image format: `"jpg"`, `"png"`, `"webp"`, `"gif"` ([`Format` enum](https://raw.githubusercontent.com/scrapfly/python-scrapfly/master/scrapfly/screenshot_config.py)) |
| `capture` | str | `"viewport"` | Capture scope: `"viewport"`, `"fullpage"`, or a CSS selector/XPath |
| `resolution` | str | None | Screen dimensions, e.g. `"1920x1080"`, `"375x812"` |
| `country` | str | None | Proxy country (ISO 3166-1 alpha-2, e.g. `"us"`, `"de"`) |
| `timeout` | int | None | Total request timeout in milliseconds |
| `rendering_wait` | int | 1000 | Wait time in ms after page load before capture |
| `wait_for_selector` | str | None | Wait until this CSS/XPath selector is visible before capture |
| `options` | list\[str\] or list\[`Options`\] | None | Screenshot behavior flags, e.g. `"load_images"`, `"dark_mode"`, `"block_banners"`, `"print_media_format"` ([`Options` enum](https://raw.githubusercontent.com/scrapfly/python-scrapfly/master/scrapfly/screenshot_config.py)) |
| `auto_scroll` | bool | False | Scroll to bottom to trigger lazy-loading before capture |
| `js` | str | None | Raw JavaScript to execute before capture (SDK will base64‑encode; max 16KB) |
| `cache` | bool | None | Enable response caching for this screenshot request |
| `cache_ttl` | bool or int | None | Cache TTL (time-to-live); only effective when `cache=True` |
| `cache_clear` | bool | None | Clear existing cache entry for this URL when `cache=True` |
| `vision_deficiency` | str or `VisionDeficiency` | None | Simulate vision impairment for accessibility testing, e.g. `"deuteranopia"`, `"protanopia"`, `"achromatopsia"` ([`VisionDeficiency` enum](https://raw.githubusercontent.com/scrapfly/python-scrapfly/master/scrapfly/screenshot_config.py)) |
| `webhook` | str | None | Webhook name to receive async screenshot completion callbacks |
| `raise_on_upstream_error` | bool | True | Raise an exception if the upstream page returns an error status |

## Examples

### Basic viewport screenshot

```python
from scrapfly import ScrapflyClient, ScreenshotConfig
import os

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/products",
))

# Save the screenshot image
with open("screenshot.jpg", "wb") as f:
    f.write(result.image)
```

### Full-page screenshot as PNG

```python
result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/products",
    format="png",
    capture="fullpage",
    auto_scroll=True,
))

with open("fullpage.png", "wb") as f:
    f.write(result.image)
```

### Screenshot of a specific element

```python
result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/product/1",
    rendering_wait=5000,
    format="png",
    capture="div.features" # CSS selector
))

with open("element.png", "wb") as f:
    f.write(result.image)
```

### Custom resolution (mobile)

```python
result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/products",
    resolution="375x812",  # iPhone viewport
))

with open("mobile.jpg", "wb") as f:
    f.write(result.image)
```

### Screenshot with banner blocking and dark mode

```python
result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/products",
    options=["block_banners", "dark_mode"],
    rendering_wait=2000,
))

with open("clean_dark.jpg", "wb") as f:
    f.write(result.image)
```

### Wait for dynamic content before capture

```python
result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/products",
    wait_for_selector="div.products",
    rendering_wait=3000,
))

with open("fully_loaded_screenshot.jpg", "wb") as f:
    f.write(result.image)
```

### Screenshot with geo-targeting

```python
result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/products",
    country="de",
    resolution="1920x1080",
))

with open("german_version.jpg", "wb") as f:
    f.write(result.image)
```

### Execute JavaScript before capture

Run custom JS before the screenshot (e.g. remove the navbar, cookie modal, or sidebar). Replace the selectors with ones that match your target page—inspect the page in DevTools to find them.

```python
# Example: remove navbar (adjust selector for your page: nav, header, .navbar, etc.)

js_code = "var el = document.querySelector('.navbar-collapse'); if (el) el.remove();"

result = client.screenshot(ScreenshotConfig(
    url="https://web-scraping.dev/products",
    js=js_code, # remove the nav bar
    rendering_wait=1000,
))

with open("clean.jpg", "wb") as f:
    f.write(result.image)
```

## Error Handling

```python
from scrapfly.errors import ScrapflyError

try:
    result = client.screenshot(ScreenshotConfig(url="https://web-scraping.dev/products"))
    with open("screenshot.jpg", "wb") as f:
        f.write(result.image)
except ScrapflyError as e:
    print(f"Screenshot failed: {e.message}")
```

## Important Notes

- The `image` attribute on the response contains the raw binary image data
- The browser captures JPEG, so `"jpg"` (the default) is the only format that is not
  re-encoded; `"png"`, `"webp"` and `"gif"` are converted from that capture and are not
  pixel-perfect sources
- `auto_scroll=True` is recommended for `capture="fullpage"` to trigger lazy-loaded images
- `block_banners` removes cookie consent banners and overlay popups
- `rendering_wait` default is 1000ms; increase for JS-heavy pages
- `js` takes the raw script (the SDK base64-encodes it); keep it under 16KB
- Screenshots are stored by Scrapfly and retrievable via URL in the `X-Scrapfly-Screenshot-Url` response header
