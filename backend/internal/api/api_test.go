package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/inventory"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
	"github.com/Aertom/vm-monitoring/backend/internal/store"
)

func newTestServer() *Server {
	st := store.New()
	st.ReplaceVMs([]model.VM{
		{ID: "sm1", Hostname: "app-sm-01", IP: "10.0.0.1", Family: model.FamilySM},
		{ID: "cm1", Hostname: "app-cm-01", IP: "10.0.0.2", Family: model.FamilyCM,
			EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.1", Hostname: "app-sm-01"}}},
	})
	return &Server{Store: st}
}

func TestListGroupsEnriched(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	rec := httptest.NewRecorder()
	NewRouter(srv).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/groups = %d", rec.Code)
	}
	var groups []groupResponse
	if err := json.NewDecoder(rec.Body).Decode(&groups); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe, obtenu %d", len(groups))
	}
	g := groups[0]
	if g.Status != "available" {
		t.Errorf("status attendu available, obtenu %q", g.Status)
	}
	if len(g.VMs) != 2 {
		t.Errorf("vms enrichis attendus 2, obtenus %d", len(g.VMs))
	}
	if g.Members[model.FamilySM] != "sm1" {
		t.Errorf("members.sm attendu sm1, obtenu %+v", g.Members)
	}
}

func TestCheckoutAcceptsUserAndInUseBy(t *testing.T) {
	for _, body := range []string{`{"user":"alice"}`, `{"inUseBy":"alice"}`} {
		srv := newTestServer()
		router := NewRouter(srv)
		gid := srv.Store.ListGroups()[0].ID

		req := httptest.NewRequest(http.MethodPost, "/api/groups/"+gid+"/checkout", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("checkout %s = %d (%s)", body, rec.Code, rec.Body.String())
		}
		var g groupResponse
		if err := json.NewDecoder(rec.Body).Decode(&g); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if g.Status != "checkedOut" || g.InUseBy != "alice" {
			t.Errorf("checkout %s: status=%q inUseBy=%q", body, g.Status, g.InUseBy)
		}
	}
}

func TestCheckoutErrors(t *testing.T) {
	srv := newTestServer()
	router := NewRouter(srv)
	gid := srv.Store.ListGroups()[0].ID

	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"vide":          {`{}`, http.StatusBadRequest},
		"user vide":     {`{"user":""}`, http.StatusBadRequest},
		"JSON invalide": {`{`, http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/groups/"+gid+"/checkout", bytes.NewBufferString(tc.body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: checkout = %d, attendu %d (%s)", name, rec.Code, tc.want, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/groups/inexistant/checkout", bytes.NewBufferString(`{"user":"alice"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("groupe inexistant: = %d, attendu 404", rec.Code)
	}

	mustCheckout := func(user string, want int) {
		req := httptest.NewRequest(http.MethodPost, "/api/groups/"+gid+"/checkout", bytes.NewBufferString(`{"user":"`+user+`"}`))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("checkout %q = %d, attendu %d", user, rec.Code, want)
		}
	}
	mustCheckout("alice", http.StatusOK)
	mustCheckout("bob", http.StatusConflict)
}

func TestCheckin(t *testing.T) {
	srv := newTestServer()
	router := NewRouter(srv)
	gid := srv.Store.ListGroups()[0].ID

	req := httptest.NewRequest(http.MethodPost, "/api/groups/"+gid+"/checkout", bytes.NewBufferString(`{"user":"alice"}`))
	router.ServeHTTP(httptest.NewRecorder(), req)

	req = httptest.NewRequest(http.MethodPost, "/api/groups/"+gid+"/checkin", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("checkin = %d", rec.Code)
	}
	var g groupResponse
	if err := json.NewDecoder(rec.Body).Decode(&g); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if g.Status != "available" || g.InUseBy != "" {
		t.Errorf("après checkin: status=%q inUseBy=%q", g.Status, g.InUseBy)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/groups/inexistant/checkin", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("checkin inexistant = %d, attendu 404", rec.Code)
	}
}

