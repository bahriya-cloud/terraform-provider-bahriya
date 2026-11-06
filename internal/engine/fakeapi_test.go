package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bahriya-cloud/terraform-provider-bahriya/internal/bmap"
)

const fakeOrg = "00000000-0000-4000-8000-00000000000f"

// fakeAPI is an in-memory console API for the descriptors' routes. It behaves
// as the real API does where the engine depends on it:
//   - the {code, status, data} envelope; writes answer {id, quota};
//   - an unknown body key is a 400 (Valinor rejects it);
//   - absent writable fields take the spec default; read-only id, created,
//     updated (moved by every PUT), status and version fields are its own;
//   - write-only and sensitive values are never returned, a masked one comes
//     back as its sentinel;
//   - rotate-only material changes only through POST …/rotate;
//   - an asynchronous kind is terminated through its delete route and reads
//     as terminated afterwards.
type fakeAPI struct {
	t       *testing.T
	mu      sync.Mutex
	server  *httptest.Server
	items   map[string]map[string]*bmap.Map // kind → id → record
	nextID  int
	clock   int
	calls   []string // "METHOD path" in order
	refuse  map[string]string
	derived map[string]func(record *bmap.Map)
}

func newFakeAPI(t *testing.T) *fakeAPI {
	api := &fakeAPI{t: t, items: map[string]map[string]*bmap.Map{}, refuse: map[string]string{}, derived: map[string]func(*bmap.Map){}}
	api.server = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.server.Close)
	return api
}

// Calls returns the requests matching a prefix, such as "POST" or "PUT".
func (a *fakeAPI) Calls(prefix string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for _, c := range a.calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func (a *fakeAPI) reply(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	status := "OK"
	if code >= 400 {
		status = "ERROR"
	}
	_, _ = io.WriteString(w, `{"code":`+fmt.Sprint(code)+`,"status":"`+status+`","data":`+bmap.Encode(data)+`}`)
}

func (a *fakeAPI) now() *bmap.Map {
	a.clock++
	return bmap.MapOf("date", fmt.Sprintf("2026-11-07 10:%02d:00.000000", a.clock%60), "timezonetype", int64(3), "timezone", "UTC")
}

// route finds the descriptor and id for a path: base, item, rotate, delete.
func (a *fakeAPI) route(p string) (d Descriptor, id, action string, ok bool) {
	for _, desc := range Descriptors {
		base := fmt.Sprintf(desc.URLBase, fakeOrg)
		switch {
		case p == base:
			return desc, "", "base", true
		case strings.HasPrefix(p, base+"/"):
			rest := strings.Split(strings.TrimPrefix(p, base+"/"), "/")
			if len(rest) == 1 {
				return desc, rest[0], "item", true
			}
			return desc, rest[0], rest[1], true
		case desc.HandlePath != "" && strings.HasPrefix(p, fmt.Sprintf("/organisations/%s/%s/", fakeOrg, desc.HandlePath)):
			return desc, strings.TrimPrefix(p, fmt.Sprintf("/organisations/%s/%s/", fakeOrg, desc.HandlePath)), "handle", true
		}
	}
	return Descriptor{}, "", "", false
}

func (a *fakeAPI) serve(w http.ResponseWriter, req *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, req.Method+" "+req.URL.Path)

	d, id, action, ok := a.route(req.URL.Path)
	if !ok {
		a.reply(w, 404, "Not found.")
		return
	}
	k := bmap.Embedded().Kind(d.Name)
	if a.items[d.Name] == nil {
		a.items[d.Name] = map[string]*bmap.Map{}
	}
	var body *bmap.Map
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		if len(raw) > 0 {
			decoded, err := bmap.Decode(raw)
			if err != nil {
				a.reply(w, 400, err.Error())
				return
			}
			body, _ = decoded.(*bmap.Map)
		}
	}

	switch {
	case action == "handle" && req.Method == http.MethodGet:
		for _, r := range a.items[d.Name] {
			if h, _ := r.Get("handle"); h == id {
				a.reply(w, 200, bmap.MapOf("id", r))
				return
			}
		}
		a.reply(w, 404, "Not found.")

	case action == "base" && req.Method == http.MethodPost:
		if msg, refused := a.refuse[d.Name]; refused {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(409)
			_, _ = io.WriteString(w, `{"code":409,"status":"CONFLICT","data":`+msg+`}`)
			return
		}
		if bad := a.unexpected(k, d, body, false); bad != "" {
			a.reply(w, 400, "Unexpected key(s) "+bad)
			return
		}
		a.nextID++
		newID := fmt.Sprintf("00000000-0000-4000-8000-%012d", a.nextID)
		record := bmap.NewMap()
		record.Set("id", newID)
		for _, f := range k.Fields {
			if !k.IsWritable(f) {
				continue
			}
			if v, has := body.Get(f.Name); has {
				record.Set(f.Name, v)
			} else if def, hasDef := k.Defaults.Get(f.Name); hasDef {
				record.Set(f.Name, bmap.Clone(def))
			}
		}
		record.Set("created", a.now())
		record.Set("updated", nil)
		if k.Field("status") != nil {
			record.Set("status", "running")
		}
		if k.Field("currentversion") != nil {
			record.Set("currentversion", int64(1))
		}
		if k.Field("versioncount") != nil {
			record.Set("versioncount", int64(1))
		}
		if k.Field("maxversions") != nil {
			record.Set("maxversions", int64(5)) // every create hard-codes 5
		}
		if k.Field("organisation") != nil {
			record.Set("organisation", fakeOrg)
		}
		if derive := a.derived[d.Name]; derive != nil {
			derive(record)
		}
		a.items[d.Name][newID] = record
		a.reply(w, 201, bmap.MapOf("id", newID, "quota", nil))

	case action == "item" && req.Method == http.MethodGet, action == "item" && req.Method == http.MethodPut:
		record, found := a.items[d.Name][id]
		if !found {
			a.reply(w, 404, "Not found.")
			return
		}
		if req.Method == http.MethodPut {
			if bad := a.unexpected(k, d, body, true); bad != "" {
				a.reply(w, 400, "Unexpected key(s) "+bad)
				return
			}
			for _, f := range k.Fields {
				if !k.IsWritable(f) || f.Desc.RotateOnly || f.Desc.Immutable {
					continue // the API ignores rotate-only material and immutable fields on update
				}
				if v, has := body.Get(f.Name); has {
					record.Set(f.Name, v)
				}
			}
			record.Set("updated", a.now())
			if derive := a.derived[d.Name]; derive != nil {
				derive(record)
			}
			a.reply(w, 200, bmap.MapOf("id", id, "quota", nil))
			return
		}
		out := a.view(k, record)
		if d.CollectionKey != "" {
			a.reply(w, 200, bmap.MapOf(d.CollectionKey, []any{out}))
			return
		}
		a.reply(w, 200, out)

	case action == "rotate" && req.Method == http.MethodPost:
		record, found := a.items[d.Name][id]
		if !found {
			a.reply(w, 404, "Not found.")
			return
		}
		for _, f := range k.Fields {
			if f.Desc.RotateOnly {
				v, has := body.Get(f.Name)
				if !has || v == nil {
					a.reply(w, 400, "Missing "+f.Name)
					return
				}
				record.Set(f.Name, v)
			}
		}
		n, _ := record.Get("currentversion")
		record.Set("currentversion", n.(int64)+1)
		c, _ := record.Get("versioncount")
		record.Set("versioncount", c.(int64)+1)
		record.Set("rotatedat", a.now())
		a.reply(w, 200, bmap.MapOf("id", id))

	case req.Method == d.deleteMethod() && req.URL.Path == fmt.Sprintf(d.deleteURL(), fakeOrg, id):
		record, found := a.items[d.Name][id]
		if !found {
			a.reply(w, 404, "Not found.")
			return
		}
		if d.ConfirmDelete {
			if name, _ := record.Get("name"); body == nil || !body.Has("confirmName") || mustGet(body, "confirmName") != name {
				a.reply(w, 400, "confirmName must match")
				return
			}
		}
		if d.HasStatus {
			record.Set("status", d.TerminatedStatus)
		} else {
			delete(a.items[d.Name], id)
		}
		a.reply(w, 200, id)

	default:
		a.reply(w, 405, "Method not allowed")
	}
}

