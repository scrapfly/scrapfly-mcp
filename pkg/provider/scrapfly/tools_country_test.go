package scrapflyprovider

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// Each tool that names a proxy country takes a list or an exclusion when its API does, and one code for a Cloud Browser session.
func TestEachToolTakesTheCountrySelectionsItsAPITakes(t *testing.T) {
	registered := staticTools(NewScrapflyToolProvider(nil, nil, nil))

	for _, tc := range []struct {
		tool      string
		takesList bool
	}{
		{"web_scrape", true},
		{"web_get_page", true},
		{"screenshot", true},
		{"browser_unblock", true},
		{"cloud_browser_open", false},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			encoded, err := json.Marshal(registered[tc.tool].Tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}

			var schema jsonschema.Schema
			if err := json.Unmarshal(encoded, &schema); err != nil {
				t.Fatal(err)
			}

			resolved, err := schema.Resolve(nil)
			if err != nil {
				t.Fatal(err)
			}

			if err := resolved.Validate(map[string]any{"url": "https://web-scraping.dev", "country": "us"}); err != nil {
				t.Errorf("%s refuses country=us: %v", tc.tool, err)
			}

			for _, country := range []string{"-us", "us,ca"} {
				err := resolved.Validate(map[string]any{"url": "https://web-scraping.dev", "country": country})
				if (err == nil) != tc.takesList {
					t.Errorf("%s takes country=%q: %v, want %v (%v)", tc.tool, country, err == nil, tc.takesList, err)
				}
			}
		})
	}
}
