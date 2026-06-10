package transforms

// Implies maps a trigger field name to a set of fields that must be set when
// the trigger is present and non-empty. Mirrors the OpenAPI `x-implies`
// extension (e.g. setting `autoscalingmaxreplicas` implies
// `autoscalingenabled: true`).
type Implies map[string]map[string]any

// Apply walks the payload and sets implied fields where the trigger field is
// present with a non-empty value. Empty strings, nil, empty slices, and
// empty maps are treated as "not set" and do not trigger implication.
func Apply(payload map[string]any, rules Implies) {
	for trigger, sets := range rules {
		v, ok := payload[trigger]
		if !ok || isEmpty(v) {
			continue
		}
		for k, val := range sets {
			payload[k] = val
		}
	}
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}
