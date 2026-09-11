package grouping

import (
	"testing"

	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func TestRebuild_Empty(t *testing.T) {
	if got := Rebuild(nil); len(got) != 0 {
		t.Fatalf("attendu 0 groupes, obtenu %d", len(got))
	}
}

func TestRebuild_UnknownIgnored(t *testing.T) {
	vms := []model.VM{
		{ID: "u1", IP: "10.0.0.9", Family: model.FamilyUnknown},
		{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM},
	}
	groups := Rebuild(vms)
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe (unknown ignorée), obtenu %d", len(groups))
	}
	if _, ok := groups[0].Members[model.FamilyUnknown]; ok {
		t.Fatalf("unknown ne devrait pas apparaître: %+v", groups[0].Members)
	}
}

func TestRebuild_OrphanOA(t *testing.T) {
	vms := []model.VM{
		{ID: "oa1", IP: "10.0.0.5", Family: model.FamilyOA},
		{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM},
	}
	groups := Rebuild(vms)
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe, obtenu %d", len(groups))
	}
	if _, ok := groups[0].Members[model.FamilyOA]; ok {
		t.Fatalf("oa orpheline ne devrait pas être rattachée: %+v", groups[0].Members)
	}
}

func TestRebuild_Transitive(t *testing.T) {
	vms := []model.VM{
		{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM,
			EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.2"}}},
		{ID: "cm1", IP: "10.0.0.2", Family: model.FamilyCM,
			EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.3"}}},
		{ID: "ws1", IP: "10.0.0.3", Family: model.FamilyWS},
	}
	groups := Rebuild(vms)
	if len(groups) != 1 {
		t.Fatalf("fusion transitive attendue (1 groupe), obtenu %d", len(groups))
	}
}
