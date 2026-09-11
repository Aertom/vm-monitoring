// Command server démarre l'API HTTP de VM Monitor. Dans ce commit initial,
// les VMs proviennent uniquement de l'inventaire statique déclaré dans
// config.yaml (staticVMs) ; la découverte automatique par hyperviseur sera
// ajoutée dans des commits ultérieurs.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/api"
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
	loadStaticVMs(st, cfg)

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

// loadStaticVMs convertit l'inventaire statique de la config en VMs du store.
// Aucune collecte SSH n'est effectuée ici : Status reste "unknown" et Family
// est déduite du hostname, comme pour les VMs découvertes dynamiquement.
func loadStaticVMs(st *store.Store, cfg *config.Config) {
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
	st.ReplaceVMs(vms)
}
