// Command generator writes the provider's generated resource files from the
// embedded resource-schema.json (internal/bmap): one file per kind holding its
// schema and the engine constructor, the 12 project attachment resources, and
// the registry. Run it through `make generate`, which first copies the
// package's resource-schema.json into internal/bmap.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// kinds are the engine-served resources, in registry order, with the Go name
// their constructor uses.
var kinds = []struct{ Name, GoName string }{
	{"project", "Project"},
	{"registry", "Registry"},
	{"secret", "Secret"},
	{"memcached", "Memcached"},
	{"valkey", "Valkey"},
	{"mysql", "Mysql"},
	{"container", "Container"},
	{"tls_bundle", "TLSBundle"},
	{"x509_cert", "X509Cert"},
	{"gpg_keypair", "GpgKeypair"},
	{"ssh_keypair", "SSHKeypair"},
	{"encryption_key", "EncryptionKey"},
	{"env_file", "EnvFile"},
	{"yaml_config", "YAMLConfig"},
	{"json_config", "JSONConfig"},
	{"plain_config", "PlainConfig"},
	{"network_policy", "NetworkPolicy"},
	// bahriya_path_rule (x-terraform-type), under its container (x-parent).
	{"pathrule", "PathRule"},
}

// attachments are the per-type project attachment resources
// (bahriya_project_<type>_attachment): thin create/read/delete resources on
// /projects/{pid}/attach/{type}/{handle}.
var attachments = []string{"registry", "secret", "tls_bundle", "x509_cert", "gpg_keypair", "ssh_keypair", "encryption_key", "env_file", "yaml_config", "json_config", "plain_config", "network_policy"}

// handWritten are constructors implemented in their own files that the
// registry must still list.
var handWritten = []string{"NewRoleResource", "NewResourceGrantResource", "NewBillingDetailsResource"}

func main() {
	outputDir := flag.String("out", "internal/resources", "directory to write generated resource files to")
	flag.Parse()
	if err := run(*outputDir); err != nil {
		log.Fatal(err)
	}
}

func run(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("ensure output dir: %w", err)
	}
	for _, k := range kinds {
		src, err := kindFile(k.Name, k.GoName)
		if err != nil {
			return err
		}
		if err := writeGo(filepath.Join(outputDir, k.Name+"_resource.go"), src); err != nil {
			return err
		}
		fmt.Printf("generated %s_resource.go\n", k.Name)
	}
	for _, name := range attachments {
		src, err := attachmentFile(AttachmentResource{Name: name, GoName: goName(name), APIType: apiSlug(name)})
		if err != nil {
			return err
		}
		if err := writeGo(filepath.Join(outputDir, "project_"+name+"_attachment_resource.go"), src); err != nil {
			return err
		}
		fmt.Printf("generated project_%s_attachment_resource.go\n", name)
	}
	return writeGo(filepath.Join(outputDir, "registry.go"), registryFile())
}

func goName(name string) string {
	for _, k := range kinds {
		if k.Name == name {
			return k.GoName
		}
	}
	panic("generator: no Go name for " + name)
}

// apiSlug is the URL path segment of the project-attachments API
// (POST/DELETE /projects/{pid}/attach/{slug}/{handle}): snake_case plurals,
// with single-word plurals for registries and secrets.
func apiSlug(s string) string {
	switch s {
	case "registry":
		return "registries"
	case "secret":
		return "secrets"
	case "network_policy":
		return "network_policies"
	}
	return s + "s"
}
