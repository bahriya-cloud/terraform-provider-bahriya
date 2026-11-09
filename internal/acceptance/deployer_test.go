package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/provider"
)

// The local dev stack has no deployer, so a deployable would sit in
// "provisioning" (and, on destroy, "terminating") and the provider's wait would
// time out. standInDeployer plays the deployer's part for the resources this
// run creates: it reports "running" for a provisioning deployable and "deleted"
// for a terminating one, through the same admin endpoint the deployer calls
// (POST /internal/v1/pipeline/events). It runs only against a local API; a
// remote run has the real deployer.
//
//	BAHRIYA_ACC_ADMIN_URL=http://localhost:9080
//	BAHRIYA_ACC_PIPELINE_TOKEN=<the dev admin API's PIPELINE_CALLBACK_TOKEN>
type standInDeployer struct {
	api, admin, token, org, orgHandle string
	mu                                sync.Mutex
	suffixes                          map[string]bool
}

func local() bool {
	url := os.Getenv(provider.EnvAPIURL)
	return strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1")
}

// startDeployer starts the stand-in for a local run and returns it; it stops
// with the test. A remote run returns nil (the real deployer answers).
func startDeployer(t *testing.T) *standInDeployer {
	t.Helper()
	if !local() {
		return nil
	}
	d := &standInDeployer{
		api:      strings.TrimRight(os.Getenv(provider.EnvAPIURL), "/"),
		admin:    strings.TrimRight(os.Getenv("BAHRIYA_ACC_ADMIN_URL"), "/"),
		token:    os.Getenv("BAHRIYA_ACC_PIPELINE_TOKEN"),
		org:      os.Getenv(provider.EnvOrganisationID),
		suffixes: map[string]bool{},
	}
	if d.admin == "" || d.token == "" {
		t.Fatal("a local run needs BAHRIYA_ACC_ADMIN_URL and BAHRIYA_ACC_PIPELINE_TOKEN: the dev stack has no deployer, so the matrix reports deployables running and deleted itself")
	}
	var orgs struct {
		Data struct {
			Organisations []struct{ ID, Handle string } `json:"organisations"`
		} `json:"data"`
	}
	if err := d.get(context.Background(), "/organisations?limit=200", &orgs); err != nil {
		t.Fatalf("stand-in deployer: %v", err)
	}
	for _, o := range orgs.Data.Organisations {
		if o.ID == d.org {
			d.orgHandle = o.Handle
		}
	}
	if d.orgHandle == "" {
		t.Fatalf("stand-in deployer: organisation %s not found", d.org)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			d.tick(ctx, t)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()
	t.Cleanup(func() { cancel(); <-done })
	return d
}

// watch makes the stand-in answer for handles that contain sfx.
func (d *standInDeployer) watch(sfx string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.suffixes[sfx] = true
	d.mu.Unlock()
}

func (d *standInDeployer) ours(handle string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for s := range d.suffixes {
		if strings.Contains(handle, s) {
			return true
		}
	}
	return false
}

type deployable struct {
	Handle, Status, Project string
	Activeregions           []string
}

func (d *standInDeployer) tick(ctx context.Context, t *testing.T) {
	lists := []struct{ path, key, resourcetype string }{
		{"/containers/type/http", "containers", "container"},
		{"/containers/type/worker", "containers", "container"},
		{"/containers/type/cronjob", "containers", "container"},
		{"/memcached", "memcached", "memcached"},
		{"/valkey", "valkey", "valkey"},
		{"/mysql", "mysql", "mysql"},
	}
	for _, l := range lists {
		var body struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := d.get(ctx, "/organisations/"+d.org+l.path+"?limit=100", &body); err != nil {
			continue
		}
		var items []deployable
		if err := json.Unmarshal(body.Data[l.key], &items); err != nil {
			continue
		}
		for _, it := range items {
			if !d.ours(it.Handle) {
				continue
			}
			event := map[string]string{"provisioning": "running", "terminating": "deleted"}[it.Status]
			if event == "" {
				continue
			}
			region := region
			if len(it.Activeregions) > 0 {
				region = it.Activeregions[0]
			}
			if err := d.post(ctx, map[string]string{
				"handle": it.Handle, "project_handle": it.Project, "orghandle": d.orgHandle,
				"region": region, "event": event, "resourcetype": l.resourcetype,
				"message": "terraform acceptance matrix stand-in deployer",
			}); err != nil && ctx.Err() == nil {
				t.Logf("stand-in deployer: %s %s: %v", event, it.Handle, err)
			}
		}
	}
}

func (d *standInDeployer) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.api+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv(provider.EnvToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *standInDeployer) post(ctx context.Context, event map[string]string) error {
	b, _ := json.Marshal(event)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.admin+"/internal/v1/pipeline/events", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST pipeline event: %d", resp.StatusCode)
	}
	return nil
}
