package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/lifecycle"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/tfval"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// asyncTimeout bounds every wait for a status. A deploy can spend up to ~5
// minutes in the platform's wait-for-bahriya-secrets gate (30×10s) before helm
// and the health checks even start.
var asyncTimeout = 20 * time.Minute

// Resource is the generic resource every kind uses.
type Resource struct {
	desc   Descriptor
	kind   *bmap.Kind
	schema schema.Schema
	client *client.Client
}

var (
	_ resource.Resource                   = (*Resource)(nil)
	_ resource.ResourceWithConfigure      = (*Resource)(nil)
	_ resource.ResourceWithModifyPlan     = (*Resource)(nil)
	_ resource.ResourceWithImportState    = (*Resource)(nil)
	_ resource.ResourceWithValidateConfig = (*Resource)(nil)
	_ resource.ResourceWithUpgradeState   = (*Resource)(nil)
)

// NewResource is a kind's resource with its generated schema (the files
// cmd/generator writes in internal/resources). It panics when the kind has no
// descriptor or is missing from the embedded resource-schema.json: build-time
// faults the generated-schema drift test catches.
func NewResource(name string, s schema.Schema) resource.Resource {
	desc, ok := Descriptors[name]
	if !ok {
		panic("engine: no descriptor for " + name)
	}
	k := bmap.Embedded().Kind(name)
	if k == nil {
		panic("engine: no kind " + name + " in resource-schema.json")
	}
	return &Resource{desc: desc, kind: k, schema: s}
}

// New returns the constructor for a kind's resource with its schema built at
// runtime, for tests and the generator.
func New(name string) func() resource.Resource {
	s, err := KindSchema(name)
	if err != nil {
		panic(err.Error())
	}
	return func() resource.Resource { return NewResource(name, s) }
}

// KindSchema builds a kind's schema with its resource description.
func KindSchema(name string) (schema.Schema, error) {
	if _, ok := Descriptors[name]; !ok {
		return schema.Schema{}, fmt.Errorf("engine: no descriptor for %s", name)
	}
	k := bmap.Embedded().Kind(name)
	if k == nil {
		return schema.Schema{}, fmt.Errorf("engine: no kind %s in resource-schema.json", name)
	}
	return BuildSchema(k, Description(name))
}

// Description is a kind's resource description, for cmd/generator.
func Description(name string) string {
	if k := bmap.Embedded().Kind(name); k != nil {
		return "Manages a Bahriya " + k.TypeName() + "."
	}
	return Descriptors[name].description()
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.kind.TypeName()
}

// KindName is the kind in resource-schema.json (pathrule for bahriya_path_rule).
func (r *Resource) KindName() string {
	return r.desc.Name
}

// parentOf is the URL parent's id from a plan or state record ("" when the
// kind has none).
func (r *Resource) parentOf(record *bmap.Map) string {
	if r.desc.Parent == "" {
		return ""
	}
	v, _ := record.Get(r.desc.Parent)
	if v == nil || tfval.IsUnknown(v) {
		return ""
	}
	return fmt.Sprint(v)
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.schema
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *Resource) objectType(ctx context.Context) tftypes.Type {
	return r.schema.Type().TerraformType(ctx)
}

// ------------------------------------------------------------------ config

// ValidateConfig maps the configuration to the API shape so a mapping error
// (an unknown env key form, a bad list item) is reported at validate time.
// Values still unknown at this stage are skipped.
func (r *Resource) ValidateConfig(_ context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	cfg, err := r.document(req.Config.Raw)
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	if containsUnknown(cfg) {
		return
	}
	if _, err := r.kind.ToAPI(cfg, false); err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
	}
}

// document reads a config, plan or state value as a YAML-shaped document with
// nulls removed (R4: null means absent).
func (r *Resource) document(raw tftypes.Value) (*bmap.Map, error) {
	if raw.IsNull() {
		return bmap.NewMap(), nil
	}
	v, err := tfval.ToGo(raw)
	if err != nil {
		return nil, err
	}
	m, ok := stripNulls(v).(*bmap.Map)
	if !ok {
		return nil, fmt.Errorf("expected an object, got %T", v)
	}
	return m, nil
}

