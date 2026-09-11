package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
