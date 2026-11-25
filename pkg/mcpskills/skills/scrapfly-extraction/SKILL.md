---
name: scrapfly-extraction
description: Extract structured data from web content using the Scrapfly Extraction API with the Python SDK
---

# Scrapfly Extraction

Use the Scrapfly Extraction API to extract structured data from HTML, markdown, or text using LLM prompts, pre-trained AI models, or custom extraction templates.

## When to use

- Extracting structured data from web page content
- Using natural language prompts to pull specific information from documents
- Extracting product, article, review, or real estate data with auto AI models
- Parsing HTML/markdown into structured formats with custom templates
- Asking questions about document content and getting AI-powered answers

## Setup

```bash
pip install scrapfly-sdk
```

The API key must be provided via environment variable `SCRAPFLY_API_KEY` or passed directly to the client.

## API Reference

**Endpoint:** `POST https://api.scrapfly.io/extraction`

### ScrapflyClient

```python
from scrapfly import ScrapflyClient, ExtractionConfig
import os

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])
```

### ExtractionConfig Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `body` | str | **required** | Document content to extract from |
| `content_type` | str | **required** | Input format: `"text/html"`, `"text/markdown"`, `"text/plain"`, `"text/xml"` |
| `url` | str | None | Base URL for resolving relative links in HTML |
| `charset` | str | None | Document encoding (auto-detected if omitted) |
| `extraction_prompt` | str | None | Natural language instruction for LLM extraction |
| `extraction_model` | str | None | Pre-trained model: `"product"`, `"article"`, `"review_list"`, `"real_estate_property_listing"` |
| `extraction_template` | str | None | Slug of a template saved on Scrapfly |
| `extraction_ephemeral_template` | dict | None | Inline template definition |
| `timeout` | int | None | Processing timeout in seconds (60-155) |
| `webhook` | str | None | Webhook name for async processing |

**You must provide exactly one of:** `extraction_prompt`, `extraction_model`, `extraction_template`, or `extraction_ephemeral_template`.

### Three Extraction Methods

#### 1. LLM Prompt Extraction
Use natural language to describe what data to extract. The AI interprets the content and returns structured results.

#### 2. Auto AI Models
Pre-trained models for common data types. Returns standardized schemas with quality scores.
- `"article"` - News/blog articles (title, author, date, content, etc.)
- `"event"` - Events (name, date, location, description, etc.)
- `"food_recipe"` - Recipes (ingredients, steps, servings, etc.)
- `"hotel"` - Single hotel/property (name, amenities, rating, etc.)
- `"hotel_listing"` - Hotel search/list results
- `"job_listing"` - Job search/list results
- `"job_posting"` - Single job (title, company, salary, description, etc.)
- `"organization"` - Company/organization (name, contact, description, etc.)
- `"product"` - E-commerce product (name, price, description, images, etc.)
- `"product_listing"` - Product search/category listing
- `"real_estate_property"` - Single property (price, address, features, etc.)
- `"real_estate_property_listing"` - Property search/list results
- `"review_list"` - Lists of reviews (reviewer, rating, text, date, etc.)
- `"search_engine_results"` - SERP data (results, snippets, etc.)
- `"social_media_post"` - Social post (author, content, engagement, etc.)
- `"software"` - Software/app (name, description, pricing, etc.)
- `"stock"` - Stock/market data
- `"vehicle_ad"` - Single vehicle listing
- `"vehicle_ad_listing"` - Vehicle search/list results

#### 3. Custom Templates
Structured extraction rules for consistent parsing across similar pages. Can be defined inline (a JSON object), or stored on Scrapfly and referenced by slug.

##### Saved Templates (reference by slug)

If you have saved a template in the Scrapfly dashboard (Extraction API → Templates), pass its slug as a plain string. The platform resolves it to the currently-published version and applies it:

```python
result = client.extract(ExtractionConfig(
    body=html_content,
    content_type="text/html",
    url="https://web-scraping.dev/product/1",
    extraction_template="product-card",
))
```

Saved templates support:
- A full version history with a separate publish pointer (publish a new version atomically; roll back at any time).
- Optional URL scope (`match_domain` / `match_path`) configured in the dashboard. Calls against non-matching URLs return `ERR::EXTRACTION::TEMPLATE_URL_MISMATCH`.
- Cache-on-read with a 60-second TTL: a newly-published version becomes live within one minute, no client redeployment needed.

The slug shape is lowercase letters / digits / dashes, 3 to 128 characters.

##### Inline Template

Pass a Python dict to extract once without saving. Useful for one-off scripts and for templates that vary per call.

```python
extraction_template = {
    "source": "html",
    "selectors": [
        {
            "name": "title",
            "query": "h3.product-title::text",
            "type": "css",
            "formatters": [
                {
                    "name": "uppercase"
                }
            ],
        },
        {
            "name": "description",
            "query": "p.product-description::text",
            "type": "css"
        },
        {
            "extractor": {
                "name": "price"
            },
            "name": "price",
            "query": ".product-price::text",
            "type": "css"
        },
        {
            "name": "variants",
            "query": "div.variants",
            "type": "css",
            "nested": [
                {
                    "name": "name",
                    "query": "//a[@data-variant-id]/@data-variant-id",
                    "type": "xpath",
                    "multiple": True,
                },
                {
                    "name": "link",
                    "query": "//a[@data-variant-id]/@href",
                    "type": "xpath",
                    "multiple": True,
                },
            ]
        },
        {
            "name": "reviews",
            "query": "div.review>p::text",
            "type": "css",
            "multiple": True,
        }
    ]
}
```
## Examples

### LLM prompt extraction from HTML