// stateRecord reads a plan or state value as API names, nulls kept as nil.
func (r *Resource) stateRecord(raw tftypes.Value) (*bmap.Map, error) {
	if raw.IsNull() {
		return nil, nil
	}
	v, err := tfval.ToGo(raw)
	if err != nil {
		return nil, err
	}
	m, ok := v.(*bmap.Map)
	if !ok {
		return nil, fmt.Errorf("expected an object, got %T", v)
	}
	return r.kind.FromState(m)
}

// ------------------------------------------------------------------ resolver

// ModifyPlan resolves every writable field from the configuration, in this
// order (R5, R8, R9 and the Terraform deviations in CLAUDE.md):
//
//  1. the configured value, with implications and defaults applied;
//  2. a sensitive or write-only value the API never returns: the prior value;
//  3. no configuration, no default: the prior value, or unknown on create
//     (the API decides), or null for a secret the API never returns.
//
// Read-only fields keep their prior value, except a volatile one (x-volatile)
// when anything writable changes, which becomes unknown. A change to a
// create-only field or the handle replaces the item; a change to an immutable
// field is refused; a change to rotate-only material is a rotation, applied in
// Update.
func (r *Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return // destroy
	}
	cfg, err := r.document(req.Config.Raw)
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	if containsUnknown(cfg) {
		return // a value depends on another resource: plan again at apply
	}
	explicit, _, violations, err := r.kind.ToAPIReport(cfg, false)
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	// R19: the provider refuses a field the resource's type does not allow, and a
	// field it requires that the configuration leaves out.
	if len(violations) > 0 {
		handle, _ := cfg.Get("handle")
		for _, v := range violations {
			if v.Rule == "required" {
				resp.Diagnostics.AddAttributeError(attrPath(v.Path), "Required for "+v.Type+" containers",
					fmt.Sprintf("%s is not set on %s %q. State it for %s containers: true or false.", v.Path, r.desc.noun(), handle, v.Type))
				continue
			}
			resp.Diagnostics.AddAttributeError(attrPath(v.Path), "Not allowed on "+v.Type+" containers",
				fmt.Sprintf("%s is set on %s %q, but %s containers do not use it. Remove it.", v.Path, r.desc.noun(), handle, v.Type))
		}
		return
	}
	resolved, err := r.kind.ToAPI(cfg, true)
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	prior, err := r.stateRecord(req.State.Raw)
	if err != nil {
		resp.Diagnostics.AddError("Invalid state", err.Error())
		return
	}

	planned := bmap.NewMap()
	changed := prior == nil
	for _, f := range r.kind.Fields {
		if !r.kind.IsWritable(f) {
			continue
		}
		priorValue, _ := prior.Get(f.Name)
		secret := f.Desc.Sensitive || f.Desc.WriteOnly
		var value any
		switch {
		case explicit.Has(f.Name) || (resolved.Has(f.Name) && !secret):
			value, _ = resolved.Get(f.Name)
		case prior != nil:
			// The item exists: keep what the API holds, null included, so an
			// unconfigured field the API leaves empty plans no diff.
			value = priorValue
		case secret:
			value = nil // never returned by the API, so never unknown
		default:
			value = tfval.Unknown // create: the API decides
		}
		if prior != nil && !tfval.IsUnknown(value) && !bmap.JSONEquals(value, priorValue) {
			changed = true
		}
		planned.Set(f.Name, value)
	}
	// The URL parent comes from the configuration: read-only in the body, but required.
	var configured *bmap.Map
	if r.desc.Parent != "" {
		if configured, err = r.kind.FromState(cfg); err != nil {
			resp.Diagnostics.AddError("Invalid configuration", err.Error())
			return
		}
	}
	for _, f := range r.kind.Fields {
		if r.kind.IsWritable(f) {
			continue
		}
		priorValue, hasPrior := prior.Get(f.Name)
		if f.Desc.Parent {
			v, _ := configured.Get(f.Name)
			planned.Set(f.Name, v)
			continue
		}
		if prior == nil || !hasPrior || (f.Desc.Volatile && changed) {
			planned.Set(f.Name, tfval.Unknown)
		} else {
			planned.Set(f.Name, priorValue)
		}
	}

	raw, err := r.stateRaw(ctx, planned)
	if err != nil {
		resp.Diagnostics.AddError("Plan", err.Error())
		return
	}
	resp.Plan.Raw = raw

	if prior == nil {
		return
	}
	differs := func(name string) bool {
		was, _ := prior.Get(name)
		now, _ := planned.Get(name)
		return was != nil && !tfval.IsUnknown(now) && !bmap.JSONEquals(was, now)
	}
	newHandle := differs("handle")
	for _, f := range r.kind.Fields {
		if !differs(f.Name) {
			continue
		}
		switch {
		case f.Desc.Immutable:
			resp.Diagnostics.AddAttributeError(attrPath(f.Path), "This value is fixed when the item is created",
				fmt.Sprintf("%s is fixed when a %s is created; to use another value, create a new %s and move its attachments.", f.Path, r.desc.Name, r.desc.Name))
		case f.Name == "handle":
			resp.RequiresReplace = append(resp.RequiresReplace, attrPath(f.Path))
		case f.Desc.Parent:
			// Another parent is another item. Replacing is safe here: a path rule
			// holds no data, has no dependants, and its handle is reusable once removed.
			resp.RequiresReplace = append(resp.RequiresReplace, attrPath(f.Path))
		case f.Desc.CreateOnly && newHandle:
			resp.RequiresReplace = append(resp.RequiresReplace, attrPath(f.Path))
		case f.Desc.CreateOnly:
			// A replacement keeping the handle would destroy this item and then fail:
			// a terminated item keeps its handle for good, so the create is refused
			// with "Handle already in use". Refuse in the plan, before anything is lost.
			resp.Diagnostics.AddAttributeError(attrPath(f.Path), "This value is fixed when the "+r.desc.noun()+" is created",
				fmt.Sprintf("%s is fixed when a %s is created, and changing it means a new %s. Its handle stays taken after "+
					"the old one is terminated, so give the new %s a new handle in the same change: Terraform then replaces it.",
					f.Path, r.desc.Name, r.desc.noun(), r.desc.noun()))
		}
	}
}

