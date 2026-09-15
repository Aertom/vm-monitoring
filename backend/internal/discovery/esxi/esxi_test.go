package esxi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeRoundTripper simule les réponses HTTP de l'API REST vSphere sans appel réseau réel.
type fakeRoundTripper struct {
	statusCode int
	body       string
	err        error
}

func (f *fakeRoundTripper) Do(req *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

func TestDiscoverSuccess(t *testing.T) {
	body := `{"value":[{"name":"vm-web-01","power_state":"POWERED_ON"},{"name":"vm-db-01","power_state":"POWERED_OFF"}]}`
	client := &fakeRoundTripper{statusCode: http.StatusOK, body: body}
	d := NewDiscoverer(Config{Host: "esxi.example.com", Username: "u", Password: "p"}, client)

	vms, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("erreur inattendue: %v", err)
	}
	if len(vms) != 2 {
		t.Fatalf("attendu 2 VMs, obtenu %d", len(vms))
	}
	if vms[0].Name != "vm-web-01" || vms[0].Hypervisor != "esxi" {
		t.Errorf("VM[0] inattendue: %+v", vms[0])
	}
	if vms[1].PowerState != "POWERED_OFF" {
		t.Errorf("VM[1].PowerState inattendu: %s", vms[1].PowerState)
	}
}

func TestDiscoverMissingHost(t *testing.T) {
	d := NewDiscoverer(Config{}, &fakeRoundTripper{})
	if _, err := d.Discover(context.Background()); err == nil {
		t.Fatal("attendu une erreur pour host manquant")
	}
}

func TestDiscoverHTTPError(t *testing.T) {
	client := &fakeRoundTripper{statusCode: http.StatusUnauthorized, body: `{}`}
	d := NewDiscoverer(Config{Host: "esxi.example.com"}, client)
	if _, err := d.Discover(context.Background()); err == nil {
		t.Fatal("attendu une erreur pour statut HTTP 401")
	}
}

func TestDiscoverInvalidJSON(t *testing.T) {
	client := &fakeRoundTripper{statusCode: http.StatusOK, body: `not-json`}
	d := NewDiscoverer(Config{Host: "esxi.example.com"}, client)
	if _, err := d.Discover(context.Background()); err == nil {
		t.Fatal("attendu une erreur de décodage JSON")
	}
}

// scriptedRT rejoue des réponses selon (méthode, chemin), pour le flux session.
type scriptedRT struct {
	t      *testing.T
	calls  []string
	list   string
	sessOK bool
}

func (s *scriptedRT) Do(req *http.Request) (*http.Response, error) {
	s.calls = append(s.calls, req.Method+" "+req.URL.Path)
	switch {
	case req.URL.Path == "/rest/vcenter/vm" && req.Header.Get("vmware-api-session-id") == "sess-1":
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(s.list))}, nil
	case req.URL.Path == "/rest/vcenter/vm":
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	case req.URL.Path == "/rest/com/vmware/cis/session" && s.sessOK:
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"value":"sess-1"}`))}, nil
	case strings.HasSuffix(req.URL.Path, "/guest/identity"):
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"value":{"ip_address":"10.0.0.5"}}`))}, nil
	default:
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	}
}

func TestDiscoverSessionFallback(t *testing.T) {
	rt := &scriptedRT{t: t, sessOK: true, list: `{"value":[{"vm":"vm-1","name":"a","power_state":"x"}]}`}
	d := NewDiscoverer(Config{Host: "h", Username: "u", Password: "p"}, rt)
	vms, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("session fallback: %v", err)
	}
	if len(vms) != 1 || vms[0].ID != "vm-1" {
		t.Fatalf("VM inattendue: %+v", vms)
	}
}

func TestDiscoverSessionFailure(t *testing.T) {
	rt := &scriptedRT{t: t, sessOK: false}
	d := NewDiscoverer(Config{Host: "h"}, rt)
	if _, err := d.Discover(context.Background()); err == nil {
		t.Fatal("attendu une erreur quand la session échoue")
	}
}

func TestGuestIP(t *testing.T) {
	rt := &scriptedRT{t: t, sessOK: true}
	d := NewDiscoverer(Config{Host: "h"}, rt)
	ip, err := d.GuestIP(context.Background(), "vm-1")
	if err != nil {
		t.Fatalf("GuestIP: %v", err)
	}
	if ip != "10.0.0.5" {
		t.Fatalf("IP=%q", ip)
	}
	if _, err := d.GuestIP(context.Background(), ""); err == nil {
		t.Fatal("attendu une erreur sans vm id")
	}
}

func TestParseGuestIPAddress(t *testing.T) {
	out := "foo = \"bar\",\nipAddress = \"192.168.1.20\",\n"
	if got := ParseGuestIPAddress(out); got != "192.168.1.20" {
		t.Fatalf("IP=%q", got)
	}
	if got := ParseGuestIPAddress("rien ici"); got != "" {
		t.Fatalf("attendu vide, obtenu %q", got)
	}
}

func TestParseVimCmdGetAllVMs(t *testing.T) {
	out := "Vmid     Name          File                              Guest OS      Version\n" +
		"128      cm-prod-01    [datastore1] cm-prod-01/cm.vmx     otherLinux64  vmx-21\n" +
		"256      ws-prod-01    [datastore1] ws-prod-01/ws.vmx     otherLinux64  vmx-21\n" +
		"\n" +
		"ligne parasite\n"
	got := ParseVimCmdGetAllVMs(out)
	if len(got) != 2 {
		t.Fatalf("attendu 2 VMs, obtenu %+v", got)
	}
	if got[0].Name != "cm-prod-01" || got[0].Hypervisor != "esxi" {
		t.Errorf("VM[0] inattendue: %+v", got[0])
	}
	if got[1].Name != "ws-prod-01" {
		t.Errorf("VM[1] inattendue: %+v", got[1])
	}
}

func TestParseVimCmdGetAllVMsEmpty(t *testing.T) {
	if got := ParseVimCmdGetAllVMs("\n  \nVmid Name\n"); len(got) != 0 {
		t.Fatalf("attendu 0 VMs, obtenu %+v", got)
	}
}
