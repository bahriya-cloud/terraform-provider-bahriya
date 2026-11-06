package engine

// Descriptor holds a kind's API routing facts: where it lives, how it is
// deleted and whether it has an asynchronous status. These are facts about
// the console API's endpoints, not mapping facts, so they live here rather
// than in resource-schema.json (which holds every mapping fact, CLAUDE.md).
// Moved over unchanged from the v0.x generator's descriptors.
type Descriptor struct {
	// Name is the Terraform type suffix and the kind in resource-schema.json.
	Name string
	// URLBase and URLItem are printf patterns: organisation id, then item id.
	URLBase string
	URLItem string
	// HandlePath is the handle-exists route segment: GET
	// /organisations/{org}/{HandlePath}/{handle}.
	HandlePath string
	// CollectionKey is set when GET-one returns a list wrapper ({key: [item]})
	// rather than the bare object.
	CollectionKey string
	// DeleteMethod and DeleteURL override DELETE URLItem.
	DeleteMethod string
	DeleteURL    string
	// ConfirmDelete sends {"confirmName": name} with the delete.
	ConfirmDelete bool
	// DeleteConflictHint is appended to a 409 from the delete.
	DeleteConflictHint string
	// SendOrganisation adds the `organisation` body key on create and update.
	// Most DTOs tolerate it, Registry and Secret reject it, Project needs it.
	SendOrganisation bool
	// HasStatus marks an asynchronous kind: create and update wait for
	// ReadyStatus, delete waits for TerminatedStatus.
	HasStatus         bool
	ReadyStatus       string
	TerminatingStatus string
	TerminatedStatus  string
	// Noun is how diagnostics refer to one item ("instance"); defaults to Name.
	Noun string
}

func (d Descriptor) noun() string {
	if d.Noun != "" {
		return d.Noun
	}
	return d.Name
}

func (d Descriptor) description() string {
	return "Manages a Bahriya " + d.Name + "."
}

func (d Descriptor) deleteMethod() string {
	if d.DeleteMethod != "" {
		return d.DeleteMethod
	}
	return "DELETE"
}

func (d Descriptor) deleteURL() string {
	if d.DeleteURL != "" {
		return d.DeleteURL
	}
	return d.URLItem
}

// asyncStatus fills the status lifecycle every asynchronous kind shares.
func asyncStatus(d Descriptor) Descriptor {
	d.HasStatus = true
	d.ReadyStatus = "running"
	d.TerminatingStatus = "terminating"
	d.TerminatedStatus = "terminated"
	return d
}

// vault is a vault or config item: org-scoped, synchronous, organisation sent.
func vault(name string) Descriptor {
	return Descriptor{
		Name:             name,
		URLBase:          "/organisations/%s/" + name + "s",
		URLItem:          "/organisations/%s/" + name + "s/%s",
		HandlePath:       name,
		SendOrganisation: true,
	}
}

// Descriptors lists every kind the engine serves, by name.
var Descriptors = map[string]Descriptor{
	"project": {
		Name:             "project",
		URLBase:          "/organisations/%s/projects",
		URLItem:          "/organisations/%s/projects/%s",
		HandlePath:       "project",
		CollectionKey:    "projects",
		ConfirmDelete:    true,
		SendOrganisation: true,
		DeleteConflictHint: "Hint: a project can only be deleted once every container, memcached, registry, " +
			"and secret it owns is gone. For containers and memcached this means status " +
			"'terminated' — instances stuck in 'error', 'terminating', 'provisioning', " +
			"'running', or 'suspended' all count as active. Containers reach 'error' when " +
			"the deploy rolled back and they do not self-recover; investigate via the " +
			"admin console and explicitly terminate them. Registries and secrets must be " +
			"detached from the project before delete.",
	},
	"registry": {
		Name:       "registry",
		URLBase:    "/organisations/%s/registries",
		URLItem:    "/organisations/%s/registries/%s",
		HandlePath: "registry",
	},
	"secret": {
		Name:       "secret",
		URLBase:    "/organisations/%s/secrets",
		URLItem:    "/organisations/%s/secrets/%s",
		HandlePath: "secret",
	},
	"memcached": asyncStatus(Descriptor{
		Name:             "memcached",
		URLBase:          "/organisations/%s/memcached",
		URLItem:          "/organisations/%s/memcached/%s",
		HandlePath:       "memcache",
		Noun:             "instance",
		SendOrganisation: true,
	}),
	"valkey": asyncStatus(Descriptor{
		Name:    "valkey",
		URLBase: "/organisations/%s/valkey",
		URLItem: "/organisations/%s/valkey/%s",
		// /valkey-handle/{handle}: /valkey/{handle} would collide with
		// /valkey/{vid} (SR-2026-004).
		HandlePath:       "valkey-handle",
		Noun:             "instance",
		SendOrganisation: true,
	}),
	"mysql": asyncStatus(Descriptor{
		Name:    "mysql",
		URLBase: "/organisations/%s/mysql",
		URLItem: "/organisations/%s/mysql/%s",
		// /mysql-handle/{handle}, like valkey (SR-2026-008).
		HandlePath:       "mysql-handle",
		Noun:             "instance",
		SendOrganisation: true,
	}),
	"container": asyncStatus(Descriptor{
		Name:             "container",
		URLBase:          "/organisations/%s/containers",
		URLItem:          "/organisations/%s/containers/%s",
		HandlePath:       "container",
		DeleteMethod:     "POST",
		DeleteURL:        "/organisations/%s/containers/%s/terminate",
		SendOrganisation: true,
	}),
	"tls_bundle":     vault("tls_bundle"),
	"x509_cert":      vault("x509_cert"),
	"gpg_keypair":    vault("gpg_keypair"),
	"ssh_keypair":    vault("ssh_keypair"),
	"encryption_key": vault("encryption_key"),
	"env_file":       vault("env_file"),
	"yaml_config":    vault("yaml_config"),
	"json_config":    vault("json_config"),
	"plain_config":   vault("plain_config"),
	"network_policy": {
		Name:             "network_policy",
		URLBase:          "/organisations/%s/network_policies",
		URLItem:          "/organisations/%s/network_policies/%s",
		HandlePath:       "network_policy",
		SendOrganisation: true,
	},
}
