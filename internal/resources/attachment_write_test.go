package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Since the quota work (console/api f5e28ca, 2026-08-28) attach answers
// {id, quota} rather than a bare id. Every project attachment resource decoded
// a bare string, so creating any of them failed with "Decode join id failed".
// Each must accept the write result and keep the join id from it.
func TestEveryAttachmentAcceptsTheWriteResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 201, "status": "CREATED", "data": map[string]any{"id": "join-1", "quota": nil},
		})
	}))
	defer server.Close()
	c := client.New(server.URL, "test-token", "org-1")
	ctx := context.Background()

	for name, ctor := range map[string]func() resource.Resource{
		"encryption_key": NewProjectEncryptionKeyAttachmentResource, "env_file": NewProjectEnvFileAttachmentResource,
		"gpg_keypair": NewProjectGpgKeypairAttachmentResource, "json_config": NewProjectJSONConfigAttachmentResource,
		"network_policy": NewProjectNetworkPolicyAttachmentResource, "plain_config": NewProjectPlainConfigAttachmentResource,
		"registry": NewProjectRegistryAttachmentResource, "secret": NewProjectSecretAttachmentResource,
		"ssh_keypair": NewProjectSSHKeypairAttachmentResource, "tls_bundle": NewProjectTLSBundleAttachmentResource,
		"x509_cert": NewProjectX509CertAttachmentResource, "yaml_config": NewProjectYAMLConfigAttachmentResource,
	} {
		t.Run(name, func(t *testing.T) {
			r := ctor()
			r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: c}, &resource.ConfigureResponse{})
			var s resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &s)
			objType := s.Schema.Type().TerraformType(ctx).(tftypes.Object)

			values := map[string]tftypes.Value{}
			for attr, typ := range objType.AttributeTypes {
				values[attr] = tftypes.NewValue(typ, tftypes.UnknownValue)
			}
			values["project_id"] = tftypes.NewValue(tftypes.String, "proj-1")
			values["handle"] = tftypes.NewValue(tftypes.String, "item")

			resp := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema, Raw: tftypes.NewValue(objType, nil)}}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: tftypes.NewValue(objType, values)}}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("create: %v", resp.Diagnostics)
			}
			var joinID string
			resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("join_id"), &joinID)...)
			if joinID != "join-1" {
				t.Errorf("join_id = %q, want join-1", joinID)
			}
		})
	}
}
