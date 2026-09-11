package collector

import (
	"reflect"
	"testing"

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
