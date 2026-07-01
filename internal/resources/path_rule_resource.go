package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

// path_rule is hand-written rather than generated. The generator templates
// assume a single-parent /organisations/{org}/{kind} URL pattern; path rules
// live two levels deep at /organisations/{org}/containers/{container}/pathrules,
// scoped to a container the user manages separately. Hand-writing this is
// simpler than extending the generator for one resource — the schema follows
// the same conventions as the generated resources so users see no difference.

var (
	_ resource.Resource                = &pathRuleResource{}
	_ resource.ResourceWithConfigure   = &pathRuleResource{}
	_ resource.ResourceWithImportState = &pathRuleResource{}
)

func NewPathRuleResource() resource.Resource {
	return &pathRuleResource{}
}

type pathRuleResource struct {
	client *client.Client
}

type pathRuleModel struct {
	ID          types.String `tfsdk:"id"`
	ContainerID types.String `tfsdk:"container_id"`

	Handle   types.String `tfsdk:"handle"`
	Path     types.String `tfsdk:"path"`
	Priority types.Int64  `tfsdk:"priority"`

	Ratelimitingenabled           types.Bool  `tfsdk:"ratelimitingenabled"`
	Ratelimitingrequestspersecond types.Int64 `tfsdk:"ratelimitingrequestspersecond"`
	Ratelimitingrequestsperminute types.Int64 `tfsdk:"ratelimitingrequestsperminute"`
	Ratelimitingrequestsperhour   types.Int64 `tfsdk:"ratelimitingrequestsperhour"`

	Ipwhitelistenabled types.Bool `tfsdk:"ipwhitelistenabled"`
	Ipwhitelist        types.List `tfsdk:"ipwhitelist"`

	Ipblacklistenabled types.Bool `tfsdk:"ipblacklistenabled"`
	Ipblacklist        types.List `tfsdk:"ipblacklist"`

	Basicauthenabled     types.Bool `tfsdk:"basicauthenabled"`
	Basicauthcredentials types.List `tfsdk:"basicauthcredentials"`
}

func (r *pathRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_path_rule"
}

func (r *pathRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Path-scoped HTTP control plugin on a Bahriya container. Apply different basic auth, rate limit, IP allow/deny rules per URL prefix.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the path rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "ID (UUID) of the HTTP container this rule attaches to. Changing this replaces the rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"handle": schema.StringAttribute{
				Required:    true,
				Description: "Handle for the rule. DNS-1123 compliant (lowercase alphanumeric + hyphens). Unique among active rules on the container. Changing it replaces the rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "URL path prefix. Must start with /. Longest matching prefix wins per request.",
			},
			"priority": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Tiebreaker when two rules share equal path-prefix length. Higher values win. Defaults to 0.",
			},

			"ratelimitingenabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable per-IP rate limiting on this path.",
			},
			"ratelimitingrequestspersecond": schema.Int64Attribute{
				Optional:    true,
				Description: "Max requests per second per IP (requires ratelimitingenabled).",
			},
			"ratelimitingrequestsperminute": schema.Int64Attribute{
				Optional:    true,
				Description: "Max requests per minute per IP (requires ratelimitingenabled).",
			},
			"ratelimitingrequestsperhour": schema.Int64Attribute{
				Optional:    true,
				Description: "Max requests per hour per IP (requires ratelimitingenabled).",
			},

			"ipwhitelistenabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable IP allow-list. Only IPs in the list can access this path.",
			},
			"ipwhitelist": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses or CIDR ranges allowed on this path.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},

			"ipblacklistenabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable IP deny-list. IPs in the list are blocked on this path.",
			},
			"ipblacklist": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses or CIDR ranges blocked on this path.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},

			"basicauthenabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable HTTP basic authentication on this path.",
			},
			"basicauthcredentials": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of {username, password} credential pairs accepted on this path.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"username": schema.StringAttribute{
							Required:    true,
							Description: "Username (alphanumeric plus . _ - allowed; 1-128 chars).",
						},
						"password": schema.StringAttribute{
							Required:    true,
							Sensitive:   true,
							Description: "Plaintext password. The API stores it encrypted; reading returns the sentinel \"value-hidden-for-your-own-good\" — TF treats the sentinel as a no-op so re-applying without rotating the password does not drift.",
						},
					},
				},
			},
		},
	}
}

func (r *pathRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *pathRuleResource) basePath(containerID string) string {
	return fmt.Sprintf("/organisations/%s/containers/%s/pathrules", r.client.OrganisationID(), containerID)
}

