package scrapflyprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/tools"
)

// Cloud Browser credential vault, the full 15-endpoint surface.
//
// The vault key is customer-held and Scrapfly keeps no copy, so no endpoint can
// return a plaintext item secret and nothing here may imply one does. The vault
// key, the X-Vault-Key header and the 1Password service-account token are secret
// material: they are never logged, never echoed into a tool result and never put
// into an error message.
//
// The ten core endpoints go through the SDK's CloudBrowserVault* wrappers. The
// five /vault/{id}/service routes have no wrapper in go-scrapfly, so they are
// issued here against the same client, which keeps the -browser-host override and
// the SDK's http.Client in play.

// defaultCloudBrowserRESTHost matches the SDK default and is only reached when the
// client's own CDP URL does not parse.
const defaultCloudBrowserRESTHost = "https://browser.scrapfly.io"

// cloudBrowserRESTHost recovers the REST host from the client. The SDK exports no
// getter for it, and the CDP URL is the only place the configured host surfaces;
// its query carries the api key, so nothing but scheme and host is read out.
func cloudBrowserRESTHost(c *scrapfly.Client) string {
	u, err := url.Parse(c.CloudBrowser(nil))
	if err != nil || u.Host == "" {
		return defaultCloudBrowserRESTHost
	}
	if u.Scheme == "ws" || u.Scheme == "http" {
		return "http://" + u.Host
	}
	return "https://" + u.Host
}

// vaultServiceCall issues one /vault/{id}/service request. vaultKey goes on the
// X-Vault-Key header and is never part of the URL, the log line or the error; the
// error carries the API's own body, which the link flow never puts a token in.
func (p *ScrapflyToolProvider) vaultServiceCall(
	ctx context.Context,
	c *scrapfly.Client,
	method, action, vaultID, vaultKey string,
	query url.Values,
	body any,
) (map[string]any, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/vault/%s/service%s",
		cloudBrowserRESTHost(c), url.PathEscape(vaultID), action))
	if err != nil {
		return nil, fmt.Errorf("vault service endpoint: %w", err)
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("key", c.APIKey())
	endpoint.RawQuery = query.Encode()

	var payload io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal vault service body: %w", err)
		}
		payload = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if vaultKey != "" {
		req.Header.Set("X-Vault-Key", vaultKey)
	}

	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault service request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vault service read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("vault service call failed with status %d: %s", resp.StatusCode, respBody)
	}
	out := map[string]any{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &out); err != nil {
			return nil, fmt.Errorf("decode vault service response: %w", err)
		}
	}
	return out, nil
}

func vaultErr(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		IsError: true,
	}
}

func vaultJSON(payload any) *mcp.CallToolResult {
	buf, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return vaultErr(fmt.Errorf("marshal vault response: %w", err))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(buf)}}}
}

// vaultDryRun previews a mutating call. Callers build the preview from non-secret
// fields only: this text reaches the model and the transcript.
func vaultDryRun(action string, preview map[string]any) *mcp.CallToolResult {
	buf, _ := json.MarshalIndent(preview, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("DRY RUN — %s would be invoked with:\n%s\n\nRe-invoke with confirm=true to commit.", action, string(buf)),
		}},
	}
}

// secretPresence reports whether a secret was supplied without revealing it.
func secretPresence(v string) string {
	if v == "" {
		return "not supplied"
	}
	return "supplied (withheld)"
}

