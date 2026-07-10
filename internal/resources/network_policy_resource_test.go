package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// apiToNetworkPolicyModel + planToNetworkPolicyPayload form the wire
// round-trip for the network policy resource. Network policy is the first
// *generated* attachable that carries both string lists (ingress peers,
// egress CIDRs/FQDNs) and a nested list (ports), so these tests lock the
// smushed field shape and the nested port projection.

func TestApiToNetworkPolicyModel(t *testing.T) {
	raw := map[string]any{
		"id":           "11111111-1111-1111-1111-111111111111",
		"handle":       "web-tier",
		"name":         "Web tier ingress",
		"ingresspeers": []any{"frontend", "gateway"},
		"egresscidrs":  []any{"10.0.0.0/8", "203.0.113.0/24"},
		"egressfqdns":  []any{"api.example.com"},
		"ports":        []any{map[string]any{"port": float64(443), "protocol": "TCP"}},
		"billable":     true,
		"organisation": "22222222-2222-2222-2222-222222222222",
	}

	m := apiToNetworkPolicyModel(raw)

	if got := m.ID.ValueString(); got != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", got)
	}
	if got := m.Handle.ValueString(); got != "web-tier" {
		t.Errorf("Handle = %q", got)
	}
	if got := m.Name.ValueString(); got != "Web tier ingress" {
		t.Errorf("Name = %q", got)
	}
	var peers []string
	m.Ingresspeers.ElementsAs(context.Background(), &peers, false)
	if len(peers) != 2 || peers[0] != "frontend" || peers[1] != "gateway" {
		t.Errorf("Ingresspeers = %v", peers)
	}
	var cidrs []string
	m.Egresscidrs.ElementsAs(context.Background(), &cidrs, false)
	if len(cidrs) != 2 || cidrs[0] != "10.0.0.0/8" {
		t.Errorf("Egresscidrs = %v", cidrs)
	}
	var fqdns []string
	m.Egressfqdns.ElementsAs(context.Background(), &fqdns, false)
	if len(fqdns) != 1 || fqdns[0] != "api.example.com" {
		t.Errorf("Egressfqdns = %v", fqdns)
	}
	var ports []network_policyPortsModel
	m.Ports.ElementsAs(context.Background(), &ports, false)
	if len(ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(ports))
	}
	if got := ports[0].Port.ValueInt64(); got != 443 {
		t.Errorf("port = %d", got)
	}
	if got := ports[0].Protocol.ValueString(); got != "TCP" {
		t.Errorf("protocol = %q", got)
	}
	if !m.Billable.ValueBool() {
		t.Error("Billable expected true")
	}
	if got := m.Organisation.ValueString(); got != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("Organisation = %q", got)
	}
}

func TestPlanToNetworkPolicyPayload(t *testing.T) {
	peers, _ := types.ListValue(types.StringType, []attr.Value{
		types.StringValue("frontend"),
	})
	cidrs, _ := types.ListValue(types.StringType, []attr.Value{
		types.StringValue("10.0.0.0/8"),
	})
	portObj, _ := types.ObjectValue(network_policyPortsAttrTypes(), map[string]attr.Value{
		"port":     types.Int64Value(443),
		"protocol": types.StringValue("TCP"),
	})
	ports, _ := types.ListValue(network_policyPortsObjectType(), []attr.Value{portObj})

	m := &network_policyModel{
		Handle:       types.StringValue("web-tier"),
		Name:         types.StringValue("Web tier ingress"),
		Ingresspeers: peers,
		Egresscidrs:  cidrs,
		Egressfqdns:  types.ListNull(types.StringType),
		Ports:        ports,
	}

	out, diags := planToNetworkPolicyPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if out["handle"] != "web-tier" {
		t.Errorf("handle = %v", out["handle"])
	}
	if out["name"] != "Web tier ingress" {
		t.Errorf("name = %v", out["name"])
	}
	gotPeers, _ := out["ingresspeers"].([]string)
	if len(gotPeers) != 1 || gotPeers[0] != "frontend" {
		t.Errorf("ingresspeers = %v", out["ingresspeers"])
	}
	gotCidrs, _ := out["egresscidrs"].([]string)
	if len(gotCidrs) != 1 || gotCidrs[0] != "10.0.0.0/8" {
		t.Errorf("egresscidrs = %v", out["egresscidrs"])
	}
	// Null list still emits an empty slice so an update clears the field.
	gotFqdns, ok := out["egressfqdns"].([]string)
	if !ok || len(gotFqdns) != 0 {
		t.Errorf("egressfqdns = %v (want empty slice)", out["egressfqdns"])
	}
	gotPorts, _ := out["ports"].([]map[string]any)
	if len(gotPorts) != 1 {
		t.Fatalf("expected 1 port, got %v", out["ports"])
	}
	if gotPorts[0]["port"] != int64(443) {
		t.Errorf("port = %v", gotPorts[0]["port"])
	}
	if gotPorts[0]["protocol"] != "TCP" {
		t.Errorf("protocol = %v", gotPorts[0]["protocol"])
	}
}
