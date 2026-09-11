package grouping

import (
	"testing"

	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func customFamilies(t *testing.T) *model.FamilySet {
	t.Helper()
	s, err := model.NewFamilySet([]model.FamilyDef{
		{Name: "sm"},
		{Name: "wks", Match: []string{"wks"}},
		{Name: "ops", Shared: true},
	})
	if err != nil {
		t.Fatalf("NewFamilySet: %v", err)
	}
	return s
}

func TestRebuildWithSet_RenamedCoreAndShared(t *testing.T) {
	set := customFamilies(t)
	vms := []model.VM{
		{ID: "sm1", IP: "10.0.0.1", Family: "sm",
			EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.2", Hostname: "wks1"}}},
		{ID: "wks1", IP: "10.0.0.2", Family: "wks",
			EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.5", Hostname: "ops1"}}},
		{ID: "ops1", IP: "10.0.0.5", Family: "ops"},
		{ID: "ws-old", IP: "10.0.0.9", Family: "ws"},
	}
	groups := RebuildWithSet(vms, set)
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe (sm+wks fusionnés, ws-old hors registre ignorée), obtenu %d", len(groups))
	}
	var fused *model.Group
	for i := range groups {
		if groups[i].Members["sm"] == "sm1" {
			fused = &groups[i]
		}
	}
	if fused == nil {
		t.Fatalf("groupe sm+wks introuvable: %+v", groups)
	}
	if fused.Members["wks"] != "wks1" || fused.Members["ops"] != "ops1" {
		t.Fatalf("membres incorrects: %+v", fused.Members)
	}
	if again := RebuildWithSet(vms, set); len(again) != 1 || again[0].ID != groups[0].ID {
		t.Fatalf("RebuildWithSet non déterministe")
	}
}
