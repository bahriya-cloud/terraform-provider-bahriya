// Package acceptance holds the live end-to-end matrix for provider v1 (epic
// 2223, card 2262). It runs only with BAHRIYA_ACC=1 and credentials, and only
// against the local dev API unless BAHRIYA_ACC_ALLOW_REMOTE=1: every case
// creates real resources in the organisation the credentials name.
//
//	BAHRIYA_ACC=1 BAHRIYA_API_URL=http://localhost:8080/console/v1 \
//	BAHRIYA_TOKEN=pat_... BAHRIYA_ORGANISATION_ID=... go test ./internal/acceptance -run TestAcc -v -timeout 60m
package acceptance

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const region = "falkenstein-1"

// v1 is the provider under test, built from this tree.
func v1() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"bahriya": providerserver.NewProtocol6WithError(provider.New("acc")()),
	}
}

// preCheck skips unless the matrix was asked for, and refuses a remote API
// unless that was asked for too.
func preCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("BAHRIYA_ACC") != "1" {
		t.Skip("set BAHRIYA_ACC=1 with BAHRIYA_API_URL, BAHRIYA_TOKEN and BAHRIYA_ORGANISATION_ID to run the live matrix")
	}
	for _, k := range []string{provider.EnvAPIURL, provider.EnvToken, provider.EnvOrganisationID} {
		if os.Getenv(k) == "" {
			t.Fatalf("%s is required with BAHRIYA_ACC=1", k)
		}
	}
	url := os.Getenv(provider.EnvAPIURL)
	if !strings.Contains(url, "localhost") && !strings.Contains(url, "127.0.0.1") && os.Getenv("BAHRIYA_ACC_ALLOW_REMOTE") != "1" {
		t.Fatalf("%s is %s: the matrix creates real resources, so a remote API needs BAHRIYA_ACC_ALLOW_REMOTE=1 (and Meezaan's yes for production)", provider.EnvAPIURL, url)
	}
	// os.Setenv, not t.Setenv: the cases run in parallel, and Go refuses
	// t.Parallel under a test that used t.Setenv. TF_ACC only gates plugin-testing.
	if err := os.Setenv("TF_ACC", "1"); err != nil {
		t.Fatal(err)
	}
}

// deleteLock is how long after creation a project can be deleted:
// BAHRIYA_ACC_DELETE_LOCK (a Go duration), else 65s locally (the dev API's one
// minute) and 11m elsewhere (production's ten minutes).
func deleteLock(t *testing.T) time.Duration {
	t.Helper()
	if v := os.Getenv("BAHRIYA_ACC_DELETE_LOCK"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			t.Fatalf("BAHRIYA_ACC_DELETE_LOCK: %v", err)
		}
		return d
	}
	if local() {
		return 65 * time.Second
	}
	return 11 * time.Minute
}

// suffix keeps handles unique in an organisation that keeps old debris.
func suffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// material generates real key material, as terraform-e2e's gen-fixtures.sh
// does, and returns the paths by name.
func material(t *testing.T, sfx string) map[string]string {
	t.Helper()
	dir := t.TempDir()
	p := func(name string) string { return filepath.Join(dir, name) }
	run := func(env []string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run(nil, "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", p("ca.key"), "-out", p("ca.pem"),
		"-days", "30", "-subj", "/CN=Bahriya Acc CA "+sfx)
	run(nil, "openssl", "req", "-newkey", "rsa:2048", "-nodes", "-keyout", p("server.key"), "-out", p("server.csr"),
		"-subj", "/CN=acc-"+sfx+".bahriya.test")
	run(nil, "openssl", "x509", "-req", "-in", p("server.csr"), "-CA", p("ca.pem"), "-CAkey", p("ca.key"),
		"-CAcreateserial", "-days", "30", "-sha256", "-out", p("server.pem"))
	run(nil, "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", p("x509.key"), "-out", p("x509.pem"),
		"-days", "30", "-subj", "/CN=acc-x509-"+sfx+".bahriya.test")
	run(nil, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "acc-"+sfx+"@bahriya.test", "-f", p("ssh"))
	gnupg := t.TempDir()
	genv := []string{"GNUPGHOME=" + gnupg}
	run(genv, "gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "", "--quick-generate-key",
		"Bahriya Acc "+sfx+" <acc-"+sfx+"@bahriya.test>", "ed25519", "default", "never")
	run(genv, "sh", "-c", "gpg --export --armor > "+p("gpg.pub")+" && gpg --export-secret-keys --armor --pinentry-mode loopback --passphrase '' > "+p("gpg.key"))
	run(nil, "sh", "-c", "openssl rand -base64 32 | tr -d '\\n' > "+p("aes.b64"))
	return map[string]string{
		"ca": p("ca.pem"), "cert": p("server.pem"), "key": p("server.key"), "x509": p("x509.pem"),
		"ssh_pub": p("ssh.pub"), "ssh_key": p("ssh"), "gpg_pub": p("gpg.pub"), "gpg_key": p("gpg.key"), "aes": p("aes.b64"),
	}
}

// matrixCase is one resource kind: a configuration for create and one for
// update (each may declare what it depends on), the address under test, how to
// build its import ID, and the attributes an import cannot recover because the
// API never returns them.
type matrixCase struct {
	name     string
	address  string
	create   string
	update   string
	importID func(s *terraform.State) (string, error)
	ignore   []string
}

func byID(address string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[address]
		if !ok {
			return "", fmt.Errorf("%s not in state", address)
		}
		return rs.Primary.ID, nil
	}
}

