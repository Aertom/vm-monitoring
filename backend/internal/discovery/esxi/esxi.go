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
	"net/http"
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
		Name       string `json:"name"`
		PowerState string `json:"power_state"`
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("esxi: réponse HTTP inattendue: %d", resp.StatusCode)
	}

	var payload esxiVMPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("esxi: décodage réponse: %w", err)
	}

	vms := make([]VM, 0, len(payload.Value))
	for _, v := range payload.Value {
		vms = append(vms, VM{
			Name:       v.Name,
			PowerState: v.PowerState,
			Hypervisor: "esxi",
		})
	}
	return vms, nil
}
