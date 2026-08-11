package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
)

// bahriya_billing_details is hand-written: it is a SINGLETON per organisation
// (GET/PUT /organisations/{org}/billing/details) with no create or delete on
// the API, which the generator's collection-CRUD templates don't model.
// Create and Update are both a full-replace PUT; Delete clears every declared
// identity field (the billing email is never cleared — omitting it leaves it
// unchanged). Attribute names are the API's x-aliases, matching Reis YAML.

var (
	_ resource.Resource                = &billingDetailsResource{}
	_ resource.ResourceWithConfigure   = &billingDetailsResource{}
	_ resource.ResourceWithImportState = &billingDetailsResource{}
)

func NewBillingDetailsResource() resource.Resource {
	return &billingDetailsResource{}
}

type billingDetailsResource struct {
	client *client.Client
}

type billingDetailsModel struct {
	ID                 types.String `tfsdk:"id"`
	LegalName          types.String `tfsdk:"legal_name"`
	Email              types.String `tfsdk:"email"`
	EntityType         types.String `tfsdk:"entity_type"`
	AddressLine1       types.String `tfsdk:"address_line1"`
	AddressLine2       types.String `tfsdk:"address_line2"`
	City               types.String `tfsdk:"city"`
	State              types.String `tfsdk:"state"`
	Postcode           types.String `tfsdk:"postcode"`
	Country            types.String `tfsdk:"country"`
	TaxID              types.String `tfsdk:"tax_id"`
	TaxIDType          types.String `tfsdk:"tax_id_type"`
	RegistrationNumber types.String `tfsdk:"registration_number"`
	BillingReference   types.String `tfsdk:"billing_reference"`
	Complete           types.Bool   `tfsdk:"complete"`
}

// aliasToAPI maps every writable attribute (except email, which has
// omitted-means-unchanged semantics) to its smushed API field name.
var billingDetailsFieldMap = []struct {
	api string
	get func(m *billingDetailsModel) types.String
}{
	{"legalname", func(m *billingDetailsModel) types.String { return m.LegalName }},
	{"billingentitytype", func(m *billingDetailsModel) types.String { return m.EntityType }},
	{"addressline1", func(m *billingDetailsModel) types.String { return m.AddressLine1 }},
	{"addressline2", func(m *billingDetailsModel) types.String { return m.AddressLine2 }},
	{"addresscity", func(m *billingDetailsModel) types.String { return m.City }},
	{"addressstate", func(m *billingDetailsModel) types.String { return m.State }},
	{"addresspostcode", func(m *billingDetailsModel) types.String { return m.Postcode }},
	{"addresscountry", func(m *billingDetailsModel) types.String { return m.Country }},
	{"taxid", func(m *billingDetailsModel) types.String { return m.TaxID }},
	{"taxidtype", func(m *billingDetailsModel) types.String { return m.TaxIDType }},
	{"registrationnumber", func(m *billingDetailsModel) types.String { return m.RegistrationNumber }},
	{"billingreference", func(m *billingDetailsModel) types.String { return m.BillingReference }},
}

func (r *billingDetailsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_billing_details"
}

func (r *billingDetailsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The organisation's billing identity: the legal name, postal address, country, tax registration and invoice reference printed in the invoice Bill To block. One per organisation — a singleton, not a collection. Destroying this resource clears every declared field (the billing email is left unchanged).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the organisation the details belong to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"legal_name": schema.StringAttribute{
				Optional:    true,
				Description: "Registered legal entity name, printed on invoices. Invoices fall back to the organisation display name when unset.",
			},
			"email": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The billing email all organisation-scoped email goes to. Omitting it leaves the current address unchanged.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"entity_type": schema.StringAttribute{
				Optional:    true,
				Description: "Whether the organisation bills as an individual or a company. One of: individual, company.",
			},
			"address_line1": schema.StringAttribute{
				Optional:    true,
				Description: "First line of the postal address.",
			},
			"address_line2": schema.StringAttribute{
				Optional:    true,
				Description: "Second line of the postal address.",
			},
			"city": schema.StringAttribute{
				Optional:    true,
				Description: "City.",
			},
			"state": schema.StringAttribute{
				Optional:    true,
				Description: "State, province or emirate.",
			},
			"postcode": schema.StringAttribute{
				Optional:    true,
				Description: "Postal code. Not required everywhere, deliberately optional.",
			},
			"country": schema.StringAttribute{
				Optional:    true,
				Description: "ISO 3166-1 alpha-2 country code, e.g. AE or DE. Unrelated to Bahriya's deployment regions.",
			},
			"tax_id": schema.StringAttribute{
				Optional:    true,
				Description: "Tax registration number (VAT / TRN / GST / ABN / EIN). Requires tax_id_type.",
			},
			"tax_id_type": schema.StringAttribute{
				Optional:    true,
				Description: "The kind of tax registration, driving the label the invoice prints. One of: vat, trn, gst, abn, ein, other.",
			},
			"registration_number": schema.StringAttribute{
				Optional:    true,
				Description: "Company registration or trade licence number.",
			},
			"billing_reference": schema.StringAttribute{
				Optional:    true,
				Description: "A PO number or cost centre printed on invoices.",
			},
			"complete": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the identity is invoice-ready. Advisory only — invoices are issued either way.",
			},
		},
	}
}

