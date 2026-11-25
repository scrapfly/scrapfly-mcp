---
name: scrapfly-sdk-rust
description: >-
  Web scraping from Rust with the Scrapfly SDK (`scrapfly-sdk` crate). Use when
  the user asks to scrape a URL, bypass anti-bot protection, render JavaScript,
  capture screenshots, or extract structured data from an async Rust program.
---

# Scrapfly Rust SDK

Use the `scrapfly-sdk` crate to scrape web pages from Rust with proxy rotation,
anti-bot bypass (Unblocker), JavaScript rendering, screenshots, and AI extraction.
The client is async (Tokio) and uses typed builders for every config.

## When to use

- Scraping web pages (HTML, JSON, text, markdown) from a Rust codebase
- Bypassing anti-bot protections (Cloudflare, DataDome, PerimeterX, Kasada)
- Rendering JavaScript-heavy pages with a headless browser
- Capturing screenshots or extracting structured data

## Setup

```bash
cargo add scrapfly-sdk
cargo add tokio --features full
export SCRAPFLY_API_KEY="your-api-key"
```

Get your API key at [scrapfly.io](https://scrapfly.io).

## Basic scrape

```rust
use scrapfly_sdk::{Client, ScrapeConfig};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::builder()
        .api_key(std::env::var("SCRAPFLY_API_KEY")?)
        .build()?;

    let config = ScrapeConfig::builder("https://web-scraping.dev/products").build()?;
    let result = client.scrape(&config).await?;

    println!(
        "status={} content={}",
        result.result.status_code,
        result.result.content,
    );
    Ok(())
}
```

## Anti-bot bypass, geo-targeting, JS rendering

Chain typed builder methods. `proxy_pool` and `format` take enums.

```rust
use scrapfly_sdk::{Client, ScrapeConfig, Format, ProxyPool};

let config = ScrapeConfig::builder("https://web-scraping.dev/products")
    .unblocker(true)                             // bypass anti-bot
    .country("us")                               // proxy country (ISO 3166-1 alpha-2)
    .proxy_pool(ProxyPool::PublicResidentialPool) // or PublicDatacenterPool
    .render_js(true)                             // headless browser rendering
    .wait_for_selector("div.product")            // requires render_js
    .rendering_wait(5000)                        // ms, requires render_js
    .format(Format::Markdown)                    // clean, LLM-friendly content
    .build()?;

let result = client.scrape(&config).await?;
```

## Screenshots and extraction

```rust
use scrapfly_sdk::{Client, ScreenshotConfig, ExtractionConfig};

let shot = client
    .screenshot(&ScreenshotConfig::builder("https://web-scraping.dev/products").build()?)
    .await?;
// shot.image is the raw bytes; shot.save("products", None)? writes it to disk

let html = result.result.content.into_bytes();
let extraction = ExtractionConfig::builder(html, "text/html")
    .extraction_prompt("extract all product names and prices")
    .build()?;
let data = client.extract(&extraction).await?;
println!("{}", serde_json::to_string_pretty(&data.data)?);
```

## Concurrent scraping

`concurrent_scrape` returns a `Stream` (powered by `buffer_unordered`):

```rust
use futures_util::StreamExt;

let configs = vec![
    ScrapeConfig::builder("https://web-scraping.dev/products?page=1").build()?,
    ScrapeConfig::builder("https://web-scraping.dev/products?page=2").build()?,
];

let mut stream = client.concurrent_scrape(configs, 5); // concurrency = 5
while let Some(result) = stream.next().await {
    let result = result?;
    println!("{}", result.result.content.len());
}
```

## Important notes

- `render_js(true)` is required for `rendering_wait`, `wait_for_selector`, and screenshots
- `ProxyPool::PublicResidentialPool` is recommended alongside `unblocker(true)`
- `unblocker(true)` is the current builder input for the anti-bot bypass;
  `asp(true)` is the deprecated alias and keeps working forever. Only the name
  changed; the SDK sends `asp` on the wire either way. Call one, not both; an
  explicitly supplied `asp` wins. On a built `ScrapeConfig`, read it with
  `unblocker_enabled()` and write it with `set_unblocker(bool)`
- Use `Format::Markdown` for clean, LLM-friendly content
- Read the page body via `result.result.content`; the upstream HTTP status is `result.result.status_code`
- The crate bundles no HTML parser — bring your own (`scraper`, `kuchiki`)
