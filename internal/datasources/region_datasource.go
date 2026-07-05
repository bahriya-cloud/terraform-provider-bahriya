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
	_ datasource.DataSource              = &regionDataSource{}
	_ datasource.DataSourceWithConfigure = &regionDataSource{}
)

func NewRegionDataSource() datasource.DataSource {
	return &regionDataSource{}
}

type regionDataSource struct {
	client *client.Client
}

type regionModel struct {
	ID          types.String  `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Description types.String  `tfsdk:"description"`
	Class       types.String  `tfsdk:"class"`
	Status      types.String  `tfsdk:"status"`
	City        types.String  `tfsdk:"city"`
	State       types.String  `tfsdk:"state"`
	Country     types.String  `tfsdk:"country"`
	Latitude    types.Float64 `tfsdk:"latitude"`
	Longitude   types.Float64 `tfsdk:"longitude"`
}

func (d *regionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_region"
}

func (d *regionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details for a single Bahriya region by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Region identifier (e.g. \"helsinki-1\").",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Region display name.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Human-readable region description.",
				Computed:    true,
			},
			"class": schema.StringAttribute{
				Description: "Region class (e.g. \"standard\").",
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "Region status (e.g. \"active\").",
				Computed:    true,
			},
			"city": schema.StringAttribute{
				Description: "City where the region is located.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "State or province of the region.",
				Computed:    true,
			},
			"country": schema.StringAttribute{
				Description: "Country of the region.",
				Computed:    true,
			},
			"latitude": schema.Float64Attribute{
				Description: "Geographic latitude.",
				Computed:    true,
			},
			"longitude": schema.Float64Attribute{
				Description: "Geographic longitude.",
				Computed:    true,
			},
		},
	}
}

func (d *regionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *regionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config regionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	regionID := config.ID.ValueString()
	data, status, err := d.client.Do(ctx, http.MethodGet, fmt.Sprintf("/regions/%s", regionID), nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch region", fmt.Sprintf("HTTP %d: %v", status, err))
		return
	}

	raw, err := unwrapRegion(data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to decode region response", err.Error())
		return
	}

	state := rawToRegionModel(raw)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func unwrapRegion(data json.RawMessage) (map[string]any, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if regions, ok := raw["regions"].([]any); ok && len(regions) > 0 {
		if obj, ok := regions[0].(map[string]any); ok {
			return obj, nil
		}
	}
	return raw, nil
}

func rawToRegionModel(raw map[string]any) regionModel {
	m := regionModel{
		ID:          stringFromRaw(raw, "id"),
		Name:        stringFromRaw(raw, "name"),
		Description: stringFromRaw(raw, "description"),
		Class:       stringFromRaw(raw, "class"),
		Status:      stringFromRaw(raw, "status"),
	}

	if loc, ok := raw["location"].(map[string]any); ok {
		m.City = stringFromRaw(loc, "city")
		m.State = stringFromRaw(loc, "state")
		m.Country = stringFromRaw(loc, "country")
		if v, ok := loc["latitude"].(float64); ok {
			m.Latitude = types.Float64Value(v)
		} else {
			m.Latitude = types.Float64Null()
		}
		if v, ok := loc["longitude"].(float64); ok {
			m.Longitude = types.Float64Value(v)
		} else {
			m.Longitude = types.Float64Null()
		}
	}
	return m
}