// ------------------------------------------------------------------ CRUD

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	plan, err := r.stateRecord(req.Plan.Raw)
	if err != nil {
		resp.Diagnostics.AddError("Plan", err.Error())
		return
	}
	payload := r.payload(plan, false)
	org := r.client.OrganisationID()
	parent := r.parentOf(plan)

	if handle, _ := payload.Get("handle"); handle != nil && r.desc.HandlePath != "" {
		exists, err := r.client.HandleExists(ctx, r.desc.HandlePath, fmt.Sprint(handle))
		if err != nil {
			resp.Diagnostics.AddError("Handle existence check failed", err.Error())
			return
		}
		if exists {
			resp.Diagnostics.AddError("Handle already in use",
				fmt.Sprintf("A %s with handle %q already exists in this organisation. Handles cannot be reused.", r.desc.Name, handle))
			return
		}
	}

	data, status, err := r.client.Do(ctx, http.MethodPost, r.desc.base(org, parent), payload)
	if status == http.StatusConflict {
		// A refusal on project resource limits is the one 409 with a structured body,
		// and the one a practitioner can act on: which ceiling, in which region, by how much.
		if verdict := client.DecodeQuotaRefusal(apiErrorBody(err, data)); verdict != nil {
			resp.Diagnostics.AddError("Project resource limits exceeded", client.QuotaDetail(verdict))
			return
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Create failed", err.Error())
		return
	}
	if status != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected status from Create", fmt.Sprintf("HTTP %d: %s", status, string(data)))
		return
	}
	id, verdict, err := client.DecodeWrite(data)
	if err != nil {
		resp.Diagnostics.AddError("Decode created id failed", err.Error())
		return
	}
	if warning := client.QuotaWarning(verdict); warning != "" {
		// Applied, and running. It may still be unable to reach the replica count the
		// configuration asks for, which nothing else in the plan or state would reveal.
		resp.Diagnostics.AddWarning("Scaling limited by project resource limits", warning)
	}

	if r.desc.HasStatus {
		if record, werr := r.waitReady(ctx, parent, id); werr != nil {
			// The item WAS created: its handle is now taken for good. Returning an error
			// would taint it, and the replace on the next apply would fail with 409 on the
			// immutable handle, wedging the configuration. Save it with its real status
			// and warn; a later apply re-reads the status.
			if record == nil {
				resp.Diagnostics.AddError(fmt.Sprintf("The %s was created but could not be read back", r.desc.noun()), werr.Error())
				return
			}
			resp.Diagnostics.Append(r.setState(ctx, &resp.State.Raw, record, plan)...)
			last, _ := record.Get("status")
			resp.Diagnostics.AddWarning(fmt.Sprintf("Created but not yet %s", r.desc.ReadyStatus),
				fmt.Sprintf("The %s %v was created but did not reach %q within %s (last status %q). "+
					"It is saved to state; run `terraform apply` again to re-check once the deploy settles. "+
					"If it stays in \"error\", investigate the deploy and terminate the %s explicitly — it "+
					"is not auto-replaced, because the handle is immutable and cannot be reused.",
					r.desc.noun(), handleOf(payload), r.desc.ReadyStatus, asyncTimeout, last, r.desc.noun()))
			return
		}
	}

	record, diags := r.fetch(ctx, parent, id)
	if resp.Diagnostics.Append(diags...); diags.HasError() {
		return
	}
	resp.Diagnostics.Append(r.setState(ctx, &resp.State.Raw, record, plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	prior, err := r.stateRecord(req.State.Raw)
	if err != nil {
		resp.Diagnostics.AddError("State", err.Error())
		return
	}
	id, _ := prior.Get("id")
	record, diags := r.fetch(ctx, r.parentOf(prior), fmt.Sprint(id))
	if diags.HasError() {
		if notFound(diags) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(r.setState(ctx, &resp.State.Raw, record, prior)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan, err := r.stateRecord(req.Plan.Raw)
	if err != nil {
		resp.Diagnostics.AddError("Plan", err.Error())
		return
	}
	prior, err := r.stateRecord(req.State.Raw)
	if err != nil {
		resp.Diagnostics.AddError("State", err.Error())
		return
	}
	id := fmt.Sprint(mustGet(prior, "id"))
	org := r.client.OrganisationID()
	parent := r.parentOf(prior)

	rotate, update := false, false
	for _, f := range r.kind.Fields {
		if !r.kind.IsWritable(f) {
			continue
		}
		now, _ := plan.Get(f.Name)
		was, _ := prior.Get(f.Name)
		if tfval.IsUnknown(now) || bmap.JSONEquals(now, was) {
			continue
		}
		if f.Desc.RotateOnly {
			rotate = true
		} else {
			update = true
		}
	}

	if update {
		data, status, err := r.client.Do(ctx, http.MethodPut, r.desc.item(r.desc.URLItem, org, parent, id), r.payload(plan, true))
		if err != nil {
			// The API's message (a 409 explaining a policy is already attached, say)
			// is what the practitioner acts on, so surface it, not just the code.
			resp.Diagnostics.AddError("Update failed", err.Error())
			return
		}
		if status != http.StatusOK {
			resp.Diagnostics.AddError("Unexpected status from Update", fmt.Sprintf("HTTP %d: %s", status, string(data)))
			return
		}
	}
	if rotate {
		// Versioned material changes only by rotation (x-rotate-only): the rotate body
		// is every rotate-only field, the unchanged ones from prior state.
		body := bmap.NewMap()
		for _, f := range r.kind.Fields {
			if f.Desc.RotateOnly {
				v, _ := plan.Get(f.Name)
				body.Set(f.Name, v)
			}
		}
		if _, _, err := r.client.Do(ctx, http.MethodPost, r.desc.item(r.desc.URLItem, org, parent, id)+"/rotate", body); err != nil {
			resp.Diagnostics.AddError("Rotation failed", err.Error())
			return
		}
	}

	if r.desc.HasStatus && (update || rotate) {
		if record, werr := r.waitReady(ctx, parent, id); werr != nil {
			if record == nil {
				resp.Diagnostics.AddError(fmt.Sprintf("The %s was updated but could not be read back", r.desc.noun()), werr.Error())
				return
			}
			resp.Diagnostics.Append(r.setState(ctx, &resp.State.Raw, record, plan)...)
			last, _ := record.Get("status")
			resp.Diagnostics.AddWarning(fmt.Sprintf("Updated but not yet %s", r.desc.ReadyStatus),
				fmt.Sprintf("The update was accepted but the %s did not reach %q within %s (last status %q). "+
					"It is saved to state; run `terraform apply` again to re-check once the deploy settles.",
					r.desc.noun(), r.desc.ReadyStatus, asyncTimeout, last))
			return
		}
	}

	record, diags := r.fetch(ctx, parent, id)
	if resp.Diagnostics.Append(diags...); diags.HasError() {
		return
	}
	resp.Diagnostics.Append(r.setState(ctx, &resp.State.Raw, record, plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	prior, err := r.stateRecord(req.State.Raw)
	if err != nil {
		resp.Diagnostics.AddError("State", err.Error())
		return
	}
	id := fmt.Sprint(mustGet(prior, "id"))
	org := r.client.OrganisationID()
	parent := r.parentOf(prior)
	target := r.desc.item(r.desc.deleteURL(), org, parent, id)

	if r.desc.HasStatus {
		// Terminated already (torn down out of band, or an interrupted destroy that has
		// since finished): nothing to do. Terminating already: re-issuing the terminate
		// 4xxs ("cannot be terminated from its current state"), so only wait.
		skip := false
		if record, diags := r.fetch(ctx, parent, id); !diags.HasError() {
			switch s, _ := record.Get("status"); s {
			case r.desc.TerminatedStatus:
				return
			case r.desc.TerminatingStatus:
				skip = true
			}
		}
		if !skip {
			if _, status, err := r.client.Do(ctx, r.desc.deleteMethod(), target, nil); err != nil && status != http.StatusNotFound {
				resp.Diagnostics.AddError("Delete failed", err.Error())
				return
			}
		}
		_, werr := lifecycle.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
			record, diags := r.fetch(ctx, parent, id)
			if diags.HasError() {
				return r.desc.TerminatedStatus, nil
			}
			if s, ok := record.Get("status"); ok && s != nil {
				return fmt.Sprint(s), nil
			}
			return r.desc.TerminatedStatus, nil
		}, lifecycle.WaitOptions{Target: r.desc.TerminatedStatus, Timeout: asyncTimeout})
		if werr != nil {
			resp.Diagnostics.AddError("Resource did not reach terminated status", werr.Error())
		}
		return
	}

	var body any
	if r.desc.ConfirmDelete {
		body = map[string]any{"confirmName": mustGet(prior, "name")}
	}
	if _, status, err := r.client.Do(ctx, r.desc.deleteMethod(), target, body); err != nil && status != http.StatusNotFound {
		if status == http.StatusConflict && r.desc.DeleteConflictHint != "" {
			resp.Diagnostics.AddError("Delete blocked by active dependencies", err.Error()+"\n\n"+r.desc.DeleteConflictHint)
			return
		}
		resp.Diagnostics.AddError("Delete failed", err.Error())
	}
}

// ImportState takes the item's id, or "<parent id>:<id>" for a kind with a URL
// parent (a path rule: "<container id>:<path rule id>", as in v0.x).
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.desc.Parent == "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
		return
	}
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("Import IDs for bahriya_%s look like '<%s id>:<id>'.", r.kind.TypeName(), r.desc.Parent))
		return
	}
	parentPath := attrPath(r.kind.Field(r.desc.Parent).Path)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, parentPath, parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// ------------------------------------------------------------------ API

