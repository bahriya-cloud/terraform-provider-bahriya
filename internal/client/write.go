package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// QuotaBreach is one project resource ceiling, in one region, that a change would exceed or could
// not grow into.
type QuotaBreach struct {
	Region    string   `json:"region"`
	Axis      string   `json:"axis"`
	Label     string   `json:"label"`
	Project   string   `json:"project"`
	Used      string   `json:"used"`
	Requested string   `json:"requested"`
	Available string   `json:"available"`
	Headroom  string   `json:"headroom"`
	Blamed    []string `json:"blamed"`
}

// QuotaVerdict is how a change sits against the project's resource limits.
//
// Returned two ways. A refusal makes it the whole response body; a write that succeeded carries it
// alongside the new id, where it may still report that the workload cannot grow to the size it was
// configured for.
type QuotaVerdict struct {
	Blocked            bool           `json:"blocked"`
	Message            string         `json:"message"`
	Warning            string         `json:"warning"`
	Breaches           []QuotaBreach  `json:"breaches"`
	Warnings           []QuotaBreach  `json:"warnings"`
	AttainableReplicas map[string]int `json:"attainablereplicas"`
}

// writeResult is the body of a successful write to a resource that consumes project capacity.
type writeResult struct {
	ID    string        `json:"id"`
	Quota *QuotaVerdict `json:"quota"`
}

// DecodeWrite reads the id out of a create or update response, whichever shape it arrived in.
//
// Writes that consume project capacity answer with {id, quota} so they can report what the change
// cost the project; every other write answers with the id as a bare JSON string. Decoding straight
// into a string — which is what the generated resources did — turns the first kind into "Decode
// created id failed", a message that says nothing about the project having run out of room.
//
// The verdict is returned alongside so a caller can surface a warning on an otherwise successful
// apply. It is nil when the response carried none.
func DecodeWrite(data []byte) (string, *QuotaVerdict, error) {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		return id, nil, nil
	}

	var result writeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return "", nil, fmt.Errorf("response was neither an id nor a write result: %w", err)
	}
	if result.ID == "" {
		return "", nil, fmt.Errorf("write result carried no id: %s", string(data))
	}

	return result.ID, result.Quota, nil
}

// DecodeQuotaRefusal reads the verdict out of a 409 body, or returns nil when the conflict was
// something else.
//
// These endpoints return 409 for several reasons — a handle already taken, a hostname in use, a
// policy scoped to another project — and those bodies are plain sentences. Only a resource-limit
// refusal is an object, so the shape is what tells them apart.
func DecodeQuotaRefusal(data []byte) *QuotaVerdict {
	var verdict QuotaVerdict
	if err := json.Unmarshal(data, &verdict); err != nil {
		return nil
	}
	if !verdict.Blocked {
		return nil
	}

	return &verdict
}

// QuotaDetail renders a refusal for a Terraform diagnostic.
//
// Terraform shows one error next to the resource that failed, and the practitioner is usually
// looking at a plan rather than a console. Listing every axis means one `terraform apply` reveals
// the whole shortfall instead of one ceiling at a time.
func QuotaDetail(verdict *QuotaVerdict) string {
	if verdict == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(verdict.Message)
	for _, breach := range verdict.Breaches {
		b.WriteString(fmt.Sprintf(
			"\n  %s / %s: needs %s, %s free of %s",
			breach.Region, breach.Label, breach.Requested, breach.Headroom, breach.Available,
		))
	}

	return b.String()
}

// QuotaWarning renders the advisory on a write that succeeded, or "" when there is nothing to say.
func QuotaWarning(verdict *QuotaVerdict) string {
	if verdict == nil || len(verdict.Warnings) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(verdict.Warning)
	for region, replicas := range verdict.AttainableReplicas {
		b.WriteString(fmt.Sprintf("\n  %s: reaches %d replicas", region, replicas))
	}

	return b.String()
}