// secretFieldNames lists a secret object's keys for a preview. Values stay out.
func secretFieldNames(secret map[string]any) []string {
	if len(secret) == 0 {
		return nil
	}
	names := make([]string, 0, len(secret))
	for k := range secret {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

// stripItemCiphertext drops secret_blob from an item list. The ciphertext is
// unreadable without the customer-held key and only costs the model context.
func stripItemCiphertext(out map[string]any) map[string]any {
	items, ok := out["items"].([]any)
	if !ok {
		return out
	}
	for _, raw := range items {
		if item, ok := raw.(map[string]any); ok {
			delete(item, "secret_blob")
		}
	}
	return out
}

// vaultConfirm gates every mutating vault tool. Prompt injection reaches this
// surface, so the first call renders the request and only confirm=true commits.
type vaultConfirm struct {
	Confirm bool `json:"confirm,omitempty" jsonschema:"REQUIRED to actually perform the action. When false (default) the tool returns the rendered request, with secret material replaced by a presence marker, so the model and the user can review before committing."`
}

type VaultGetInput struct {
	VaultID string `json:"vault_id" jsonschema:"Vault id from cloud_browser_vault_list."`
}

type VaultCreateInput struct {
	vaultConfirm
	Name        string `json:"name"                  jsonschema:"Vault name, letters and digits only (A-Z, a-z, 0-9). Cloud Browser sessions select a vault by this name, so keep it short and stable."`
	Description string `json:"description,omitempty" jsonschema:"Optional free-text note about what the vault holds."`
}

type VaultUpdateInput struct {
	vaultConfirm
	VaultID     string `json:"vault_id"              jsonschema:"Vault id to patch."`
	Name        string `json:"name,omitempty"        jsonschema:"New vault name, letters and digits only (A-Z, a-z, 0-9). Omit to leave unchanged. Renaming breaks any session config that selects the vault by its old name."`
	Description string `json:"description,omitempty" jsonschema:"New description. Omit to leave unchanged."`
}

type VaultDeleteInput struct {
	vaultConfirm
	VaultID string `json:"vault_id" jsonschema:"Vault id to delete with every item it holds. Cannot be undone."`
}

type VaultRotateInput struct {
	vaultConfirm
	VaultID  string `json:"vault_id"  jsonschema:"Vault id whose key is rotated."`
	VaultKey string `json:"vault_key" jsonschema:"The CURRENT base64 32-byte vault key. Required: the server rewraps every item under a fresh key and cannot do that without the current one."`
}

type VaultItemListInput struct {
	VaultID string `json:"vault_id" jsonschema:"Vault id whose items to list."`
}

type VaultItemCreateInput struct {
	vaultConfirm
	VaultID  string         `json:"vault_id"           jsonschema:"Vault id to add the item to."`
	VaultKey string         `json:"vault_key"          jsonschema:"Base64 32-byte vault key. Required: the secret is sealed under it and the server verifies it before writing anything."`
	Type     string         `json:"type"               jsonschema:"Item type: password, passkey, cookie, totp or blob. Decides which fields secret must carry."`
	Label    string         `json:"label"              jsonschema:"Human label for the item, shown in the dashboard and used to reference it from an agent run."`
	Origin   string         `json:"origin"             jsonschema:"Origin the credential is injected on, for example https://example.com/login. Required by the API for every type."`
	Username string         `json:"username,omitempty" jsonschema:"Account identifier for password and passkey items. Not a secret."`
	Secret   map[string]any `json:"secret"             jsonschema:"Secret payload for the chosen type. password: {password}. passkey: {credentialId, privateKey, userHandle, signCount}. cookie: {name, value, domain, path, secure, httpOnly, sameSite, expires}. totp: {seed, issuer, account, algorithm, digits, period}. blob: {data, content_type}."`
	Metadata map[string]any `json:"metadata,omitempty" jsonschema:"Optional non-secret JSON stored alongside the item."`
}

type VaultItemUpdateInput struct {
	vaultConfirm
	VaultID  string         `json:"vault_id"           jsonschema:"Vault id holding the item."`
	ItemID   string         `json:"item_id"            jsonschema:"Item id from cloud_browser_vault_item_list."`
	VaultKey string         `json:"vault_key,omitempty" jsonschema:"Base64 32-byte vault key. Required only when secret is sent; a label, origin, username or metadata patch needs no key."`
	Label    string         `json:"label,omitempty"    jsonschema:"New label. Omit to leave unchanged."`
	Origin   string         `json:"origin,omitempty"   jsonschema:"New origin. Omit to leave unchanged."`
	Username string         `json:"username,omitempty" jsonschema:"New account identifier. Omit to leave unchanged."`
	Secret   map[string]any `json:"secret,omitempty"   jsonschema:"New secret payload, same shape as cloud_browser_vault_item_create for the item's type. Overwrites the stored secret with no way back."`
	Metadata map[string]any `json:"metadata,omitempty" jsonschema:"New non-secret JSON. Omit to leave unchanged."`
}

type VaultItemDeleteInput struct {
	vaultConfirm
	VaultID string `json:"vault_id" jsonschema:"Vault id holding the item."`
	ItemID  string `json:"item_id"  jsonschema:"Item id to delete. Cannot be undone."`
}

type VaultServiceLinkInput struct {
	vaultConfirm
	VaultID           string         `json:"vault_id"                     jsonschema:"Vault id to link. It must be a manual vault with no rows owned by another service."`
	VaultKey          string         `json:"vault_key"                    jsonschema:"Base64 32-byte vault key. Required: the token is sealed under it and the server verifies it first."`
	LinkedService     string         `json:"linked_service,omitempty"     jsonschema:"Provider discriminator. 1password is the only accepted value and is used when omitted."`
	Token             string         `json:"token"                        jsonschema:"1Password service-account token. Required. Treated as secret material: it is sealed in the vault and never returned by any endpoint."`
	LinkedServiceData map[string]any `json:"linked_service_data,omitempty" jsonschema:"Non-secret selection rules: {vault_id or vault_name, title_filter, tags, sync_mode (manual or on_session), sync_ttl_s}. One of vault_id or vault_name is required. Call cloud_browser_vault_service_test first to learn which upstream vaults the token can see."`
}

type VaultServiceUpdateInput struct {
	vaultConfirm
	VaultID           string         `json:"vault_id"                     jsonschema:"Linked vault id to patch."`
	VaultKey          string         `json:"vault_key,omitempty"          jsonschema:"Base64 32-byte vault key. Required only when token is sent."`
	Token             string         `json:"token,omitempty"              jsonschema:"Replacement 1Password service-account token. Omit to keep the sealed one. Secret material: never returned by any endpoint."`
	LinkedServiceData map[string]any `json:"linked_service_data,omitempty" jsonschema:"Replacement selection rules. REPLACES the stored document wholesale, so send every field you want kept. token_item_id is ignored; the stored id wins."`
}

type VaultServiceUnlinkInput struct {
	vaultConfirm
	VaultID string `json:"vault_id" jsonschema:"Linked vault id to unlink."`
	// Pointer so an omitted flag cannot read as false: the API defaults to keep
	// and dropping the mirrored rows is irreversible.
	KeepItems *bool `json:"keep_items,omitempty" jsonschema:"Defaults to true, which keeps the mirrored rows as manual items. false deletes every mirrored row and cannot be undone."`
}

type VaultServiceSyncInput struct {
	VaultID  string `json:"vault_id"  jsonschema:"Linked vault id to sync now."`
	VaultKey string `json:"vault_key" jsonschema:"Base64 32-byte vault key. Required: the sealed service-account token can only be opened with it."`
}

type VaultServiceTestInput struct {
	VaultID       string `json:"vault_id"                 jsonschema:"Vault id the probe is scoped to. The vault does not need to be linked yet when a token is supplied."`
	VaultKey      string `json:"vault_key"                jsonschema:"Base64 32-byte vault key. Required on this endpoint even when a candidate token is supplied."`
	LinkedService string `json:"linked_service,omitempty" jsonschema:"Provider discriminator. 1password is the only accepted value and is assumed when omitted."`
	Token         string `json:"token,omitempty"          jsonschema:"Candidate 1Password service-account token to probe. Omit to probe the token already sealed in the vault. Secret material: never echoed back."`
}

func (p *ScrapflyToolProvider) CloudBrowserVaultList(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ DummyInput,
) (*mcp.CallToolResult, any, error) {
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	p.logger.Println("Executing tool: cloud_browser_vault_list")
	out, err := c.CloudBrowserVaultList()
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultGet(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultGetInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := c.CloudBrowserVaultGet(in.VaultID)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultCreate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultCreateInput,
) (*mcp.CallToolResult, any, error) {
	if in.Name == "" {
		return vaultErr(fmt.Errorf("name is required")), nil, nil
	}
	if !in.Confirm {
		return vaultDryRun("POST /vault", map[string]any{
			"name":        in.Name,
			"description": in.Description,
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := c.CloudBrowserVaultCreate(in.Name, in.Description)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultUpdate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultUpdateInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	if in.Name == "" && in.Description == "" {
		return vaultErr(fmt.Errorf("name or description is required")), nil, nil
	}
	if !in.Confirm {
		return vaultDryRun("PATCH /vault/"+in.VaultID, map[string]any{
			"name":        in.Name,
			"description": in.Description,
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := c.CloudBrowserVaultUpdate(in.VaultID, in.Name, in.Description)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultDelete(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultDeleteInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	if !in.Confirm {
		return vaultDryRun("DELETE /vault/"+in.VaultID, map[string]any{
			"vault_id": in.VaultID,
			"effect":   "deletes the vault and every item in it, cannot be undone",
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := c.CloudBrowserVaultDelete(in.VaultID)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultRotate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultRotateInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	if in.VaultKey == "" {
		return vaultErr(fmt.Errorf("vault_key is required: rotation rewraps every item under a new key")), nil, nil
	}
	if !in.Confirm {
		return vaultDryRun("POST /vault/"+in.VaultID+"/rotate", map[string]any{
			"vault_id":  in.VaultID,
			"vault_key": secretPresence(in.VaultKey),
			"effect":    "invalidates the current key and returns a new one once",
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := c.CloudBrowserVaultRotate(in.VaultID, in.VaultKey)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultItemList(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultItemListInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := c.CloudBrowserVaultItemList(in.VaultID)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(stripItemCiphertext(out)), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultItemCreate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultItemCreateInput,
) (*mcp.CallToolResult, any, error) {
	switch {
	case in.VaultID == "":
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	case in.VaultKey == "":
		return vaultErr(fmt.Errorf("vault_key is required: the secret is sealed under it")), nil, nil
	case in.Type == "":
		return vaultErr(fmt.Errorf("type is required: password, passkey, cookie, totp or blob")), nil, nil
	case len(in.Secret) == 0:
		return vaultErr(fmt.Errorf("secret is required for type %q", in.Type)), nil, nil
	}
	if !in.Confirm {
		return vaultDryRun("POST /vault/"+in.VaultID+"/item", map[string]any{
			"type":          in.Type,
			"label":         in.Label,
			"origin":        in.Origin,
			"username":      in.Username,
			"secret_fields": secretFieldNames(in.Secret),
			"vault_key":     secretPresence(in.VaultKey),
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	item := map[string]any{
		"type":   in.Type,
		"label":  in.Label,
		"origin": in.Origin,
		"secret": in.Secret,
	}
	if in.Username != "" {
		item["username"] = in.Username
	}
	if len(in.Metadata) > 0 {
		item["metadata"] = in.Metadata
	}
	out, err := c.CloudBrowserVaultItemCreate(in.VaultID, in.VaultKey, item)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultItemUpdate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultItemUpdateInput,
) (*mcp.CallToolResult, any, error) {
	switch {
	case in.VaultID == "":
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	case in.ItemID == "":
		return vaultErr(fmt.Errorf("item_id is required")), nil, nil
	// Refused here rather than as a 400: the API seals the new secret under this
	// key and verifies it first, so an absent key is a caller mistake, not a retry.
	case len(in.Secret) > 0 && in.VaultKey == "":
		return vaultErr(fmt.Errorf("vault_key is required when secret is sent")), nil, nil
	}
	patch := map[string]any{}
	if in.Label != "" {
		patch["label"] = in.Label
	}
	if in.Origin != "" {
		patch["origin"] = in.Origin
	}
	if in.Username != "" {
		patch["username"] = in.Username
	}
	if len(in.Metadata) > 0 {
		patch["metadata"] = in.Metadata
	}
	if len(in.Secret) > 0 {
		patch["secret"] = in.Secret
	}
	if len(patch) == 0 {
		return vaultErr(fmt.Errorf("nothing to patch: send label, origin, username, metadata or secret")), nil, nil
	}
	if !in.Confirm {
		preview := map[string]any{
			"item_id":   in.ItemID,
			"label":     in.Label,
			"origin":    in.Origin,
			"username":  in.Username,
			"vault_key": secretPresence(in.VaultKey),
		}
		if len(in.Secret) > 0 {
			preview["secret_fields"] = secretFieldNames(in.Secret)
			preview["effect"] = "overwrites the stored secret, the previous one is not recoverable"
		}
		return vaultDryRun("PATCH /vault/"+in.VaultID+"/item/"+in.ItemID, preview), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	// Empty vault key means the SDK omits X-Vault-Key, which is what a
	// metadata-only patch must send.
	out, err := c.CloudBrowserVaultItemUpdate(in.VaultID, in.ItemID, in.VaultKey, patch)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultItemDelete(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultItemDeleteInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" || in.ItemID == "" {
		return vaultErr(fmt.Errorf("vault_id and item_id are required")), nil, nil
	}
	if !in.Confirm {
		return vaultDryRun("DELETE /vault/"+in.VaultID+"/item/"+in.ItemID, map[string]any{
			"item_id": in.ItemID,
			"effect":  "deletes the credential, cannot be undone",
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := c.CloudBrowserVaultItemDelete(in.VaultID, in.ItemID)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultServiceLink(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultServiceLinkInput,
) (*mcp.CallToolResult, any, error) {
	switch {
	case in.VaultID == "":
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	case in.VaultKey == "":
		return vaultErr(fmt.Errorf("vault_key is required: the provider token is sealed under it")), nil, nil
	case in.Token == "":
		return vaultErr(fmt.Errorf("token is required to link a service")), nil, nil
	}
	// The API registers one discriminator and rejects an empty one outright.
	service := in.LinkedService
	if service == "" {
		service = "1password"
	}
	if !in.Confirm {
		return vaultDryRun("POST /vault/"+in.VaultID+"/service", map[string]any{
			"linked_service":      service,
			"linked_service_data": in.LinkedServiceData,
			"token":               secretPresence(in.Token),
			"vault_key":           secretPresence(in.VaultKey),
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := p.vaultServiceCall(ctx, c, http.MethodPost, "", in.VaultID, in.VaultKey, nil, map[string]any{
		"linked_service":      service,
		"token":               in.Token,
		"linked_service_data": in.LinkedServiceData,
	})
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultServiceUpdate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultServiceUpdateInput,
) (*mcp.CallToolResult, any, error) {
	switch {
	case in.VaultID == "":
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	case in.Token != "" && in.VaultKey == "":
		return vaultErr(fmt.Errorf("vault_key is required when token is sent")), nil, nil
	case in.Token == "" && len(in.LinkedServiceData) == 0:
		return vaultErr(fmt.Errorf("send token, linked_service_data or both")), nil, nil
	}
	if !in.Confirm {
		return vaultDryRun("PATCH /vault/"+in.VaultID+"/service", map[string]any{
			"linked_service_data": in.LinkedServiceData,
			"token":               secretPresence(in.Token),
			"vault_key":           secretPresence(in.VaultKey),
			"effect":              "linked_service_data replaces the stored document wholesale",
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	body := map[string]any{}
	if in.Token != "" {
		body["token"] = in.Token
	}
	if len(in.LinkedServiceData) > 0 {
		body["linked_service_data"] = in.LinkedServiceData
	}
	out, err := p.vaultServiceCall(ctx, c, http.MethodPatch, "", in.VaultID, in.VaultKey, nil, body)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultServiceUnlink(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultServiceUnlinkInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	keepItems := "true (server default)"
	query := url.Values{}
	if in.KeepItems != nil {
		keepItems = fmt.Sprintf("%t", *in.KeepItems)
		query.Set("keep_items", keepItems)
	}
	if !in.Confirm {
		return vaultDryRun("DELETE /vault/"+in.VaultID+"/service", map[string]any{
			"keep_items": keepItems,
			"effect":     "deletes the sealed provider token; with keep_items=false it also deletes every mirrored item, which cannot be undone",
		}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	out, err := p.vaultServiceCall(ctx, c, http.MethodDelete, "", in.VaultID, "", query, nil)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultServiceSync(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultServiceSyncInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	if in.VaultKey == "" {
		return vaultErr(fmt.Errorf("vault_key is required: the sealed provider token is opened with it")), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	p.logger.Printf("Executing tool: cloud_browser_vault_service_sync (vault=%s)", in.VaultID)
	out, err := p.vaultServiceCall(ctx, c, http.MethodPost, "/sync", in.VaultID, in.VaultKey, nil, nil)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserVaultServiceTest(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in VaultServiceTestInput,
) (*mcp.CallToolResult, any, error) {
	if in.VaultID == "" {
		return vaultErr(fmt.Errorf("vault_id is required")), nil, nil
	}
	if in.VaultKey == "" {
		return vaultErr(fmt.Errorf("vault_key is required on this endpoint")), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	body := map[string]any{}
	if in.LinkedService != "" {
		body["linked_service"] = in.LinkedService
	}
	if in.Token != "" {
		body["token"] = in.Token
	}
	out, err := p.vaultServiceCall(ctx, c, http.MethodPost, "/test", in.VaultID, in.VaultKey, nil, body)
	if err != nil {
		return vaultErr(err), nil, nil
	}
	return vaultJSON(out), nil, nil
}

// cloudBrowserVaultTools is folded into staticTools: a vault is managed without a
// browser session, so the family is never mounted or unmounted.
//
// Annotations are what a client gates on. Read-only on the three tools that only
// read; destructive and non-idempotent on delete, item delete, unlink and rotate,
// which all destroy something the customer cannot get back from Scrapfly.
func cloudBrowserVaultTools(provider *ScrapflyToolProvider) tools.HandledToolSet {
	HandledTools := tools.NewHandledToolset()

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_list",
		Title:       "Scrapfly Cloud Browser Vault — List",
		Description: "List the Cloud Browser credential vaults this API key can see in its project and environment, with item counts, vault_type, link state and last sync status. No secret material is returned. Start here to resolve a vault id for every other vault tool.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — List",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultList)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_get",
		Title:       "Scrapfly Cloud Browser Vault — Get",
		Description: "Fetch one vault: name, description, item count and, for a linked vault, the 1Password selection rules plus last sync time, sync status, sync error and counts. No secret material is returned. Use cloud_browser_vault_list when the id is unknown, cloud_browser_vault_item_list for the items themselves.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Get",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultGet)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_create",
		Title:       "Scrapfly Cloud Browser Vault — Create",
		Description: "Create an empty credential vault. The response carries the vault key once. Scrapfly stores no copy, so it cannot be retrieved again and losing it makes every item in the vault unreadable: show it to the user and have them save it before adding items. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Create",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultCreate)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_update",
		Title:       "Scrapfly Cloud Browser Vault — Update",
		Description: "Rename a vault or change its description. Items, the vault key and any linked service stay as they are. A rename breaks session configs that select the vault by its old name. Use cloud_browser_vault_service_update for 1Password selection rules. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Update",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultUpdate)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_delete",
		Title:       "Scrapfly Cloud Browser Vault — Delete",
		Description: "Delete a vault and every item it holds. This cannot be undone, and any Cloud Browser session that names the vault stops resolving credentials. Prefer cloud_browser_vault_item_delete when only one credential is unwanted. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Delete",
			DestructiveHint: &trueBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultDelete)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_rotate",
		Title:       "Scrapfly Cloud Browser Vault — Rotate Key",
		Description: "Rotate the vault key: send the current key, get a fresh one back once. The previous key is invalidated immediately, so every stored config and script still holding it stops opening the vault. Scrapfly keeps no copy of either key. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Rotate Key",
			DestructiveHint: &trueBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultRotate)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_item_list",
		Title:       "Scrapfly Cloud Browser Vault — List Items",
		Description: "List a vault's items: id, type, label, origin, username, provenance and timestamps. Secrets stay encrypted under the customer-held key, which Scrapfly does not have, and the ciphertext is dropped from this result, so no plaintext is ever returned. Rows whose source is not manual belong to the linked service and refuse edits.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — List Items",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultItemList)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_item_create",
		Title:       "Scrapfly Cloud Browser Vault — Create Item",
		Description: "Add one credential to a vault. The vault key is required because the secret is sealed under it, and the server verifies the key before writing, so a wrong key fails here instead of leaving a row nothing can open. type is password, passkey, cookie, totp or blob, and secret must carry that type's fields. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Create Item",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultItemCreate)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_item_update",
		Title:       "Scrapfly Cloud Browser Vault — Update Item",
		Description: "Patch one vault item. Fields you send overwrite the row. Sending secret replaces the stored secret with no way back and requires the vault key; a label, origin, username or metadata patch does not. Items owned by a linked service are refused with a conflict. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Update Item",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultItemUpdate)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_item_delete",
		Title:       "Scrapfly Cloud Browser Vault — Delete Item",
		Description: "Delete one item from a vault. This cannot be undone and the credential is gone from every later Cloud Browser session. Items owned by a linked service are refused with a conflict; unlink the service first or drop the item upstream. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Delete Item",
			DestructiveHint: &trueBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultItemDelete)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_service_link",
		Title:       "Scrapfly Cloud Browser Vault — Link 1Password",
		Description: "Link a vault to 1Password so its items mirror an upstream vault. The vault key is required and verified first: the service-account token is sealed under it, and a well-formed wrong key would link the vault and leave a token every later sync fails to open. Run cloud_browser_vault_service_test first to pick the upstream vault. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Link 1Password",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultServiceLink)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_service_update",
		Title:       "Scrapfly Cloud Browser Vault — Update Link",
		Description: "Change a linked vault's 1Password selection rules, rotate its service-account token, or both. linked_service_data replaces the stored document wholesale, so send every field you want kept. The vault key is required only when a token is sent, and token_item_id in the body is ignored. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Update Link",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultServiceUpdate)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_service_unlink",
		Title:       "Scrapfly Cloud Browser Vault — Unlink 1Password",
		Description: "Unlink a vault from 1Password. keep_items defaults to true and leaves the mirrored rows behind as manual items; keep_items=false deletes every mirrored row and that cannot be undone. The sealed service-account token is deleted either way. Two-step confirmation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Unlink 1Password",
			DestructiveHint: &trueBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultServiceUnlink)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_service_sync",
		Title:       "Scrapfly Cloud Browser Vault — Sync Now",
		Description: "Sync a linked vault from 1Password now, bypassing the freshness window and the one hour back-off a failed run leaves behind. Needs the vault key to open the sealed token. Returns imported, updated, deleted, skipped, unmirrored, status and warnings. The server budget is 25 seconds.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Sync Now",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultServiceSync)

	tools.MustAddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_vault_service_test",
		Title:       "Scrapfly Cloud Browser Vault — Test Link",
		Description: "Probe a 1Password service-account token and list the upstream vaults it can reach, changing nothing. The vault key is required. Pass token to check one that is not stored yet, or omit it to probe the token already sealed in the vault. Every call spends the provider's rate-limit budget. The server budget is 10 seconds.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser Vault — Test Link",
			DestructiveHint: &falseBool,
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserVaultServiceTest)

	return HandledTools
}
