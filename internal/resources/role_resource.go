package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

// bahriya_role is hand-written: its `permissions` matrix is a repeated nested
// block ({level, resource, permission}) the generator templates don't model.
// It manages CUSTOM roles. System roles (owner/admin/member/viewer) are
// read-only on the API — updating or deleting one returns an error, which
// Terraform surfaces to the user.

var (
	_ resource.Resource                = &roleResource{}
	_ resource.ResourceWithConfigure   = &roleResource{}
	_ resource.ResourceWithImportState = &roleResource{}
)

func NewRoleResource() resource.Resource {
	return &roleResource{}
}

type roleResource struct {
	client *client.Client
}

type roleModel struct {
	ID          types.String `tfsdk:"id"`
	Handle      types.String `tfsdk:"handle"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Issystem    types.Bool   `tfsdk:"issystem"`
	Permissions types.List   `tfsdk:"permissions"`
	Created     types.String `tfsdk:"created"`
	Updated     types.String `tfsdk:"updated"`
}

func (r *roleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *roleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A custom organisation role: a named set of permission grants across resource kinds. System roles (owner/admin/member/viewer) are managed by Bahriya and cannot be created, changed or deleted here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the role.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"handle": schema.StringAttribute{
				Computed:    true,
				Description: "Machine handle (slug), derived from the name by the server and immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable role name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional description of what the role is for.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"issystem": schema.BoolAttribute{
				Computed:    true,
				Description: "True for the built-in system roles (read-only).",
			},
			"permissions": schema.ListNestedAttribute{
				Required:    true,
				Description: "The permission grants that make up the role.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"level": schema.StringAttribute{
							Required:    true,
							Description: "Scope of the grant: organisation or project.",
						},
						"resource": schema.StringAttribute{
							Required:    true,
							Description: "Resource kind, e.g. attachables_registries, deployables_container_http.",
						},
						"permission": schema.StringAttribute{
							Required:    true,
							Description: "One of: create, read, update, delete.",
						},
					},
				},
			},
			"created": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp (ISO 8601).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated": schema.StringAttribute{
				Computed:    true,
				Description: "Last-update timestamp (ISO 8601).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *roleResource) basePath() string {
	return fmt.Sprintf("/organisations/%s/roles", r.client.OrganisationID())
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":        plan.Name.ValueString(),
		"description": stringOrNil(plan.Description),
		"permissions": permissionsToAPI(plan.Permissions),
	}

	data, status, err := r.client.Do(ctx, http.MethodPost, r.basePath(), body)
	if err != nil {
		resp.Diagnostics.AddError("Create role failed", err.Error())
		return
	}
	if status != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected status from Create", fmt.Sprintf("HTTP %d: %s", status, string(data)))
		return
	}

	// The create endpoint returns the full Role, so map it straight into state.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		resp.Diagnostics.AddError("Decode created role failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, apiToRoleModel(raw))...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fresh, diags := r.fetch(ctx, state.ID.ValueString())
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

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":        plan.Name.ValueString(),
		"description": stringOrNil(plan.Description),
		"permissions": permissionsToAPI(plan.Permissions),
	}

	_, status, err := r.client.Do(ctx, http.MethodPut, r.basePath()+"/"+state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Update role failed", err.Error())
		return
	}
	if status != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected status from Update", fmt.Sprintf("HTTP %d", status))
		return
	}

	fresh, diags := r.fetch(ctx, state.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, status, err := r.client.Do(ctx, http.MethodDelete, r.basePath()+"/"+state.ID.ValueString(), nil)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Delete role failed", err.Error())
		return
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPath("id"), req.ID)...)
}

func (r *roleResource) fetch(ctx context.Context, id string) (roleModel, diagnostics) {
	var diags diagnostics
	data, status, err := r.client.Do(ctx, http.MethodGet, r.basePath()+"/"+id, nil)
	if err != nil {
		if status == http.StatusNotFound {
			diags.AddError("not_found", "role was deleted out-of-band")
			return roleModel{}, diags
		}
		diags.AddError("Fetch role failed", err.Error())
		return roleModel{}, diags
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		diags.AddError("Decode role response failed", err.Error())
		return roleModel{}, diags
	}
	return apiToRoleModel(raw), diags
}

func apiToRoleModel(raw map[string]any) roleModel {
	return roleModel{
		ID:          stringFromAPI(raw, "id"),
		Handle:      stringFromAPI(raw, "handle"),
		Name:        stringFromAPI(raw, "name"),
		Description: stringFromAPI(raw, "description"),
		Issystem:    boolFromAPI(raw, "issystem"),
		Permissions: permissionsFromAPI(raw, "permissions"),
		Created:     stringFromAPI(raw, "created"),
		Updated:     stringFromAPI(raw, "updated"),
	}
}

// ---------------------------------------------------------------------------
// permissions nested-block marshalling
// ---------------------------------------------------------------------------

func permissionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"level":      types.StringType,
		"resource":   types.StringType,
		"permission": types.StringType,
	}
}

func permissionObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: permissionAttrTypes()}
}

func permissionsToAPI(list types.List) []map[string]any {
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
		level, _ := attrs["level"].(types.String)
		res, _ := attrs["resource"].(types.String)
		perm, _ := attrs["permission"].(types.String)
		out = append(out, map[string]any{
			"level":      level.ValueString(),
			"resource":   res.ValueString(),
			"permission": perm.ValueString(),
		})
	}
	return out
}

func permissionsFromAPI(raw map[string]any, key string) types.List {
	items, ok := raw[key].([]any)
	if !ok {
		return types.ListNull(permissionObjectType())
	}
	elems := make([]attr.Value, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		level, _ := obj["level"].(string)
		res, _ := obj["resource"].(string)
		perm, _ := obj["permission"].(string)
		objVal, _ := types.ObjectValue(permissionAttrTypes(), map[string]attr.Value{
			"level":      types.StringValue(level),
			"resource":   types.StringValue(res),
			"permission": types.StringValue(perm),
		})
		elems = append(elems, objVal)
	}
	listVal, _ := types.ListValue(permissionObjectType(), elems)
	return listVal
}

func stringOrNil(v types.String) any {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueString()
}
