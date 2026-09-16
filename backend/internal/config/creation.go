// Package config — suite : configuration de création de VMs (vmCreation.yaml).
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ISOEntry décrit une image RedHat proposée dans le formulaire.
type ISOEntry struct {
	Name      string `yaml:"name" json:"name"`
	File      string `yaml:"file" json:"file"`
	OSVariant string `yaml:"osVariant" json:"osVariant"`
	GuestOS   string `yaml:"guestOS" json:"guestOS"`
	Image     string `yaml:"image" json:"image"`
}

// VMTypePreset regroupe les ressources par type (serveur, workstation).
type VMTypePreset struct {
	CPU    int `yaml:"cpu" json:"cpu"`
	RAMGB  int `yaml:"ramGB" json:"ramGB"`
	DiskGB int `yaml:"diskGB" json:"diskGB"`
}

// CreationConfig pilote l'onglet Create (vmCreation.yaml).
type CreationConfig struct {
	ISOs          []ISOEntry              `yaml:"isos" json:"isos"`
	Types         map[string]VMTypePreset `yaml:"types" json:"types"`
	ESXiHWVersion string                  `yaml:"esxiHWVersion" json:"esxiHWVersion"`
	ESXiFirmware  string                  `yaml:"esxiFirmware" json:"esxiFirmware"`
	KVMGraphics   string                  `yaml:"kvmGraphics" json:"kvmGraphics"`
}

// DefaultCreation retourne les réglages par défaut (types/ESXi/KVM).
// La liste des ISOs n'a pas de défaut : elle vient du yaml.
func DefaultCreation() *CreationConfig {
	return &CreationConfig{
		Types: map[string]VMTypePreset{
			"serveur":     {CPU: 4, RAMGB: 16, DiskGB: 100},
			"workstation": {CPU: 2, RAMGB: 8, DiskGB: 60},
		},
		ESXiHWVersion: "20",
		ESXiFirmware:  "efi",
		KVMGraphics:   "vnc",
	}
}

// LoadCreation charge vmCreation.yaml et fusionne sur les défauts
// (le fichier gagne quand renseigné). Fichier absent = défauts seuls.
func LoadCreation(path string) (*CreationConfig, error) {
	cfg := DefaultCreation()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("lecture vmCreation %q: %w", path, err)
	}
	var file CreationConfig
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing vmCreation %q: %w", path, err)
	}
	if len(file.ISOs) > 0 {
		cfg.ISOs = file.ISOs
	}
	if cfg.Types == nil {
		cfg.Types = map[string]VMTypePreset{}
	}
	for k, v := range file.Types {
		cfg.Types[k] = v
	}
	if file.ESXiHWVersion != "" {
		cfg.ESXiHWVersion = file.ESXiHWVersion
	}
	if file.ESXiFirmware != "" {
		cfg.ESXiFirmware = file.ESXiFirmware
	}
	if file.KVMGraphics != "" {
		cfg.KVMGraphics = file.KVMGraphics
	}
	return cfg, nil
}
