// Package ahv fournit un client de découverte des VMs hébergées sur un
// cluster Nutanix AHV via l'API REST Prism (v2 /vms).
//
// Ce package est volontairement autonome (dépendances stdlib uniquement)
// afin de pouvoir être testé et intégré indépendamment du reste de
// l'application. L'intégration avec les types internes (model, store)
// sera réalisée dans un lot dédié.
package ahv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VM représente une machine virtuelle telle que renvoyée par l'API Prism
// (sous-ensemble minimal des champs utiles à la découverte).
type VM struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	PowerState string `json:"power_state"`
	NumVCPUs   int    `json:"num_vcpus"`
	MemoryMB   int64  `json:"memory_mb"`
	HostUUID   string `json:"host_uuid"`
}

// vmListResponse modélise la réponse paginée de l'API Prism v2 /vms.
type vmListResponse struct {
	Entities []VM `json:"entities"`
}

// HTTPClient est l'interface minimale nécessaire à l'appel HTTP,
// permettant l'injection d'un client de test (mock).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config regroupe les paramètres de connexion à un cluster Nutanix AHV.
type Config struct {
	// Host est l'adresse (IP ou nom DNS) du Prism Element/Central, sans schéma.
	Host string
	// Port est le port HTTPS de l'API Prism (par défaut 9440 si vide/0).
	Port int
	// Username et Password sont les identifiants d'authentification basique.
	Username string
	Password string
	// InsecureSkipVerify n'est pas géré ici directement : le HTTPClient
	// fourni doit être configuré en conséquence par l'appelant.
}

// Client permet d'interroger un cluster Nutanix AHV pour lister ses VMs.
type Client struct {
	cfg        Config
	httpClient HTTPClient
}

// NewClient crée un client de découverte AHV. Si httpClient est nil,
// http.DefaultClient est utilisé avec un timeout de 15 secondes.
func NewClient(cfg Config, httpClient HTTPClient) (*Client, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("ahv: host is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if cfg.Port == 0 {
		cfg.Port = 9440
	}
	return &Client{cfg: cfg, httpClient: httpClient}, nil
}

// ListVMs interroge l'API Prism v2 /vms et retourne la liste des VMs
// découvertes sur le cluster.
func (c *Client) ListVMs(ctx context.Context) ([]VM, error) {
	url := fmt.Sprintf("https://%s:%d/PrismGateway/services/rest/v2.0/vms/", c.cfg.Host, c.cfg.Port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ahv: building request: %w", err)
	}
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ahv: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ahv: reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ahv: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var parsed vmListResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("ahv: decoding response: %w", err)
	}

	return parsed.Entities, nil
}
