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
	_ datasource.DataSource              = &projectQuotaDataSource{}
	_ datasource.DataSourceWithConfigure = &projectQuotaDataSource{}
)

func NewProjectQuotaDataSource() datasource.DataSource {
	return &projectQuotaDataSource{}
}

type projectQuotaDataSource struct {
	client *client.Client
}

type projectQuotaModel struct {
	Project types.String              `tfsdk:"project"`
	Handle  types.String              `tfsdk:"handle"`
	Regions []projectQuotaRegionModel `tfsdk:"regions"`
}

type projectQuotaRegionModel struct {
	Region    types.String            `tfsdk:"region"`
	Used      projectQuotaAmountModel `tfsdk:"used"`
	Peak      projectQuotaAmountModel `tfsdk:"peak"`
	Available projectQuotaAmountModel `tfsdk:"available"`
}

type projectQuotaAmountModel struct {
	ReservedCPU    types.String `tfsdk:"reserved_cpu"`
	ReservedMemory types.String `tfsdk:"reserved_memory"`
	CPUCeiling     types.String `tfsdk:"cpu_ceiling"`
	MemoryCeiling  types.String `tfsdk:"memory_ceiling"`
}

// The wire shape, which uses the API's smushed names.
type projectQuotaWire struct {
	Project string `json:"project"`
	Handle  string `json:"handle"`
	Regions []struct {
		Region    string                 `json:"region"`
		Used      projectQuotaAmountWire `json:"used"`
		Peak      projectQuotaAmountWire `json:"peak"`
		Available projectQuotaAmountWire `json:"available"`
	} `json:"regions"`
}

type projectQuotaAmountWire struct {
	ReservedCPU    string `json:"reservedcpu"`
	ReservedMemory string `json:"reservedmemory"`
	CPUCeiling     string `json:"cpuceiling"`
	MemoryCeiling  string `json:"memoryceiling"`
}

func (d *projectQuotaDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_quota"
}

func amountAttributes(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: description,
		Computed:    true,
		Attributes: map[string]schema.Attribute{
			"reserved_cpu": schema.StringAttribute{
				Description: "CPU guaranteed to the workloads, e.g. \"1500m\".",
				Computed:    true,
			},
			"reserved_memory": schema.StringAttribute{
				Description: "Memory guaranteed to the workloads, e.g. \"2Gi\".",
				Computed:    true,
			},
			"cpu_ceiling": schema.StringAttribute{
				Description: "The most CPU the workloads may use, e.g. \"8000m\".",
				Computed:    true,
			},
			"memory_ceiling": schema.StringAttribute{
				Description: "The most memory the workloads may use, e.g. \"16G\".",
				Computed:    true,
			},
		},
	}
}

func (d *projectQuotaDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a project's resource allowance and how much of it is in use, per region.\n\n" +
			"Every deployment is checked against this before it is created, so reading it first is how " +
			"you size configuration that will be accepted rather than discovering the limit by applying " +
			"and being refused. The allowance is read-only here: raising it is a support request.\n\n" +
			"Figures are reported per region because the allowance is replicated into each region a " +
			"project runs in rather than divided between them — a project can be full in one region and " +
			"nearly empty in another.",
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Description: "Project identifier.",
				Required:    true,
			},
			"handle": schema.StringAttribute{
				Description: "Project handle.",
				Computed:    true,
			},
			"regions": schema.ListNestedAttribute{
				Description: "One entry per region the project runs in, including regions with nothing deployed yet.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"region": schema.StringAttribute{
							Description: "Region identifier.",
							Computed:    true,
						},
						"used":      amountAttributes("Reserved by what is running now."),
						"peak":      amountAttributes("Reserved if every autoscaled workload reached its configured maximum at the same time."),
						"available": amountAttributes("The ceiling. Identical in every region."),
					},
				},
			},
		},
	}
}

func (d *projectQuotaDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *projectQuotaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectQuotaModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := config.Project.ValueString()
	path := fmt.Sprintf("/organisations/%s/projects/%s/quota", d.client.OrganisationID(), projectID)

	data, status, err := d.client.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read project resource allowance", fmt.Sprintf("HTTP %d: %v", status, err))
		return
	}
	if status != http.StatusOK {
		resp.Diagnostics.AddError(
			"Failed to read project resource allowance",
			fmt.Sprintf("HTTP %d: %s", status, string(data)),
		)
		return
	}

	var wire projectQuotaWire
	if err := json.Unmarshal(data, &wire); err != nil {
		resp.Diagnostics.AddError("Decode project resource allowance failed", err.Error())
		return
	}

	state := projectQuotaModel{
		Project: types.StringValue(wire.Project),
		Handle:  types.StringValue(wire.Handle),
	}
	for _, region := range wire.Regions {
		state.Regions = append(state.Regions, projectQuotaRegionModel{
			Region:    types.StringValue(region.Region),
			Used:      toAmountModel(region.Used),
			Peak:      toAmountModel(region.Peak),
			Available: toAmountModel(region.Available),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func toAmountModel(wire projectQuotaAmountWire) projectQuotaAmountModel {
	return projectQuotaAmountModel{
		ReservedCPU:    types.StringValue(wire.ReservedCPU),
		ReservedMemory: types.StringValue(wire.ReservedMemory),
		CPUCeiling:     types.StringValue(wire.CPUCeiling),
		MemoryCeiling:  types.StringValue(wire.MemoryCeiling),
	}
}
