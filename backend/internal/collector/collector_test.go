package collector

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func TestParseAppVersions(t *testing.T) {
	got := ParseAppVersions("appli1_1.2.3\nappli2-2.0\nmon-app_10.1.4-rc1\nsans-version\n\n")
	want := []model.AppVersion{
		{Name: "appli1", Version: "1.2.3"},
		{Name: "appli2", Version: "2.0"},
		{Name: "mon-app", Version: "10.1.4-rc1"},
		{Name: "sans-version", Version: ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("obtenu %+v, attendu %+v", got, want)
	}
}

func TestParseAppVersionsEmpty(t *testing.T) {
	if got := ParseAppVersions("\n  \n"); len(got) != 0 {
		t.Fatalf("attendu 0 apps, obtenu %+v", got)
	}
}

func TestDefaultAppDirs(t *testing.T) {
	if got := DefaultAppDirs(); len(got) != 1 || got[0] != "/opt" {
		t.Fatalf("défaut inattendu: %v", got)
	}
}

func TestExpandPath(t *testing.T) {
	t.Setenv("HOME", "/tmp/fakehome")
	if got := expandPath("~/.ssh/id_rsa"); got != "/tmp/fakehome/.ssh/id_rsa" {
		t.Fatalf("expansion ~ incorrecte: %q", got)
	}
	if got := expandPath("/absolu/clé"); got != "/absolu/clé" {
		t.Fatalf("chemin absolu modifié: %q", got)
	}
	if got := expandPath("relatif/clé"); got != "relatif/clé" {
		t.Fatalf("chemin relatif modifié: %q", got)
	}
}

func TestParseEtcHosts(t *testing.T) {
	in := "# commentaire\n\n127.0.0.1 localhost\n::1 localhost\n" +
		"10.0.0.1 sm-prod-01\n10.0.0.2 cm-prod-01 alias-cm\n" +
		"malformée\n"
	got := ParseEtcHosts(in)
	want := []model.EtcHostsEntry{
		{IP: "10.0.0.1", Hostname: "sm-prod-01"},
		{IP: "10.0.0.2", Hostname: "cm-prod-01"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("obtenu %+v, attendu %+v", got, want)
	}
}

func TestDirsForFamily(t *testing.T) {
	cfg := &config.Config{AppDirs: map[string][]string{
		"cm": {"/opt", "/appli"},
		"sm": {"/srv"},
	}}
	if got := DirsForFamily(cfg, "cm"); !reflect.DeepEqual(got, []string{"/opt", "/appli"}) {
		t.Fatalf("cm: %v", got)
	}
	if got := DirsForFamily(cfg, "CM"); !reflect.DeepEqual(got, []string{"/opt", "/appli"}) {
		t.Fatalf("casse non gérée: %v", got)
	}
	if got := DirsForFamily(cfg, "ws"); !reflect.DeepEqual(got, DefaultAppDirs()) {
		t.Fatalf("famille absente: %v", got)
	}
	if got := DirsForFamily(nil, "sm"); !reflect.DeepEqual(got, DefaultAppDirs()) {
		t.Fatalf("config nil: %v", got)
	}
}

func TestResolveAuth(t *testing.T) {
	global := config.SSHConfig{User: "monitor", Password: "glob"}
	if u, p := ResolveAuth("", "", global); u != "monitor" || p != "glob" {
		t.Fatalf("global: %q %q", u, p)
	}
	if u, p := ResolveAuth("root", "s3cr3t", global); u != "root" || p != "s3cr3t" {
		t.Fatalf("surcharge VM: %q %q", u, p)
	}
	if u, _ := ResolveAuth("root", "", global); u != "root" {
		t.Fatalf("user seul: %q", u)
	}
}

func TestSSHConfigured(t *testing.T) {
	if SSHConfigured(nil) {
		t.Fatalf("nil devrait être désactivé")
	}
	if SSHConfigured(&config.Config{}) {
		t.Fatalf("sans clé ni mot de passe devrait être désactivé")
	}
	if !SSHConfigured(&config.Config{SSH: config.SSHConfig{PrivateKeyPath: "/k"}}) {
		t.Fatalf("clé globale devrait activer")
	}
	if !SSHConfigured(&config.Config{SSH: config.SSHConfig{Password: "p"}}) {
		t.Fatalf("mot de passe global devrait activer")
	}
	cfg := &config.Config{StaticVMs: []config.StaticVM{{ID: "a", SSHPassword: "p"}}}
	if !SSHConfigured(cfg) {
		t.Fatalf("mot de passe par VM devrait activer")
	}
}

func TestDialNoAuthMethod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := Dial(ctx, "192.0.2.1", "u", 22, "", "", time.Second)
	if err == nil || !strings.Contains(err.Error(), "aucune méthode") {
		t.Fatalf("attendu erreur d'auth, obtenu %v", err)
	}
}

func TestDialPasswordOnlyReachesNetwork(t *testing.T) {
	// Sans clé mais avec mot de passe, Dial doit tenter le réseau
	// (ici une IP TEST-NET-1, échec de connexion attendu, pas d'auth).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Dial(ctx, "192.0.2.1", "u", 22, "", "pw", 2*time.Second)
	if err == nil || strings.Contains(err.Error(), "aucune méthode") {
		t.Fatalf("le mot de passe seul devrait être accepté comme méthode, obtenu %v", err)
	}
}
