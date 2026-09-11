// Command server démarre l'API HTTP de VM Monitor. Les VMs proviennent de
// l'inventaire statique déclaré dans config.yaml (staticVMs) ; les versions
// d'applications sont collectées en SSH dans les dossiers configurés par
// famille (appDirs, défaut /opt) à chaque pollIntervalSeconds.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/api"
	"github.com/Aertom/vm-monitoring/backend/internal/collector"
	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
	"github.com/Aertom/vm-monitoring/backend/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yaml", "chemin du fichier de configuration")
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

	st := store.New()
	vms := buildStaticVMs(cfg)
	st.ReplaceVMs(vms)

	go collectLoop(st, cfg, vms)

	srv := &api.Server{Store: st}
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
// Aucune collecte SSH n'est effectuée ici : Status reste "unknown" et Family
// est déduite du hostname, comme pour les VMs découvertes dynamiquement.
func buildStaticVMs(cfg *config.Config) []model.VM {
	vms := make([]model.VM, 0, len(cfg.StaticVMs))
	now := time.Now().UTC()
	for _, sv := range cfg.StaticVMs {
		vms = append(vms, model.VM{
			ID:         sv.ID,
			Hostname:   sv.Hostname,
			IP:         sv.IP,
			Family:     model.DetectFamily(sv.Hostname),
			Hypervisor: model.HypervisorStatic,
			Status:     model.StatusUnknown,
			LastSeen:   now,
		})
	}
	return vms
}

// collectLoop rafraîchit les versions d'applications via SSH à chaque
// pollIntervalSeconds. Sans clé SSH configurée, la collecte est désactivée.
// ReplaceVMs préserve checkout et renommages d'un cycle à l'autre.
func collectLoop(st *store.Store, cfg *config.Config, vms []model.VM) {
	if cfg.SSH.PrivateKeyPath == "" {
		log.Print("collecte versions désactivée (ssh.privateKeyPath vide)")
		return
	}
	dirsByID := make(map[string][]string, len(cfg.StaticVMs))
	for _, sv := range cfg.StaticVMs {
		dirsByID[sv.ID] = collector.DirsForFamily(cfg, string(model.DetectFamily(sv.Hostname)))
	}
	collectAll(st, cfg, vms, dirsByID)
	ticker := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		collectAll(st, cfg, vms, dirsByID)
	}
}

func collectAll(st *store.Store, cfg *config.Config, vms []model.VM, dirsByID map[string][]string) {
	timeout := time.Duration(cfg.SSH.TimeoutSeconds) * time.Second
	var wg sync.WaitGroup
	for i := range vms {
		wg.Add(1)
		go func(vm *model.VM) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			apps, err := collector.Collect(ctx, vm.IP, cfg.SSH, dirsByID[vm.ID])
			vm.LastSeen = time.Now().UTC()
			if err != nil {
				vm.Status = model.StatusError
				vm.LastError = err.Error()
				return
			}
			vm.Status = model.StatusOK
			vm.LastError = ""
			vm.Apps = apps
		}(&vms[i])
	}
	wg.Wait()
	st.ReplaceVMs(vms)
}