// joined builds "<parent attribute>:<own attribute>" import IDs.
func joined(address, parentAttr, ownAttr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[address]
		if !ok {
			return "", fmt.Errorf("%s not in state", address)
		}
		return rs.Primary.Attributes[parentAttr] + ":" + rs.Primary.Attributes[ownAttr], nil
	}
}

func project(sfx string) string {
	return fmt.Sprintf(`
resource "bahriya_project" "p" {
  name    = "Acc %[1]s"
  handle  = "acc-%[1]s"
  regions = [%[2]q]
}
`, sfx, region)
}

func httpContainer(sfx string, replicas int) string {
	return project(sfx) + fmt.Sprintf(`
resource "bahriya_container" "web" {
  name    = "Acc Web"
  handle  = "acc-web-%[1]s"
  project = bahriya_project.p.id
  regions = [%[2]q]

  workload = {
    image  = "nginx:1.27-alpine"
    port   = 80
    cpu    = 100
    memory = 128
    env = {
      "MODE" = "acc"
    }
  }

  scaling = {
    replicas = %[3]d
  }

  networking = {
    external = true
  }
}
`, sfx, region, replicas)
}

func cases(sfx string, m map[string]string) []matrixCase {
	item := func(kind, body string) string {
		return fmt.Sprintf("resource \"bahriya_%s\" \"i\" {\n%s\n}\n", kind, body)
	}
	named := func(kind, name, extra string) string {
		return item(kind, fmt.Sprintf("  name   = %q\n  handle = \"acc-%s-%s\"\n%s", name, strings.ReplaceAll(kind, "_", "-"), sfx, extra))
	}
	file := func(key string) string { return fmt.Sprintf("file(%q)", m[key]) }
	vault := []struct{ kind, extra string; ignore []string }{
		{"secret", "  value = \"acc-value\"", []string{"value"}},
		{"registry", "  server   = \"ghcr.io\"\n  username = \"acc\"\n  password = \"acc-token\"", []string{"password"}},
		{"tls_bundle", "  ca   = " + file("ca") + "\n  cert = " + file("cert") + "\n  key  = " + file("key"), []string{"key"}},
		{"x509_cert", "  cert = " + file("x509"), nil},
		{"ssh_keypair", "  public_key  = " + file("ssh_pub") + "\n  private_key = " + file("ssh_key"), []string{"private_key"}},
		{"gpg_keypair", "  public_key  = " + file("gpg_pub") + "\n  private_key = " + file("gpg_key"), []string{"private_key"}},
		{"encryption_key", "  format = \"base64\"\n  key    = " + file("aes"), []string{"key"}},
		{"env_file", "  content = \"MODE=acc\\n\"", nil},
		{"yaml_config", "  content = \"mode: acc\\n\"", nil},
		{"json_config", "  content = jsonencode({ mode = \"acc\" })", nil},
		{"plain_config", "  content = \"mode acc\\n\"", nil},
		{"network_policy", "  egress_cidrs = [\"10.0.0.0/8\"]", nil},
	}
	var out []matrixCase
	for _, v := range vault {
		out = append(out, matrixCase{
			name:     v.kind,
			address:  "bahriya_" + v.kind + ".i",
			create:   named(v.kind, "Acc "+v.kind, v.extra),
			update:   named(v.kind, "Acc "+v.kind+" renamed", v.extra),
			importID: byID("bahriya_" + v.kind + ".i"),
			ignore:   v.ignore,
		})
		// The project attachment for the same kind: attach, then import by
		// "<project id>:<handle>". Its update step changes nothing (an
		// attachment has no mutable attribute), so it re-applies the same
		// configuration and must plan empty.
		attach := project(sfx) + named(v.kind, "Acc "+v.kind, v.extra) + fmt.Sprintf(`
resource "bahriya_project_%[1]s_attachment" "a" {
  project_id = bahriya_project.p.id
  handle     = bahriya_%[1]s.i.handle
}
`, v.kind)
		out = append(out, matrixCase{
			name:     "project_" + v.kind + "_attachment",
			address:  "bahriya_project_" + v.kind + "_attachment.a",
			create:   attach,
			update:   attach,
			importID: joined("bahriya_project_"+v.kind+"_attachment.a", "project_id", "handle"),
			// The attachments list does not return the join id, so an import
			// cannot recover it (the attribute's description says so).
			ignore: []string{"join_id"},
		})
	}
	out = append(out,
		matrixCase{
			name: "project", address: "bahriya_project.p",
			create: project(sfx), update: strings.Replace(project(sfx), `"Acc `+sfx+`"`, `"Acc `+sfx+` renamed"`, 1),
			importID: byID("bahriya_project.p"),
		},
		matrixCase{
			name: "container_http", address: "bahriya_container.web",
			create: httpContainer(sfx, 1), update: httpContainer(sfx, 2),
			importID: byID("bahriya_container.web"),
		},
		matrixCase{
			name: "container_worker", address: "bahriya_container.w",
			create: project(sfx) + worker(sfx, 100), update: project(sfx) + worker(sfx, 200),
			importID: byID("bahriya_container.w"),
		},
		matrixCase{
			name: "container_cronjob", address: "bahriya_container.c",
			create: project(sfx) + cronjob(sfx, "*/30 * * * *"), update: project(sfx) + cronjob(sfx, "0 * * * *"),
			importID: byID("bahriya_container.c"),
		},
		matrixCase{
			name: "path_rule", address: "bahriya_path_rule.r",
			create: httpContainer(sfx, 1) + pathRule("/acc", 60), update: httpContainer(sfx, 1) + pathRule("/acc", 120),
			importID: joined("bahriya_path_rule.r", "container", "id"),
		},
		matrixCase{
			name: "memcached", address: "bahriya_memcached.m",
			create: project(sfx) + memcached(sfx, 64), update: project(sfx) + memcached(sfx, 128),
			importID: byID("bahriya_memcached.m"),
		},
		matrixCase{
			name: "valkey", address: "bahriya_valkey.v",
			create: project(sfx) + valkey(sfx, "Acc Valkey"), update: project(sfx) + valkey(sfx, "Acc Valkey renamed"),
			importID: byID("bahriya_valkey.v"), ignore: []string{"security.password"},
		},
		matrixCase{
			name: "mysql", address: "bahriya_mysql.d",
			create: project(sfx) + mysql(sfx, "Acc MySQL"), update: project(sfx) + mysql(sfx, "Acc MySQL renamed"),
			importID: byID("bahriya_mysql.d"),
		},
		matrixCase{
			name: "role", address: "bahriya_role.r",
			create: role(sfx, "read"), update: role(sfx, "update"),
			importID: byID("bahriya_role.r"),
		},
	)
	return out
}