// payload is the plan's writable fields (R16: every resolved field is sent).
// Unknown and null values are left out. On update, rotate-only material is
// left out too: an update ignores it and rotation sends it.
func (r *Resource) payload(plan *bmap.Map, update bool) *bmap.Map {
	out := bmap.NewMap()
	for _, f := range r.kind.Fields {
		if !r.kind.IsWritable(f) || (update && f.Desc.RotateOnly) {
			continue
		}
		v, _ := plan.Get(f.Name)
		if v != nil && !tfval.IsUnknown(v) {
			out.Set(f.Name, v)
		}
	}
	if r.desc.SendOrganisation {
		out.Set("organisation", r.client.OrganisationID())
	}
	return out
}

// fetch GETs one item. A 404, an empty list wrapper, or an item marked removed
// is a "not_found" error. A kind with a URL parent gets the parent's id set on
// the record, since the API addresses the item by it.
func (r *Resource) fetch(ctx context.Context, parent, id string) (*bmap.Map, diag.Diagnostics) {
	record, diags := r.get(ctx, parent, id)
	if diags.HasError() {
		return nil, diags
	}
	if removed, _ := record.Get("removedat"); removed != nil {
		diags.AddError("not_found", "resource was removed")
		return nil, diags
	}
	if r.desc.Parent != "" && parent != "" {
		if v, _ := record.Get(r.desc.Parent); v == nil {
			record.Set(r.desc.Parent, parent)
		}
	}
	return record, diags
}