```python
from scrapfly import ScrapflyClient, ExtractionConfig
import os

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

html_content = "<html><body><h1>iPhone 15</h1><span class='price'>$999</span><p>Latest Apple smartphone</p></body></html>"

result = client.extract(ExtractionConfig(
    body=html_content,
    content_type="text/html",
    extraction_prompt="Extract the product name, price, and description as JSON",
))

# result
result.extraction_result['data']
# or
print(result.data)

# result content_type
result.extraction_result['content_type']
# or
print(result.content_type)
```

### LLM prompt extraction from markdown

```python
markdown_content = """
# Best Restaurants in NYC

1. **Le Bernardin** - French, $$$, 4.8 stars
2. **Peter Luger** - Steakhouse, $$$, 4.5 stars
3. **Di Fara Pizza** - Italian, $, 4.7 stars
"""

result = client.extract(ExtractionConfig(
    body=markdown_content,
    content_type="text/markdown",
    extraction_prompt="Extract each restaurant as a JSON array with name, cuisine, price_range, and rating fields",
))
print(result.data)
```

### Auto AI model: product extraction

```python
from scrapfly import ScrapflyClient, ExtractionConfig, ScrapeConfig
# First scrape the page, then extract
scrape_result = client.scrape(ScrapeConfig(url="https://web-scraping.dev/product/1"))

result = client.extract(ExtractionConfig(
    body=scrape_result.content,
    content_type="text/html",
    url="https://web-scraping.dev/product/1",
    extraction_model="product",
))

print(result.data)
# Returns: {"name": "...", "price": "...", "currency": "...", "description": "...", ...}
```

### Ask a question about content

```python
result = client.extract(ExtractionConfig(
    body=html_content,
    content_type="text/html",
    extraction_prompt="What is the most expensive item on this page and how much does it cost?",
))
print(result.data)
```

### Scrape + Extract in one flow

```python
from scrapfly import ScrapflyClient, ScrapeConfig, ExtractionConfig
import os

client = ScrapflyClient(key=os.environ["SCRAPFLY_API_KEY"])

# Step 1: Scrape the page
scrape_result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/product/1",
    format="markdown",
))

# Step 2: Extract structured data from the content
extraction_result = client.extract(ExtractionConfig(
    body=scrape_result.content,
    content_type="text/markdown",
    extraction_prompt="Extract all products as a JSON array with fields: name, price, availability",
))

print(extraction_result.data)
```

### Inline extraction with the Scrape API

You can also extract data directly within a scrape request:

```python
result = client.scrape(ScrapeConfig(
    url="https://web-scraping.dev/product/1",
    extraction_prompt="Extract the product name, price, and description as JSON",
))
# Extraction result is included in the scrape response
print(result.scrape_result["extracted_data"])
```

### Custom extraction template (inline)

```python
# First, scrape the web page to retrieve its HTML
api_response = client.scrape(scrape_config=ScrapeConfig(
    url='https://web-scraping.dev/product/1',
    render_js=True
))

html = api_response.content

# extraction template for HTML parsing instructions. It accepts the following:
# selectors: CSS, XPath, JMESPath, Regex, Nested (nesting multiple selector types)
# extractors: extracts commonly accessed data types: price, image, links, emails
# formatters: transforms the extracted data for common methods: lowercase, uppercase, datatime, etc.
# refer to the docs for more details: https://scrapfly.io/docs/extraction-api/rules-and-template#rules
extraction_template = {
    "source": "html",
    "selectors": [
        {
            "name": "title",
            "query": "h3.product-title::text",
            "type": "css",
            "formatters": [
                {
                    "name": "uppercase"
                }
            ],
        },
        {
            "name": "description",
            "query": "p.product-description::text",
            "type": "css"
        },
        {
            "extractor": {
                "name": "price"
            },
            "name": "price",
            "query": ".product-price::text",
            "type": "css"
        },
        {
            "name": "variants",
            "query": "div.variants",
            "type": "css",
            "nested": [
                {
                    "name": "name",
                    "query": "//a[@data-variant-id]/@data-variant-id",
                    "type": "xpath",
                    "multiple": True,
                },
                {
                    "name": "link",
                    "query": "//a[@data-variant-id]/@href",
                    "type": "xpath",
                    "multiple": True,
                },
            ]
        },
        {
            "name": "reviews",
            "query": "div.review>p::text",
            "type": "css",
            "multiple": True,
        }
    ]
}

extraction_api_response = client.extract(
    extraction_config=ExtractionConfig(
        body=html, # pass the HTML content
        content_type='text/html', # content data type
        charset='utf-8', # passed content charset, use `auto` if you aren't sure
        extraction_ephemeral_template=extraction_template # declared template defintion or template name saved on the dashboard
    )
)

# extracted data
print(extraction_api_response.data)

# extracted data content_type
print(extraction_api_response.content_type)
```

## Error Handling

```python
from scrapfly.errors import ScrapflyError

try:
    result = client.extract(ExtractionConfig(
        body="<html><body><h1>iPhone 15</h1><span class='price'>$999</span><p>Latest Apple smartphone</p></body></html",
        content_type="text/html",
        extraction_prompt="Extract the product price",
    ))
    print(result.data)
except ScrapflyError as e:
    print(f"Extraction failed: {e.message}")
```

## Important Notes

- Provide exactly one extraction method per request: `extraction_prompt`, `extraction_model`, `extraction_template`, or `extraction_ephemeral_template`
- For LLM prompts, be specific about the desired output format (e.g., "as JSON")
- The `url` parameter helps resolve relative links in HTML but is not required
- For large documents, consider using `format="markdown"` or `format="text"` in the scrape step first to reduce token usage
- Auto AI models return quality metrics alongside extracted data
- Extraction can also be done inline during a scrape by adding `extraction_prompt` or `extraction_model` to `ScrapeConfig`
