package grouping

import (
	"testing"

	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func TestRebuild_SingleGroupWithOA(t *testing.T) {
	vms := []model.VM{
		{
			ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM,
			EtcHosts: []model.EtcHostsEntry{
				{IP: "10.0.0.2", Hostname: "cm1"},
				{IP: "10.0.0.3", Hostname: "ws1"},
			},
		},
		{
			ID: "cm1", IP: "10.0.0.2", Family: model.FamilyCM,
			EtcHosts: []model.EtcHostsEntry{
				{IP: "10.0.0.1", Hostname: "sm1"},
			},
		},
		{
			ID: "ws1", IP: "10.0.0.3", Family: model.FamilyWS,
			EtcHosts: []model.EtcHostsEntry{
				{IP: "10.0.0.4", Hostname: "oa1"},
			},
		},
		{
			ID: "oa1", IP: "10.0.0.4", Family: model.FamilyOA,
		},
	}

	groups := Rebuild(vms)
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe, obtenu %d", len(groups))
	}
	g := groups[0]
	if g.Members[model.FamilySM] != "sm1" || g.Members[model.FamilyCM] != "cm1" || g.Members[model.FamilyWS] != "ws1" {
		t.Fatalf("membres core incorrects: %+v", g.Members)
	}
	if g.Members[model.FamilyOA] != "oa1" {
		t.Fatalf("VM oa non rattachée au groupe: %+v", g.Members)
	}
}

func TestRebuild_IsolatedVM(t *testing.T) {
	vms := []model.VM{
		{ID: "sm-isolated", IP: "10.0.0.9", Family: model.FamilySM},
	}
	groups := Rebuild(vms)
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe (VM isolée seule dans son groupe), obtenu %d", len(groups))
	}
	if groups[0].Members[model.FamilySM] != "sm-isolated" {
		t.Fatalf("VM isolée absente du groupe: %+v", groups[0].Members)
	}
	if len(groups[0].Members) != 1 {
		t.Fatalf("le groupe isolé ne devrait avoir qu'un seul membre: %+v", groups[0].Members)
	}
}

func TestRebuild_OAAttachedToMultipleGroups(t *testing.T) {
	vms := []model.VM{
		{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM, EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.5", Hostname: "oa1"}}},
		{ID: "sm2", IP: "10.0.0.2", Family: model.FamilySM, EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.5", Hostname: "oa1"}}},
		{ID: "oa1", IP: "10.0.0.5", Family: model.FamilyOA},
	}
	groups := Rebuild(vms)
	if len(groups) != 2 {
		t.Fatalf("attendu 2 groupes distincts, obtenu %d", len(groups))
	}
	for _, g := range groups {
		if g.Members[model.FamilyOA] != "oa1" {
			t.Errorf("oa1 devrait être rattachée à chaque groupe: %+v", g.Members)
		}
	}
}

func TestRebuild_Deterministic(t *testing.T) {
	vms := []model.VM{
		{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM, EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.2", Hostname: "cm1"}}},
		{ID: "cm1", IP: "10.0.0.2", Family: model.FamilyCM},
	}
	g1 := Rebuild(vms)
	g2 := Rebuild(vms)
	if len(g1) != 1 || len(g2) != 1 || g1[0].ID != g2[0].ID {
		t.Fatalf("Rebuild non déterministe: %+v vs %+v", g1, g2)
	}
}
