package inventory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/discovery/kvm"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func TestMerge_MatchEnrichesStatic(t *testing.T) {
	static := []config.StaticVM{{ID: "vm1", Hostname: "SM-prod-01", IP: "10.0.0.1"}}
	discovered := []DiscoveredVM{{Name: "sm-prod-01", PowerState: "poweredOn", Source: model.HypervisorESXi}}
	got := Merge(static, discovered)
	if len(got) != 1 {
		t.Fatalf("attendu 1 VM, obtenu %d", len(got))
	}
	if got[0].ID != "vm1" || got[0].IP != "10.0.0.1" {
		t.Fatalf("identité statique perdue: %+v", got[0])
	}
	if got[0].Hypervisor != model.HypervisorESXi {
		t.Fatalf("source non enrichie: %+v", got[0])
	}
	if got[0].Family != model.FamilySM {
		t.Fatalf("famille incorrecte: %+v", got[0])
	}
}

func TestMerge_NewDiscovered(t *testing.T) {
	got := Merge(nil, []DiscoveredVM{{Name: "CM-prod-09", Source: model.HypervisorNutanix}})
	if len(got) != 1 {
		t.Fatalf("attendu 1 VM, obtenu %d", len(got))
	}
	if got[0].ID != "disc-nutanix-cm-prod-09" {
		t.Fatalf("ID dérivé incorrect: %q", got[0].ID)
	}
	if got[0].IP != "" || got[0].Family != model.FamilyCM {
		t.Fatalf("VM découverte incorrecte: %+v", got[0])
	}
}

func TestMerge_StaticKeptAndDedup(t *testing.T) {
	static := []config.StaticVM{{ID: "s1", Hostname: "a-sm", IP: "10.0.0.1"}}
	discovered := []DiscoveredVM{
		{Name: ""},
		{Name: "new-ws", Source: model.HypervisorKVM},
		{Name: "NEW-WS", Source: model.HypervisorKVM},
	}
	got := Merge(static, discovered)
	if len(got) != 2 {
		t.Fatalf("attendu 2 VMs (vide ignorée, doublon fusionné), obtenu %d", len(got))
	}
}

func TestSplitURL(t *testing.T) {
	cases := []struct {
		in       string
		def      int
		wantHost string
		wantPort int
	}{
		{"https://192.168.0.10", 443, "192.168.0.10", 443},
		{"https://192.168.0.20:9440", 9440, "192.168.0.20", 9440},
		{"192.168.0.30", 443, "192.168.0.30", 443},
		{"", 443, "", 443},
	}
	for _, c := range cases {
		h, p := splitURL(c.in, c.def)
		if h != c.wantHost || p != c.wantPort {
			t.Errorf("splitURL(%q) = (%q,%d), attendu (%q,%d)", c.in, h, p, c.wantHost, c.wantPort)
		}
	}
}

func TestDiscoverESXi_Mapping(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/vcenter/vm" {
			t.Errorf("chemin inattendu: %s", r.URL.Path)
		}
		if u, _, _ := r.BasicAuth(); u != "root" {
			t.Errorf("auth inattendue: %q", u)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]string{{"name": "sm-prod-01", "power_state": "poweredOn"}},
		})
	}))
	defer srv.Close()
	got, err := discoverESXi(context.Background(), config.ESXiConfig{
		Name: "esx-01", URL: srv.URL, Username: "root", Password: "pw", Insecure: true,
	})
	if err != nil {
		t.Fatalf("discoverESXi: %v", err)
	}
	want := []DiscoveredVM{{Name: "sm-prod-01", PowerState: "poweredOn", Source: model.HypervisorESXi, SourceName: "esx-01"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("obtenu %+v, attendu %+v", got, want)
	}
}

func TestDiscoverAHV_Mapping(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"entities": []map[string]any{{"name": "cm-prod-01", "power_state": "ON"}},
		})
	}))
	defer srv.Close()
	got, err := discoverAHV(context.Background(), config.NutanixConfig{
		Name: "ahv-01", URL: srv.URL, Username: "admin", Password: "pw", Insecure: true,
	})
	if err != nil {
		t.Fatalf("discoverAHV: %v", err)
	}
	if len(got) != 1 || got[0].Name != "cm-prod-01" || got[0].Source != model.HypervisorNutanix {
		t.Fatalf("mapping incorrect: %+v", got)
	}
}

type fakeExecutor struct{ out string }

func (f fakeExecutor) Run(_ context.Context, _ string, _ ...string) (string, error) {
	return f.out, nil
}

func TestDiscoverKVM_Mapping(t *testing.T) {
	c, err := kvm.NewClient(fakeExecutor{out: "Id   Name   State\n--------------------------\n 1    ws-kvm-01 running\n"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := discoverKVMWithExecutor(context.Background(), "kvm-01", c)
	if err != nil {
		t.Fatalf("discoverKVM: %v", err)
	}
	if len(got) != 1 || got[0].Name != "ws-kvm-01" || got[0].Source != model.HypervisorKVM {
		t.Fatalf("mapping incorrect: %+v", got)
	}
}

func TestDiscoverAll_PartialFailure(t *testing.T) {
	hcfg := &config.HypervisorsConfig{
		ESXi: []config.ESXiConfig{{Name: "bad", URL: ""}},
	}
	got, rep := DiscoverAll(context.Background(), hcfg, config.SSHConfig{})
	if len(got) != 0 {
		t.Fatalf("attendu 0 VMs, obtenu %d", len(got))
	}
	if len(rep.Errors) != 1 {
		t.Fatalf("attendu 1 erreur agrégée, obtenu %v", rep.Errors)
	}
}

func TestMergeWithSet_Renamed(t *testing.T) {
	set, err := model.NewFamilySet([]model.FamilyDef{{Name: "sm"}, {Name: "wks", Match: []string{"wks"}}})
	if err != nil {
		t.Fatalf("NewFamilySet: %v", err)
	}
	static := []config.StaticVM{{ID: "w1", Hostname: "app-wks-01", IP: "10.0.0.7"}}
	got := MergeWithSet(static, nil, set)
	if len(got) != 1 || got[0].Family != "wks" {
		t.Fatalf("famille renommée non détectée: %+v", got)
	}
}
