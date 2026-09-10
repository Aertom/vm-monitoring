// Package model définit les types de données partagés par tout le backend :
// VM, Famille, Groupe, Statuts. Ces types constituent l'unique source de
// vérité utilisée par le store, l'API et les collecteurs.
package model

import "time"

// Family représente la famille fonctionnelle d'une VM, déduite d'un mot-clé
// présent (insensible à la casse) dans son hostname.
type Family string

const (
	FamilySM      Family = "sm"
	FamilyCM      Family = "cm"
	FamilyWS      Family = "ws"
	FamilyOA      Family = "oa"
	FamilyUnknown Family = "unknown"
)

// AllFamilies liste les mots-clés de famille reconnus, dans l'ordre de
// détection. L'ordre n'a pas d'incidence fonctionnelle mais est déterministe.
var AllFamilies = []Family{FamilySM, FamilyCM, FamilyWS, FamilyOA}

// Status représente l'état de joignabilité d'une VM lors de la dernière
// collecte SSH.
type Status string

const (
	StatusOK      Status = "ok"
	StatusError   Status = "error"
	StatusUnknown Status = "unknown"
)

// Hypervisor identifie la source de découverte d'une VM.
type Hypervisor string

const (
	HypervisorESXi    Hypervisor = "esxi"
	HypervisorNutanix Hypervisor = "nutanix"
	HypervisorKVM     Hypervisor = "kvm"
	HypervisorStatic  Hypervisor = "static"
)

// AppVersion représente une application détectée sur la VM et sa version.
type AppVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// VM représente une machine virtuelle découverte et supervisée.
type VM struct {
	// ID est un identifiant stable de la VM (ex: nom d'instance hyperviseur).
	ID string `json:"id"`
	// Hostname est le nom d'hôte tel que rapporté par l'hyperviseur ou le SSH.
	Hostname string `json:"hostname"`
	// IP est l'adresse IP principale utilisée pour la collecte SSH.
	IP string `json:"ip"`
	// Family est la famille déduite du hostname (sm/cm/ws/oa/unknown).
	Family Family `json:"family"`
	// Hypervisor est la source de découverte de cette VM.
	Hypervisor Hypervisor `json:"hypervisor"`
	// Status est le dernier statut de joignabilité SSH connu.
	Status Status `json:"status"`
	// Apps liste les applications/versions détectées lors de la dernière collecte.
	Apps []AppVersion `json:"apps"`
	// EtcHosts contient les lignes brutes utiles du fichier /etc/hosts,
	// utilisées pour la reconstruction des groupes.
	EtcHosts []EtcHostsEntry `json:"etcHosts"`
	// LastSeen est l'horodatage de la dernière collecte réussie ou tentée.
	LastSeen time.Time `json:"lastSeen"`
	// LastError contient le dernier message d'erreur de collecte, le cas échéant.
	LastError string `json:"lastError,omitempty"`
}

// EtcHostsEntry représente une entrée du fichier /etc/hosts d'une VM.
type EtcHostsEntry struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// Group représente un groupe fonctionnel de VMs (au plus une par famille
// sm/cm/ws, et une ou plusieurs oa), reconstruit à chaque cycle de collecte
// à partir des références croisées trouvées dans /etc/hosts.
type Group struct {
	// ID est un identifiant stable et déterministe du groupe (voir grouping.go).
	ID string `json:"id"`
	// Members associe chaque famille à l'ID de la VM occupant ce rôle dans le groupe.
	Members map[Family]string `json:"members"`
	// InUseBy est le nom de l'utilisateur ayant "checkout" ce groupe, vide si libre.
	InUseBy string `json:"inUseBy,omitempty"`
	// CheckedOutAt est l'horodatage du checkout, zéro si le groupe est libre.
	CheckedOutAt time.Time `json:"checkedOutAt,omitempty"`
}

// IsInUse indique si le groupe est actuellement marqué comme utilisé.
func (g *Group) IsInUse() bool {
	return g.InUseBy != ""
}
