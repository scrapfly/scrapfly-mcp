package scrapflyprovider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

// alertConfirm is embedded in every mutating-alert input so the model can
// preview the rendered request before committing. Without confirm=true the
// tool returns a dry-run envelope instead of hitting the API.
type alertConfirm struct {
	Confirm bool `json:"confirm,omitempty" jsonschema:"description: REQUIRED to actually perform the action. When false (default) the tool returns the rendered request body so the model and user can review before committing."`
}

func alertDryRun(action string, payload any) *mcp.CallToolResult {
	buf, _ := json.MarshalIndent(payload, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("DRY RUN — %s would be invoked with:\n%s\n\nRe-invoke with confirm=true to commit.", action, string(buf)),
		}},
	}
}

func alertErr(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		IsError: true,
	}
}

func alertJSON(payload any) *mcp.CallToolResult {
	buf, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return alertErr(fmt.Errorf("marshal alert response: %w", err))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(buf)}}}
}

type AlertListInput struct {
	ProjectUUID string `json:"project_uuid,omitempty" jsonschema:"description: Optional. Scope to a single project UUID."`
	State       string `json:"state,omitempty"        jsonschema:"description: Optional. Filter by lifecycle state: ok|pending|triggered|recovering|no_data|snoozed."`
	MetricID    string `json:"metric_id,omitempty"    jsonschema:"description: Optional. Filter by metric family ID — see alert_metric_families for valid values."`
}

type AlertGetInput struct {
	AlertUUID string `json:"alert_uuid" jsonschema:"description: ULID of the alert definition to fetch."`
}

type AlertCountActiveInput struct {
	ProjectUUID string `json:"project_uuid,omitempty" jsonschema:"description: Optional. Restrict count to a single project UUID."`
}

type AlertMetricFamiliesInput struct{}

type AlertSeriesInput struct {
	AlertUUID    string `json:"alert_uuid"               jsonschema:"description: ULID of the alert whose series to fetch."`
	RangeMinutes int    `json:"range_minutes,omitempty"  jsonschema:"description: Lookback window in minutes (default 240, max 10080). Out-of-range values are clamped server-side."`
}

type AlertCreateInput struct {
	alertConfirm

	Name        string `json:"name"                  jsonschema:"description: Human-readable alert name shown in the dashboard."`
	Description string `json:"description,omitempty" jsonschema:"description: Optional longer-form description of what the alert detects."`
	ProjectUUID string `json:"project_uuid,omitempty" jsonschema:"description: OPTIONAL. Leave EMPTY (omit the field) to fall back to the caller's currently-selected project. Do NOT pass info_account.account.account_id here — that's the account ID, not a project ID, and the server returns ERR::ALERT::PROJECT_NOT_FOUND."`

	MetricID         string          `json:"metric_id"                   jsonschema:"description: Metric family ID — call alert_metric_families first to discover valid values and allowed_dimensions."`
	MetricDimensions json.RawMessage `json:"metric_dimensions,omitempty" jsonschema:"description: Optional JSON object of string key/value pairs filtering the metric. Keys must be in the metric family's allowed_dimensions list."`

	Comparator         string  `json:"comparator"                     jsonschema:"description: Threshold operator: gt|lt|gte|lte|eq|neq."`
	Threshold          float64 `json:"threshold"                      jsonschema:"description: Numeric threshold the metric is compared against."`
	SustainedMinutes   int     `json:"sustained_minutes,omitempty"    jsonschema:"description: How long the breach must persist before firing (1-1440). Defaults to the metric family's recommended value."`
	RecoveryMinutes    int     `json:"recovery_minutes,omitempty"     jsonschema:"description: How long the metric must stay healthy before flipping back to OK. Default 0 (instant recovery)."`
	EvaluationWindowM  int     `json:"evaluation_window_m,omitempty"  jsonschema:"description: Aggregation window in minutes for each evaluation. Defaults to sustained_minutes."`
	EvalCadenceSeconds int     `json:"eval_cadence_seconds,omitempty" jsonschema:"description: How often to re-evaluate, in seconds. Default 300."`

	NotifyChannels  []scrapfly.AlertNotifyChannel `json:"notify_channels" jsonschema:"description: List of delivery targets. Each has kind (email|webhook|inapp), target (address|url|empty), and optional opts (e.g. webhook headers)."`
	RenotifyMinutes int                           `json:"renotify_minutes,omitempty" jsonschema:"description: Re-notification cadence while breach is active. Default 60."`
	NoDataPolicy    string                        `json:"no_data_policy,omitempty"   jsonschema:"description: What to do when the evaluation window has no rows: ok|triggered|ignore. Default ignore."`
}

