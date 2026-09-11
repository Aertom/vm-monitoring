// Package inventory branche la découverte automatique des hyperviseurs
// (internal/discovery) sur le modèle interne : il convertit les VMs brutes
// ESXi/AHV/KVM en DiscoveredVM, les découvre en parallèle, puis les fusionne
// avec l'inventaire statique (Merge). Le statique fait foi pour l'identité
// (id, hostname, ip) ; la découverte enrichit (source hyperviseur) et ajoute
// les VMs inconnues du statique.
package inventory

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/collector"
	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/discovery/ahv"
	"github.com/Aertom/vm-monitoring/backend/internal/discovery/esxi"
	"github.com/Aertom/vm-monitoring/backend/internal/discovery/kvm"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

// DiscoveredVM est une VM découverte sur un hyperviseur, normalisée.
type DiscoveredVM struct {
	// Name est le nom tel que déclaré sur l'hyperviseur.
	Name string
	// PowerState est l'état remonté (ex: poweredOn), vide si inconnu.
	PowerState string
	// Source est le type d'hyperviseur (esxi, nutanix, kvm).
	Source model.Hypervisor
	// SourceName est le nom logique de l'hyperviseur dans hypervisors.yaml.
	SourceName string
}

// Report résume le dernier cycle de découverte (exposé via /api/discovery).
type Report struct {
	At      time.Time      `json:"at"`
	Total   int            `json:"total"`
	Sources map[string]int `json:"sources"`
	Errors  []string       `json:"errors"`
}

// splitURL découpe une URL (ou hôte nu) en hôte + port.
func splitURL(raw string, defaultPort int) (string, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", defaultPort
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return strings.Trim(raw, "/"), defaultPort
	}
	port := defaultPort
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	return u.Hostname(), port
}

// displayName préfère le nom logique, sinon l'hôte (pour les logs/erreurs).
func displayName(name, host string) string {
	if name != "" {
		return name
	}
	return host
}

// insecureClient construit un client HTTP honorant le flag insecure
// (les vCenter/Prism utilisent souvent des certificats auto-signés).
func insecureClient(insecure bool, timeout time.Duration) *http.Client {
	if !insecure {
		return &http.Client{Timeout: timeout}
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // opt-in via config
		},
	}
}

// discoverESXi interroge un hôte ESXi ou vCenter (API REST /rest/vcenter/vm).
func discoverESXi(ctx context.Context, cfg config.ESXiConfig) ([]DiscoveredVM, error) {
	host, port := splitURL(cfg.URL, 443)
	if host == "" {
		return nil, fmt.Errorf("esxi %q: url vide", displayName(cfg.Name, host))
	}
	d := esxi.NewDiscoverer(esxi.Config{
		Host:               net.JoinHostPort(host, strconv.Itoa(port)),
		Username:           cfg.Username,
		Password:           cfg.Password,
		InsecureSkipVerify: cfg.Insecure,
		Timeout:            15 * time.Second,
	}, insecureClient(cfg.Insecure, 15*time.Second))
	vms, err := d.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("esxi %s: %w", displayName(cfg.Name, host), err)
	}
	out := make([]DiscoveredVM, 0, len(vms))
	for _, vm := range vms {
		out = append(out, DiscoveredVM{
			Name:       vm.Name,
			PowerState: vm.PowerState,
			Source:     model.HypervisorESXi,
			SourceName: displayName(cfg.Name, host),
		})
	}
	return out, nil
}

// discoverAHV interroge un cluster Nutanix (API Prism v2 /vms).
func discoverAHV(ctx context.Context, cfg config.NutanixConfig) ([]DiscoveredVM, error) {
	host, port := splitURL(cfg.URL, 9440)
	if host == "" {
		return nil, fmt.Errorf("ahv %q: url vide", displayName(cfg.Name, host))
	}
	c, err := ahv.NewClient(ahv.Config{
		Host:     host,
		Port:     port,
		Username: cfg.Username,
		Password: cfg.Password,
	}, insecureClient(cfg.Insecure, 15*time.Second))
	if err != nil {
		return nil, fmt.Errorf("ahv %s: %w", displayName(cfg.Name, host), err)
	}
	vms, err := c.ListVMs(ctx)
	if err != nil {
		return nil, fmt.Errorf("ahv %s: %w", displayName(cfg.Name, host), err)
	}
	out := make([]DiscoveredVM, 0, len(vms))
	for _, vm := range vms {
		out = append(out, DiscoveredVM{
			Name:       vm.Name,
			PowerState: vm.PowerState,
			Source:     model.HypervisorNutanix,
			SourceName: displayName(cfg.Name, host),
		})
	}
	return out, nil
}

// localExecutor exécute virsh en local (backend colocalisé avec libvirt).
type localExecutor struct{}

