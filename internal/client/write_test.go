package client

import (
	"strings"
	"testing"
)

// The provider must keep working against both write shapes.
//
// Writes that consume project capacity answer with {id, quota}; everything else still answers with
// a bare id string. Before DecodeWrite the generated resources unmarshalled straight into a string,
// so the first shape failed with "Decode created id failed" — a message that says nothing about
// the project having run out of room, on a resource the practitioner had just sized correctly.
func TestDecodeWriteAcceptsABareIdString(t *testing.T) {
	id, verdict, err := DecodeWrite([]byte(`"065df92e-4e46-436a-a0a0-aaaaaaaaaaaa"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "065df92e-4e46-436a-a0a0-aaaaaaaaaaaa" {
		t.Fatalf("got id %q", id)
	}
	if verdict != nil {
		t.Fatalf("a bare id carries no verdict, got %+v", verdict)
	}
}

func TestDecodeWriteAcceptsTheEnvelope(t *testing.T) {
	id, verdict, err := DecodeWrite([]byte(`{"id":"abc-123","quota":{"blocked":false,"breaches":[],"warnings":[]}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "abc-123" {
		t.Fatalf("got id %q", id)
	}
	if verdict == nil || verdict.Blocked {
		t.Fatalf("expected a clear verdict, got %+v", verdict)
	}
}

// A null quota means the write was not checked against project limits at all — a resource that
// belongs to no project has no ceilings to check. That is distinct from "checked and clear", and
// the provider must not confuse the two.
func TestDecodeWriteTreatsANullQuotaAsAbsent(t *testing.T) {
	_, verdict, err := DecodeWrite([]byte(`{"id":"abc-123","quota":null}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != nil {
		t.Fatalf("null quota must decode to no verdict, got %+v", verdict)
	}
}

func TestDecodeWriteRejectsAnEnvelopeWithNoId(t *testing.T) {
	if _, _, err := DecodeWrite([]byte(`{"quota":null}`)); err == nil {
		t.Fatal("expected an error rather than an empty id silently reaching state")
	}
}

// These endpoints 409 for several reasons — a handle already taken, a hostname in use, a policy
// scoped elsewhere — and only the resource-limit refusal is structured. Misreading a plain sentence
// as a verdict would produce an empty, confident-looking error.
func TestDecodeQuotaRefusalIgnoresAPlainConflictMessage(t *testing.T) {
	if v := DecodeQuotaRefusal([]byte(`"A container with the handle \"my-api\" already exists"`)); v != nil {
		t.Fatalf("a sentence is not a verdict, got %+v", v)
	}
}

func TestDecodeQuotaRefusalReadsABlockingVerdict(t *testing.T) {
	body := `{"blocked":true,"message":"no room","breaches":[{"region":"falkenstein-1","label":"CPU ceiling","requested":"6600m","headroom":"6500m","available":"8000m"}],"warnings":[]}`

	verdict := DecodeQuotaRefusal([]byte(body))
	if verdict == nil {
		t.Fatal("expected a verdict")
	}

	detail := QuotaDetail(verdict)
	for _, want := range []string{"falkenstein-1", "CPU ceiling", "6600m", "6500m", "8000m"} {
		if !strings.Contains(detail, want) {
			t.Errorf("diagnostic is missing %q:\n%s", want, detail)
		}
	}
}

// A verdict that is not blocking is not a refusal, whatever status it arrived with.
func TestDecodeQuotaRefusalIgnoresANonBlockingVerdict(t *testing.T) {
	if v := DecodeQuotaRefusal([]byte(`{"blocked":false,"breaches":[],"warnings":[]}`)); v != nil {
		t.Fatalf("expected nil, got %+v", v)
	}
}

// The advisory has to name the replica count that IS reachable. "Your maximum is out of reach"
// leaves the reader to derive it, and the obvious calculation gives the wrong answer because the
// binding axis is normally the limit rather than the request.
func TestQuotaWarningNamesTheReachableReplicaCount(t *testing.T) {
	verdict := &QuotaVerdict{
		Warning:            "cannot reach the configured maximum",
		Warnings:           []QuotaBreach{{Region: "falkenstein-1", Axis: "limits.cpu"}},
		AttainableReplicas: map[string]int{"falkenstein-1": 5},
	}

	warning := QuotaWarning(verdict)
	if !strings.Contains(warning, "falkenstein-1: reaches 5 replicas") {
		t.Fatalf("warning does not state what is reachable:\n%s", warning)
	}
}

func TestQuotaWarningIsEmptyWhenThereIsNothingToAdvise(t *testing.T) {
	if w := QuotaWarning(&QuotaVerdict{Blocked: false}); w != "" {
		t.Fatalf("expected no warning, got %q", w)
	}
	if w := QuotaWarning(nil); w != "" {
		t.Fatalf("expected no warning for a nil verdict, got %q", w)
	}
}
