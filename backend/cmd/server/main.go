// Command server démarre l'API HTTP de VM Monitor. Chaque cycle
// (pollIntervalSeconds) : découverte automatique sur les hyperviseurs
// configurés (hypervisors.yaml), fusion avec l'inventaire statique
// (staticVMs, référence d'identité), puis collecte SSH par VM (versions
// d'applications dans appDirs + /etc/hosts pour les groupes).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/api"
	"github.com/Aertom/vm-monitoring/backend/internal/collector"
	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/inventory"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
	"github.com/Aertom/vm-monitoring/backend/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yaml", "chemin du fichier de configuration")
	hypervisorsPath := flag.String("hypervisors", "hypervisors.yaml", "chemin des hyperviseurs (absent = découverte désactivée)")
	healthcheck := flag.Bool("healthcheck", false, "vérifie /healthz local et quitte (pour HEALTHCHECK Docker)")
	healthURL := flag.String("health-url", "http://127.0.0.1:8080/healthz", "URL sondée par --healthcheck")
	flag.Parse()

	if *healthcheck {
		os.Exit(runHealthcheck(*healthURL))
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("chargement config: %v", err)
	}
	hcfg, err := config.LoadHypervisors(*hypervisorsPath)
	if err != nil {
		log.Fatalf("chargement hyperviseurs: %v", err)
	}
	log.Printf("hyperviseurs: %d esxi, %d nutanix, %d kvm",
		len(hcfg.ESXi), len(hcfg.Nutanix), len(hcfg.KVM))

	famSet, err := model.NewFamilySet(cfg.Families)
	if err != nil {
		log.Fatalf("familles invalides: %v", err)
	}
	log.Printf("familles: %v", famSet.Names())

	if cfg.SSH.PrivateKeyPath != "" {
		if err := checkSSHKey(cfg.SSH.PrivateKeyPath); err != nil {
			log.Fatalf("clé SSH: %v", err)
		}
	}

	st := store.NewWithFamilies(famSet)
	st.ReplaceVMs(buildStaticVMs(cfg, famSet))

	var lastReport atomic.Value
	lastReport.Store(inventory.Report{At: time.Now().UTC(), Sources: map[string]int{}})

	go collectLoop(st, cfg, hcfg, famSet, &lastReport)

	srv := &api.Server{Store: st, Families: famSet, Discovery: func() inventory.Report {
		rep, _ := lastReport.Load().(inventory.Report)
		return rep
	}}
	handler := api.NewRouter(srv)

	log.Printf("VM Monitor backend démarré sur %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatalf("serveur HTTP: %v", err)
	}
}

// runHealthcheck sonde l'endpoint /healthz et retourne le code de sortie
// adapté à un HEALTHCHECK Docker (0 = sain).
func runHealthcheck(url string) int {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: statut %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

// checkSSHKey échoue vite si la clé privée est illisible. En conteneur, le
// chemin hôte n'existe pas : montez la clé (ex. ./id_ed25519:/ssh/key:ro)
// et pointez privateKeyPath dessus (ex. /ssh/key).
func checkSSHKey(path string) error {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("illisible %q (en conteneur : montez ./id_ed25519:/ssh/key:ro et mettez privateKeyPath: /ssh/key): %w", path, err)
	}
	return f.Close()
}

// buildStaticVMs convertit l'inventaire statique de la config en VMs du store.
// État initial avant le premier cycle : Status "unknown", Family déduite.
func buildStaticVMs(cfg *config.Config, famSet *model.FamilySet) []model.VM {
	return inventory.MergeWithSet(cfg.StaticVMs, nil, famSet)
}

// collectLoop exécute un cycle complet (découverte + fusion + collecte SSH)
// immédiatement puis à chaque pollIntervalSeconds. ReplaceVMs préserve
// checkout et renommages d'un cycle à l'autre.
func collectLoop(st *store.Store, cfg *config.Config, hcfg *config.HypervisorsConfig, famSet *model.FamilySet, lastReport *atomic.Value) {
	runCycle(st, cfg, hcfg, famSet, lastReport)
	ticker := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		runCycle(st, cfg, hcfg, famSet, lastReport)
	}
}

func runCycle(st *store.Store, cfg *config.Config, hcfg *config.HypervisorsConfig, famSet *model.FamilySet, lastReport *atomic.Value) {
	timeout := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	discovered, rep := inventory.DiscoverAll(ctx, hcfg, cfg.SSH)
	lastReport.Store(rep)
	for _, e := range rep.Errors {
		log.Printf("découverte: %s", e)
	}
	vms := inventory.MergeWithSet(cfg.StaticVMs, discovered, famSet)
	if !collector.SSHConfigured(cfg) {
		log.Print("collecte SSH désactivée (ni clé ni mot de passe configurés)")
		st.ReplaceVMs(vms)
		return
	}
	collectAll(st, cfg, vms)
	log.Printf("cycle: %d VMs (%d découvertes), %d groupes",
		len(vms), len(discovered), len(st.ListGroups()))
}

func collectAll(st *store.Store, cfg *config.Config, vms []model.VM) {
	// Instantané précédent : en cas d'échec SSH on conserve apps/hosts connus.
	prev := make(map[string]model.VM, len(vms))
	for _, vm := range st.ListVMs() {
		prev[vm.ID] = vm
	}
	// Credentials par VM (surcharges staticVMs), sinon globaux.
	staticByID := make(map[string]config.StaticVM, len(cfg.StaticVMs))
	for _, sv := range cfg.StaticVMs {
		staticByID[sv.ID] = sv
	}
	timeout := time.Duration(cfg.SSH.TimeoutSeconds) * time.Second
	var wg sync.WaitGroup
	for i := range vms {
		wg.Add(1)
		go func(vm *model.VM) {
			defer wg.Done()
			if vm.IP == "" {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			vcfg := cfg.SSH
			if sv, ok := staticByID[vm.ID]; ok {
				vcfg.User, vcfg.Password = collector.ResolveAuth(sv.SSHUser, sv.SSHPassword, cfg.SSH)
			}
			data, err := collector.CollectFull(ctx, vm.IP, vcfg, collector.DirsForFamily(cfg, string(vm.Family)))
			vm.LastSeen = time.Now().UTC()
			if err != nil {
				vm.Status = model.StatusError
				vm.LastError = err.Error()
				if old, ok := prev[vm.ID]; ok {
					vm.Apps = old.Apps
					vm.EtcHosts = old.EtcHosts
				}
				return
			}
			vm.Status = model.StatusOK
			vm.LastError = ""
			vm.Apps = data.Apps
			vm.EtcHosts = data.EtcHosts
		}(&vms[i])
	}
	wg.Wait()
	st.ReplaceVMs(vms)
}