func (r *Resource) get(ctx context.Context, parent, id string) (*bmap.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	data, status, err := r.client.Do(ctx, http.MethodGet, r.desc.item(r.desc.URLItem, r.client.OrganisationID(), parent, id), nil)
	if err != nil {
		if status == http.StatusNotFound {
			diags.AddError("not_found", "resource was deleted out-of-band")
			return nil, diags
		}
		diags.AddError("Fetch failed", err.Error())
		return nil, diags
	}
	decoded, err := bmap.Decode(data)
	if err != nil {
		diags.AddError("Decode fetch response failed", err.Error())
		return nil, diags
	}
	record, ok := decoded.(*bmap.Map)
	if !ok {
		diags.AddError("Decode fetch response failed", fmt.Sprintf("expected an object, got %T", decoded))
		return nil, diags
	}
	if r.desc.CollectionKey != "" {
		items, _ := record.Get(r.desc.CollectionKey)
		list, _ := items.([]any)
		if len(list) == 0 {
			diags.AddError("not_found", "resource was deleted out-of-band")
			return nil, diags
		}
		if record, ok = list[0].(*bmap.Map); !ok {
			diags.AddError("Decode fetch response failed", "unexpected item shape")
			return nil, diags
		}
	}
	return record, diags
}

// waitReady polls until the item reaches ReadyStatus. On a timeout or the
// "error" status it returns the last record it read, if any, with the error.
func (r *Resource) waitReady(ctx context.Context, parent, id string) (*bmap.Map, error) {
	var last *bmap.Map
	_, werr := lifecycle.WaitForStatus(ctx, func(ctx context.Context) (string, error) {
		record, diags := r.fetch(ctx, parent, id)
		if diags.HasError() {
			return "", errors.New("fetch error during wait")
		}
		last = record
		s, _ := record.Get("status")
		return fmt.Sprint(s), nil
	}, lifecycle.WaitOptions{Target: r.desc.ReadyStatus, Terminal: []string{"error"}, Timeout: asyncTimeout})
	if werr != nil {
		if record, diags := r.fetch(ctx, parent, id); !diags.HasError() {
			last = record
		}
		return last, werr
	}
	return last, nil
}

