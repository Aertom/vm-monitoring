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
