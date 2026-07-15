package datasources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

var (
	_ datasource.DataSource              = &organisationDataSource{}
	_ datasource.DataSourceWithConfigure = &organisationDataSource{}
)

func NewOrganisationDataSource() datasource.DataSource {
	return &organisationDataSource{}
}

type organisationDataSource struct {
	client *client.Client
}

type organisationModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Handle types.String `tfsdk:"handle"`
	Email  types.String `tfsdk:"email"`
	Role   types.String `tfsdk:"role"`
}

func (d *organisationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organisation"
}

func (d *organisationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the current organisation configured in the provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Organisation UUID.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Organisation display name.",
				Computed:    true,
			},
			"handle": schema.StringAttribute{
				Description: "Organisation handle (permanent, unique identifier).",
				Computed:    true,
			},
			"email": schema.StringAttribute{
				Description: "Organisation contact email.",
				Computed:    true,
			},
			"role": schema.StringAttribute{
				Description: "Authenticated user's role in this organisation.",
				Computed:    true,
			},
		},
	}
}

func (d *organisationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = c
}

func (d *organisationDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	orgID := d.client.OrganisationID()

	// There is no per-organisation show route; the API only exposes the list
	// endpoint (GET /organisations returns the orgs the caller can access).
	// Fetch the list and pick the one matching the configured organisation id.
	data, status, err := d.client.Do(ctx, http.MethodGet, "/organisations", nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch organisations", fmt.Sprintf("HTTP %d: %v", status, err))
		return
	}

	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		resp.Diagnostics.AddError("Failed to decode organisation response", err.Error())
		return
	}

	var raw map[string]any
	if orgs, ok := envelope["organisations"].([]any); ok {
		for _, o := range orgs {
			obj, ok := o.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := obj["id"].(string); id == orgID {
				raw = obj
				break
			}
		}
	}
	if raw == nil {
		resp.Diagnostics.AddError(
			"Organisation not found",
			fmt.Sprintf("Configured organisation %q was not present in the accessible organisation list.", orgID),
		)
		return
	}

	state := organisationModel{
		ID:     stringFromRaw(raw, "id"),
		Name:   stringFromRaw(raw, "name"),
		Handle: stringFromRaw(raw, "handle"),
		Email:  stringFromRaw(raw, "email"),
		Role:   stringFromRaw(raw, "role"),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func stringFromRaw(raw map[string]any, key string) types.String {
	if v, ok := raw[key].(string); ok {
		return types.StringValue(v)
	}
	return types.StringNull()
}