// setState writes the API record in state mode. A secret the API does not
// return (write-only, or a masked sentinel) keeps the value from `known`, the
// plan or prior state; list items are matched by x-item-key for that.
func (r *Resource) setState(ctx context.Context, out *tftypes.Value, record, known *bmap.Map) diag.Diagnostics {
	var diags diag.Diagnostics
	merged := bmap.Clone(record).(*bmap.Map)
	for _, f := range r.kind.Fields {
		knownValue, _ := known.Get(f.Name)
		if tfval.IsUnknown(knownValue) {
			knownValue = nil
		}
		value, present := merged.Get(f.Name)
		switch {
		case (f.Desc.WriteOnly || f.Desc.Sensitive) && (!present || value == nil),
			f.Desc.Masked != "" && value == f.Desc.Masked:
			merged.Set(f.Name, bmap.Clone(knownValue))
		case f.Desc.ItemKey != "" && len(f.Item) > 0:
			merged.Set(f.Name, mergeItems(f, value, knownValue))
		}
	}
	raw, err := r.stateRaw(ctx, merged)
	if err != nil {
		diags.AddError("State", err.Error())
		return diags
	}
	*out = raw
	return diags
}

// toState is an API-named record in the v1 state shape (ToYAML, state mode),
// with free-form map values as strings: Terraform holds them as Map(String)
// (an accepted deviation), while the API returns them typed (max_connections
// 300). JSON semantics, as v0.x did; the API accepts the strings back.
func (r *Resource) toState(record *bmap.Map) (*bmap.Map, error) {
	out := bmap.Clone(record).(*bmap.Map)
	for _, f := range r.kind.Fields {
		if !f.Desc.Freeform {
			continue
		}
		m, ok := mustGet(out, f.Name).(*bmap.Map)
		if !ok {
			continue
		}
		strs := bmap.NewMap()
		for _, k := range m.Keys() {
			v, _ := m.Get(k)
			strs.Set(k, bmap.Cast(v))
		}
		out.Set(f.Name, strs)
	}
	return r.kind.ToYAML(out, true)
}

