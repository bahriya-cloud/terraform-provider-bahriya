package engine

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// Every step applies, then plugin-testing plans again and fails the step
// unless that plan is empty. A passing step therefore proves both "no
// inconsistent result after apply" and "no perpetual diff".

type testProvider struct{ api *fakeAPI }

func (p *testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "bahriya"
}
func (p *testProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (p *testProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	c := client.New(p.api.server.URL, "test-token", fakeOrg)
	resp.ResourceData = c
	resp.DataSourceData = c
}
func (p *testProvider) Resources(context.Context) []func() resource.Resource {
	var out []func() resource.Resource
	for name := range Descriptors {
		out = append(out, New(name))
	}
	return out
}
func (p *testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func factories(api *fakeAPI) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"bahriya": providerserver.NewProtocol6WithError(&testProvider{api: api}),
	}
}

func at(names ...string) tfjsonpath.Path {
	p := tfjsonpath.New(names[0])
	for _, n := range names[1:] {
		p = p.AtMapKey(n)
	}
	return p
}

func init() {
	asyncTimeout = 30 * time.Second
}

// A secret: create, re-plan empty, rename in place, change the value (a PUT,
// which the API treats as a rotation), and the value never read back stays
// in state.
func TestSecretLifecycle(t *testing.T) {
	api := newFakeAPI(t)
	cfg := func(name, value string) string {
		return fmt.Sprintf(`
resource "bahriya_secret" "s" {
  name   = %q
  handle = "orders-key"
  value  = %q
}`, name, value)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{
				Config: cfg("Orders key", "v1"),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("bahriya_secret.s", "value", "v1"),
					tfresource.TestCheckResourceAttr("bahriya_secret.s", "max_versions", "5"),
					tfresource.TestCheckResourceAttrSet("bahriya_secret.s", "id"),
					tfresource.TestCheckResourceAttrSet("bahriya_secret.s", "created.date"),
				),
			},
			{
				Config: cfg("Orders key renamed", "v1"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("bahriya_secret.s", plancheck.ResourceActionUpdate),
					plancheck.ExpectUnknownValue("bahriya_secret.s", at("updated")),
					plancheck.ExpectKnownValue("bahriya_secret.s", at("value"), knownvalue.StringExact("v1")),
				}},
			},
			{
				Config: cfg("Orders key renamed", "v2"),
				Check:  tfresource.TestCheckResourceAttr("bahriya_secret.s", "value", "v2"),
			},
		},
	})
	if puts := api.Calls("PUT"); len(puts) != 2 {
		t.Errorf("want 2 PUTs (rename, value), got %v", puts)
	}
}

// Rotate-only material is rotated, never sent in a PUT nor replaced; the
// version counters move in the same apply without an inconsistent result.
func TestEnvFileContentIsRotated(t *testing.T) {
	api := newFakeAPI(t)
	cfg := func(content string) string {
		return fmt.Sprintf(`
resource "bahriya_env_file" "e" {
  name    = "Orders env"
  handle  = "orders-env"
  content = %q
}`, content)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{Config: cfg("A=1\n")},
			{
				Config: cfg("A=2\n"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("bahriya_env_file.e", plancheck.ResourceActionUpdate),
				}},
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("bahriya_env_file.e", "content", "A=2\n"),
					tfresource.TestCheckResourceAttr("bahriya_env_file.e", "currentversion", "2"),
				),
			},
		},
	})
	if rotates := api.Calls("POST /organisations/" + fakeOrg + "/env_files/"); len(rotates) != 1 {
		t.Errorf("want 1 rotate, got %v", rotates)
	}
	if puts := api.Calls("PUT"); len(puts) != 0 {
		t.Errorf("a content-only change must not PUT, got %v", puts)
	}
}

// An immutable field cannot change: the plan says why, nothing is sent.
func TestEncryptionKeyFormatIsRefused(t *testing.T) {
	api := newFakeAPI(t)
	cfg := func(format string) string {
		return fmt.Sprintf(`
resource "bahriya_encryption_key" "k" {
  name   = "Orders key"
  handle = "orders-enc"
  key    = "c2VjcmV0"
  format = %q
}`, format)
	}
	formats := bmap.Embedded().Kind("encryption_key").Field("format").Desc.Enum
	if len(formats) < 2 {
		t.Skip("encryption_key format has fewer than two values")
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{Config: cfg(fmt.Sprint(formats[0]))},
			{Config: cfg(fmt.Sprint(formats[1])), ExpectError: regexp.MustCompile(`fixed when the item is created`)},
		},
	})
}

// A handle change replaces the item; a create-time quota refusal reads as the
// structured message, not "Create failed".
func TestHandleReplacesAndQuotaRefusalIsReadable(t *testing.T) {
	api := newFakeAPI(t)
	cfg := func(handle string) string {
		return fmt.Sprintf(`
resource "bahriya_plain_config" "p" {
  name    = "Banner"
  handle  = %q
  content = "hello"
}`, handle)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{Config: cfg("banner")},
			{
				Config: cfg("banner-two"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("bahriya_plain_config.p", plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
		},
	})

	api2 := newFakeAPI(t)
	api2.refuse["plain_config"] = `{"blocked":true,"message":"Not enough CPU","breaches":[{"region":"helsinki-1","axis":"requests.cpu","label":"CPU requests","requested":"2000m","available":"500m"}]}`
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api2),
		Steps: []tfresource.TestStep{
			{Config: cfg("banner"), ExpectError: regexp.MustCompile(`Project resource limits exceeded`)},
		},
	})
}

