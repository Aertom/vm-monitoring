// Package store fournit le stockage en mémoire, thread-safe, des VMs et des
// groupes reconstruits. C'est la seule source de vérité en lecture pour
// l'API HTTP et le frontend.
package store

import (
	"strings"
	"sync"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/grouping"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

// Store est un stockage en mémoire thread-safe des VMs et des groupes.
type Store struct {
	mu       sync.RWMutex
	vms      map[string]model.VM
	groups   map[string]model.Group
	families *model.FamilySet
}

// New crée un Store vide (familles par défaut).
func New() *Store {
	return NewWithFamilies(nil)
}

// NewWithFamilies crée un Store vide avec un registre de familles
// configurable (nil = défaut).
func NewWithFamilies(set *model.FamilySet) *Store {
	if set == nil {
		set = model.DefaultFamilies()
	}
	return &Store{
		vms:      make(map[string]model.VM),
		groups:   make(map[string]model.Group),
		families: set,
	}
}

// ReplaceVMs remplace intégralement l'ensemble des VMs connues (typiquement
// après un cycle de découverte+collecte), puis reconstruit les groupes à
// partir des /etc/hosts collectés. Les statuts InUseBy/CheckedOutAt et l'alias
// Name des groupes existants sont préservés si le groupe reconstruit a le même ID.
func (s *Store) ReplaceVMs(vms []model.VM) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newVMs := make(map[string]model.VM, len(vms))
	for _, vm := range vms {
		newVMs[vm.ID] = vm
	}
	s.vms = newVMs

	rebuilt := grouping.RebuildWithSet(vms, s.families)
	newGroups := make(map[string]model.Group, len(rebuilt))
	for _, g := range rebuilt {
		if old, ok := s.groups[g.ID]; ok {
			g.InUseBy = old.InUseBy
			g.CheckedOutAt = old.CheckedOutAt
			g.Name = old.Name
		}
		newGroups[g.ID] = g
	}
	s.groups = newGroups
}

// ListVMs retourne une copie de toutes les VMs connues.
func (s *Store) ListVMs() []model.VM {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.VM, 0, len(s.vms))
	for _, vm := range s.vms {
		out = append(out, vm)
	}
	return out
}

// GetVM retourne la VM d'ID donné.
func (s *Store) GetVM(id string) (model.VM, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vm, ok := s.vms[id]
	return vm, ok
}

// ListGroups retourne une copie de tous les groupes connus.
func (s *Store) ListGroups() []model.Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, g)
	}
	return out
}

// GetGroup retourne le groupe d'ID donné.
func (s *Store) GetGroup(id string) (model.Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[id]
	return g, ok
}

// ErrGroupNotFound est retourné par Checkout/Checkin si le groupe n'existe pas.
var ErrGroupNotFound = &StoreError{"groupe introuvable"}

// ErrGroupAlreadyInUse est retourné par Checkout si le groupe est déjà utilisé.
var ErrGroupAlreadyInUse = &StoreError{"groupe déjà en cours d'utilisation"}

// ErrInvalidName est retourné par Rename si l'alias est vide ou trop long (64 max).
var ErrInvalidName = &StoreError{"nom invalide"}

// StoreError est une erreur simple du package store.
type StoreError struct{ msg string }

func (e *StoreError) Error() string { return e.msg }

// Checkout marque un groupe comme "en cours d'utilisation" par l'utilisateur
// donné. Échoue si le groupe n'existe pas ou est déjà utilisé par un autre.
func (s *Store) Checkout(groupID, user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}
	if g.InUseBy != "" && g.InUseBy != user {
		return ErrGroupAlreadyInUse
	}
	g.InUseBy = user
	g.CheckedOutAt = time.Now().UTC()
	s.groups[groupID] = g
	return nil
}

// Checkin libère un groupe précédemment marqué "en cours d'utilisation".
func (s *Store) Checkin(groupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}
	g.InUseBy = ""
	g.CheckedOutAt = time.Time{}
	s.groups[groupID] = g
	return nil
}

// Rename définit l'alias libre d'un groupe (le ID déterministe est inchangé).
func (s *Store) Rename(groupID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}
	g.Name = name
	s.groups[groupID] = g
	return nil
}
