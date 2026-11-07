package provider

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Every docs example (examples/resources/*, examples/data-sources/*, and the
// provider example) is valid Terraform for this provider: `terraform validate`
// against a dev override, which never configures the provider. Each file
// stands alone, declaring the variables and resources it refers to.
// `make docs-validate` runs this test.
func TestDocsExamplesValidate(t *testing.T) {
	tf, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform is not on PATH")
	}
	root := filepath.Join("..", "..", "examples")
	var files []string
	for _, pattern := range []string{"resources/*/*.tf", "data-sources/*/*.tf", "provider/*.tf"} {
		matched, _ := filepath.Glob(filepath.Join(root, pattern))
		files = append(files, matched...)
	}
	if len(files) == 0 {
		t.Fatal("no examples found")
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
	versions := "terraform {\n  required_providers {\n    bahriya = { source = \"bahriya-cloud/bahriya\" }\n  }\n}\n"

	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		body := string(src)
		if !strings.Contains(body, "required_providers") {
			body = versions + body
		}
		if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(tf, "validate", "-no-color")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+rc)
		if out, err := cmd.CombinedOutput(); err != nil {
			rel, _ := filepath.Rel(root, file)
			t.Errorf("examples/%s does not validate:\n%s", rel, out)
		}
	}

	// Every other directory under examples/ with .tf files at its top level is
	// a whole configuration (complete, e2e-prod, …): validate its .tf files
	// together, so no example anywhere can drift back to old attribute names.
	// Only the .tf files are copied; state and logs beside them are ignored.
	modules, _ := filepath.Glob(filepath.Join(root, "*", "*.tf"))
	seen := map[string]bool{}
	for _, file := range modules {
		src := filepath.Dir(file)
		// provider/ holds standalone single-file examples, validated one by one above.
		if seen[src] || filepath.Base(src) == "provider" {
			continue
		}
		seen[src] = true
		tfs, _ := filepath.Glob(filepath.Join(src, "*.tf"))
		dir := t.TempDir()
		declared := false
		for _, f := range tfs {
			body, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			declared = declared || strings.Contains(string(body), "required_providers")
			if err := os.WriteFile(filepath.Join(dir, filepath.Base(f)), body, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if !declared {
			if err := os.WriteFile(filepath.Join(dir, "versions.tf"), []byte(versions), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(tf, "validate", "-no-color")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+rc)
		if out, err := cmd.CombinedOutput(); err != nil {
			rel, _ := filepath.Rel(root, src)
			t.Errorf("examples/%s/ does not validate:\n%s", rel, out)
		}
	}
}
