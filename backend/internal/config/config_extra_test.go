package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatalf("attendu une erreur pour fichier absent")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte(":\t:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("attendu une erreur YAML invalide")
	}
}

func TestLoadHypervisorsValidAndInvalid(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.yaml")
	content := "esxi:\n  - name: e1\n    url: https://h\n    username: u\nkvm:\n  - name: k1\n    host: h\n"
	if err := os.WriteFile(ok, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadHypervisors(ok)
	if err != nil {
		t.Fatalf("LoadHypervisors: %v", err)
	}
	if len(cfg.ESXi) != 1 || len(cfg.KVM) != 1 {
		t.Fatalf("contenu inattendu: %+v", cfg)
	}

	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte(":\t:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadHypervisors(bad); err == nil {
		t.Fatalf("attendu une erreur YAML invalide")
	}
}

func TestLoadExampleFiles(t *testing.T) {
	cfg, err := Load("../../../config.example.yaml")
	if err != nil {
		t.Fatalf("config.example.yaml invalide: %v", err)
	}
	if len(cfg.StaticVMs) == 0 {
		t.Fatalf("config.example.yaml devrait contenir des staticVMs")
	}
	hcfg, err := LoadHypervisors("../../../hypervisors.example.yaml")
	if err != nil {
		t.Fatalf("hypervisors.example.yaml invalide: %v", err)
	}
	if len(hcfg.ESXi) == 0 || len(hcfg.KVM) == 0 {
		t.Fatalf("hypervisors.example.yaml devrait contenir esxi + kvm: %+v", hcfg)
	}
}
