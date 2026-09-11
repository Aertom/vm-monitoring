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

	st := store.New()
	st.ReplaceVMs(buildStaticVMs(cfg))

	var lastReport atomic.Value
	lastReport.Store(inventory.Report{At: time.Now().UTC(), Sources: map[string]int{}})

	go collectLoop(st, cfg, hcfg, &lastReport)

	srv := &api.Server{Store: st, Discovery: func() inventory.Report {
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

// buildStaticVMs convertit l'inventaire statique de la config en VMs du store.
// État initial avant le premier cycle : Status "unknown", Family déduite.
func buildStaticVMs(cfg *config.Config) []model.VM {
	return inventory.Merge(cfg.StaticVMs, nil)
}

// collectLoop exécute un cycle complet (découverte + fusion + collecte SSH)
// immédiatement puis à chaque pollIntervalSeconds. ReplaceVMs préserve
// checkout et renommages d'un cycle à l'autre.
func collectLoop(st *store.Store, cfg *config.Config, hcfg *config.HypervisorsConfig, lastReport *atomic.Value) {
	runCycle(st, cfg, hcfg, lastReport)
	ticker := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		runCycle(st, cfg, hcfg, lastReport)
	}
}

func runCycle(st *store.Store, cfg *config.Config, hcfg *config.HypervisorsConfig, lastReport *atomic.Value) {
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
	vms := inventory.Merge(cfg.StaticVMs, discovered)
	if cfg.SSH.PrivateKeyPath == "" {
		log.Print("collecte SSH désactivée (ssh.privateKeyPath vide)")
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
			data, err := collector.CollectFull(ctx, vm.IP, cfg.SSH, collector.DirsForFamily(cfg, string(vm.Family)))
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
