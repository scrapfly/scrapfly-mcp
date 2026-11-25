---
name: scrapfly-sdk-go
description: >-
  Web scraping from Go with the Scrapfly SDK
  (`github.com/scrapfly/go-scrapfly`). Use when the user asks to scrape a URL,
  bypass anti-bot protection, render JavaScript, capture screenshots, or
  extract structured data from a Go program.
---

# Scrapfly Go SDK

Use the `go-scrapfly` module to scrape web pages from Go with proxy rotation,
anti-bot bypass (Unblocker), JavaScript rendering, screenshots, and AI extraction.
It ships a built-in `goquery` selector for parsing the response.

## When to use

- Scraping web pages (HTML, JSON, text, markdown) from a Go codebase
- Bypassing anti-bot protections (Cloudflare, DataDome, PerimeterX, Kasada)
- Rendering JavaScript-heavy pages with a headless browser
- Capturing screenshots or extracting structured data

## Setup

```bash
go get github.com/scrapfly/go-scrapfly
export SCRAPFLY_API_KEY="your-api-key"
```

The package name is `scrapfly`. Get your API key at [scrapfly.io](https://scrapfly.io).

## Basic scrape

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/scrapfly/go-scrapfly"
)

func main() {
    client, err := scrapfly.New(os.Getenv("SCRAPFLY_API_KEY"))
    if err != nil {
        log.Fatalf("failed to create client: %v", err)
    }

    apiResponse, err := client.Scrape(&scrapfly.ScrapeConfig{
        URL: "https://web-scraping.dev/product/1",
    })
    if err != nil {
        log.Fatalf("scrape failed: %v", err)
    }

    fmt.Println(apiResponse.Result.Content)
}
```

## Anti-bot bypass, geo-targeting, JS rendering

`ScrapeConfig` fields are Go-idiomatic PascalCase. `ProxyPool` takes a typed
constant. `Unblocker` is a `*bool` (nil = unset), so set it with the
`scrapfly.BoolPtr` helper.

```go
apiResponse, err := client.Scrape(&scrapfly.ScrapeConfig{
    URL:             "https://web-scraping.dev/product/1",
    Unblocker:       scrapfly.BoolPtr(true),         // bypass anti-bot
    Country:         "us",                           // proxy country (ISO 3166-1 alpha-2)
    ProxyPool:       scrapfly.PublicResidentialPool, // or scrapfly.PublicDataCenterPool
    RenderJS:        true,                           // headless browser rendering
    WaitForSelector: "div.product",                  // requires RenderJS
    RenderingWait:   5000,                           // ms, requires RenderJS
    Format:          "markdown",                     // raw | clean_html | json | markdown | text
})
```

## Built-in HTML selector

```go
selector, err := apiResponse.Selector() // goquery document
if err != nil {
    log.Fatal(err)
}
fmt.Println("Title:", selector.Find("h3").First().Text())
```

## Screenshots and extraction

```go
shot, err := client.Screenshot(&scrapfly.ScreenshotConfig{
    URL: "https://web-scraping.dev/products",
})

data, err := client.Extract(&scrapfly.ExtractionConfig{
    Body:             []byte(apiResponse.Result.Content),
    ContentType:      "text/html",
    ExtractionPrompt: "extract all product names and prices",
})
```

## Important notes

- `RenderJS: true` is required for `RenderingWait`, `WaitForSelector`, and screenshots
- `scrapfly.PublicResidentialPool` is recommended alongside `Unblocker: scrapfly.BoolPtr(true)`
- `Unblocker` (`*bool`) is the current name for the anti-bot bypass; the plain
  `bool` field `ASP` is the deprecated alias and keeps working forever. Only the
  name changed; the SDK sends `asp` on the wire either way. Set one, not both;
  an explicit `ASP: true` wins over `Unblocker`
- Use `Format: "markdown"` for clean, LLM-friendly content
- Read the page body via `apiResponse.Result.Content`; parse with `apiResponse.Selector()`
