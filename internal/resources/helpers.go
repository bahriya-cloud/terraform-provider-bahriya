package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// knownValue resolves a sensitive attribute's value to persist after an apply.
//
// Sensitive attributes are Optional+Computed with no plan modifier, so whenever
// the practitioner leaves one out of config the framework plans it as UNKNOWN on
// any diff — and an unknown value must never be written to final state. The API
// never returns these (that is what makes them sensitive), so there is nothing
// to read back either. Fall back to the prior value: null on create, the
// previous state on update.
//
// Lives here rather than in a generated file because every generated resource
// shares this package — emitting it per-file would be a redeclaration.
func knownValue(planned, prior types.String) types.String {
	if planned.IsUnknown() {
		return prior
	}
	return planned
}