func (r *pathRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pathRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := planToPathRulePayload(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, status, err := r.client.Do(ctx, http.MethodPost, r.basePath(plan.ContainerID.ValueString()), body)
	if err != nil {
		resp.Diagnostics.AddError("Create failed", err.Error())
		return
	}
	if status != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected status from Create", fmt.Sprintf("HTTP %d: %s", status, string(data)))
		return
	}

	var id string
	if err := json.Unmarshal(data, &id); err != nil {
		resp.Diagnostics.AddError("Decode created id failed", err.Error())
		return
	}

	state, diags := r.fetch(ctx, plan.ContainerID.ValueString(), id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Preserve user-provided credentials so the API's masked-password sentinel
	// doesn't cause spurious drift on the next plan.
	state.Basicauthcredentials = mergeCredentials(plan.Basicauthcredentials, state.Basicauthcredentials)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *pathRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pathRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fresh, diags := r.fetch(ctx, state.ContainerID.ValueString(), state.ID.ValueString())
	if diags.HasError() {
		if diagsContainNotFound(diags) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diags...)
		return
	}
	fresh.Basicauthcredentials = mergeCredentials(state.Basicauthcredentials, fresh.Basicauthcredentials)

	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *pathRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state pathRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := planToPathRulePayload(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, status, err := r.client.Do(ctx, http.MethodPut, r.basePath(state.ContainerID.ValueString())+"/"+state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Update failed", err.Error())
		return
	}
	if status != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected status from Update", fmt.Sprintf("HTTP %d", status))
		return
	}

	fresh, diags := r.fetch(ctx, state.ContainerID.ValueString(), state.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	fresh.Basicauthcredentials = mergeCredentials(plan.Basicauthcredentials, fresh.Basicauthcredentials)
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *pathRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pathRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, status, err := r.client.Do(ctx, http.MethodDelete, r.basePath(state.ContainerID.ValueString())+"/"+state.ID.ValueString(), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete failed", err.Error())
		return
	}
}

// ImportState accepts the composite id "<container_id>:<path_rule_id>" so a
// user can adopt a rule that already exists on a container they manage outside
// Terraform.
func (r *pathRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Path rule import IDs must look like '<container_id>:<path_rule_id>'.",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPath("container_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPath("id"), parts[1])...)
}

func (r *pathRuleResource) fetch(ctx context.Context, containerID, id string) (pathRuleModel, diagnostics) {
	var diags diagnostics
	data, status, err := r.client.Do(ctx, http.MethodGet, r.basePath(containerID)+"/"+id, nil)
	if err != nil {
		if status == http.StatusNotFound {
			diags.AddError("not_found", "path rule was deleted out-of-band")
			return pathRuleModel{}, diags
		}
		diags.AddError("Fetch failed", err.Error())
		return pathRuleModel{}, diags
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		diags.AddError("Decode fetch response failed", err.Error())
		return pathRuleModel{}, diags
	}
	return apiToPathRuleModel(raw, containerID), diags
}

func planToPathRulePayload(_ context.Context, m *pathRuleModel) (map[string]any, diagnostics) {
	var diags diagnostics
	out := map[string]any{
		"handle":   m.Handle.ValueString(),
		"path":     m.Path.ValueString(),
		"priority": int(m.Priority.ValueInt64()),

		"ratelimitingenabled":           m.Ratelimitingenabled.ValueBool(),
		"ratelimitingrequestspersecond": int64Ptr(m.Ratelimitingrequestspersecond),
		"ratelimitingrequestsperminute": int64Ptr(m.Ratelimitingrequestsperminute),
		"ratelimitingrequestsperhour":   int64Ptr(m.Ratelimitingrequestsperhour),

		"ipwhitelistenabled": m.Ipwhitelistenabled.ValueBool(),
		"ipwhitelist":        stringListOrNil(m.Ipwhitelist),

		"ipblacklistenabled": m.Ipblacklistenabled.ValueBool(),
		"ipblacklist":        stringListOrNil(m.Ipblacklist),

		"basicauthenabled":     m.Basicauthenabled.ValueBool(),
		"basicauthcredentials": credentialsToAPI(m.Basicauthcredentials),
	}
	return out, diags
}

func apiToPathRuleModel(raw map[string]any, containerID string) pathRuleModel {
	m := pathRuleModel{
		ID:          stringFromAPI(raw, "id"),
		ContainerID: types.StringValue(containerID),
		Handle:      stringFromAPI(raw, "handle"),
		Path:        stringFromAPI(raw, "path"),
		Priority:    int64FromAPI(raw, "priority"),

		Ratelimitingenabled:           boolFromAPI(raw, "ratelimitingenabled"),
		Ratelimitingrequestspersecond: int64NullableFromAPI(raw, "ratelimitingrequestspersecond"),
		Ratelimitingrequestsperminute: int64NullableFromAPI(raw, "ratelimitingrequestsperminute"),
		Ratelimitingrequestsperhour:   int64NullableFromAPI(raw, "ratelimitingrequestsperhour"),

		Ipwhitelistenabled: boolFromAPI(raw, "ipwhitelistenabled"),
		Ipwhitelist:        stringListFromAPI(raw, "ipwhitelist"),

		Ipblacklistenabled: boolFromAPI(raw, "ipblacklistenabled"),
		Ipblacklist:        stringListFromAPI(raw, "ipblacklist"),

		Basicauthenabled:     boolFromAPI(raw, "basicauthenabled"),
		Basicauthcredentials: credentialsFromAPI(raw, "basicauthcredentials"),
	}
	return m
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

// API password sentinel returned by the read endpoint. When the plan carries
// real credentials and the API echoes back the sentinel, we treat the plan as
// authoritative so re-applying without password change doesn't drift.
const passwordSentinel = "value-hidden-for-your-own-good"

func pathRuleCredentialAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"username": types.StringType,
		"password": types.StringType,
	}
}

func pathRuleCredentialObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: pathRuleCredentialAttrTypes()}
}

func stringFromAPI(raw map[string]any, key string) types.String {
	if v, ok := raw[key].(string); ok {
		return types.StringValue(v)
	}
	return types.StringNull()
}

func int64FromAPI(raw map[string]any, key string) types.Int64 {
	if v, ok := raw[key].(float64); ok {
		return types.Int64Value(int64(v))
	}
	return types.Int64Null()
}

// int64NullableFromAPI is the same as int64FromAPI today; named separately
// to flag that the API explicitly emits null for these (rate limit windows).
func int64NullableFromAPI(raw map[string]any, key string) types.Int64 {
	return int64FromAPI(raw, key)
}

func boolFromAPI(raw map[string]any, key string) types.Bool {
	if v, ok := raw[key].(bool); ok {
		return types.BoolValue(v)
	}
	return types.BoolNull()
}

func stringListFromAPI(raw map[string]any, key string) types.List {
	items, ok := raw[key].([]any)
	if !ok {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			elems = append(elems, types.StringValue(s))
		}
	}
	listVal, _ := types.ListValue(types.StringType, elems)
	return listVal
}

func credentialsFromAPI(raw map[string]any, key string) types.List {
	items, ok := raw[key].([]any)
	if !ok {
		return types.ListNull(pathRuleCredentialObjectType())
	}
	elems := make([]attr.Value, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		username, _ := obj["username"].(string)
		password, _ := obj["password"].(string)
		objVal, _ := types.ObjectValue(pathRuleCredentialAttrTypes(), map[string]attr.Value{
			"username": types.StringValue(username),
			"password": types.StringValue(password),
		})
		elems = append(elems, objVal)
	}
	listVal, _ := types.ListValue(pathRuleCredentialObjectType(), elems)
	return listVal
}

func credentialsToAPI(list types.List) []map[string]any {
	if list.IsNull() || list.IsUnknown() {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(list.Elements()))
	for _, el := range list.Elements() {
		obj, ok := el.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		username, _ := attrs["username"].(types.String)
		password, _ := attrs["password"].(types.String)
		out = append(out, map[string]any{
			"username": username.ValueString(),
			"password": password.ValueString(),
		})
	}
	return out
}

func stringListOrNil(list types.List) any {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	out := make([]string, 0, len(list.Elements()))
	for _, el := range list.Elements() {
		if s, ok := el.(types.String); ok && !s.IsNull() && !s.IsUnknown() {
			out = append(out, s.ValueString())
		}
	}
	return out
}

func int64Ptr(v types.Int64) any {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueInt64()
}

// mergeCredentials returns the credential list to commit to state. When the
// API returns the password sentinel and the plan carries the real password,
// we use the plan's password to avoid spurious drift. Falls back to the API
// value when the plan list is null/unknown (e.g. import).
func mergeCredentials(plan, fromAPI types.List) types.List {
	if plan.IsNull() || plan.IsUnknown() {
		return fromAPI
	}
	planElems := plan.Elements()
	apiElems := fromAPI.Elements()
	merged := make([]attr.Value, 0, len(apiElems))
	for i, el := range apiElems {
		apiObj, ok := el.(types.Object)
		if !ok {
			merged = append(merged, el)
			continue
		}
		apiAttrs := apiObj.Attributes()
		apiUsername, _ := apiAttrs["username"].(types.String)
		apiPassword, _ := apiAttrs["password"].(types.String)

		password := apiPassword
		if apiPassword.ValueString() == passwordSentinel && i < len(planElems) {
			if planObj, ok := planElems[i].(types.Object); ok {
				if pwd, ok := planObj.Attributes()["password"].(types.String); ok && !pwd.IsNull() && !pwd.IsUnknown() {
					password = pwd
				}
			}
		}

		objVal, _ := types.ObjectValue(pathRuleCredentialAttrTypes(), map[string]attr.Value{
			"username": apiUsername,
			"password": password,
		})
		merged = append(merged, objVal)
	}
	listVal, _ := types.ListValue(pathRuleCredentialObjectType(), merged)
	return listVal
}