// An asynchronous kind waits for running, groups default, and a destroy
// terminates and waits for terminated.
func TestMemcachedGroupsAndStatus(t *testing.T) {
	api := newFakeAPI(t)
	cfg := func(threads string) string {
		return fmt.Sprintf(`
resource "bahriya_memcached" "m" {
  name    = "Cache"
  handle  = "cache"
  project = "2e01bd37-f4ba-4c96-a158-aaaaaaaaaaaa"
  regions = ["helsinki-1"]
  tuning = {
    memory = 128
%s
  }
}`, threads)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{
				Config: cfg(""),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("bahriya_memcached.m", "status", "running"),
					tfresource.TestCheckResourceAttrSet("bahriya_memcached.m", "tuning.threads"),
				),
			},
			{
				Config: cfg("    threads = 8"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue("bahriya_memcached.m", at("tuning", "threads"), knownvalue.Int64Exact(8)),
					plancheck.ExpectUnknownValue("bahriya_memcached.m", at("updated")),
				}},
			},
		},
	})
}

// An unset boolean takes the spec default, never false: valkey authenabled
// defaults to true, and a forced false once created unauthenticated
// instances (2026-08 production e2e).
func TestValkeyUnsetBooleansTakeTheSpecDefault(t *testing.T) {
	api := newFakeAPI(t)
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{{
			Config: `
resource "bahriya_valkey" "v" {
  name    = "Cache"
  handle  = "cache"
  tier    = "single"
  purpose = "cache"
  memory  = 256
  regions = ["helsinki-1"]
}`,
			Check: tfresource.TestCheckResourceAttr("bahriya_valkey.v", "status", "running"),
		}},
	})
	body := api.created["valkey"]
	for key, want := range map[string]any{"authenabled": true, "tlsenabled": false, "backupenabled": false, "externalenabled": false} {
		if got, _ := body.Get(key); got != want {
			t.Errorf("%s sent as %v, want the spec default %v (body %s)", key, got, want, bmap.Encode(body))
		}
	}
}

// MySQL: tier and both encryption choices are fixed at creation, so changing
// one replaces the instance; the managed vault items are read-only; no
// credential is ever an attribute.
func TestMysqlCreateTimeChoicesAndCredentials(t *testing.T) {
	s, err := BuildSchema(bmap.Embedded().Kind("mysql"), "")
	if err != nil {
		t.Fatal(err)
	}
	for name, attr := range s.Attributes {
		if name == "password" || name == "rootpassword" {
			t.Errorf("no credential may be a resource attribute (%q, %T)", name, attr)
		}
	}
	if !s.Attributes["root_secret"].IsComputed() || s.Attributes["root_secret"].IsOptional() {
		t.Error("root_secret must be computed only")
	}

	api := newFakeAPI(t)
	cfg := func(handle, tier string) string {
		return fmt.Sprintf(`
resource "bahriya_mysql" "m" {
  name      = "Orders"
  handle    = %q
  tier      = %q
  cpu       = 500
  memory    = 1024
  regions   = ["helsinki-1"]
  databases = [{ name = "app", username = "app-user" }]
  persistence = {
    storage = 10
  }
}`, handle, tier)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{Config: cfg("orders", "single")},
			// Same handle: a replacement would terminate the instance and then fail on
			// the handle a terminated instance keeps, so the plan refuses it.
			{Config: cfg("orders", "replicated"), ExpectError: regexp.MustCompile(`give the new\s+instance a new handle`)},
			// A new handle with it: an ordinary replacement.
			{
				Config: cfg("orders-two", "replicated"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("bahriya_mysql.m", plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
		},
	})
	if posts := api.Calls("POST /organisations/" + fakeOrg + "/mysql"); len(posts) != 2 {
		t.Errorf("want 2 creates (original, replacement), got %v", posts)
	}
}

// R16: an omitted field without a default keeps its current value, and an
// explicit empty list clears it (v0.x cleared on omission; the upgrade guide
// says so). Nested list items map through their aliases with their defaults.
func TestNetworkPolicyListsFollowR16(t *testing.T) {
	api := newFakeAPI(t)
	cfg := func(egress string) string {
		return fmt.Sprintf(`
resource "bahriya_network_policy" "n" {
  name          = "Web tier"
  handle        = "web-tier"
  ingress_peers = ["frontend"]
  ports         = [{ port = 443 }]
%s
}`, egress)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{
				Config: cfg(`  egress_cidrs = ["10.0.0.0/8"]`),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("bahriya_network_policy.n", "ports.0.protocol", "TCP"),
					tfresource.TestCheckResourceAttr("bahriya_network_policy.n", "egress_cidrs.0", "10.0.0.0/8"),
				),
			},
			{
				Config: cfg(""),
				Check:  tfresource.TestCheckResourceAttr("bahriya_network_policy.n", "egress_cidrs.0", "10.0.0.0/8"),
			},
			{
				Config: cfg(`  egress_cidrs = []`),
				Check:  tfresource.TestCheckResourceAttr("bahriya_network_policy.n", "egress_cidrs.#", "0"),
			},
		},
	})
	body := api.created["network_policy"]
	ports, _ := body.Get("ports")
	if bmap.Encode(ports) != `[{"port":443,"protocol":"TCP"}]` {
		t.Errorf("ports sent as %s, want the item default applied", bmap.Encode(ports))
	}
}

// A project reads through its list wrapper and is deleted with confirmName.
func TestProjectWrapperAndConfirmDelete(t *testing.T) {
	api := newFakeAPI(t)
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{
				Config: `
resource "bahriya_project" "p" {
  name    = "Shop"
  handle  = "shop"
  regions = ["helsinki-1"]
}`,
				Check: tfresource.TestCheckResourceAttr("bahriya_project.p", "name", "Shop"),
			},
		},
	})
	if dels := api.Calls("DELETE /organisations/" + fakeOrg + "/projects/"); len(dels) != 1 {
		t.Errorf("want 1 project DELETE, got %v", dels)
	}
}
