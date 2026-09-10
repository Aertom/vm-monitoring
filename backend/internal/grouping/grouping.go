// Package grouping reconstruit les groupes fonctionnels de VMs à partir des
// références croisées trouvées dans les fichiers /etc/hosts collectés par SSH.
//
// Règle : un groupe contient au plus une VM par famille sm/cm/ws, et peut
// contenir une ou plusieurs VM oa (une VM oa peut appartenir à plusieurs
// groupes). L'appartenance est déduite par IP : si le /etc/hosts d'une VM A
// contient l'IP d'une VM B (d'une autre famille sm/cm/ws), alors A et B sont
// considérées comme faisant partie du même groupe. Les groupes ainsi trouvés
// sont fusionnés par transitivité (union-find) pour sm/cm/ws ; une VM oa est
// ensuite rattachée à chaque groupe qui la référence.
package grouping

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"

	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

// Rebuild reconstruit l'ensemble des groupes à partir de la liste de VMs
// fournie (avec leurs EtcHosts déjà renseignés). Elle est pure et
// déterministe : à entrée égale, la sortie est toujours identique, ce qui
// permet de préserver les statuts InUseBy/CheckedOutAt d'un cycle à l'autre
// via un remapping fait par l'appelant (le store) sur la base de l'ID de groupe.
func Rebuild(vms []model.VM) []model.Group {
	byID := make(map[string]model.VM, len(vms))
	ipToID := make(map[string]string, len(vms))
	for _, vm := range vms {
		byID[vm.ID] = vm
		if vm.IP != "" {
			ipToID[vm.IP] = vm.ID
		}
	}

	// union-find restreint aux familles sm/cm/ws.
	parent := make(map[string]string)
	var find func(string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	coreFamilies := map[model.Family]bool{
		model.FamilySM: true,
		model.FamilyCM: true,
		model.FamilyWS: true,
	}

	for _, vm := range vms {
		if coreFamilies[vm.Family] {
			parent[vm.ID] = vm.ID
		}
	}

	// Références croisées : si le /etc/hosts d'une VM core référence l'IP
	// d'une autre VM core, on les unit dans le même groupe.
	for _, vm := range vms {
		if !coreFamilies[vm.Family] {
			continue
		}
		for _, entry := range vm.EtcHosts {
			otherID, ok := ipToID[entry.IP]
			if !ok || otherID == vm.ID {
				continue
			}
			other := byID[otherID]
			if coreFamilies[other.Family] {
				union(vm.ID, otherID)
			}
		}
	}

	// Regroupement des VMs core par racine union-find.
	rootMembers := make(map[string]map[model.Family]string)
	for id := range parent {
		root := find(id)
		vm := byID[id]
		if rootMembers[root] == nil {
			rootMembers[root] = make(map[model.Family]string)
		}
		rootMembers[root][vm.Family] = id
	}

	groups := make(map[string]*model.Group)
	rootToGroupID := make(map[string]string)
	for root, members := range rootMembers {
		gid := groupID(members)
		rootToGroupID[root] = gid
		groups[gid] = &model.Group{ID: gid, Members: cloneMembers(members)}
	}

	// Rattachement des VMs oa : une VM oa est rattachée à un groupe si son
	// IP est référencée dans le /etc/hosts d'une VM core de ce groupe, ou si
	// son propre /etc/hosts référence une VM core de ce groupe.
	for _, vm := range vms {
		if vm.Family != model.FamilyOA {
			continue
		}
		attachedRoots := make(map[string]bool)

		// Cas 1: le /etc/hosts de la VM oa référence une VM core.
		for _, entry := range vm.EtcHosts {
			otherID, ok := ipToID[entry.IP]
			if !ok {
				continue
			}
			other := byID[otherID]
			if coreFamilies[other.Family] {
				if root := find(otherID); root != "" {
					attachedRoots[root] = true
				}
			}
		}

		// Cas 2: une VM core référence l'IP de cette VM oa dans son /etc/hosts.
		for _, core := range vms {
			if !coreFamilies[core.Family] {
				continue
			}
			for _, entry := range core.EtcHosts {
				if entry.IP == vm.IP {
					if root := find(core.ID); root != "" {
						attachedRoots[root] = true
					}
				}
			}
		}

		for root := range attachedRoots {
			gid := rootToGroupID[root]
			if gid == "" {
				continue
			}
			groups[gid].Members[model.FamilyOA] = vm.ID
		}
	}

	result := make([]model.Group, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// groupID calcule un identifiant stable et déterministe pour un groupe à
// partir des IDs de VMs sm/cm/ws qui le composent (l'oa n'entre pas dans le
// calcul car une même oa peut être rattachée à plusieurs groupes).
func groupID(members map[model.Family]string) string {
	keys := []model.Family{model.FamilySM, model.FamilyCM, model.FamilyWS}
	parts := make([]string, 0, 3)
	for _, f := range keys {
		if id, ok := members[f]; ok {
			parts = append(parts, string(f)+":"+id)
		}
	}
	sort.Strings(parts)
	h := sha1.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func cloneMembers(m map[model.Family]string) map[model.Family]string {
	out := make(map[model.Family]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