type AlertUpdateInput struct {
	alertConfirm

	AlertUUID string `json:"alert_uuid" jsonschema:"description: ULID of the alert to patch."`

	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`

	Comparator        *string  `json:"comparator,omitempty"`
	Threshold         *float64 `json:"threshold,omitempty"`
	SustainedMinutes  *int     `json:"sustained_minutes,omitempty"`
	RecoveryMinutes   *int     `json:"recovery_minutes,omitempty"`
	EvaluationWindowM *int     `json:"evaluation_window_m,omitempty"`

	NotifyChannels  []scrapfly.AlertNotifyChannel `json:"notify_channels,omitempty"`
	RenotifyMinutes *int                          `json:"renotify_minutes,omitempty"`
	NoDataPolicy    *string                       `json:"no_data_policy,omitempty"`
}

type AlertDeleteInput struct {
	alertConfirm
	AlertUUID string `json:"alert_uuid" jsonschema:"description: ULID of the alert to delete. Cannot be undone."`
}

type AlertSnoozeInput struct {
	alertConfirm
	AlertUUID     string `json:"alert_uuid"               jsonschema:"description: ULID of the alert to snooze."`
	Minutes       int    `json:"minutes,omitempty"        jsonschema:"description: Mute for this many minutes. Mutually exclusive with until_resolved."`
	UntilResolved bool   `json:"until_resolved,omitempty" jsonschema:"description: Mute until the next OK transition. Mutually exclusive with minutes."`
}

type AlertUnsnoozeInput struct {
	alertConfirm
	AlertUUID string `json:"alert_uuid" jsonschema:"description: ULID of the alert to unsnooze."`
}

type AlertTestInput struct {
	alertConfirm
	AlertUUID string `json:"alert_uuid" jsonschema:"description: ULID of the alert. Fires a synthetic notification on every configured channel without touching alert state."`
}

type AlertPreviewInput struct {
	MetricID          string          `json:"metric_id"`
	MetricDimensions  json.RawMessage `json:"metric_dimensions,omitempty"`
	ProjectUUID       string          `json:"project_uuid,omitempty"`
	Comparator        string          `json:"comparator"`
	Threshold         float64         `json:"threshold"`
	SustainedMinutes  int             `json:"sustained_minutes,omitempty"`
	EvaluationWindowM int             `json:"evaluation_window_m,omitempty"`
	RangeMinutes      int             `json:"range_minutes,omitempty" jsonschema:"description: Lookback range in minutes. Default 1440 (24h)."`
	NoDataPolicy      string          `json:"no_data_policy,omitempty"`
}

func (p *ScrapflyToolProvider) AlertList(ctx context.Context, _ *mcp.CallToolRequest, in AlertListInput) (*mcp.CallToolResult, any, error) {
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.ListAlerts(scrapfly.AlertListOptions{
		ProjectUUID: in.ProjectUUID,
		State:       scrapfly.AlertState(in.State),
		MetricID:    in.MetricID,
	})
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertGet(ctx context.Context, _ *mcp.CallToolRequest, in AlertGetInput) (*mcp.CallToolResult, any, error) {
	if in.AlertUUID == "" {
		return alertErr(fmt.Errorf("alert_uuid is required")), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.GetAlert(in.AlertUUID)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertCountActive(ctx context.Context, _ *mcp.CallToolRequest, in AlertCountActiveInput) (*mcp.CallToolResult, any, error) {
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.CountActiveAlerts(in.ProjectUUID)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertMetricFamilies(ctx context.Context, _ *mcp.CallToolRequest, _ AlertMetricFamiliesInput) (*mcp.CallToolResult, any, error) {
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.ListAlertMetricFamilies()
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertSeries(ctx context.Context, _ *mcp.CallToolRequest, in AlertSeriesInput) (*mcp.CallToolResult, any, error) {
	if in.AlertUUID == "" {
		return alertErr(fmt.Errorf("alert_uuid is required")), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.GetAlertSeries(in.AlertUUID, in.RangeMinutes)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertPreview(ctx context.Context, _ *mcp.CallToolRequest, in AlertPreviewInput) (*mcp.CallToolResult, any, error) {
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.PreviewAlert(scrapfly.AlertPreviewRequest{
		MetricID:          in.MetricID,
		MetricDimensions:  in.MetricDimensions,
		ProjectUUID:       in.ProjectUUID,
		Comparator:        scrapfly.AlertComparator(in.Comparator),
		Threshold:         in.Threshold,
		SustainedMinutes:  in.SustainedMinutes,
		EvaluationWindowM: in.EvaluationWindowM,
		RangeMinutes:      in.RangeMinutes,
		NoDataPolicy:      scrapfly.AlertNoDataPolicy(in.NoDataPolicy),
	})
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertCreate(ctx context.Context, _ *mcp.CallToolRequest, in AlertCreateInput) (*mcp.CallToolResult, any, error) {
	req := scrapfly.AlertCreateRequest{
		Name:               in.Name,
		Description:        in.Description,
		ProjectUUID:        in.ProjectUUID,
		MetricID:           in.MetricID,
		MetricDimensions:   in.MetricDimensions,
		Comparator:         scrapfly.AlertComparator(in.Comparator),
		Threshold:          in.Threshold,
		SustainedMinutes:   in.SustainedMinutes,
		RecoveryMinutes:    in.RecoveryMinutes,
		EvaluationWindowM:  in.EvaluationWindowM,
		EvalCadenceSeconds: in.EvalCadenceSeconds,
		NotifyChannels:     in.NotifyChannels,
		RenotifyMinutes:    in.RenotifyMinutes,
		NoDataPolicy:       scrapfly.AlertNoDataPolicy(in.NoDataPolicy),
	}
	if !in.Confirm {
		return alertDryRun("POST /alert", req), nil, nil
	}
	if err := scrapfly.ValidateAlertCreate(req); err != nil {
		return alertErr(err), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.CreateAlert(req)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertUpdate(ctx context.Context, _ *mcp.CallToolRequest, in AlertUpdateInput) (*mcp.CallToolResult, any, error) {
	if in.AlertUUID == "" {
		return alertErr(fmt.Errorf("alert_uuid is required")), nil, nil
	}
	patch := scrapfly.AlertUpdateRequest{
		Name:              in.Name,
		Description:       in.Description,
		Enabled:           in.Enabled,
		Threshold:         in.Threshold,
		SustainedMinutes:  in.SustainedMinutes,
		RecoveryMinutes:   in.RecoveryMinutes,
		EvaluationWindowM: in.EvaluationWindowM,
		NotifyChannels:    in.NotifyChannels,
		RenotifyMinutes:   in.RenotifyMinutes,
	}
	if in.Comparator != nil {
		v := scrapfly.AlertComparator(*in.Comparator)
		patch.Comparator = &v
	}
	if in.NoDataPolicy != nil {
		v := scrapfly.AlertNoDataPolicy(*in.NoDataPolicy)
		patch.NoDataPolicy = &v
	}
	if !in.Confirm {
		return alertDryRun("PUT /alert/"+in.AlertUUID, patch), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.UpdateAlert(in.AlertUUID, patch)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertDelete(ctx context.Context, _ *mcp.CallToolRequest, in AlertDeleteInput) (*mcp.CallToolResult, any, error) {
	if in.AlertUUID == "" {
		return alertErr(fmt.Errorf("alert_uuid is required")), nil, nil
	}
	if !in.Confirm {
		return alertDryRun("DELETE /alert/"+in.AlertUUID, map[string]string{"alert_uuid": in.AlertUUID}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.DeleteAlert(in.AlertUUID)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertSnooze(ctx context.Context, _ *mcp.CallToolRequest, in AlertSnoozeInput) (*mcp.CallToolResult, any, error) {
	if in.AlertUUID == "" {
		return alertErr(fmt.Errorf("alert_uuid is required")), nil, nil
	}
	if in.Minutes <= 0 && !in.UntilResolved {
		return alertErr(fmt.Errorf("snooze requires minutes>0 or until_resolved=true")), nil, nil
	}
	req := scrapfly.AlertSnoozeRequest{Minutes: in.Minutes, UntilResolved: in.UntilResolved}
	if !in.Confirm {
		return alertDryRun("POST /alert/"+in.AlertUUID+"/snooze", req), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.SnoozeAlert(in.AlertUUID, req)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertUnsnooze(ctx context.Context, _ *mcp.CallToolRequest, in AlertUnsnoozeInput) (*mcp.CallToolResult, any, error) {
	if in.AlertUUID == "" {
		return alertErr(fmt.Errorf("alert_uuid is required")), nil, nil
	}
	if !in.Confirm {
		return alertDryRun("POST /alert/"+in.AlertUUID+"/unsnooze", map[string]string{"alert_uuid": in.AlertUUID}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.UnsnoozeAlert(in.AlertUUID)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}

func (p *ScrapflyToolProvider) AlertTest(ctx context.Context, _ *mcp.CallToolRequest, in AlertTestInput) (*mcp.CallToolResult, any, error) {
	if in.AlertUUID == "" {
		return alertErr(fmt.Errorf("alert_uuid is required")), nil, nil
	}
	if !in.Confirm {
		return alertDryRun("POST /alert/"+in.AlertUUID+"/test", map[string]string{"alert_uuid": in.AlertUUID}), nil, nil
	}
	c, err := p.ClientGetter(p, ctx)
	if err != nil {
		return alertErr(err), nil, nil
	}
	out, err := c.TestAlert(in.AlertUUID)
	if err != nil {
		return alertErr(err), nil, nil
	}
	return alertJSON(out), nil, nil
}
