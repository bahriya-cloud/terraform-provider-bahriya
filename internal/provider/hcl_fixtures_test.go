package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/engine"
)

// pendingHCL are kinds with hcl fixtures whose resource is not yet on the v1
// shape, with the card that moves it. The list fails once a kind is served by
// the engine, so it cannot outlive the work.
var pendingHCL = map[string]string{}

// RULES.md section 8: every hcl conformance case is valid Terraform for the
// v1 schema. Each case is validated with `terraform validate` against this
// provider through a dev override, which never configures the provider.
func TestHCLFixturesValidate(t *testing.T) {
	tf, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform is not on PATH")
	}
	for kind := range pendingHCL {
		if _, served := engine.Descriptors[kind]; served {
			t.Errorf("%s is served by the engine: remove it from pendingHCL", kind)
		}
	}

	bin := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(bin, "terraform-provider-bahriya"), "../../cmd/terraform-provider-bahriya")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	rc := filepath.Join(bin, "rc")
	override := fmt.Sprintf("provider_installation {\n  dev_overrides { \"bahriya-cloud/bahriya\" = %q }\n  direct {}\n}\n", bin)
	if err := os.WriteFile(rc, []byte(override), 0o600); err != nil {
		t.Fatal(err)
	}

	validated := 0
	for _, c := range hclCases(t) {
		if reason, pending := pendingHCL[c.Kind]; pending {
			t.Logf("%s: not validated: %s", c.ID, reason)
			continue
		}
		dir := t.TempDir()
		versions := "terraform {\n  required_providers {\n    bahriya = { source = \"bahriya-cloud/bahriya\" }\n  }\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(versions+c.Expect), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(tf, "validate", "-json", "-no-color")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+rc)
		out, _ := cmd.Output()
		var result struct {
			Diagnostics []struct {
				Severity, Summary, Detail string
			} `json:"diagnostics"`
		}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("%s: terraform validate output: %v\n%s", c.ID, err, out)
		}
		// A case is a minimal document focused on one rule, so it may leave out
		// unrelated required fields. Every other error is a real one: block
		// syntax, an unknown or misplaced name, a value of the wrong type.
		var problems []string
		for _, d := range result.Diagnostics {
			if d.Severity == "error" && d.Summary != "Missing required argument" {
				problems = append(problems, d.Summary+": "+d.Detail)
			}
		}
		if len(problems) > 0 {
			t.Errorf("%s does not validate:\n%s\n%s", c.ID, c.Expect, strings.Join(problems, "\n"))
			continue
		}
		validated++
	}
	if validated == 0 {
		t.Fatal("no hcl case was validated")
	}
}

type hclCase struct {
	ID, Kind, Type string
	Expect         any
}

type validCase struct{ ID, Kind, Expect string }

// hclCases reads the hcl cases from bahriya/openapi: BAHRIYA_OPENAPI_DIR, or
// the monorepo checkout beside this repository.
func hclCases(t *testing.T) []validCase {
	t.Helper()
	dir := os.Getenv("BAHRIYA_OPENAPI_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "packages", "bahriya-openapi")
	}
	files, err := filepath.Glob(filepath.Join(dir, "conformance", "*", "cases.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no conformance cases under %s (set BAHRIYA_OPENAPI_DIR): %v", dir, err)
	}
	var out []validCase
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var cases []hclCase
		if err := json.Unmarshal(data, &cases); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, c := range cases {
			if c.Type == "hcl" {
				text, _ := c.Expect.(string)
				out = append(out, validCase{ID: c.ID, Kind: c.Kind, Expect: strings.TrimSpace(text) + "\n"})
			}
		}
	}
	return out
}
