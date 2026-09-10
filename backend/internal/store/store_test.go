package store

import (
	"testing"

	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func TestReplaceVMsAndListGroups(t *testing.T) {
	s := New()
	vms := []model.VM{
		{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM, Hostname: "app-sm-01"},
		{ID: "cm1", IP: "10.0.0.2", Family: model.FamilyCM, Hostname: "app-cm-01",
			EtcHosts: []model.EtcHostsEntry{{IP: "10.0.0.1", Hostname: "app-sm-01"}}},
	}
	s.ReplaceVMs(vms)

	if got := s.ListVMs(); len(got) != 2 {
		t.Fatalf("attendu 2 VMs, obtenu %d", len(got))
	}
	groups := s.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe, obtenu %d", len(groups))
	}
}

func TestCheckoutCheckin(t *testing.T) {
	s := New()
	vms := []model.VM{{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM}}
	s.ReplaceVMs(vms)
	groups := s.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("attendu 1 groupe, obtenu %d", len(groups))
	}
	gid := groups[0].ID

	if err := s.Checkout(gid, "alice"); err != nil {
		t.Fatalf("checkout inattendu en erreur: %v", err)
	}
	if err := s.Checkout(gid, "bob"); err != ErrGroupAlreadyInUse {
		t.Fatalf("attendu ErrGroupAlreadyInUse, obtenu %v", err)
	}
	if err := s.Checkin(gid); err != nil {
		t.Fatalf("checkin inattendu en erreur: %v", err)
	}
	g, _ := s.GetGroup(gid)
	if g.InUseBy != "" {
		t.Fatalf("le groupe devrait être libre après checkin, InUseBy=%q", g.InUseBy)
	}
}

func TestCheckoutPreservedAcrossReplaceVMs(t *testing.T) {
	s := New()
	vms := []model.VM{{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM}}
	s.ReplaceVMs(vms)
	gid := s.ListGroups()[0].ID
	_ = s.Checkout(gid, "alice")

	// Un nouveau cycle de collecte avec les mêmes VMs doit préserver le statut.
	s.ReplaceVMs(vms)
	g, ok := s.GetGroup(gid)
	if !ok {
		t.Fatalf("groupe %q introuvable après ReplaceVMs", gid)
	}
	if g.InUseBy != "alice" {
		t.Fatalf("statut InUseBy non préservé après ReplaceVMs: %q", g.InUseBy)
	}
}

func TestCheckoutUnknownGroup(t *testing.T) {
	s := New()
	if err := s.Checkout("inexistant", "alice"); err != ErrGroupNotFound {
		t.Fatalf("attendu ErrGroupNotFound, obtenu %v", err)
	}
}
