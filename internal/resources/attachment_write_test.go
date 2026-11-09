package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Since the quota work (console/api f5e28ca, 2026-08-28) attach answers
// {id, quota} rather than a bare id. Every project attachment resource decoded
// a bare string, so creating any of them failed with "Decode join id failed"
// (found by the acceptance matrix, card 2262). Each one must accept the write
// result, keep the join id from it, and plan empty afterwards.

const attachOrg = "org-1"
const attachProject = "proj-1"

type attachProvider struct{ url string }

func (p *attachProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "bahriya"
}
func (p *attachProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (p *attachProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	c := client.New(p.url, "test-token", attachOrg)
	resp.ResourceData = c
	resp.DataSourceData = c
}
func (p *attachProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectEncryptionKeyAttachmentResource, NewProjectEnvFileAttachmentResource,
		NewProjectGpgKeypairAttachmentResource, NewProjectJSONConfigAttachmentResource,
		NewProjectNetworkPolicyAttachmentResource, NewProjectPlainConfigAttachmentResource,
		NewProjectRegistryAttachmentResource, NewProjectSecretAttachmentResource,
		NewProjectSSHKeypairAttachmentResource, NewProjectTLSBundleAttachmentResource,
		NewProjectX509CertAttachmentResource, NewProjectYAMLConfigAttachmentResource,
	}
}
func (p *attachProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

// fakeAttachAPI answers attach with the write result, lists what is attached,
// and detaches.
type fakeAttachAPI struct {
	mu       sync.Mutex
	attached map[string][]map[string]any // list key -> items
}

func (f *fakeAttachAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	reply := func(code int, data any) {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "status": http.StatusText(code), "data": data})
	}
	prefix := fmt.Sprintf("/organisations/%s/projects/%s/", attachOrg, attachProject)
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	switch {
	case r.Method == http.MethodGet && rest == "attachments":
		out := map[string]any{}
		for k, v := range f.attached {
			out[k] = v
		}
		reply(http.StatusOK, out)
	case strings.HasPrefix(rest, "attach/"):
		parts := strings.Split(strings.TrimPrefix(rest, "attach/"), "/")
		key := strings.ReplaceAll(parts[0], "_", "")
		if r.Method == http.MethodPost {
			f.attached[key] = append(f.attached[key], map[string]any{"handle": parts[1]})
			reply(http.StatusCreated, map[string]any{"id": "join-" + parts[0], "quota": nil})
			return
		}
		kept := f.attached[key][:0]
		for _, it := range f.attached[key] {
			if it["handle"] != parts[1] {
				kept = append(kept, it)
			}
		}
		f.attached[key] = kept
		reply(http.StatusOK, "detached")
	default:
		reply(http.StatusNotFound, "no route "+r.Method+" "+r.URL.Path)
	}
}

func TestEveryAttachmentAcceptsTheWriteResult(t *testing.T) {
	kinds := []struct{ resource, segment string }{
		{"encryption_key", "encryption_keys"}, {"env_file", "env_files"}, {"gpg_keypair", "gpg_keypairs"},
		{"json_config", "json_configs"}, {"network_policy", "network_policies"}, {"plain_config", "plain_configs"},
		{"registry", "registries"}, {"secret", "secrets"}, {"ssh_keypair", "ssh_keypairs"},
		{"tls_bundle", "tls_bundles"}, {"x509_cert", "x509_certs"}, {"yaml_config", "yaml_configs"},
	}
	for _, k := range kinds {
		t.Run(k.resource, func(t *testing.T) {
			server := httptest.NewServer(&fakeAttachAPI{attached: map[string][]map[string]any{}})
			defer server.Close()
			address := fmt.Sprintf("bahriya_project_%s_attachment.a", k.resource)
			tfresource.UnitTest(t, tfresource.TestCase{
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"bahriya": providerserver.NewProtocol6WithError(&attachProvider{url: server.URL}),
				},
				Steps: []tfresource.TestStep{{
					Config: fmt.Sprintf(`
resource "bahriya_project_%s_attachment" "a" {
  project_id = %q
  handle     = "item"
}
`, k.resource, attachProject),
					Check: tfresource.ComposeAggregateTestCheckFunc(
						tfresource.TestCheckResourceAttr(address, "join_id", "join-"+k.segment),
						tfresource.TestCheckResourceAttr(address, "id", attachProject+":item"),
					),
				}},
			})
		})
	}
}
