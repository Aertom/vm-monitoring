// Package config définit et charge la configuration de l'application
// (config.yaml) ainsi que celle des hyperviseurs (hypervisors.yaml).
// Ces structures sont l'unique source de vérité : elles ne doivent jamais
// être redéfinies ailleurs dans le code.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

// Config est la configuration générale de l'application (config.yaml).
type Config struct {
	// ListenAddr est l'adresse d'écoute de l'API HTTP, ex: ":8080".
	ListenAddr string `yaml:"listenAddr"`
	// PollInterval est l'intervalle entre deux cycles de découverte/collecte,
	// exprimé en secondes.
	PollIntervalSeconds int `yaml:"pollIntervalSeconds"`
	// SSH contient les paramètres par défaut de connexion SSH aux VMs.
	SSH SSHConfig `yaml:"ssh"`
	// StaticVMs est un inventaire statique de VMs utilisé en l'absence de
	// découverte automatique activée (mode "commit initial").
	StaticVMs []StaticVM `yaml:"staticVMs"`
	// AppDirs associe chaque famille de VM (sm, cm, ws, oa, unknown) aux
	// dossiers scrutés via SSH pour les versions d'applications.
	// Famille absente ou vide = ["/opt"]. Les clés doivent reprendre les
	// noms configurés dans Families.
	AppDirs map[string][]string `yaml:"appDirs"`
	// Families redéfinit les familles de VMs (détection + rôles). Vide =
	// comportement historique (sm/cm/ws core, oa partagée). Exemple pour
	// renommer ws en wks : [{name: wks, shared: false}].
	Families []model.FamilyDef `yaml:"families"`
}

// SSHConfig regroupe les paramètres de connexion SSH par défaut.
type SSHConfig struct {
	User           string `yaml:"user"`
	PrivateKeyPath string `yaml:"privateKeyPath"`
	Port           int    `yaml:"port"`
	TimeoutSeconds int    `yaml:"timeoutSeconds"`
}

// StaticVM représente une VM déclarée statiquement dans config.yaml, utilisée
// tant qu'aucune découverte automatique d'hyperviseur n'est configurée.
type StaticVM struct {
	ID       string `yaml:"id"`
	Hostname string `yaml:"hostname"`
	IP       string `yaml:"ip"`
}

// HypervisorsConfig est la configuration des hyperviseurs à interroger pour
// la découverte automatique des VMs (hypervisors.yaml). Ce type est la seule
// définition existante de la configuration hyperviseurs dans tout le projet.
type HypervisorsConfig struct {
	ESXi    []ESXiConfig    `yaml:"esxi"`
	Nutanix []NutanixConfig `yaml:"nutanix"`
	KVM     []KVMConfig     `yaml:"kvm"`
}

// ESXiConfig décrit l'accès à un hôte ESXi ou un vCenter.
type ESXiConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Insecure bool   `yaml:"insecure"`
}

// NutanixConfig décrit l'accès à un cluster Nutanix AHV via l'API Prism.
type NutanixConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Insecure bool   `yaml:"insecure"`
}

// KVMConfig décrit l'accès à un hôte KVM/libvirt via SSH+virsh.
type KVMConfig struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
	User string `yaml:"user"`
	Port int    `yaml:"port"`
}

// Load charge la configuration générale depuis le fichier YAML indiqué.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lecture config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 60
	}
	if cfg.SSH.Port <= 0 {
		cfg.SSH.Port = 22
	}
	if cfg.SSH.TimeoutSeconds <= 0 {
		cfg.SSH.TimeoutSeconds = 5
	}
	return &cfg, nil
}

// LoadHypervisors charge la configuration des hyperviseurs depuis le fichier
// YAML indiqué. Si le fichier n'existe pas, retourne une configuration vide
// sans erreur (la découverte automatique est alors simplement désactivée).
func LoadHypervisors(path string) (*HypervisorsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &HypervisorsConfig{}, nil
		}
		return nil, fmt.Errorf("lecture hypervisors %q: %w", path, err)
	}
	var cfg HypervisorsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing hypervisors %q: %w", path, err)
	}
	return &cfg, nil
}
