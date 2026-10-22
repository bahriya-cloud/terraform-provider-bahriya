package datasources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

var (
	_ datasource.DataSource              = &tlsBundleDataSource{}
	_ datasource.DataSourceWithConfigure = &tlsBundleDataSource{}
)

// NewTlsBundleDataSource reads a vault TLS bundle by handle — most usefully the
// platform-managed bundle of a MySQL or Valkey instance (`<handle>-tls`), whose
// CA certificate clients need to verify the server (ADR 2026-10-18). The
// private key is never returned by the API, so it is not an attribute.
func NewTlsBundleDataSource() datasource.DataSource {
	return &tlsBundleDataSource{}
}

type tlsBundleDataSource struct {
	client *client.Client
}

type tlsBundleModel struct {
	Handle      types.String `tfsdk:"handle"`
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	CA          types.String `tfsdk:"ca"`
	Cert        types.String `tfsdk:"cert"`
	Subject     types.String `tfsdk:"subject"`
	Issuer      types.String `tfsdk:"issuer"`
	Fingerprint types.String `tfsdk:"fingerprint"`
	NotAfter    types.String `tfsdk:"notafter"`
	Managed     types.Bool   `tfsdk:"managed"`
}

func (d *tlsBundleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_bundle"
}

func (d *tlsBundleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a TLS bundle from the organisation's vault by handle, including its CA certificate and certificate. Use it to give clients the CA of a MySQL or Valkey instance's platform-managed certificate (handle `<instance handle>-tls`). The private key is never returned.",
		Attributes: map[string]schema.Attribute{
			"handle": schema.StringAttribute{
				Description: "Handle of the TLS bundle.",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "TLS bundle UUID.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "TLS bundle display name.",
				Computed:    true,
			},
			"ca": schema.StringAttribute{
				Description: "CA certificate (PEM). Clients verifying the server trust this.",
				Computed:    true,
			},
			"cert": schema.StringAttribute{
				Description: "Certificate (PEM).",
				Computed:    true,
			},
			"subject": schema.StringAttribute{
				Description: "Certificate subject.",
				Computed:    true,
			},
			"issuer": schema.StringAttribute{
				Description: "Certificate issuer.",
				Computed:    true,
			},
			"fingerprint": schema.StringAttribute{
				Description: "Certificate fingerprint.",
				Computed:    true,
			},
			"notafter": schema.StringAttribute{
				Description: "Certificate expiry time.",
				Computed:    true,
			},
			"managed": schema.BoolAttribute{
				Description: "Whether the platform manages the bundle on behalf of a resource (for example a MySQL or Valkey instance).",
				Computed:    true,
			},
		},
	}
}

func (d *tlsBundleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *tlsBundleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config tlsBundleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	handle := config.Handle.ValueString()
	orgPath := "/organisations/" + url.PathEscape(d.client.OrganisationID())

	// The API resolves bundles by UUID only: find the handle in the list first.
	list, status, err := d.client.Do(ctx, http.MethodGet, orgPath+"/tls_bundles?limit=500", nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list TLS bundles", fmt.Sprintf("HTTP %d: %v", status, err))
		return
	}
	id, err := tlsBundleIDForHandle(list, handle)
	if err != nil {
		resp.Diagnostics.AddError("TLS bundle not found", err.Error())
		return
	}

	detail, status, err := d.client.Do(ctx, http.MethodGet, orgPath+"/tls_bundles/"+url.PathEscape(id), nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch TLS bundle", fmt.Sprintf("HTTP %d: %v", status, err))
		return
	}
	state, err := tlsBundleStateFromDetail(detail, handle)
	if err != nil {
		resp.Diagnostics.AddError("Failed to decode TLS bundle", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// tlsBundleIDForHandle picks the bundle with the given handle out of a
// GET /tls_bundles response ({"tlsbundles": [...]}).
func tlsBundleIDForHandle(list json.RawMessage, handle string) (string, error) {
	var envelope map[string]any
	if err := json.Unmarshal(list, &envelope); err != nil {
		return "", fmt.Errorf("decode TLS bundle list: %w", err)
	}
	bundles, _ := envelope["tlsbundles"].([]any)
	for _, b := range bundles {
		obj, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if h, _ := obj["handle"].(string); h == handle {
			if id, _ := obj["id"].(string); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("no TLS bundle with handle %q in this organisation", handle)
}

// tlsBundleStateFromDetail maps a GET /tls_bundles/{id} response to state.
func tlsBundleStateFromDetail(detail json.RawMessage, handle string) (tlsBundleModel, error) {
	var raw map[string]any
	if err := json.Unmarshal(detail, &raw); err != nil {
		return tlsBundleModel{}, err
	}

	managedID, _ := raw["managedbyresourceid"].(string)
	return tlsBundleModel{
		Handle:      types.StringValue(handle),
		ID:          stringFromRaw(raw, "id"),
		Name:        stringFromRaw(raw, "name"),
		CA:          stringFromRaw(raw, "ca"),
		Cert:        stringFromRaw(raw, "cert"),
		Subject:     stringFromRaw(raw, "subject"),
		Issuer:      stringFromRaw(raw, "issuer"),
		Fingerprint: stringFromRaw(raw, "fingerprint"),
		NotAfter:    dateFromRaw(raw, "notafter"),
		Managed:     types.BoolValue(managedID != ""),
	}, nil
}

// dateFromRaw reads a timestamp the API serialises either as a string or as a
// PHP DateTime object ({"date": "...", "timezone": "..."}).
func dateFromRaw(raw map[string]any, key string) types.String {
	switch v := raw[key].(type) {
	case string:
		return types.StringValue(v)
	case map[string]any:
		if date, ok := v["date"].(string); ok {
			return types.StringValue(date)
		}
	}
	return types.StringNull()
}
