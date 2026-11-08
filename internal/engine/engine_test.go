package engine

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
					tfresource.TestCheckResourceAttrSet("bahriya_secret.s", "created"),
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
					tfresource.TestCheckResourceAttr("bahriya_env_file.e", "current_version", "2"),
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

// A masked secret in a list item (basic auth passwords) keeps the configured
// value in state, matched by x-item-key, so it never drifts to the sentinel
// (the v0.x drift); x-implies turns basic auth on; a new password is sent.
func TestContainerBasicAuthPasswordsDoNotDrift(t *testing.T) {
	api := newFakeAPI(t)
	cfg := func(password string) string {
		return fmt.Sprintf(`
resource "bahriya_container" "c" {
  name    = "Shop"
  handle  = "shop"
  regions = ["helsinki-1"]
  workload = {
    image  = "nginx:1.27"
    cpu    = 250
    memory = 256
  }
  networking = {
    external = true
  }
  security = {
    basic_auth = [{ username = "shop-alice", password = %q }]
  }
}`, password)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{
				Config: cfg("s3cret"),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("bahriya_container.c", "security.basic_auth.0.password", "s3cret"),
					tfresource.TestCheckResourceAttr("bahriya_container.c", "security.basic_auth_enabled", "true"),
				),
			},
			{
				Config: cfg("n3w"),
				Check:  tfresource.TestCheckResourceAttr("bahriya_container.c", "security.basic_auth.0.password", "n3w"),
			},
		},
	})
	puts := api.Calls("PUT")
	if len(puts) != 1 {
		t.Errorf("want 1 PUT for the new password, got %v", puts)
	}
	body := api.created["container"]
	if enabled, _ := body.Get("basicauthenabled"); enabled != true {
		t.Errorf("x-implies must send basicauthenabled true, got %v", enabled)
	}
}

// A path rule is bahriya_path_rule (x-terraform-type), lives under its
// container (x-parent): the container is required, addresses every request,
// is never sent in the body, and changing it replaces the rule. Import takes
// "<container id>:<id>".
func TestPathRuleUnderItsContainer(t *testing.T) {
	api := newFakeAPI(t)
	const a, b = "aaaaaaaa-0000-4000-8000-00000000000a", "bbbbbbbb-0000-4000-8000-00000000000b"
	cfg := func(container, path string) string {
		return fmt.Sprintf(`
resource "bahriya_path_rule" "r" {
  container = %q
  handle    = "admin"
  path      = %q
  security = {
    rate_limiting = {
      enabled    = true
      per_minute = 60
    }
  }
}`, container, path)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{
			{
				Config: cfg(a, "/admin"),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("bahriya_path_rule.r", "container", a),
					tfresource.TestCheckResourceAttr("bahriya_path_rule.r", "security.rate_limiting.per_minute", "60"),
				),
			},
			{Config: cfg(a, "/admin/v2")},
			{
				Config: cfg(b, "/admin/v2"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("bahriya_path_rule.r", plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
			{
				ResourceName: "bahriya_path_rule.r",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return b + ":" + s.RootModule().Resources["bahriya_path_rule.r"].Primary.ID, nil
				},
				ImportStateVerify: true,
			},
		},
	})
	if body := api.created["pathrule"]; body.Has("container") {
		t.Errorf("the parent must never be in the body, got %s", bmap.Encode(body))
	}
	if puts := api.Calls("PUT /organisations/" + fakeOrg + "/containers/" + a + "/pathrules/"); len(puts) != 1 {
		t.Errorf("want 1 PUT under container a, got %v", puts)
	}
	if posts := api.Calls("POST /organisations/" + fakeOrg + "/containers/" + b + "/pathrules"); len(posts) != 1 {
		t.Errorf("want the replacement created under container b, got %v", posts)
	}
}

// Free-form config is Map(String) in Terraform while the API returns typed
// values (max_connections 300): state holds the string form, so the read after
// apply is consistent and re-planning is empty.
func TestMysqlTypedConfigReadsAsStrings(t *testing.T) {
	api := newFakeAPI(t)
	api.derived["mysql"] = func(record *bmap.Map) {
		cfg, ok := mustGet(record, "config").(*bmap.Map)
		if !ok {
			return
		}
		typed := bmap.NewMap()
		for _, k := range cfg.Keys() {
			v, _ := cfg.Get(k)
			if s, isString := v.(string); isString {
				if n, err := strconv.ParseInt(s, 10, 64); err == nil {
					typed.Set(k, n)
					continue
				}
				if f, err := strconv.ParseFloat(s, 64); err == nil {
					typed.Set(k, f)
					continue
				}
				if b, err := strconv.ParseBool(s); err == nil {
					typed.Set(k, b)
					continue
				}
			}
			typed.Set(k, v)
		}
		record.Set("config", typed)
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{{
			Config: `
resource "bahriya_mysql" "m" {
  name      = "Orders"
  handle    = "orders"
  tier      = "single"
  cpu       = 500
  memory    = 1024
  regions   = ["helsinki-1"]
  databases = [{ name = "app", username = "app-user" }]
  persistence = {
    storage = 10
  }
  tuning = {
    config = { max_connections = "300", long_query_time = "0.5", slow_query_log = "true" }
  }
}`,
			// The re-plan after apply must be empty (plugin-testing fails the step
			// otherwise): the string forms do not diff against the typed API values.
			Check: tfresource.ComposeAggregateTestCheckFunc(
				tfresource.TestCheckResourceAttr("bahriya_mysql.m", "tuning.config.max_connections", "300"),
				tfresource.TestCheckResourceAttr("bahriya_mysql.m", "tuning.config.long_query_time", "0.5"),
				tfresource.TestCheckResourceAttr("bahriya_mysql.m", "tuning.config.slow_query_log", "true"),
			),
		}},
	})
}

// max_versions is read-only in v1 (x-input-deprecated; nothing ever wrote it
// after create). A configuration that still sets it fails with Terraform's own
// read-only error; the upgrade guide quotes this text so users can search it.
func TestMaxVersionsInConfigIsTerraformsReadOnlyError(t *testing.T) {
	api := newFakeAPI(t)
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories(api),
		Steps: []tfresource.TestStep{{
			Config: `
resource "bahriya_secret" "s" {
  name         = "Key"
  handle       = "key"
  value        = "v1"
  max_versions = 5
}`,
			ExpectError: regexp.MustCompile(`Invalid Configuration for Read-Only Attribute[\s\S]*Cannot set value for this attribute as the provider has marked it as\s+read-only\. Remove the configuration line setting the value\.`),
		}},
	})
	if posts := api.Calls("POST"); len(posts) != 0 {
		t.Errorf("nothing may be created, got %v", posts)
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
