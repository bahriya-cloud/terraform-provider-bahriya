package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// diagnostics is a tiny alias so generated files don't need to import the
// `diag` package directly — keeps generated imports minimal.
type diagnostics = diag.Diagnostics

// schemaPath wraps path.Root for use in generated ImportState methods.
func schemaPath(name string) path.Path {
	return path.Root(name)
}

// diagsContainNotFound reports whether diags include a "not_found" error
// (used by generated Read methods to handle out-of-band deletes).
func diagsContainNotFound(diags diag.Diagnostics) bool {
	for _, d := range diags.Errors() {
		if d.Summary() == "not_found" {
			return true
		}
	}
	return false
}
