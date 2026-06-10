package transforms

import (
	"reflect"
	"testing"
)

func TestImplies_SetsImpliedWhenTriggerPresent(t *testing.T) {
	payload := map[string]any{
		"autoscalingmaxreplicas": "10",
	}
	rules := Implies{
		"autoscalingmaxreplicas": {"autoscalingenabled": true},
	}
	Apply(payload, rules)
	if payload["autoscalingenabled"] != true {
		t.Fatalf("expected autoscalingenabled=true, got %v", payload["autoscalingenabled"])
	}
}

func TestImplies_NoOpWhenTriggerAbsent(t *testing.T) {
	payload := map[string]any{
		"image": "nginx",
	}
	rules := Implies{
		"autoscalingmaxreplicas": {"autoscalingenabled": true},
	}
	Apply(payload, rules)
	if _, present := payload["autoscalingenabled"]; present {
		t.Fatalf("autoscalingenabled should not be set, got %v", payload["autoscalingenabled"])
	}
}

func TestImplies_NoOpWhenTriggerEmpty(t *testing.T) {
	cases := []map[string]any{
		{"trigger": ""},
		{"trigger": nil},
		{"trigger": []any{}},
		{"trigger": map[string]any{}},
	}
	rules := Implies{"trigger": {"implied": "yes"}}
	for i, payload := range cases {
		Apply(payload, rules)
		if _, present := payload["implied"]; present {
			t.Fatalf("case %d: implied should not be set for empty trigger", i)
		}
	}
}

func TestImplies_MultipleRulesIndependent(t *testing.T) {
	payload := map[string]any{
		"a": "value",
		"b": "",
	}
	rules := Implies{
		"a": {"a_implied": 1},
		"b": {"b_implied": 1},
	}
	Apply(payload, rules)
	want := map[string]any{
		"a":         "value",
		"b":         "",
		"a_implied": 1,
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("got %v want %v", payload, want)
	}
}