// stateRaw is a record in the v1 state shape as a Terraform value. It fails
// when the record holds a value R12 or R20 cannot read, or one that does not
// fit the schema.
func (r *Resource) stateRaw(ctx context.Context, record *bmap.Map) (tftypes.Value, error) {
	state, err := r.toState(record)
	if err != nil {
		return tftypes.Value{}, err
	}
	return tfval.FromGo(state, r.objectType(ctx))
}

// mergeItems restores secrets in list items (basic auth passwords, say) from
// the known list, matching items by x-item-key.
func mergeItems(f *bmap.Field, value, known any) any {
	list, ok := value.([]any)
	knownList, _ := known.([]any)
	if !ok {
		return value
	}
	byKey := map[string]*bmap.Map{}
	for _, k := range knownList {
		if m, isMap := k.(*bmap.Map); isMap {
			key, _ := m.Get(f.Desc.ItemKey)
			byKey[fmt.Sprint(key)] = m
		}
	}
	out := make([]any, len(list))
	for i, element := range list {
		m, isMap := element.(*bmap.Map)
		if !isMap {
			out[i] = element
			continue
		}
		m = bmap.Clone(m).(*bmap.Map)
		key, _ := m.Get(f.Desc.ItemKey)
		if prior, found := byKey[fmt.Sprint(key)]; found {
			for _, item := range f.Item {
				v, present := m.Get(item.Name)
				if ((item.Desc.WriteOnly || item.Desc.Sensitive) && (!present || v == nil)) || (item.Desc.Masked != "" && v == item.Desc.Masked) {
					pv, _ := prior.Get(item.Name)
					m.Set(item.Name, bmap.Clone(pv))
				}
			}
		}
		out[i] = m
	}
	return out
}

// ------------------------------------------------------------------ helpers

// apiErrorBody is the response body of a failed request: the client returns
// it in the APIError, not in data.
func apiErrorBody(err error, data []byte) []byte {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.Body != "" {
		return []byte(apiErr.Body)
	}
	return data
}

func notFound(diags diag.Diagnostics) bool {
	for _, d := range diags.Errors() {
		if d.Summary() == "not_found" {
			return true
		}
	}
	return false
}

func mustGet(m *bmap.Map, key string) any {
	v, _ := m.Get(key)
	return v
}

func handleOf(payload *bmap.Map) any {
	h, _ := payload.Get("handle")
	return h
}

// stripNulls drops nil values at every depth; unknowns are kept (R4).
func stripNulls(v any) any {
	switch t := v.(type) {
	case *bmap.Map:
		out := bmap.NewMap()
		for _, k := range t.Keys() {
			item, _ := t.Get(k)
			if item == nil {
				continue
			}
			out.Set(k, stripNulls(item))
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = stripNulls(t[i])
		}
		return out
	}
	return v
}

func containsUnknown(v any) bool {
	switch t := v.(type) {
	case *bmap.Map:
		for _, k := range t.Keys() {
			item, _ := t.Get(k)
			if containsUnknown(item) {
				return true
			}
		}
	case []any:
		for _, item := range t {
			if containsUnknown(item) {
				return true
			}
		}
	default:
		return tfval.IsUnknown(v)
	}
	return false
}

func attrPath(p string) path.Path {
	segments := strings.Split(p, ".")
	out := path.Root(segments[0])
	for _, s := range segments[1:] {
		out = out.AtName(s)
	}
	return out
}
