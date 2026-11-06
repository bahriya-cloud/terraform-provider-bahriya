package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringFromAPI reads a string from a decoded API object, null when absent.
func stringFromAPI(raw map[string]any, key string) types.String {
	if v, ok := raw[key].(string); ok {
		return types.StringValue(v)
	}
	return types.StringNull()
}

// boolFromAPI reads a boolean from a decoded API object, null when absent.
func boolFromAPI(raw map[string]any, key string) types.Bool {
	if v, ok := raw[key].(bool); ok {
		return types.BoolValue(v)
	}
	return types.BoolNull()
}

// diagnostics is a tiny alias so generated files don't need to import the
// `diag` package directly — keeps generated imports minimal.
type diagnostics = diag.Diagnostics

// schemaPath wraps path.Root for use in generated ImportState methods.
func schemaPath(name string) path.Path {
	return path.Root(name)
}

// diagsContainNotFound reports whether diags include a "not_found" error
// (used by the hand-written Read methods to handle out-of-band deletes).
func diagsContainNotFound(diags diag.Diagnostics) bool {
	for _, d := range diags.Errors() {
		if d.Summary() == "not_found" {
			return true
		}
	}
	return false
}
