// Package api expose l'état des VMs et des groupes via une API REST HTTP,
// en s'appuyant exclusivement sur le Store comme source de données.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Aertom/vm-monitoring/backend/internal/store"
)

// Server regroupe les dépendances de l'API HTTP.
type Server struct {
	Store *store.Store
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
	writeJSON(w, http.StatusOK, s.Store.ListGroups())
}

func (s *Server) handleListFamilies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []string{"sm", "cm", "ws", "oa"})
}

type checkoutRequest struct {
	User string `json:"user"`
}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.User == "" {
		writeError(w, http.StatusBadRequest, "champ 'user' requis")
		return
	}
	if err := s.Store.Checkout(groupID, req.User); err != nil {
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
	writeJSON(w, http.StatusOK, g)
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
	writeJSON(w, http.StatusOK, g)
}
