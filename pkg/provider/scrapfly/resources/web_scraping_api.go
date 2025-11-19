package resources

const WebScrapingApi = `
openapi: 3.0.0
info:
  title: "ScrapFly Scraping API"
  description: "Comprehensive specification for the ScrapFly Scrape API, which allows for advanced web scraping of target URLs using a configurable proxy network, headless browsers, and anti-scraping protection. This specification includes detailed parameter descriptions, usage examples, and response schemas."
  version: "1.0.1"
  contact:
    name: "ScrapFly Support"
    url: "https://scrapfly.io/contact"

servers:
  - url: "https://api.scrapfly.io"
    description: "Production Server"

tags:
  - name: "Core"
    description: "Essential and common parameters for scraping."
  - name: "Data Extraction"
    description: "Parameters for extracting structured data using templates or AI."
  - name: "Anti Scraping Protection"
    description: "Parameters to bypass advanced bot detections."
  - name: "Headless Browser / Javascript Rendering"
    description: "Control headless browser for JavaScript-heavy websites."
  - name: "Cache"
    description: "Parameters for caching scrape results."
  - name: "Session"
    description: "Parameters for managing persistent sessions."

paths:
  /scrape:
    get:
      summary: "Scrape a target URL with advanced options"
      description: "Performs a scrape request on a given URL with fine-grained control over proxies, JavaScript rendering, sessions, caching, and anti-scraping protection."
      operationId: "scrapeUrl"
      parameters:
        # Core Parameters
        - $ref: '#/components/parameters/Key'
        - $ref: '#/components/parameters/Url'
        - $ref: '#/components/parameters/ProxyPool'
        - $ref: '#/components/parameters/Country'
        - $ref: '#/components/parameters/Headers'
        - $ref: '#/components/parameters/Lang'
        - $ref: '#/components/parameters/Os'
        - $ref: '#/components/parameters/Timeout'
        - $ref: '#/components/parameters/Format'
        - $ref: '#/components/parameters/Retry'
        - $ref: '#/components/parameters/ProxifiedResponse'
        - $ref: '#/components/parameters/Debug'
        - $ref: '#/components/parameters/CorrelationId'
        - $ref: '#/components/parameters/Tags'
        - $ref: '#/components/parameters/Dns'
        - $ref: '#/components/parameters/Ssl'
        - $ref: '#/components/parameters/WebhookName'
        # Data Extraction
        - $ref: '#/components/parameters/ExtractionTemplate'
        - $ref: '#/components/parameters/ExtractionPrompt'
        - $ref: '#/components/parameters/ExtractionModel'
        # Anti Scraping Protection
        - $ref: '#/components/parameters/Asp'
        - $ref: '#/components/parameters/CostBudget'
        # Headless Browser
        - $ref: '#/components/parameters/RenderJs'
        - $ref: '#/components/parameters/RenderingWait'
        - $ref: '#/components/parameters/WaitForSelector'
        - $ref: '#/components/parameters/Js'
        - $ref: '#/components/parameters/Screenshots'
        - $ref: '#/components/parameters/ScreenshotFlags'
        - $ref: '#/components/parameters/JsScenario'
        - $ref: '#/components/parameters/Geolocation'
        - $ref: '#/components/parameters/AutoScroll'
        - $ref: '#/components/parameters/RenderingStage'
        # Cache
        - $ref: '#/components/parameters/Cache'
        - $ref: '#/components/parameters/CacheTtl'
        - $ref: '#/components/parameters/CacheClear'
        # Session
        - $ref: '#/components/parameters/Session'
        - $ref: '#/components/parameters/SessionStickyProxy'

      responses:
        "200":
          description: "Successful scrape operation. The response structure contains the result of the scrape, context, and configuration. Note that the ` + "`" + `result` + "`" + ` object's structure may change based on parameters like ` + "`" + `ssl` + "`" + ` or ` + "`" + `dns` + "`" + `."
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ScrapeResult'
        "400":
          description: "Bad Request. A parameter is missing or malformed (e.g., ` + "`" + `proxy_pool` + "`" + ` does not exist)."
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        "403":
          description: "Forbidden. The API key is invalid or missing."
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        "422":
          description: "Unprocessable Entity. The request was well-formed but could not be processed due to a semantic error (e.g., target URL is invalid, selector not found, ASP failed)."
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        "429":
          description: "Too Many Requests. The account or project quota has been reached, or a session is currently locked by another request."
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        "500":
          description: "Internal Server Error. An unexpected error occurred on ScrapFly's side."
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        "504":
          description: "Gateway Timeout. A generic timeout occurred during an internal operation."
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

components:
  parameters:
    # Core Parameters
    Key:
      name: "key"
      in: "query"
      description: "Your ScrapFly API key for authentication."
      required: true
      schema: { type: "string", example: "YOUR_API_KEY" }
      tags: ["Core"]
    Url:
      name: "url"
      in: "query"
      description: "The target URL to scrape. Must be URL-encoded."
      required: true
      schema: { type: "string", format: "uri", example: "https://web-scraping.dev/product/1" }
      tags: ["Core"]
    ProxyPool:
      name: "proxy_pool"
      in: "query"
      description: "The proxy pool to use. See your proxy dashboard for available pools."
      schema: { type: "string", default: "public_datacenter_pool", enum: ["public_datacenter_pool", "public_residential_pool"] }
      tags: ["Core"]
    Country:
      name: "country"
      in: "query"
      description: "Proxy country location in ISO 3166 alpha-2 format. Supports multiple comma-separated values, weighted randomization (e.g., ` + "`" + `us:5,ca:1` + "`" + `), and exclusion (e.g., ` + "`" + `-gb` + "`" + `)."
      schema: { type: "string", default: "random", example: "us,ca,-gb" }
      tags: ["Core"]
    Headers:
      name: "headers"
      in: "query"
      description: "Custom headers sent to the target URL. Use the format ` + "`" + `headers[Header-Name]=value` + "`" + `. Values must be URL-encoded."
      style: form
      explode: true
      schema:
        type: "object"
        additionalProperties: { type: "string" }
      example: { "User-Agent": "MyScraper/1.0", "Referer": "https://google.com" }
      tags: ["Core"]
    Lang:
      name: "lang"
      in: "query"
      description: "Sets the ` + "`" + `Accept-Language` + "`" + ` header to request content in a specific language. Can be a comma-separated list. Overrides the default language inferred from proxy location."
      schema: { type: "string", example: "fr-FR,fr;q=0.9,en-US;q=0.8" }
      tags: ["Core"]
    Os:
      name: "os"
      in: "query"
      description: "Sets the operating system for the User-Agent header and browser fingerprint. Cannot be used with a custom ` + "`" + `User-Agent` + "`" + ` header."
      schema: { type: "string", enum: ["win", "win10", "win11", "mac", "linux", "chromeos"] }
      tags: ["Core"]
    Timeout:
      name: "timeout"
      in: "query"
      description: "Maximum time in milliseconds for the entire scrape operation."
      schema: { type: "integer", default: 150000 }
      tags: ["Core"]
    Format:
      name: "format"
      in: "query"
      description: "The desired output format for the content. Supports ` + "`" + `raw` + "`" + `, ` + "`" + `clean_html` + "`" + `, ` + "`" + `json` + "`" + `, ` + "`" + `markdown` + "`" + `, and ` + "`" + `text` + "`" + `. Options can be appended (e.g., ` + "`" + `markdown:no_links` + "`" + `)."
      schema: { type: "string", default: "raw", example: "markdown:no_links,no_images" }
      tags: ["Core"]
    Retry:
      name: "retry"
      in: "query"
      description: "Enable/disable automatic retries on network failures or server errors (status code >= 500)."
      schema: { type: "boolean", default: true }
      tags: ["Core"]
    ProxifiedResponse:
      name: "proxified_response"
      in: "query"
      description: "If true, the API response body will be the raw content from the target URL, and headers/status code will be proxied."
      schema: { type: "boolean", default: false }
      tags: ["Core"]
    Debug:
      name: "debug"
      in: "query"
      description: "If true, stores the API result and provides a shareable link for support. Takes a screenshot if ` + "`" + `render_js` + "`" + ` is enabled."
      schema: { type: "boolean", default: false }
      tags: ["Core"]
    CorrelationId:
      name: "correlation_id"
      in: "query"
      description: "A custom identifier to correlate scrapes, filterable in the monitoring dashboard."
      schema: { type: "string" }
      tags: ["Core"]
    Tags:
      name: "tags"
      in: "query"
      description: "Add tags to scrapes for grouping and filtering in the monitoring dashboard. Use the format ` + "`" + `tags[]=tag_name` + "`" + `."
      style: form
      explode: true
      schema:
        type: "array"
        items: { type: "string" }
      example: ["product_page", "pricing"]
      tags: ["Core"]
    Dns:
      name: "dns"
      in: "query"
      description: "If true, retrieves the target's DNS information instead of scraping content. The response ` + "`" + `result` + "`" + ` object will contain a ` + "`" + `dns` + "`" + ` field with records (A, NS, MX, etc.)."
      schema: { type: "boolean", default: false }
      tags: ["Core"]
    Ssl:
      name: "ssl"
      in: "query"
      description: "If true, retrieves the target's SSL certificate information. The response ` + "`" + `result` + "`" + ` object will contain an ` + "`" + `ssl` + "`" + ` field with certificate details."
      schema: { type: "boolean", default: false }
      tags: ["Core"]
    WebhookName:
      name: "webhook_name"
      in: "query"
      description: "The name of a pre-configured webhook to send the scrape result to asynchronously."
      schema: { type: "string" }
      tags: ["Core"]
    # Data Extraction
    ExtractionTemplate: { name: "extraction_template", in: "query", description: "An extraction template (ephemeral or stored) to get structured data from the page.", schema: { type: "string" }, tags: ["Data Extraction"] }
    ExtractionPrompt: { name: "extraction_prompt", in: "query", description: "An LLM prompt to extract data or ask a question about the scraped content.", schema: { type: "string" }, tags: ["Data Extraction"] }
    ExtractionModel: { name: "extraction_model", in: "query", description: "The name of a pre-trained AI model to auto-parse the document for structured data.", schema: { type: "string", example: "product" }, tags: ["Data Extraction"] }
    # Anti Scraping Protection
    Asp: { name: "asp", in: "query", description: "Enables the Anti Scraping Protection (ASP) layer to bypass bot detection systems like Cloudflare.", schema: { type: "boolean", default: false }, tags: ["Anti Scraping Protection"] }
    CostBudget: { name: "cost_budget", in: "query", description: "(Requires ` + "`" + `asp=true` + "`" + `) Sets a maximum cost budget (in API credits) for the ASP to use, preventing unexpected cost overruns.", schema: { type: "integer", example: 25 }, tags: ["Anti Scraping Protection"] }
    # Headless Browser
    RenderJs: { name: "render_js", in: "query", description: "Enables a headless browser to render JavaScript on the page. Only available for GET requests.", schema: { type: "boolean", default: false }, tags: ["Headless Browser / Javascript Rendering"] }
    RenderingWait: { name: "rendering_wait", in: "query", description: "(Requires ` + "`" + `render_js=true` + "`" + `) Time in milliseconds to wait after the page load event. Max is 25000.", schema: { type: "integer", default: 1000 }, tags: ["Headless Browser / Javascript Rendering"] }
    WaitForSelector: { name: "wait_for_selector", in: "query", description: "(Requires ` + "`" + `render_js=true` + "`" + `) Waits up to 15s until the specified CSS selector, XPath, or XHR pattern is present on the page. For XHR, use the prefix ` + "`" + `xhr:` + "`" + ` (e.g. ` + "`" + `xhr:/api/data*` + "`" + `).", schema: { type: "string", example: "#product-price" }, tags: ["Headless Browser / Javascript Rendering"] }
    Js: { name: "js", in: "query", description: "(Requires ` + "`" + `render_js=true` + "`" + `) A URL-safe Base64 encoded JavaScript snippet to execute on the page. The return value will be available in the response.", schema: { type: "string" }, tags: ["Headless Browser / Javascript Rendering"] }
    Screenshots:
      name: "screenshots"
      in: "query"
      description: "(Requires ` + "`" + `render_js=true` + "`" + `) Takes screenshots of the page. Use ` + "`" + `screenshots[name]=selector` + "`" + ` or ` + "`" + `screenshots[name]=fullpage` + "`" + `. Max 10 per request."
      style: form
      explode: true
      schema:
        type: "object"
        additionalProperties: { type: "string" }
      examples:
        captureElements:
          summary: "Capture full page and a specific element"
          value: { "viewport": "fullpage", "product_image": "#img-main" }
      tags: ["Headless Browser / Javascript Rendering"]
    ScreenshotFlags: { name: "screenshot_flags", in: "query", description: "(Requires ` + "`" + `screenshots` + "`" + `) Comma-separated flags to customize screenshot behavior.", schema: { type: "string", example: "load_images,block_banners", enum: ["load_images", "dark_mode", "block_banners", "high_quality", "print_media_format"] }, tags: ["Headless Browser / Javascript Rendering"] }
    JsScenario:
      name: "js_scenario"
      in: "query"
      description: |
        (Requires ` + "`" + `render_js=true` + "`" + `) A URL-safe Base64 encoded JSON array describing a sequence of user actions to perform on the page.
        
        The JSON array consists of objects, where each object represents a single action. The supported actions are:
        
        *   **` + "`" + `click` + "`" + `**: Clicks on an element.
            ` + "`" + `{"click": {"selector": "#button"}}` + "`" + `
        *   **` + "`" + `fill` + "`" + `**: Fills an input field with a value.
            ` + "`" + `{"fill": {"selector": "#username", "value": "my_user"}}` + "`" + `
        *   **` + "`" + `wait` + "`" + `**: Pauses execution for a set number of milliseconds.
            ` + "`" + `{"wait": {"delay": 2000}}` + "`" + `
        *   **` + "`" + `scroll` + "`" + `**: Scrolls an element or the window.
            ` + "`" + `{"scroll": {"selector": "#infinite-scroll-div"}}` + "`" + ` or ` + "`" + `{"scroll": {"x": 0, "y": 1000}}` + "`" + `
        *   **` + "`" + `wait_for_selector` + "`" + `**: Waits for an element to appear in the DOM.
            ` + "`" + `{"wait_for_selector": {"selector": "#results", "timeout": 5000}}` + "`" + `
        *   **` + "`" + `wait_for_navigation` + "`" + `**: Waits for the page to navigate to a new URL.
            ` + "`" + `{"wait_for_navigation": {}}` + "`" + `
        *   **` + "`" + `condition` + "`" + `**: Executes a set of actions only if a selector exists.
            ` + "`" + `{"condition": {"selector": "#gdpr-banner", "actions": [{"click": {"selector": "#accept-cookies"}}]}}` + "`" + `
        *   **` + "`" + `execute` + "`" + `**: Executes a raw JavaScript snippet.
            ` + "`" + `{"execute": {"script": "console.log('hello from scenario');"}}` + "`" + `
      schema: { type: "string" }
      examples:
        loginFlow:
          summary: "Login Flow Scenario"
          description: |
            This example demonstrates filling a username and password, then clicking a login button.
            
            **Raw JSON:**
            ` + "```" + `json
            [
              {"fill": {"selector": "#username", "value": "my_user"}},
              {"fill": {"selector": "#password", "value": "my_secret_pass"}},
              {"click": {"selector": "#login_button"}},
              {"wait_for_navigation": {}}
            ]
            ` + "```" + `
          value: "W3siZmlsbCI6IHsic2VsZWN0b3IiOiAiI3VzZXJuYW1lIiwgInZhbHVlIjogIm15X3VzZXIifX0sIHsiZmlsbCI6IHsic2VsZWN0b3IiOiAiI3Bhc3N3b3JkIiwgInZhbHVlIjogIm15X3NlY3JldF9wYXNzIn19LCB7ImNsaWNrIjogeyJzZWxlY3RvciI6ICIjbG9naW5fYnV0dG9uIn19LCB7IndhaXRfZm9yX25hdmlnYXRpb24iOiB7fX1d"
      tags: ["Headless Browser / Javascript Rendering"]
    Geolocation: { name: "geolocation", in: "query", description: "(Requires ` + "`" + `render_js=true` + "`" + `) Spoofs the browser's geolocation. Format: ` + "`" + `latitude,longitude` + "`" + `.", schema: { type: "string", example: "48.8566,2.3522" }, tags: ["Headless Browser / Javascript Rendering"] }
    AutoScroll: { name: "auto_scroll", in: "query", description: "(Requires ` + "`" + `render_js=true` + "`" + `) If true, automatically scrolls to the bottom of the page to trigger lazy-loaded content.", schema: { type: "boolean", default: false }, tags: ["Headless Browser / Javascript Rendering"] }
    RenderingStage: { name: "rendering_stage", in: "query", description: "(Requires ` + "`" + `render_js=true` + "`" + `) The page loading stage to wait for before returning.", schema: { type: "string", default: "complete", enum: ["complete", "domcontentloaded"] }, tags: ["Headless Browser / Javascript Rendering"] }
    # Cache
    Cache: { name: "cache", in: "query", description: "Enables the cache layer. Returns a cached result if available and not expired. Cannot be used with ` + "`" + `session` + "`" + `.", schema: { type: "boolean", default: false }, tags: ["Cache"] }
    CacheTtl: { name: "cache_ttl", in: "query", description: "(Requires ` + "`" + `cache=true` + "`" + `) Cache Time-To-Live in seconds. Max is 604800 (7 days).", schema: { type: "integer", default: 86400 }, tags: ["Cache"] }
    CacheClear: { name: "cache_clear", in: "query", description: "(Requires ` + "`" + `cache=true` + "`" + `) If true, forces a fresh scrape and refreshes the cached version.", schema: { type: "boolean", default: false }, tags: ["Cache"] }
    # Session
    Session: { name: "session", in: "query", description: "A unique session identifier to reuse cookies, localStorage, sessionStorage and browser fingerprint across multiple requests. Cannot be used with ` + "`" + `cache` + "`" + `.", schema: { type: "string" }, tags: ["Session"] }
    SessionStickyProxy: { name: "session_sticky_proxy", in: "query", description: "(Requires ` + "`" + `session` + "`" + `) If true, makes a best effort to use the same proxy IP for the entire session.", schema: { type: "boolean", default: true }, tags: ["Session"] }

  schemas:
    ScrapeResult:
      type: "object"
      properties:
        result:
          type: "object"
          properties:
            content: { type: "string", description: "The HTML, JSON, or other content of the scraped page." }
            format: { type: "string" }
            status_code: { type: "integer" }
            reason: { type: "string" }
            headers: { type: "object" }
            screenshots: { type: "object", description: "Contains URLs to the captured screenshots, keyed by the names provided in the request." }
            dns: { type: "object", description: "Contains DNS records if ` + "`" + `dns=true` + "`" + ` was used." }
            ssl: { type: "object", description: "Contains SSL certificate information if ` + "`" + `ssl=true` + "`" + ` was used." }
            browser_data: { type: "object", description: "Data captured from the headless browser, including XHR calls, local storage, and JS evaluation results." }
        context: { type: "object", description: "Contextual information about the scrape, such as proxy details, cache state, or session data." }
        config: { type: "object", description: "The configuration used for this specific scrape request." }
        success: { type: "boolean", description: "Indicates if the overall ScrapFly operation was successful." }
        status_code: { type: "integer", description: "The HTTP status code of the ScrapFly API response." }
        reason: { type: "string", description: "The HTTP reason phrase of the ScrapFly API response." }
    ErrorResponse:
      type: "object"
      properties:
        status: { type: "string", example: "error" }
        http_code: { type: "integer", description: "The HTTP status code of the error.", example: 403 }
        reason: { type: "string", description: "The HTTP reason phrase.", example: "Forbidden" }
        message: { type: "string", description: "A human-readable error message.", example: "Invalid API key" }
        error_id: { type: "string", format: "uuid", description: "A unique ID for this error instance." }
        result:
          type: "object"
          properties:
            error:
              type: "object"
              properties:
                code: { type: "string", description: "ScrapFly's internal error code.", example: "ERR::SCRAPE::DOM_SELECTOR_NOT_FOUND" }
                description: { type: "string", description: "A detailed description of the error." }
                retryable: { type: "boolean" }
                doc_url: { type: "string", format: "uri" }
`