// unexpected names body keys the DTO would reject: anything not writable,
// except `organisation` where the kind sends it.
func (a *fakeAPI) unexpected(k *bmap.Kind, d Descriptor, body *bmap.Map, update bool) string {
	var bad []string
	for _, key := range body.Keys() {
		if key == "organisation" && d.SendOrganisation {
			continue
		}
		f := k.Field(key)
		if f == nil || !k.IsWritable(f) {
			bad = append(bad, key)
		}
	}
	return strings.Join(bad, ", ")
}

// view is the GET shape: secrets withheld, masked fields as their sentinel.
func (a *fakeAPI) view(k *bmap.Kind, record *bmap.Map) *bmap.Map {
	out := bmap.NewMap()
	for _, key := range record.Keys() {
		v, _ := record.Get(key)
		f := k.Field(key)
		switch {
		case f != nil && (f.Desc.WriteOnly || f.Desc.Sensitive):
			continue
		case f != nil && f.Desc.Masked != "":
			out.Set(key, f.Desc.Masked)
		case f != nil && f.Desc.ItemKey != "" && len(f.Item) > 0:
			out.Set(key, maskItems(f, v))
		default:
			out.Set(key, bmap.Clone(v))
		}
	}
	return out
}

func maskItems(f *bmap.Field, v any) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}
	out := make([]any, len(list))
	for i, element := range list {
		m, isMap := element.(*bmap.Map)
		if !isMap {
			out[i] = element
			continue
		}
		masked := bmap.Clone(m).(*bmap.Map)
		for _, item := range f.Item {
			if item.Desc.Masked != "" && masked.Has(item.Name) {
				masked.Set(item.Name, item.Desc.Masked)
			} else if item.Desc.WriteOnly || item.Desc.Sensitive {
				masked.Delete(item.Name)
			}
		}
		out[i] = masked
	}
	return out
}

// jsonOf is a test helper for error messages.
func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
