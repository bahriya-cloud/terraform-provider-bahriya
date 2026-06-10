package datasources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

var (
	_ datasource.DataSource              = &regionsDataSource{}
	_ datasource.DataSourceWithConfigure = &regionsDataSource{}
)

func NewRegionsDataSource() datasource.DataSource {
	return &regionsDataSource{}
}

type regionsDataSource struct {
	client *client.Client
}

type regionsModel struct {
	StatusFilter types.String `tfsdk:"status_filter"`
	Regions      types.List   `tfsdk:"regions"`
}

func (d *regionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_regions"
}

func regionObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":          types.StringType,
		"name":        types.StringType,
		"description": types.StringType,
		"class":       types.StringType,
		"status":      types.StringType,
		"city":        types.StringType,
		"state":       types.StringType,
		"country":     types.StringType,
		"latitude":    types.Float64Type,
		"longitude":   types.Float64Type,
	}
}

func (d *regionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	regionAttrs := map[string]schema.Attribute{
		"id":          schema.StringAttribute{Computed: true, Description: "Region identifier."},
		"name":        schema.StringAttribute{Computed: true, Description: "Region display name."},
		"description": schema.StringAttribute{Computed: true, Description: "Human-readable region description."},
		"class":       schema.StringAttribute{Computed: true, Description: "Region class."},
		"status":      schema.StringAttribute{Computed: true, Description: "Region status."},
		"city":        schema.StringAttribute{Computed: true, Description: "City."},
		"state":       schema.StringAttribute{Computed: true, Description: "State or province."},
		"country":     schema.StringAttribute{Computed: true, Description: "Country."},
		"latitude":    schema.Float64Attribute{Computed: true, Description: "Geographic latitude."},
		"longitude":   schema.Float64Attribute{Computed: true, Description: "Geographic longitude."},
	}

	resp.Schema = schema.Schema{
		Description: "Lists all available Bahriya regions. Optionally filter by status.",
		Attributes: map[string]schema.Attribute{
			"status_filter": schema.StringAttribute{
				Description: "Only return regions with this status (e.g. \"active\"). If omitted, all regions are returned.",
				Optional:    true,
			},
			"regions": schema.ListNestedAttribute{
				Description: "List of regions.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: regionAttrs,
				},
			},
		},
	}
}

func (d *regionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *regionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config regionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, status, err := d.client.Do(ctx, http.MethodGet, "/regions", nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch regions", fmt.Sprintf("HTTP %d: %v", status, err))
		return
	}

	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		resp.Diagnostics.AddError("Failed to decode regions response", err.Error())
		return
	}

	regions, ok := envelope["regions"].([]any)
	if !ok {
		resp.Diagnostics.AddError("Unexpected regions response", "expected regions array in response")
		return
	}

	statusFilter := ""
	if !config.StatusFilter.IsNull() && !config.StatusFilter.IsUnknown() {
		statusFilter = config.StatusFilter.ValueString()
	}

	objType := types.ObjectType{AttrTypes: regionObjectAttrTypes()}
	elements := make([]attr.Value, 0, len(regions))

	for _, item := range regions {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}

		if statusFilter != "" {
			if s, ok := raw["status"].(string); ok && s != statusFilter {
				continue
			}
		}

		loc, _ := raw["location"].(map[string]any)
		if loc == nil {
			loc = map[string]any{}
		}

		attrVals := map[string]attr.Value{
			"id":          stringFromRaw(raw, "id"),
			"name":        stringFromRaw(raw, "name"),
			"description": stringFromRaw(raw, "description"),
			"class":       stringFromRaw(raw, "class"),
			"status":      stringFromRaw(raw, "status"),
			"city":        stringFromRaw(loc, "city"),
			"state":       stringFromRaw(loc, "state"),
			"country":     stringFromRaw(loc, "country"),
		}

		if v, ok := loc["latitude"].(float64); ok {
			attrVals["latitude"] = types.Float64Value(v)
		} else {
			attrVals["latitude"] = types.Float64Null()
		}
		if v, ok := loc["longitude"].(float64); ok {
			attrVals["longitude"] = types.Float64Value(v)
		} else {
			attrVals["longitude"] = types.Float64Null()
		}

		objVal, diags := types.ObjectValue(regionObjectAttrTypes(), attrVals)
		resp.Diagnostics.Append(diags...)
		elements = append(elements, objVal)
	}

	listVal, diags := types.ListValue(objType, elements)
	resp.Diagnostics.Append(diags...)

	state := regionsModel{
		StatusFilter: config.StatusFilter,
		Regions:      listVal,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
