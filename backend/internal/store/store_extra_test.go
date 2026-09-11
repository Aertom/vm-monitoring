package store

import (
	"sync"
	"testing"

	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func TestCheckinUnknownGroup(t *testing.T) {
	s := New()
	if err := s.Checkin("inexistant"); err != ErrGroupNotFound {
		t.Fatalf("attendu ErrGroupNotFound, obtenu %v", err)
	}
}

func TestCheckoutSameUserIdempotent(t *testing.T) {
	s := New()
	s.ReplaceVMs([]model.VM{{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM}})
	gid := s.ListGroups()[0].ID
	if err := s.Checkout(gid, "alice"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if err := s.Checkout(gid, "alice"); err != nil {
		t.Fatalf("re-checkout même user devrait être idempotent: %v", err)
	}
	g, _ := s.GetGroup(gid)
	if g.InUseBy != "alice" {
		t.Fatalf("InUseBy=%q", g.InUseBy)
	}
}

func TestGetMissing(t *testing.T) {
	s := New()
	if _, ok := s.GetVM("x"); ok {
		t.Errorf("GetVM devrait échouer")
	}
	if _, ok := s.GetGroup("x"); ok {
		t.Errorf("GetGroup devrait échouer")
	}
}

func TestReplaceVMsEmpty(t *testing.T) {
	s := New()
	s.ReplaceVMs([]model.VM{{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM}})
	s.ReplaceVMs(nil)
	if got := s.ListVMs(); len(got) != 0 {
		t.Fatalf("attendu 0 VMs, obtenu %d", len(got))
	}
	if got := s.ListGroups(); len(got) != 0 {
		t.Fatalf("attendu 0 groupes, obtenu %d", len(got))
	}
}

func TestConcurrentCheckout(t *testing.T) {
	s := New()
	s.ReplaceVMs([]model.VM{{ID: "sm1", IP: "10.0.0.1", Family: model.FamilySM}})
	gid := s.ListGroups()[0].ID
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Checkout(gid, "alice")
			_, _ = s.GetGroup(gid)
			_ = s.ListGroups()
		}()
	}
	wg.Wait()
}
