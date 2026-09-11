package model

import (
	"fmt"
	"strings"
)

// DetectFamily recherche, de façon insensible à la casse, un mot-clé de
// famille dans le hostname fourni. Retourne FamilyUnknown si aucun mot-clé
// ne correspond. En cas de correspondances multiples, l'ordre de AllFamilies
// fait foi (sm, cm, ws, oa). Comportement par défaut ; préférez FamilySet
// quand les familles sont configurables (config.yaml: families).
func DetectFamily(hostname string) Family {
	return DefaultFamilies().Detect(hostname)
}

// FamilyDef décrit une famille configurable (config.yaml: families).
// Exemple : {name: wks, match: [wks], shared: false} pour renommer ws en wks.
type FamilyDef struct {
	// Name est l'identifiant de la famille (ex: wks). "unknown" est réservé.
	Name string `yaml:"name" json:"name"`
	// Match liste les sous-chaînes détectées dans le hostname (insensible à
	// la casse). Vide = [name].
	Match []string `yaml:"match" json:"match"`
	// Shared marque une famille partagée (rôle oa) : rattachée aux groupes
	// qui la référencent, exclue de l'ID de groupe. Sinon famille "core"
	// (regroupée par union-find, au plus une par groupe).
	Shared bool `yaml:"shared" json:"shared"`
}

// FamilySet est le registre des familles actives : détection, rôles, ordre.
type FamilySet struct {
	defs []FamilyDef
}

// DefaultFamilies retourne le jeu historique : sm/cm/ws core, oa partagée.
func DefaultFamilies() *FamilySet {
	s, _ := NewFamilySet([]FamilyDef{
		{Name: string(FamilySM)},
		{Name: string(FamilyCM)},
		{Name: string(FamilyWS)},
		{Name: string(FamilyOA), Shared: true},
	})
	return s
}

// NewFamilySet valide et construit un registre. Une liste vide donne le
// comportement par défaut. Erreur si : nom vide, "unknown" réservé, doublon
// (insensible à la casse).
func NewFamilySet(defs []FamilyDef) (*FamilySet, error) {
	if len(defs) == 0 {
		return DefaultFamilies(), nil
	}
	seen := make(map[string]bool, len(defs))
	out := make([]FamilyDef, 0, len(defs))
	for i, d := range defs {
		name := strings.ToLower(strings.TrimSpace(d.Name))
		if name == "" {
			return nil, fmt.Errorf("famille %d: nom vide", i)
		}
		if name == string(FamilyUnknown) {
			return nil, fmt.Errorf("famille %q réservée", d.Name)
		}
		if seen[name] {
			return nil, fmt.Errorf("famille %q en double", d.Name)
		}
		seen[name] = true
		match := make([]string, 0, len(d.Match))
		for _, m := range d.Match {
			if m = strings.ToLower(strings.TrimSpace(m)); m != "" {
				match = append(match, m)
			}
		}
		if len(match) == 0 {
			match = []string{name}
		}
		out = append(out, FamilyDef{Name: name, Match: match, Shared: d.Shared})
	}
	return &FamilySet{defs: out}, nil
}

// Detect retourne la famille du hostname, FamilyUnknown si aucune ne matche.
// L'ordre de déclaration fait foi en cas de correspondances multiples.
func (s *FamilySet) Detect(hostname string) Family {
	if s == nil {
		return DefaultFamilies().Detect(hostname)
	}
	lower := strings.ToLower(hostname)
	for _, d := range s.defs {
		for _, m := range d.Match {
			if strings.Contains(lower, m) {
				return Family(d.Name)
			}
		}
	}
	return FamilyUnknown
}

// IsShared indique une famille partagée (rôle oa).
func (s *FamilySet) IsShared(f Family) bool {
	if s == nil {
		return f == FamilyOA
	}
	for _, d := range s.defs {
		if Family(d.Name) == f {
			return d.Shared
		}
	}
	return false
}

// IsCore indique une famille groupée par union-find (connue et non partagée).
func (s *FamilySet) IsCore(f Family) bool {
	if s == nil {
		return f == FamilySM || f == FamilyCM || f == FamilyWS
	}
	for _, d := range s.defs {
		if Family(d.Name) == f {
			return !d.Shared
		}
	}
	return false
}

// Names retourne les noms configurés dans l'ordre (pour /api/families).
func (s *FamilySet) Names() []Family {
	if s == nil {
		return []Family{FamilySM, FamilyCM, FamilyWS, FamilyOA}
	}
	out := make([]Family, 0, len(s.defs))
	for _, d := range s.defs {
		out = append(out, Family(d.Name))
	}
	return out
}
