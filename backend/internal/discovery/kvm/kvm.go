// Package kvm fournit un client de découverte des VMs hébergées sur un
// hyperviseur KVM/libvirt, en s'appuyant sur la sortie texte de la
// commande `virsh list --all` et `virsh dominfo <name>` exécutée via un
// exécuteur injectable (permet un accès local ou distant via SSH).
//
// Ce package est volontairement autonome (dépendances stdlib uniquement)
// afin de pouvoir être testé et intégré indépendamment du reste de
// l'application. L'intégration avec les types internes (model, store)
// sera réalisée dans un lot dédié.
package kvm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// VM représente une machine virtuelle KVM découverte via virsh.
type VM struct {
	ID       string
	Name     string
	State    string
	VCPUs    int
	MemoryKB int64
}

// CommandExecutor est l'interface minimale nécessaire à l'exécution des
// commandes virsh, permettant l'injection d'un exécuteur de test (mock)
// ou d'un exécuteur distant (SSH).
type CommandExecutor interface {
	// Run exécute la commande donnée avec ses arguments et retourne sa
	// sortie standard sous forme de chaîne, ou une erreur.
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// Client permet d'interroger un hôte KVM/libvirt pour lister ses VMs.
type Client struct {
	executor CommandExecutor
}

// NewClient crée un client de découverte KVM à partir d'un exécuteur de
// commandes. L'exécuteur ne peut pas être nil.
func NewClient(executor CommandExecutor) (*Client, error) {
	if executor == nil {
		return nil, fmt.Errorf("kvm: executor is required")
	}
	return &Client{executor: executor}, nil
}

// ListVMs exécute `virsh list --all` et parse la sortie pour extraire les
// VMs présentes (nom, id, état). Le nombre de vCPU et la mémoire ne sont
// pas résolus par cette méthode (voir GetVMDetails).
func (c *Client) ListVMs(ctx context.Context) ([]VM, error) {
	out, err := c.executor.Run(ctx, "virsh", "list", "--all")
	if err != nil {
		return nil, fmt.Errorf("kvm: running virsh list: %w", err)
	}
	return parseVirshList(out), nil
}

// parseVirshList parse la sortie texte de `virsh list --all`.
//
// Format attendu :
//
//	 Id   Name        State
//	----------------------------
//	 1    web-01      running
//	 -    web-02      shut off
func parseVirshList(out string) []VM {
	lines := strings.Split(out, "\n")
	var vms []VM

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Ignore l'en-tête et la ligne de séparateurs.
		if strings.HasPrefix(line, "Id") || strings.HasPrefix(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		id := fields[0]
		name := fields[1]
		state := strings.Join(fields[2:], " ")

		vms = append(vms, VM{
			ID:    id,
			Name:  name,
			State: state,
		})
	}

	return vms
}

// GetVMDetails exécute `virsh dominfo <name>` pour un domaine donné et
// enrichit les informations avec le nombre de vCPU et la mémoire (en Ko).
func (c *Client) GetVMDetails(ctx context.Context, name string) (VM, error) {
	if name == "" {
		return VM{}, fmt.Errorf("kvm: vm name is required")
	}

	out, err := c.executor.Run(ctx, "virsh", "dominfo", name)
	if err != nil {
		return VM{}, fmt.Errorf("kvm: running virsh dominfo: %w", err)
	}

	vm := VM{Name: name}
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := splitDominfoLine(line)
		if !ok {
			continue
		}
		switch key {
		case "Id":
			vm.ID = value
		case "State":
			vm.State = value
		case "CPU(s)":
			if n, err := strconv.Atoi(value); err == nil {
				vm.VCPUs = n
			}
		case "Used memory":
			// Exemple de valeur : "4194304 KiB"
			fields := strings.Fields(value)
			if len(fields) > 0 {
				if n, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
					vm.MemoryKB = n
				}
			}
		}
	}

	return vm, nil
}

// splitDominfoLine sépare une ligne "Clé:   Valeur" issue de virsh dominfo.
func splitDominfoLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}