func worker(sfx string, cpu int) string {
	return fmt.Sprintf(`
resource "bahriya_container" "w" {
  name    = "Acc Worker"
  handle  = "acc-worker-%[1]s"
  type    = "worker"
  project = bahriya_project.p.id
  regions = [%[2]q]

  workload = {
    image   = "busybox:1.37"
    cpu     = %[3]d
    memory  = 64
    command = ["sh", "-c", "sleep 3600"]
  }
}
`, sfx, region, cpu)
}

func cronjob(sfx, schedule string) string {
	return fmt.Sprintf(`
resource "bahriya_container" "c" {
  name    = "Acc Cron"
  handle  = "acc-cron-%[1]s"
  type    = "cronjob"
  project = bahriya_project.p.id
  regions = [%[2]q]

  workload = {
    image   = "busybox:1.37"
    cpu     = 100
    memory  = 64
    command = ["sh", "-c", "date"]
  }

  schedule = {
    schedule = %[3]q
  }
}
`, sfx, region, schedule)
}

func pathRule(path string, perMinute int) string {
	return fmt.Sprintf(`
resource "bahriya_path_rule" "r" {
  container = bahriya_container.web.id
  handle    = "acc"
  path      = %q

  security = {
    rate_limiting = {
      enabled    = true
      per_minute = %d
    }
  }
}
`, path, perMinute)
}

