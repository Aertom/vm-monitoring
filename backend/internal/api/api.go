// Package api expose l'état des VMs et des groupes via une API REST HTTP,
// en s'appuyant exclusivement sur le Store comme source de données.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Aertom/vm-monitoring/backend/internal/inventory"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
	"github.com/Aertom/vm-monitoring/backend/internal/store"
)

// Server regroupe les dépendances de l'API HTTP.
type Server struct {
	Store *store.Store
	// Discovery retourne le dernier rapport de découverte (nil = non configuré).
	Discovery func() inventory.Report
}

// NewRouter construit le routeur HTTP complet de l'application.
func NewRouter(s *Server) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Route("/api", func(r chi.Router) {
		r.Get("/vms", s.handleListVMs)
		r.Get("/groups", s.handleListGroups)
		r.Get("/families", s.handleListFamilies)
		r.Post("/groups/{id}/checkout", s.handleCheckout)
		r.Post("/groups/{id}/checkin", s.handleCheckin)
		r.Post("/groups/{id}/rename", s.handleRename)
		r.Get("/discovery", s.handleDiscovery)
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return r
}

// corsMiddleware autorise les requêtes cross-origin depuis le frontend
// (utile en développement local et pour un frontend hébergé sur GitHub Pages).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleListVMs(w http.ResponseWriter, r *http.Request) {
	vms := s.Store.ListVMs()
	family := r.URL.Query().Get("family")
	if family != "" {
		filtered := vms[:0:0]
		for _, vm := range vms {
			if string(vm.Family) == family {
				filtered = append(filtered, vm)
			}
		}
		vms = filtered
	}
	writeJSON(w, http.StatusOK, vms)
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.enrichGroups(s.Store.ListGroups()))
}

func (s *Server) handleListFamilies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []string{"sm", "cm", "ws", "oa"})
}

type checkoutRequest struct {
	User    string `json:"user"`
	InUseBy string `json:"inUseBy"`
}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps JSON invalide")
		return
	}
	user := req.User
	if user == "" {
		user = req.InUseBy
	}
	if user == "" {
		writeError(w, http.StatusBadRequest, "champ 'user' requis")
		return
	}
	if err := s.Store.Checkout(groupID, user); err != nil {
		switch err {
		case store.ErrGroupNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		case store.ErrGroupAlreadyInUse:
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	g, _ := s.Store.GetGroup(groupID)
	writeJSON(w, http.StatusOK, s.enrichGroup(g))
}

func (s *Server) handleCheckin(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	if err := s.Store.Checkin(groupID); err != nil {
		switch err {
		case store.ErrGroupNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	g, _ := s.Store.GetGroup(groupID)
	writeJSON(w, http.StatusOK, s.enrichGroup(g))
}

type renameRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps JSON invalide")
		return
	}
	if err := s.Store.Rename(groupID, req.Name); err != nil {
		switch err {
		case store.ErrGroupNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		case store.ErrInvalidName:
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	g, _ := s.Store.GetGroup(groupID)
	writeJSON(w, http.StatusOK, s.enrichGroup(g))
}

func (s *Server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if s.Discovery == nil {
		writeError(w, http.StatusNotFound, "découverte non configurée")
		return
	}
	writeJSON(w, http.StatusOK, s.Discovery())
}

type groupResponse struct {
	model.Group
	VMs    []model.VM `json:"vms"`
	Status string     `json:"status"`
}

func (s *Server) enrichGroup(g model.Group) groupResponse {
	vms := make([]model.VM, 0, len(g.Members))
	for _, id := range g.Members {
		if vm, ok := s.Store.GetVM(id); ok {
			vms = append(vms, vm)
		}
	}
	return groupResponse{Group: g, VMs: vms, Status: groupStatus(g)}
}

func (s *Server) enrichGroups(groups []model.Group) []groupResponse {
	out := make([]groupResponse, 0, len(groups))
	for _, g := range groups {
		out = append(out, s.enrichGroup(g))
	}
	return out
}

func groupStatus(g model.Group) string {
	if g.InUseBy != "" {
		return "checkedOut"
	}
	return "available"
}
