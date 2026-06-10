package transforms

import "sort"

// EnvVarsToAPI converts a Terraform map[string]string (KEY -> value) into the
// API's [{name, value}] array form. Keys are emitted in sorted order so the
// payload is stable across runs.
func EnvVarsToAPI(env map[string]string) []map[string]string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]map[string]string, 0, len(env))
	for _, k := range keys {
		out = append(out, map[string]string{"name": k, "value": env[k]})
	}
	return out
}

// EnvVarsFromAPI converts the API's [{name, value}] form back into a
// map[string]string for Terraform state.
func EnvVarsFromAPI(items []map[string]any) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		name, ok1 := item["name"].(string)
		value, ok2 := item["value"].(string)
		if ok1 && ok2 {
			out[name] = value
		}
	}
	return out
}