func memcached(sfx string, memory int) string {
	return fmt.Sprintf(`
resource "bahriya_memcached" "m" {
  name    = "Acc Memcached"
  handle  = "acc-mc-%[1]s"
  project = bahriya_project.p.id
  regions = [%[2]q]

  tuning = {
    memory = %[3]d
  }
}
`, sfx, region, memory)
}

func valkey(sfx, name string) string {
	return fmt.Sprintf(`
resource "bahriya_valkey" "v" {
  name    = %[3]q
  handle  = "acc-vk-%[1]s"
  purpose = "cache"
  tier    = "single"
  memory  = 256
  project = bahriya_project.p.id
  regions = [%[2]q]
}
`, sfx, region, name)
}

func mysql(sfx, name string) string {
	return fmt.Sprintf(`
resource "bahriya_mysql" "d" {
  name    = %[3]q
  handle  = "acc-db-%[1]s"
  tier    = "single"
  cpu     = 1000
  memory  = 2048
  project = bahriya_project.p.id
  regions = [%[2]q]

  persistence = {
    storage = 10
  }

  databases = [
    { name = "acc", username = "acc-app" },
  ]
}
`, sfx, region, name)
}

func role(sfx, permission string) string {
	return fmt.Sprintf(`
resource "bahriya_role" "r" {
  name        = "Acc Role %[1]s"
  description = "Created by the provider acceptance matrix."
  permissions = [
    { level = "organisation", resource = "attachables_registries", permission = "read" },
    { level = "organisation", resource = "attachables_secrets", permission = %[2]q },
  ]
}
`, sfx, permission)
}

// TestAccMatrix: per kind, create (then plan empty: applying twice changes
// nothing), update (then plan empty), import and verify against state, and
// destroy. BAHRIYA_ACC_ONLY=a,b limits the run to those case names.
func TestAccMatrix(t *testing.T) {
	preCheck(t)
	sfx := suffix(t)
	m := material(t, sfx)
	deployer := startDeployer(t)
	only := map[string]bool{}
	for _, n := range strings.Split(os.Getenv("BAHRIYA_ACC_ONLY"), ",") {
		if n != "" {
			only[n] = true
		}
	}
	for i, c := range cases(sfx, m) {
		if len(only) > 0 && !only[c.name] {
			continue
		}
		// A deleted project keeps its handle (soft delete), so every case gets
		// its own suffix rather than reusing one project handle.
		caseSfx := suffix(t)
		deployer.watch(caseSfx)
		c = cases(caseSfx, m)[i]
		t.Run(c.name, func(t *testing.T) {
			// Cases share nothing but the key material, so they run side by side
			// (the delete lock would otherwise add a minute or more to each).
			t.Parallel()
			started := time.Now()
			steps := []tfresource.TestStep{
				{Config: c.create, Check: tfresource.TestCheckResourceAttrSet(c.address, "id")},
				{Config: c.update},
				{
					Config:                  c.update,
					ResourceName:            c.address,
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateIdFunc:       c.importID,
					ImportStateVerifyIgnore: c.ignore,
				},
			}
			if strings.Contains(c.create, `resource "bahriya_project"`) {
				// A project cannot be deleted within the delete lock of its
				// creation (by design), so the destroy waits it out: a last step
				// that changes nothing, after the lock has passed.
				steps = append(steps, tfresource.TestStep{
					PreConfig: func() { time.Sleep(time.Until(started.Add(deleteLock(t)))) },
					Config:    c.update,
				})
			}
			tfresource.Test(t, tfresource.TestCase{ProtoV6ProviderFactories: v1(), Steps: steps})
		})
	}
}

