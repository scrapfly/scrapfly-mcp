package scrapflyprovider

import (
	"context"
	"encoding/base64"
	"log"

	"github.com/scrapfly/scrapfly-mcp/internal/sanitizer"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

// Disabled because LLM are abusing it despite every try to  preveent it
// Since it's way better for this context to direction use ExtractionPrompt in scraping tools anyway thats not a probleme

type ExtractionToolInput struct {
	Content                     string                   `json:"content" jsonschema:"The raw HTML/XML content to be processed."`
	ContentType                 string                   `json:"content_type" jsonschema:"The content type of the body (e.g., 'text/html')."`
	Charset                     string                   `json:"charset,omitempty" jsonschema:"Optional: character set of the body (e.g., 'utf-8')."`
	URL                         string                   `json:"url,omitempty" jsonschema:"Optional: The original URL of the content for resolving relative links."`
	ExtractionPrompt            string                   `json:"extraction_prompt,omitempty" jsonschema:"required one of and exclusive with extraction_template and extraction_model, AI prompt to guide data extraction."`
	ExtractionModel             scrapfly.ExtractionModel `json:"extraction_model,omitempty" jsonschema:"required one of and exclusive with extraction_template and extraction_prompt, The AI model to use for extraction."`
	ExtractionTemplate          string                   `json:"extraction_template,omitempty" jsonschema:"required one of and exclusive with extraction_prompt and extraction_model, An extraction template to get structured data from the page."`
	ExtractionEphemeralTemplate map[string]any           `json:"extraction_ephemeral_template,omitempty" jsonschema:"required one of and exclusive with extraction_prompt, An ephemeral extraction template to get structured data from the page."`
	Base64                      bool                     `json:"base64,omitempty" jsonschema:"If true, the content is base64 encoded."`
}

func (p *ScrapflyToolProvider) Extract(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ExtractionToolInput,
) (*mcp.CallToolResult, *scrapfly.ExtractionResult, error) {
	log.Println("Executing tool: scrapfly.extraction")
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("extraction", err), nil, err
	}

	body := []byte(input.Content)
	if input.Base64 {
		b, err := base64.StdEncoding.DecodeString(input.Content)
		if err != nil {
			return ToolErrf("extraction: invalid base64: %v", err), nil, err
		}
		body = b
	}

	config := &scrapfly.ExtractionConfig{
		Body:                        body,
		ContentType:                 input.ContentType,
		Charset:                     input.Charset,
		URL:                         input.URL,
		ExtractionPrompt:            input.ExtractionPrompt,
		ExtractionModel:             input.ExtractionModel,
		ExtractionTemplate:          input.ExtractionTemplate,
		ExtractionEphemeralTemplate: input.ExtractionEphemeralTemplate,
	}

	extractionResult, err := client.Extract(config)
	if err != nil {
		return ToolErrFromError("extraction", err), nil, err
	}
	sanitizer.BasicSanitizeNils(extractionResult)
	return nil, extractionResult, err
}