func TestListVMsFilterAndFamiliesAndCORS(t *testing.T) {
	srv := newTestServer()
	router := NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/vms?family=sm", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var vms []model.VM
	if err := json.NewDecoder(rec.Body).Decode(&vms); err != nil {
		t.Fatalf("decode vms: %v", err)
	}
	if len(vms) != 1 || vms[0].Family != model.FamilySM {
		t.Errorf("filtre family=sm incorrect: %+v", vms)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/families", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var fams []string
	if err := json.NewDecoder(rec.Body).Decode(&fams); err != nil {
		t.Fatalf("decode families: %v", err)
	}
	if len(fams) != 4 {
		t.Errorf("families attendues 4, obtenues %v", fams)
	}

	req = httptest.NewRequest(http.MethodOptions, "/api/groups", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight CORS = %d, attendu 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("header CORS manquant")
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	NewRouter(newTestServer()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d", rec.Code)
	}
}

func TestDiscovery(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/discovery", nil)
	rec := httptest.NewRecorder()
	NewRouter(srv).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("sans Discovery = %d, attendu 404", rec.Code)
	}

	srv.Discovery = func() inventory.Report {
		return inventory.Report{Sources: map[string]int{"esxi": 2}}
	}
	req = httptest.NewRequest(http.MethodGet, "/api/discovery", nil)
	rec = httptest.NewRecorder()
	NewRouter(srv).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/discovery = %d", rec.Code)
	}
	var rep inventory.Report
	if err := json.NewDecoder(rec.Body).Decode(&rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.Sources["esxi"] != 2 {
		t.Errorf("rapport incorrect: %+v", rep)
	}
}

func TestRename(t *testing.T) {
	srv := newTestServer()
	router := NewRouter(srv)
	gid := srv.Store.ListGroups()[0].ID
	url := "/api/groups/" + gid + "/rename"

	req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(`{"name":"prod"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename = %d (%s)", rec.Code, rec.Body.String())
	}
	var g groupResponse
	if err := json.NewDecoder(rec.Body).Decode(&g); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if g.Name != "prod" {
		t.Errorf("Name=%q, attendu prod", g.Name)
	}

	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"vide":          {`{}`, http.StatusBadRequest},
		"nom vide":      {`{"name":"  "}`, http.StatusBadRequest},
		"JSON invalide": {`{`, http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(tc.body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: rename = %d, attendu %d", name, rec.Code, tc.want)
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/groups/inexistant/rename", bytes.NewBufferString(`{"name":"x"}`))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("rename inexistant = %d, attendu 404", rec.Code)
	}
}

func TestFamiliesCustom(t *testing.T) {
	set, err := model.NewFamilySet([]model.FamilyDef{{Name: "sm"}, {Name: "wks"}})
	if err != nil {
		t.Fatalf("NewFamilySet: %v", err)
	}
	srv := newTestServer()
	srv.Families = set
	req := httptest.NewRequest(http.MethodGet, "/api/families", nil)
	rec := httptest.NewRecorder()
	NewRouter(srv).ServeHTTP(rec, req)
	var fams []string
	if err := json.NewDecoder(rec.Body).Decode(&fams); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(fams) != 2 || fams[0] != "sm" || fams[1] != "wks" {
		t.Errorf("families=%v", fams)
	}
}

func TestOpenAPIDocs(t *testing.T) {
	srv := newTestServer()
	router := NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi.yaml = %d", rec.Code)
	}
	var doc struct {
		Paths map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("spec invalide: %v", err)
	}
	for _, p := range []string{
		"/api/vms", "/api/groups", "/api/families", "/api/families/detect",
		"/api/groups/{id}/checkout", "/api/groups/{id}/checkin",
		"/api/groups/{id}/rename", "/api/discovery", "/api/hypervisors",
		"/api/creation/options", "/api/creation/suggest-ip",
		"/api/creation/check-ip", "/api/creation", "/healthz",
	} {
		if _, ok := doc.Paths[p]; !ok {
			t.Errorf("chemin %q absent de la spec", p)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "swagger") {
		t.Fatalf("docs = %d (pas d'UI)", rec.Code)
	}
}

func creationTestServer() *Server {
	srv := newTestServer()
	srv.HVs = &config.HypervisorsConfig{
		ESXi: []config.ESXiConfig{{
			Name: "esx-08", URL: "https://192.168.0.10", Username: "root",
			Datastore: "datastore1", Network: "VM Network", IsoDir: "iso", Subnet: "10.9.0.0/24",
		}},
	}
	srv.Creation = &config.CreationConfig{
		ISOs:          []config.ISOEntry{{Name: "RHEL 9.5", File: "rhel-9.5.iso", GuestOS: "rhel9_64Guest"}},
		Types:         map[string]config.VMTypePreset{"serveur": {CPU: 4, RAMGB: 16, DiskGB: 100}},
		ESXiHWVersion: "20", ESXiFirmware: "efi",
	}
	srv.Families = model.DefaultFamilies()
	return srv
}

func TestCreationEndpoints(t *testing.T) {
	srv := creationTestServer()
	router := NewRouter(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/hypervisors", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "esx-08") {
		t.Fatalf("hypervisors = %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "change-me") || strings.Contains(rec.Body.String(), "Password") {
		t.Fatalf("secrets exposés: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/families/detect", bytes.NewBufferString(`{"hostname":"x-sm-1"}`))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"sm"`) {
		t.Fatalf("detect = %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/creation/options?hypervisor=esx-08", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "datastore1") {
		t.Fatalf("options = %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/creation/suggest-ip?hypervisor=esx-08", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "10.9.0.2") {
		t.Fatalf("suggest = %s", rec.Body.String())
	}

	body := `{"hypervisor":"esx-08","family":"sm","name":"sm-new-01","type":"serveur","isoFile":"rhel-9.5.iso","datastore":"datastore1","network":"VM Network","cpu":4,"ramGB":16,"diskGB":100,"ip":"10.9.0.50"}`
	req = httptest.NewRequest(http.MethodPost, "/api/creation?dryRun=true", bytes.NewBufferString(body))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "vim-cmd solo/registervm") {
		t.Fatalf("dryRun = %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/creation", bytes.NewBufferString(`{"hypervisor":"nope"}`))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalide = %d, attendu 400", rec.Code)
	}
}
