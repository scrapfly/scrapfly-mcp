package scrapflyprovider

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The discriminator is a wire value: the API matches it against its own registry,
// so the typed constant has to keep serialising to the same bytes.
func TestVaultLinkedServiceWireValue(t *testing.T) {
	if got := string(VaultLinkedServiceOnePassword); got != "1password" {
		t.Fatalf("got %q, want 1password", got)
	}
}

// A named string type infers as a bare string, so the enum only reaches tools/list
// through vaultServiceInputSchema. Asserting on the marshalled schema is what
// catches its refinement silently dropping.
func TestVaultServiceSchemasPublishTheDiscriminatorEnum(t *testing.T) {
	set := staticTools(NewScrapflyToolProvider(nil, nil, nil))
	for _, name := range []string{"cloud_browser_vault_service_link", "cloud_browser_vault_service_test"} {
		handled, ok := set[name]
		if !ok {
			t.Fatalf("tool %q is not registered", name)
		}
		spec := toolProperties(t, name, handled.Tool.InputSchema)["linked_service"]
		enum, _ := spec["enum"].([]any)
		if len(enum) != len(vaultLinkedServices) {
			t.Fatalf("tool %q: linked_service enum is %v, want %v", name, enum, vaultLinkedServices)
		}
		for i, want := range vaultLinkedServices {
			if enum[i] != string(want) {
				t.Errorf("tool %q: enum[%d] = %v, want %q", name, i, enum[i], want)
			}
		}
	}
}

// An omitted discriminator resolves to 1password before the request is rendered:
// POST /service has no server-side default to fall back on.
func TestVaultServiceLinkFillsTheDiscriminator(t *testing.T) {
	p := NewScrapflyToolProvider(nil, nil, nil)
	res, _, err := p.CloudBrowserVaultServiceLink(context.Background(), nil, VaultServiceLinkInput{
		VaultID:  "01J8",
		VaultKey: "a2V5",
		Token:    "ops_token",
	})
	if err != nil {
		t.Fatalf("handler returned %v", err)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("got %T, want TextContent", res.Content[0])
	}
	if !strings.Contains(text.Text, `"linked_service": "1password"`) {
		t.Errorf("rendered request does not carry the discriminator:\n%s", text.Text)
	}
}
