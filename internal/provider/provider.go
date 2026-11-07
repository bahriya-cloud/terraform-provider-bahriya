package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/datasources"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/resources"
)

const (
	EnvToken          = "BAHRIYA_TOKEN"
	EnvAPIURL         = "BAHRIYA_API_URL"
	EnvOrganisationID = "BAHRIYA_ORGANISATION_ID"
	DefaultAPIURL     = "https://api.bahriya.cloud/console/v1"
)

type bahriyaProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &bahriyaProvider{version: version}
	}
}

func (p *bahriyaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "bahriya"
	resp.Version = p.version
}

func (p *bahriyaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Bahriya Cloud resources: projects, containers, managed datastores, vault items, configs and network policies.",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Description: "Bahriya personal access token. If not set, the " + EnvToken + " environment variable is used.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Bahriya API base URL. If not set, the " + EnvAPIURL + " environment variable is used, and otherwise " + DefaultAPIURL + ".",
				Optional:    true,
			},
			"organisation_id": schema.StringAttribute{
				Description: "Organisation UUID. If not set, the " + EnvOrganisationID + " environment variable is used.",
				Optional:    true,
			},
		},
	}
}

type providerModel struct {
	Token          types.String `tfsdk:"token"`
	BaseURL        types.String `tfsdk:"base_url"`
	OrganisationID types.String `tfsdk:"organisation_id"`
}

func (p *bahriyaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := firstNonEmpty(cfg.Token.ValueString(), os.Getenv(EnvToken))
	baseURL := firstNonEmpty(cfg.BaseURL.ValueString(), os.Getenv(EnvAPIURL), DefaultAPIURL)
	orgID := firstNonEmpty(cfg.OrganisationID.ValueString(), os.Getenv(EnvOrganisationID))

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing Bahriya token",
			"Set provider attribute `token` or env var "+EnvToken+".",
		)
	}
	if orgID == "" {
		resp.Diagnostics.AddError(
			"Missing organisation_id",
			"Set provider attribute `organisation_id` or env var "+EnvOrganisationID+".",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c := client.New(baseURL, token, orgID)
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *bahriyaProvider) Resources(_ context.Context) []func() resource.Resource {
	return resources.All()
}

func (p *bahriyaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return datasources.All()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
