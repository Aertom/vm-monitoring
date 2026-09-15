// Package esxi fournit la découverte automatique des machines virtuelles
// hébergées sur un hyperviseur VMware ESXi (ou vCenter), via son API REST.
//
// Ce package est volontairement autonome : il ne dépend que de la
// bibliothèque standard Go, afin de pouvoir être testé et validé
// indépendamment du reste du backend. L'intégration avec les packages
// internes (model, store, api) sera réalisée dans un lot dédié.
package esxi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config décrit les paramètres de connexion à un hôte ESXi ou à un vCenter.
type Config struct {
	// Host est l'adresse (IP ou nom DNS) de l'hôte ESXi/vCenter.
	Host string
	// Username est le compte utilisé pour l'authentification à l'API.
	Username string
	// Password est le mot de passe associé à Username.
	Password string
	// InsecureSkipVerify désactive la vérification du certificat TLS
	// (utile en environnement de test avec certificats auto-signés).
	InsecureSkipVerify bool
	// Timeout définit la durée maximale allouée à chaque appel HTTP.
	Timeout time.Duration
}

// VM représente une machine virtuelle telle que découverte sur l'hyperviseur ESXi.
type VM struct {
	// ID est l'identifiant vSphere (ex: "vm-123"), nécessaire aux appels détail.
	ID string `json:"vm,omitempty"`
	// Name est le nom de la VM tel que déclaré dans l'inventaire ESXi.
	Name string `json:"name"`
	// PowerState indique l'état d'alimentation ("poweredOn", "poweredOff", "suspended").
	PowerState string `json:"power_state"`
	// IPAddress est l'adresse IP principale rapportée par VMware Tools, si disponible.
	IPAddress string `json:"ip_address,omitempty"`
	// Hypervisor identifie la source de découverte ("esxi").
	Hypervisor string `json:"hypervisor"`
}

// HTTPClient est l'interface minimale requise pour effectuer les appels API.
// Elle permet l'injection d'un client HTTP simulé dans les tests unitaires.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Discoverer réalise la découverte automatique des VMs sur un hôte ESXi.
type Discoverer struct {
	cfg    Config
	client HTTPClient
}

// NewDiscoverer construit un Discoverer prêt à l'emploi pour la configuration donnée.
// Si client est nil, un http.Client standard est utilisé avec le Timeout de cfg.
func NewDiscoverer(cfg Config, client HTTPClient) *Discoverer {
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Discoverer{cfg: cfg, client: client}
}

// esxiVMPayload modélise la portion utile de la réponse JSON de l'API REST vSphere
// pour l'endpoint de listing des VMs (/rest/vcenter/vm).
type esxiVMPayload struct {
	Value []struct {
		VM         string `json:"vm"`
		Name       string `json:"name"`
		PowerState string `json:"power_state"`
	} `json:"value"`
}

// guestIdentityPayload modélise /rest/vcenter/vm/{vm}/guest/identity
// (adresse IP via VMware Tools).
type guestIdentityPayload struct {
	Value struct {
		IPAddress string `json:"ip_address"`
	} `json:"value"`
}

// Discover interroge l'hôte ESXi configuré et retourne la liste des VMs découvertes.
// Elle retourne une erreur explicite en cas d'échec réseau, d'authentification
// ou de décodage de la réponse.
func (d *Discoverer) Discover(ctx context.Context) ([]VM, error) {
	if d.cfg.Host == "" {
		return nil, fmt.Errorf("esxi: host is required")
	}

	url := fmt.Sprintf("https://%s/rest/vcenter/vm", d.cfg.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("esxi: construction requête: %w", err)
	}
	req.SetBasicAuth(d.cfg.Username, d.cfg.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("esxi: appel API: %w", err)
	}

	// Certains vCenter/vSphere 8 refusent le basic-auth par requête (401) :
	// on bascule sur une session explicite puis on rejoue l'appel.
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		sessionID, serr := d.createSession(ctx)
		if serr != nil {
			return nil, fmt.Errorf("esxi: auth refusée puis %v", serr)
		}
		req2, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("esxi: construction requête: %w", err)
		}
		req2.Header.Set("Accept", "application/json")
		req2.Header.Set("vmware-api-session-id", sessionID)
		resp, err = d.client.Do(req2)
		if err != nil {
			return nil, fmt.Errorf("esxi: appel API (session): %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("esxi: réponse HTTP inattendue: %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload esxiVMPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("esxi: décodage réponse: %w", err)
	}

	vms := make([]VM, 0, len(payload.Value))
	for _, v := range payload.Value {
		vms = append(vms, VM{
			ID:         v.VM,
			Name:       v.Name,
			PowerState: v.PowerState,
			Hypervisor: "esxi",
		})
	}
	return vms, nil
}

// createSession ouvre une session vAPI (POST /rest/com/vmware/cis/session)
// pour les serveurs refusant le basic-auth par requête.
func (d *Discoverer) createSession(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://%s/rest/com/vmware/cis/session", d.cfg.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("construction requête session: %w", err)
	}
	req.SetBasicAuth(d.cfg.Username, d.cfg.Password)
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("appel session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("session HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("décodage session: %w", err)
	}
	if payload.Value == "" {
		return "", fmt.Errorf("session vide")
	}
	return payload.Value, nil
}

// GuestIP retourne l'IP invitée d'une VM via son identité guest
// (/rest/vcenter/vm/{vm}/guest/identity, VMware Tools requis).
// Vide + erreur si indisponible (best-effort pour l'appelant).
func (d *Discoverer) GuestIP(ctx context.Context, vmID string) (string, error) {
	if vmID == "" {
		return "", fmt.Errorf("esxi: vm id requis")
	}
	url := fmt.Sprintf("https://%s/rest/vcenter/vm/%s/guest/identity", d.cfg.Host, vmID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("esxi: construction requête guest: %w", err)
	}
	req.SetBasicAuth(d.cfg.Username, d.cfg.Password)
	req.Header.Set("Accept", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("esxi: appel guest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("esxi: guest HTTP %d", resp.StatusCode)
	}
	var payload guestIdentityPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("esxi: décodage guest: %w", err)
	}
	return payload.Value.IPAddress, nil
}

// guestIPRe extrait ipAddress de `vim-cmd vmsvc/get.guest` :
// ipAddress = "192.168.1.20",
var guestIPRe = regexp.MustCompile(`(?m)^\s*ipAddress\s*=\s*"([^"]+)"`)

// ParseGuestIPAddress extrait l'IP invitée de `vim-cmd vmsvc/get.guest`.
// Vide si absente (VM éteinte ou Tools absents).
func ParseGuestIPAddress(out string) string {
	if m := guestIPRe.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// ParseVimCmdGetAllVMs parse la sortie de `vim-cmd vmsvc/getallvms`,
// la voie de découverte des ESXi standalone (l'API REST /rest/vcenter
// n'existe que sur vCenter). Exemple :
//
//	Vmid     Name          File                            Guest OS      Version
//	128      cm-prod-01    [datastore1] cm-prod-01/...     otherLinux64  vmx-21
//
// Seuls Vmid (numérique, 1re colonne) et Name (2e colonne) sont retenus :
// l'en-tête et les lignes parasites sont ignorées, et les noms contenant
// des espaces ne sont pas supportés. PowerState reste vide (non fourni).
func ParseVimCmdGetAllVMs(out string) []VM {
	var vms []VM
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		vms = append(vms, VM{ID: fields[0], Name: fields[1], Hypervisor: "esxi"})
	}
	return vms
}
