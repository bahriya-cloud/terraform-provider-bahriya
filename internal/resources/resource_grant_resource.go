package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

// bahriya_resource_grant is a user-level instance ACL: it grants one member a
// set of permissions on one specific resource instance (container, memcached,
// or an attachable). Grants are additive — they only widen the member's access,
// never restrict it. The API has no update for a grant, so every attribute
// forces replacement; a permission change revokes and re-creates.

var (
	_ resource.Resource                = &resourceGrantResource{}
	_ resource.ResourceWithConfigure   = &resourceGrantResource{}
	_ resource.ResourceWithImportState = &resourceGrantResource{}
)

func NewResourceGrantResource() resource.Resource {
	return &resourceGrantResource{}
}

type resourceGrantResource struct {
	client *client.Client
}

type resourceGrantModel struct {
	ID           types.String `tfsdk:"id"`
	Touser       types.String `tfsdk:"touser"`
	Resourcetype types.String `tfsdk:"resourcetype"`
	Resourceid   types.String `tfsdk:"resourceid"`
	Permissions  types.Set    `tfsdk:"permissions"`
	GrantIds     types.List   `tfsdk:"grant_ids"`
}

func (r *resourceGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_grant"
}

func (r *resourceGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Shares a specific resource instance with an organisation member (a user-level instance ACL). Additive — only widens the member's access to that one instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Synthetic identifier: '<resourcetype>|<resourceid>|<touser>'.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"touser": schema.StringAttribute{
				Required:    true,
				Description: "ID (UUID) of the member to share with. Must already be a member of the organisation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resourcetype": schema.StringAttribute{
				Required:    true,
				Description: "Resource kind, e.g. deployables_container_http, attachables_registries.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resourceid": schema.StringAttribute{
				Required:    true,
				Description: "ID (UUID) of the specific instance to share.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permissions": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Permissions to grant on the instance: any of create, read, update, delete.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"grant_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The individual grant row IDs backing this share (one per permission).",
			},
		},
	}
}

func (r *resourceGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *resourceGrantResource) basePath() string {
	return fmt.Sprintf("/organisations/%s/resource-grants", r.client.OrganisationID())
}

func (r *resourceGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resourceGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"touser":       plan.Touser.ValueString(),
		"resourcetype": plan.Resourcetype.ValueString(),
		"resourceid":   plan.Resourceid.ValueString(),
		"permissions":  stringSetOrNil(plan.Permissions),
	}

	data, status, err := r.client.Do(ctx, http.MethodPost, r.basePath(), body)
	if err != nil {
		resp.Diagnostics.AddError("Create resource grant failed", err.Error())
		return
	}
	if status != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected status from Create", fmt.Sprintf("HTTP %d: %s", status, string(data)))
		return
	}

	state, diags := r.read(ctx, plan.Touser.ValueString(), plan.Resourcetype.ValueString(), plan.Resourceid.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *resourceGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resourceGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fresh, diags := r.read(ctx, state.Touser.ValueString(), state.Resourcetype.ValueString(), state.Resourceid.ValueString())
	if diags.HasError() {
		if diagsContainNotFound(diags) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

// Update is unreachable — every attribute forces replacement — but the
// interface requires it.
func (r *resourceGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resourceGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *resourceGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resourceGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, el := range state.GrantIds.Elements() {
		s, ok := el.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		_, status, err := r.client.Do(ctx, http.MethodDelete, r.basePath()+"/"+s.ValueString(), nil)
		if err != nil && status != http.StatusNotFound {
			resp.Diagnostics.AddError("Revoke grant failed", err.Error())
			return
		}
	}
}

func (r *resourceGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "|", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Resource grant import IDs must look like '<resourcetype>|<resourceid>|<touser>'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPath("resourcetype"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPath("resourceid"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPath("touser"), parts[2])...)
}

// read lists the grants on an instance and reconstructs the state for one member.
func (r *resourceGrantResource) read(ctx context.Context, touser, resourcetype, resourceid string) (resourceGrantModel, diagnostics) {
	var diags diagnostics
	q := fmt.Sprintf("?resourcetype=%s&resourceid=%s", url.QueryEscape(resourcetype), url.QueryEscape(resourceid))
	data, status, err := r.client.Do(ctx, http.MethodGet, r.basePath()+q, nil)
	if err != nil {
		if status == http.StatusNotFound {
			diags.AddError("not_found", "no grants on this instance")
			return resourceGrantModel{}, diags
		}
		diags.AddError("Read grants failed", err.Error())
		return resourceGrantModel{}, diags
	}

	var envelope struct {
		Grants []map[string]any `json:"grants"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		diags.AddError("Decode grants response failed", err.Error())
		return resourceGrantModel{}, diags
	}

	perms := make([]string, 0)
	ids := make([]string, 0)
	for _, g := range envelope.Grants {
		if u, _ := g["touser"].(string); u != touser {
			continue
		}
		if p, ok := g["permission"].(string); ok {
			perms = append(perms, p)
		}
		if id, ok := g["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		diags.AddError("not_found", "member has no grants on this instance")
		return resourceGrantModel{}, diags
	}
	sort.Strings(perms)
	sort.Strings(ids)

	return resourceGrantModel{
		ID:           types.StringValue(resourcetype + "|" + resourceid + "|" + touser),
		Touser:       types.StringValue(touser),
		Resourcetype: types.StringValue(resourcetype),
		Resourceid:   types.StringValue(resourceid),
		Permissions:  stringSetFromSlice(perms),
		GrantIds:     stringListFromSlice(ids),
	}, diags
}

func stringSetOrNil(set types.Set) any {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	out := make([]string, 0, len(set.Elements()))
	for _, el := range set.Elements() {
		if s, ok := el.(types.String); ok && !s.IsNull() && !s.IsUnknown() {
			out = append(out, s.ValueString())
		}
	}
	return out
}

func stringSetFromSlice(items []string) types.Set {
	elems := make([]attr.Value, 0, len(items))
	for _, s := range items {
		elems = append(elems, types.StringValue(s))
	}
	setVal, _ := types.SetValue(types.StringType, elems)
	return setVal
}

func stringListFromSlice(items []string) types.List {
	elems := make([]attr.Value, 0, len(items))
	for _, s := range items {
		elems = append(elems, types.StringValue(s))
	}
	listVal, _ := types.ListValue(types.StringType, elems)
	return listVal
}
