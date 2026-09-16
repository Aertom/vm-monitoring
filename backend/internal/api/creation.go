// Package api — suite : création de VMs (onglet Create).
package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/provision"
)

// creationDeps agrège les dépendances optionnelles (nil = fonctionnalité off).
func (s *Server) creationReady() (*config.HypervisorsConfig, *config.CreationConfig) {
	if s.HVs == nil || s.Creation == nil {
		return nil, nil
	}
	return s.HVs, s.Creation
}

type hypervisorRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// handleListHypervisors liste les hyperviseurs configurés (noms + types,
// sans secrets).
func (s *Server) handleListHypervisors(w http.ResponseWriter, r *http.Request) {
	out := []hypervisorRef{}
	if s.HVs != nil {
		for _, h := range s.HVs.ESXi {
			out = append(out, hypervisorRef{Name: h.Name, Type: "esxi"})
		}
		for _, h := range s.HVs.Nutanix {
			out = append(out, hypervisorRef{Name: h.Name, Type: "nutanix"})
		}
		for _, h := range s.HVs.KVM {
			out = append(out, hypervisorRef{Name: h.Name, Type: "kvm"})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type detectFamilyRequest struct {
	Hostname string `json:"hostname"`
}

// handleDetectFamily déduit la famille d'un hostname (pré-remplissage).
func (s *Server) handleDetectFamily(w http.ResponseWriter, r *http.Request) {
	var req detectFamilyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Hostname) == "" {
		writeError(w, http.StatusBadRequest, "champ 'hostname' requis")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"family": string(s.families().Detect(req.Hostname))})
}

type creationOptions struct {
	Kind      string                         `json:"kind"`
	Datastore string                         `json:"datastore"`
	Network   string                         `json:"network"`
	IsoDir    string                         `json:"isoDir"`
	Subnet    string                         `json:"subnet"`
	Container string                         `json:"container,omitempty"`
	ISOs      []config.ISOEntry              `json:"isos"`
	Types     map[string]config.VMTypePreset `json:"types"`
}

// handleCreationOptions pré-remplit le formulaire pour un hyperviseur
// (infra sans secrets + ISOs + presets de ressources).
func (s *Server) handleCreationOptions(w http.ResponseWriter, r *http.Request) {
	hvs, cc := s.creationReady()
	if hvs == nil {
		writeError(w, http.StatusNotFound, "création non configurée (vmCreation.yaml)")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("hypervisor"))
	out := creationOptions{ISOs: cc.ISOs, Types: cc.Types}
	if out.ISOs == nil {
		out.ISOs = []config.ISOEntry{}
	}
	if out.Types == nil {
		out.Types = map[string]config.VMTypePreset{}
	}
	for _, h := range hvs.ESXi {
		if h.Name == name {
			out.Kind, out.Datastore, out.Network, out.IsoDir, out.Subnet =
				"esxi", h.Datastore, h.Network, h.IsoDir, h.Subnet
			writeJSON(w, http.StatusOK, out)
			return
		}
	}
	for _, h := range hvs.Nutanix {
		if h.Name == name {
			out.Kind, out.Datastore, out.Network, out.Subnet, out.Container =
				"nutanix", h.Container, h.Network, h.Subnet, h.Container
			writeJSON(w, http.StatusOK, out)
			return
		}
	}
	for _, h := range hvs.KVM {
		if h.Name == name {
			out.Kind, out.Datastore, out.Network, out.IsoDir, out.Subnet =
				"kvm", h.PoolDir, h.Network, h.IsoDir, h.Subnet
			writeJSON(w, http.StatusOK, out)
			return
		}
	}
	writeError(w, http.StatusNotFound, "hyperviseur inconnu")
}

// usedIPs agrège les IPs connues (statique + découvertes) pour suggérer/valider.
func (s *Server) usedIPs() []string {
	var out []string
	seen := map[string]bool{}
	for _, vm := range s.Store.ListVMs() {
		if ip := strings.TrimSpace(vm.IP); ip != "" && !seen[ip] {
			seen[ip] = true
			out = append(out, ip)
		}
	}
	return out
}

// handleSuggestIP propose la première IP libre du sous-réseau.
func (s *Server) handleSuggestIP(w http.ResponseWriter, r *http.Request) {
	hvs, _ := s.creationReady()
	if hvs == nil {
		writeError(w, http.StatusNotFound, "création non configurée (vmCreation.yaml)")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("hypervisor"))
	subnet := ""
	for _, h := range hvs.ESXi {
		if h.Name == name {
			subnet = h.Subnet
		}
	}
	for _, h := range hvs.Nutanix {
		if h.Name == name {
			subnet = h.Subnet
		}
	}
	for _, h := range hvs.KVM {
		if h.Name == name {
			subnet = h.Subnet
		}
	}
	if subnet == "" {
		writeError(w, http.StatusNotFound, "aucun sous-réseau pour cet hyperviseur")
		return
	}
	ip, err := provision.SuggestIP(subnet, s.usedIPs())
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ip": ip})
}

// handleCheckIP vérifie une IP (plage + occupation).
func (s *Server) handleCheckIP(w http.ResponseWriter, r *http.Request) {
	hvs, _ := s.creationReady()
	if hvs == nil {
		writeError(w, http.StatusNotFound, "création non configurée (vmCreation.yaml)")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("hypervisor"))
	ip := strings.TrimSpace(r.URL.Query().Get("ip"))
	subnet := ""
	for _, h := range hvs.ESXi {
		if h.Name == name {
			subnet = h.Subnet
		}
	}
	for _, h := range hvs.Nutanix {
		if h.Name == name {
			subnet = h.Subnet
		}
	}
	for _, h := range hvs.KVM {
		if h.Name == name {
			subnet = h.Subnet
		}
	}
	used := false
	for _, u := range s.usedIPs() {
		if u == ip {
			used = true
		}
	}
	inRange := true
	if subnet != "" {
		if _, ipnet, err := net.ParseCIDR(strings.TrimSpace(subnet)); err != nil {
			inRange = false
		} else if parsed := net.ParseIP(ip); parsed == nil || !ipnet.Contains(parsed) {
			inRange = false
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ip": ip, "used": used, "inRange": inRange})
}

// handleCreateVM valide, construit le plan et l'exécute (ou dry-run).
func (s *Server) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	hvs, cc := s.creationReady()
	if hvs == nil {
		writeError(w, http.StatusNotFound, "création non configurée (vmCreation.yaml)")
		return
	}
	var req provision.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps JSON invalide")
		return
	}
	if err := provision.Validate(req, hvs, cc, s.families(), s.usedIPs()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan, err := provision.BuildPlan(req, hvs, cc, s.SSHCfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if r.URL.Query().Get("dryRun") == "true" {
		writeJSON(w, http.StatusOK, provision.Result{
			DryRun: true, Hypervisor: req.Hypervisor, Name: req.Name, IP: req.IP,
			Commands: plan.Commands, Log: plan.Summary,
		})
		return
	}
	ex, _, err := provision.RunnerFor(hvs, req.Hypervisor, s.SSHCfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	log, err := provision.Execute(ctx, plan, ex)
	if err != nil {
		writeError(w, http.StatusBadGateway, "création échouée: "+err.Error()+"\n"+log)
		return
	}
	writeJSON(w, http.StatusCreated, provision.Result{
		Hypervisor: req.Hypervisor, Name: req.Name, IP: req.IP,
		PoweredOn: true, Log: plan.Summary + "\n" + log,
	})
}
