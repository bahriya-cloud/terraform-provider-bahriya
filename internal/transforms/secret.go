package transforms

import "sort"

// SecretsToAPI converts a Terraform map[string]string (secret handle -> env
// var name) into the API's [{secret, name}] array form. Keys are emitted in
// sorted order for payload stability.
func SecretsToAPI(secrets map[string]string) []map[string]string {
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]map[string]string, 0, len(secrets))
	for _, k := range keys {
		out = append(out, map[string]string{"secret": k, "name": secrets[k]})
	}
	return out
}

// SecretsFromAPI converts the API's [{secret, name}] form back into a
// map[string]string for Terraform state.
func SecretsFromAPI(items []map[string]any) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		handle, ok1 := item["secret"].(string)
		name, ok2 := item["name"].(string)
		if ok1 && ok2 {
			out[handle] = name
		}
	}
	return out
}