// v0Config is the same estate written for provider 0.x: a project, a secret,
// an HTTP container that reads it, a Memcached instance, a network policy and an
// env file.
func v0Config(sfx string) string {
	return fmt.Sprintf(`
resource "bahriya_project" "p" {
  name    = "Acc Upgrade %[1]s"
  handle  = "acc-up-%[1]s"
  regions = [%[2]q]
}

resource "bahriya_secret" "s" {
  name   = "Acc Upgrade Secret"
  handle = "acc-up-secret-%[1]s"
  value  = "acc-value"
}

resource "bahriya_project_secret_attachment" "a" {
  project_id = bahriya_project.p.id
  handle     = bahriya_secret.s.handle
}

resource "bahriya_container" "web" {
  name            = "Acc Upgrade Web"
  handle          = "acc-up-web-%[1]s"
  image           = "nginx:1.27-alpine"
  containerport   = "80"
  healthcheckpath = "/"
  mincpu          = "100"
  minmemory       = "128"
  activeregions   = [%[2]q]
  project         = bahriya_project.p.id

  autoscalingminreplicas = "1"

  newenvvar = [
    { key = "MODE", value = "acc" },
  ]

  secretsenvvar = [
    { name = "ACC_SECRET", secret = bahriya_project_secret_attachment.a.handle },
  ]
}

resource "bahriya_memcached" "m" {
  name          = "Acc Upgrade Memcached"
  handle        = "acc-up-mc-%[1]s"
  memorymb      = 64
  activeregions = [%[2]q]
  project       = bahriya_project.p.id
}

resource "bahriya_network_policy" "n" {
  name        = "Acc Upgrade Policy"
  handle      = "acc-up-np-%[1]s"
  egresscidrs = ["10.0.0.0/8"]
}

resource "bahriya_env_file" "e" {
  name    = "Acc Upgrade Env"
  handle  = "acc-up-env-%[1]s"
  content = "MODE=acc\n"
}
`, sfx, region)
}

// v1Config is v0Config rewritten for version 1, following the upgrade guide.
func v1Config(sfx string) string {
	return fmt.Sprintf(`
resource "bahriya_project" "p" {
  name    = "Acc Upgrade %[1]s"
  handle  = "acc-up-%[1]s"
  regions = [%[2]q]
}

resource "bahriya_secret" "s" {
  name   = "Acc Upgrade Secret"
  handle = "acc-up-secret-%[1]s"
  value  = "acc-value"
}

resource "bahriya_project_secret_attachment" "a" {
  project_id = bahriya_project.p.id
  handle     = bahriya_secret.s.handle
}

resource "bahriya_container" "web" {
  name    = "Acc Upgrade Web"
  handle  = "acc-up-web-%[1]s"
  project = bahriya_project.p.id
  regions = [%[2]q]

  workload = {
    image       = "nginx:1.27-alpine"
    port        = 80
    healthcheck = "/"
    cpu         = 100
    memory      = 128

    env = {
      "MODE" = "acc"
    }

    secrets = {
      "ACC_SECRET" = bahriya_project_secret_attachment.a.handle
    }
  }

  networking = {
    # The 0.x container was public: keep it public.
    external = true
  }
}

resource "bahriya_memcached" "m" {
  name    = "Acc Upgrade Memcached"
  handle  = "acc-up-mc-%[1]s"
  project = bahriya_project.p.id
  regions = [%[2]q]

  tuning = {
    memory = 64
  }
}

resource "bahriya_network_policy" "n" {
  name         = "Acc Upgrade Policy"
  handle       = "acc-up-np-%[1]s"
  egress_cidrs = ["10.0.0.0/8"]
}

resource "bahriya_env_file" "e" {
  name    = "Acc Upgrade Env"
  handle  = "acc-up-env-%[1]s"
  content = "MODE=acc\n"
}
`, sfx, region)
}

// TestAccUpgradeFromV0 applies the estate with a released 0.x provider from
// the Terraform Registry, then plans the rewritten configuration with version
// 1: the state upgrades on that plan and the plan must be empty.
func TestAccUpgradeFromV0(t *testing.T) {
	preCheck(t)
	deployer := startDeployer(t)
	for _, version := range []string{"0.4.0", "0.3.8"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			sfx := suffix(t)
			deployer.watch(sfx)
			started := time.Now()
			tfresource.Test(t, tfresource.TestCase{
				Steps: []tfresource.TestStep{
					{
						ExternalProviders: map[string]tfresource.ExternalProvider{
							"bahriya": {Source: "bahriya-cloud/bahriya", VersionConstraint: version},
						},
						Config: v0Config(sfx),
					},
					{
						ProtoV6ProviderFactories: v1(),
						Config:                   v1Config(sfx),
						ConfigPlanChecks: tfresource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
					{
						// Wait out the project delete lock before the destroy.
						PreConfig:                func() { time.Sleep(time.Until(started.Add(deleteLock(t)))) },
						ProtoV6ProviderFactories: v1(),
						Config:                   v1Config(sfx),
					},
				},
			})
		})
	}
}