func (r *billingDetailsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *billingDetailsResource) path() string {
	return fmt.Sprintf("/organisations/%s/billing/details", r.client.OrganisationID())
}

func (r *billingDetailsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan billingDetailsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.put(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *billingDetailsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fresh, diags := r.fetch(ctx)
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

func (r *billingDetailsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan billingDetailsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.put(ctx, &plan, &resp.State, &resp.Diagnostics)
}

// Delete clears every declared identity field with an explicit-null PUT. The
// billing email is deliberately left as-is — an organisation must always have
// one, and PUT treats an omitted email as unchanged.
func (r *billingDetailsResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	body := map[string]any{}
	for _, f := range billingDetailsFieldMap {
		body[f.api] = nil
	}
	_, status, err := r.client.Do(ctx, http.MethodPut, r.path(), body)
	if err != nil && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Clear billing details failed", err.Error())
		return
	}
}

func (r *billingDetailsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The singleton lives at a fixed path in the configured organisation, so
	// the import ID is informational; Read refreshes everything.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, schemaPath("id"), req.ID)...)
}

func (r *billingDetailsResource) put(ctx context.Context, plan *billingDetailsModel, state *tfsdk.State, diags *diagnostics) {
	body := map[string]any{}
	for _, f := range billingDetailsFieldMap {
		body[f.api] = stringOrNil(f.get(plan))
	}
	if v := stringOrNil(plan.Email); v != nil {
		body["email"] = v
	}

	data, status, err := r.client.Do(ctx, http.MethodPut, r.path(), body)
	if err != nil {
		diags.AddError("Update billing details failed", err.Error())
		return
	}
	if status != http.StatusOK {
		diags.AddError("Unexpected status from billing details update", fmt.Sprintf("HTTP %d: %s", status, string(data)))
		return
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		diags.AddError("Decode billing details response failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, apiToBillingDetailsModel(raw))...)
}

func (r *billingDetailsResource) fetch(ctx context.Context) (billingDetailsModel, diagnostics) {
	var diags diagnostics
	data, status, err := r.client.Do(ctx, http.MethodGet, r.path(), nil)
	if err != nil {
		if status == http.StatusNotFound {
			diags.AddError("not_found", "organisation billing details unavailable")
			return billingDetailsModel{}, diags
		}
		diags.AddError("Fetch billing details failed", err.Error())
		return billingDetailsModel{}, diags
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		diags.AddError("Decode billing details response failed", err.Error())
		return billingDetailsModel{}, diags
	}
	return apiToBillingDetailsModel(raw), diags
}

func apiToBillingDetailsModel(raw map[string]any) billingDetailsModel {
	return billingDetailsModel{
		ID:                 stringFromAPI(raw, "organisationid"),
		LegalName:          stringFromAPI(raw, "legalname"),
		Email:              stringFromAPI(raw, "email"),
		EntityType:         stringFromAPI(raw, "billingentitytype"),
		AddressLine1:       stringFromAPI(raw, "addressline1"),
		AddressLine2:       stringFromAPI(raw, "addressline2"),
		City:               stringFromAPI(raw, "addresscity"),
		State:              stringFromAPI(raw, "addressstate"),
		Postcode:           stringFromAPI(raw, "addresspostcode"),
		Country:            stringFromAPI(raw, "addresscountry"),
		TaxID:              stringFromAPI(raw, "taxid"),
		TaxIDType:          stringFromAPI(raw, "taxidtype"),
		RegistrationNumber: stringFromAPI(raw, "registrationnumber"),
		BillingReference:   stringFromAPI(raw, "billingreference"),
		Complete:           boolFromAPI(raw, "complete"),
	}
}
