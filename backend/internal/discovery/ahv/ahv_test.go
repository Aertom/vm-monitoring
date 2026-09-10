package ahv

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc permet de transformer une fonction en http.RoundTripper,
// utilisé ici comme mock du client HTTP.
type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockClient(fn roundTripFunc) HTTPClient {
	return fn
}

func TestNewClient_MissingHost(t *testing.T) {
	_, err := NewClient(Config{}, nil)
	if err == nil {
		t.Fatal("expected error when host is missing, got nil")
	}
}

func TestNewClient_DefaultPort(t *testing.T) {
	c, err := NewClient(Config{Host: "prism.example.com"}, newMockClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"entities":[]}`))}, nil
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.Port != 9440 {
		t.Fatalf("expected default port 9440, got %d", c.cfg.Port)
	}
}

func TestListVMs_Success(t *testing.T) {
	mock := newMockClient(func(req *http.Request) (*http.Response, error) {
		body := `{"entities":[{"uuid":"vm-1","name":"web-01","power_state":"on","num_vcpus":2,"memory_mb":4096,"host_uuid":"host-1"}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})

	c, err := NewClient(Config{Host: "prism.example.com", Username: "admin", Password: "secret"}, mock)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	vms, err := c.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	if vms[0].Name != "web-01" {
		t.Fatalf("expected VM name 'web-01', got %q", vms[0].Name)
	}
}

func TestListVMs_HTTPError(t *testing.T) {
	mock := newMockClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`unauthorized`))}, nil
	})

	c, _ := NewClient(Config{Host: "prism.example.com"}, mock)
	_, err := c.ListVMs(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
}

func TestListVMs_InvalidJSON(t *testing.T) {
	mock := newMockClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`not-json`))}, nil
	})

	c, _ := NewClient(Config{Host: "prism.example.com"}, mock)
	_, err := c.ListVMs(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
