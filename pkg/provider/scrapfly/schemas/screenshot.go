package schemas

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/scrapfly/go-scrapfly"
)

func MakeScreenshotFormatSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Title:       "Image Format",
		Enum:        []any{string(scrapfly.FormatJPG), string(scrapfly.FormatPNG), string(scrapfly.FormatWEBP), string(scrapfly.FormatGIF)},
		Default:     json.RawMessage(`"jpg"`),
		Description: "The image format to use for the screenshot.",
	}
}

func MakeScreenshotOptionsSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "array",
		Description: "Screenshot options to use for the screenshot.",
		Items: &jsonschema.Schema{
			Type: "string",
			Enum: []any{string(scrapfly.OptionLoadImages), string(scrapfly.OptionDarkMode), string(scrapfly.OptionBlockBanners), string(scrapfly.OptionPrintMediaFormat)},
		},
		Default: json.RawMessage(`[]`),
	}
}

func MakeScreenshotCaptureSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Title:       "Capture",
		Default:     json.RawMessage(`"fullpage"`),
		Description: "The capture to use for the screenshot. Either fullpage or a CSS selector	",
	}
}

func MakeScreenshotResolutionSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Title:       "Resolution",
		Description: "The resolution to use for the screenshot. e.g. 1920x1080",
		Pattern:     "^[0-9]+x[0-9]+$",
		Default:     json.RawMessage(`"1920x1080"`),
	}
}

func MustRefineScreenshotToolInputSchema[T any]() *jsonschema.Schema {
	schema := SchemaFor[T]()
	schema.Properties["url"] = MakeUrlSchema()
	schema.Properties["format"] = MakeScreenshotFormatSchema()
	schema.Properties["capture"] = MakeScreenshotCaptureSchema()
	schema.Properties["resolution"] = MakeScreenshotResolutionSchema()
	schema.Properties["country"] = MakeCountrySchema()
	schema.Properties["rendering_wait"] = MakeRenderingWaitSchema()
	schema.Properties["options"] = MakeScreenshotOptionsSchema()
	return schema
}
