package model

import "strings"

// DetectFamily recherche, de façon insensible à la casse, un mot-clé de
// famille dans le hostname fourni. Retourne FamilyUnknown si aucun mot-clé
// ne correspond. En cas de correspondances multiples, l'ordre de AllFamilies
// fait foi (sm, cm, ws, oa).
func DetectFamily(hostname string) Family {
	lower := strings.ToLower(hostname)
	for _, f := range AllFamilies {
		if strings.Contains(lower, string(f)) {
			return f
		}
	}
	return FamilyUnknown
}
