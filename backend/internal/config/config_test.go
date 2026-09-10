package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
listenAddr: ":9090"
pollIntervalSeconds: 30
ssh:
  user: "monitor"
  privateKeyPath: "/tmp/key"
  port: 22
  timeoutSeconds: 5
staticVMs:
  - id: "vm1"
    hostname: "app-sm-01"
    ip: "10.0.0.1"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("écriture fichier temporaire: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load a échoué: %v", err)
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, attendu :9090", cfg.ListenAddr)
	}
	if cfg.PollIntervalSeconds != 30 {
		t.Errorf("PollIntervalSeconds = %d, attendu 30", cfg.PollIntervalSeconds)
	}
	if len(cfg.StaticVMs) != 1 || cfg.StaticVMs[0].Hostname != "app-sm-01" {
		t.Errorf("StaticVMs incorrect: %+v", cfg.StaticVMs)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("écriture fichier temporaire: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load a échoué: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("valeur par défaut ListenAddr incorrecte: %q", cfg.ListenAddr)
	}
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("valeur par défaut PollIntervalSeconds incorrecte: %d", cfg.PollIntervalSeconds)
	}
}

func TestLoadHypervisorsMissingFile(t *testing.T) {
	cfg, err := LoadHypervisors(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("LoadHypervisors ne devrait pas échouer si le fichier est absent: %v", err)
	}
	if len(cfg.ESXi) != 0 || len(cfg.Nutanix) != 0 || len(cfg.KVM) != 0 {
		t.Errorf("configuration attendue vide, obtenu: %+v", cfg)
	}
}
