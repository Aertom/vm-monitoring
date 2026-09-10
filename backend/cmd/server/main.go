// Command server démarre l'API HTTP de VM Monitor. Dans ce commit initial,
// les VMs proviennent uniquement de l'inventaire statique déclaré dans
// config.yaml (staticVMs) ; la découverte automatique par hyperviseur sera
// ajoutée dans des commits ultérieurs.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/api"
	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
	"github.com/Aertom/vm-monitoring/backend/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yaml", "chemin du fichier de configuration")
	flag.Parse()

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