func (localExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("virsh local: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// sshExecutor exécute virsh sur un hôte KVM distant via SSH (même clé que les VMs).
type sshExecutor struct {
	host    string
	user    string
	port    int
	keyPath string
	timeout time.Duration
}

func (e *sshExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	client, err := collector.Dial(ctx, e.host, e.user, e.port, e.keyPath, e.timeout)
	if err != nil {
		return "", err
	}
	defer client.Close()
	parts := append([]string{name}, args...)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, "'"+strings.ReplaceAll(p, "'", `'\''`)+"'")
	}
	s, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer s.Close()
	out, err := s.CombinedOutput(strings.Join(quoted, " "))
	if err != nil {
		return "", fmt.Errorf("virsh distant: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// discoverKVM liste les domaines libvirt en local ou via SSH.
func discoverKVM(ctx context.Context, cfg config.KVMConfig, sshCfg config.SSHConfig) ([]DiscoveredVM, error) {
	label := displayName(cfg.Name, cfg.Host)
	var ex kvm.CommandExecutor = localExecutor{}
	if cfg.Host != "" && cfg.Host != "localhost" && cfg.Host != "127.0.0.1" {
		timeout := time.Duration(sshCfg.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		user := cfg.User
		if user == "" {
			user = sshCfg.User
		}
		ex = &sshExecutor{host: cfg.Host, user: user, port: cfg.Port, keyPath: sshCfg.PrivateKeyPath, timeout: timeout}
	}
	c, err := kvm.NewClient(ex)
	if err != nil {
		return nil, fmt.Errorf("kvm %s: %w", label, err)
	}
	return discoverKVMWithExecutor(ctx, label, c)
}

// discoverKVMWithExecutor liste les VMs via un client déjà construit
// (point d'injection pour les tests).
func discoverKVMWithExecutor(ctx context.Context, label string, c *kvm.Client) ([]DiscoveredVM, error) {
	vms, err := c.ListVMs(ctx)
	if err != nil {
		return nil, fmt.Errorf("kvm %s: %w", label, err)
	}
	out := make([]DiscoveredVM, 0, len(vms))
	for _, vm := range vms {
		out = append(out, DiscoveredVM{
			Name:       vm.Name,
			PowerState: vm.State,
			Source:     model.HypervisorKVM,
			SourceName: label,
		})
	}
	return out, nil
}

// DiscoverAll interroge tous les hyperviseurs configurés en parallèle.
// Chaque échec est isolé : les résultats partiels sont conservés et les
// erreurs agrégées dans le rapport (pas d'échec global).
func DiscoverAll(ctx context.Context, hcfg *config.HypervisorsConfig, sshCfg config.SSHConfig) ([]DiscoveredVM, Report) {
	rep := Report{At: time.Now().UTC(), Sources: map[string]int{}}
	if hcfg == nil {
		return nil, rep
	}
	var mu sync.Mutex
	var out []DiscoveredVM
	var wg sync.WaitGroup
	run := func(fn func(context.Context) ([]DiscoveredVM, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vms, err := fn(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				rep.Errors = append(rep.Errors, err.Error())
				return
			}
			for _, vm := range vms {
				out = append(out, vm)
				rep.Sources[string(vm.Source)]++
			}
		}()
	}
	for _, cfg := range hcfg.ESXi {
		cfg := cfg
		run(func(ctx context.Context) ([]DiscoveredVM, error) { return discoverESXi(ctx, cfg) })
	}
	for _, cfg := range hcfg.Nutanix {
		cfg := cfg
		run(func(ctx context.Context) ([]DiscoveredVM, error) { return discoverAHV(ctx, cfg) })
	}
	for _, cfg := range hcfg.KVM {
		cfg := cfg
		run(func(ctx context.Context) ([]DiscoveredVM, error) { return discoverKVM(ctx, cfg, sshCfg) })
	}
	wg.Wait()
	rep.Total = len(out)
	return out, rep
}

// slug normalise un nom pour construire un ID stable (disc-<source>-<slug>).
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteRune('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "vm"
	}
	return out
}

// Merge fusionne l'inventaire statique (référence d'identité : id, hostname,
// ip) avec la découverte : une VM découverte dont le nom égale (insensible à
// la casse) un hostname statique enrichit cette entrée (source hyperviseur) ;
// sinon elle est ajoutée avec un ID stable dérivé (disc-<source>-<nom>).
// Utilise les familles par défaut ; préférez MergeWithSet.
func Merge(static []config.StaticVM, discovered []DiscoveredVM) []model.VM {
	return MergeWithSet(static, discovered, model.DefaultFamilies())
}

// MergeWithSet est Merge avec un registre de familles configurable.
func MergeWithSet(static []config.StaticVM, discovered []DiscoveredVM, set *model.FamilySet) []model.VM {
	if set == nil {
		set = model.DefaultFamilies()
	}
	now := time.Now().UTC()
	byName := make(map[string]int)
	out := make([]model.VM, 0, len(static)+len(discovered))
	for _, sv := range static {
		out = append(out, model.VM{
			ID:         sv.ID,
			Hostname:   sv.Hostname,
			IP:         sv.IP,
			Family:     set.Detect(sv.Hostname),
			Hypervisor: model.HypervisorStatic,
			Status:     model.StatusUnknown,
			LastSeen:   now,
		})
		byName[strings.ToLower(sv.Hostname)] = len(out) - 1
	}
	for _, d := range discovered {
		if strings.TrimSpace(d.Name) == "" {
			continue
		}
		if i, ok := byName[strings.ToLower(d.Name)]; ok {
			out[i].Hypervisor = d.Source
			continue
		}
		id := "disc-" + string(d.Source) + "-" + slug(d.Name)
		out = append(out, model.VM{
			ID:         id,
			Hostname:   d.Name,
			Family:     set.Detect(d.Name),
			Hypervisor: d.Source,
			Status:     model.StatusUnknown,
			LastSeen:   now,
		})
		byName[strings.ToLower(d.Name)] = len(out) - 1
	}
	return out
}
