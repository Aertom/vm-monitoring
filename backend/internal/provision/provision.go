// Package provision crée des VMs sur les hyperviseurs depuis l'IHM :
// ESXi standalone via SSH (vmkfstools + .vmx + vim-cmd, 100% on-hyperviseur,
// équivalent sans dépendance à un flux ovftool), KVM via virt-install en SSH,
// Nutanix via acli en SSH sur la CVM. Les builders sont purs et testés.
package provision

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Aertom/vm-monitoring/backend/internal/collector"
	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

// Request décrit une demande de création (POST /api/creation).
type Request struct {
	Hypervisor string `json:"hypervisor"`
	Family     string `json:"family"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	ISOFile    string `json:"isoFile"`
	Datastore  string `json:"datastore"`
	Network    string `json:"network"`
	CPU        int    `json:"cpu"`
	RAMGB      int    `json:"ramGB"`
	DiskGB     int    `json:"diskGB"`
	IP         string `json:"ip"`
}

// Plan est un scénario d'exécution prêt : machine cible + commandes shell.
type Plan struct {
	Hypervisor string   `json:"hypervisor"`
	Kind       string   `json:"kind"`
	Host       string   `json:"host"`
	Commands   []string `json:"commands"`
	Summary    string   `json:"summary"`
}

// Result est le retour d'une création exécutée (ou simulée en dry-run).
type Result struct {
	DryRun     bool     `json:"dryRun"`
	Hypervisor string   `json:"hypervisor"`
	Name       string   `json:"name"`
	IP         string   `json:"ip"`
	PoweredOn  bool     `json:"poweredOn"`
	Commands   []string `json:"commands,omitempty"`
	Log        string   `json:"log,omitempty"`
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// Validate contrôle une demande (champs, bornes, IP dans le sous-réseau et libre).
func Validate(req Request, hvs *config.HypervisorsConfig, cc *config.CreationConfig, set *model.FamilySet, usedIPs []string) error {
	if hvs == nil {
		return fmt.Errorf("aucun hyperviseur configuré")
	}
	kind, _, _, _ := findHypervisor(hvs, req.Hypervisor)
	if kind == "" {
		return fmt.Errorf("hyperviseur %q inconnu", req.Hypervisor)
	}
	if !nameRe.MatchString(req.Name) || len(req.Name) > 64 {
		return fmt.Errorf("nom invalide (lettres, chiffres, _.-, 64 max)")
	}
	if set != nil {
		known := false
		for _, f := range set.Names() {
			if string(f) == strings.ToLower(strings.TrimSpace(req.Family)) {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("famille %q inconnue", req.Family)
		}
	}
	if cc == nil {
		return fmt.Errorf("création non configurée (vmCreation.yaml)")
	}
	if _, ok := cc.Types[req.Type]; !ok {
		return fmt.Errorf("type %q inconnu", req.Type)
	}
	foundISO := false
	for _, iso := range cc.ISOs {
		if iso.File == req.ISOFile {
			foundISO = true
		}
	}
	if !foundISO {
		return fmt.Errorf("iso %q inconnue", req.ISOFile)
	}
	if req.CPU < 1 || req.CPU > 64 {
		return fmt.Errorf("cpu hors bornes (1-64)")
	}
	if req.RAMGB < 1 || req.RAMGB > 1024 {
		return fmt.Errorf("ram hors bornes (1-1024 Go)")
	}
	if req.DiskGB < 10 || req.DiskGB > 4000 {
		return fmt.Errorf("disque hors bornes (10-4000 Go)")
	}
	if req.Datastore == "" || req.Network == "" {
		return fmt.Errorf("datastore et network requis")
	}
	ip := net.ParseIP(strings.TrimSpace(req.IP))
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("ip %q invalide (IPv4 attendue)", req.IP)
	}
	subnet := hypervisorSubnet(hvs, req.Hypervisor)
	if subnet != "" {
		_, ipnet, err := net.ParseCIDR(strings.TrimSpace(subnet))
		if err != nil {
			return fmt.Errorf("subnet %q invalide: %w", subnet, err)
		}
		if !ipnet.Contains(ip) {
			return fmt.Errorf("ip %q hors du sous-réseau %s", req.IP, subnet)
		}
	}
	for _, u := range usedIPs {
		if strings.TrimSpace(u) == strings.TrimSpace(req.IP) {
			return fmt.Errorf("ip %q déjà utilisée", req.IP)
		}
	}
	return nil
}

// hypervisorKind localise une entrée par nom : kind + index.
func findHypervisor(hvs *config.HypervisorsConfig, name string) (kind string, esxi *config.ESXiConfig, nutanix *config.NutanixConfig, kvm *config.KVMConfig) {
	if hvs == nil {
		return "", nil, nil, nil
	}
	for i := range hvs.ESXi {
		if hvs.ESXi[i].Name == name {
			return "esxi", &hvs.ESXi[i], nil, nil
		}
	}
	for i := range hvs.Nutanix {
		if hvs.Nutanix[i].Name == name {
			return "nutanix", nil, &hvs.Nutanix[i], nil
		}
	}
	for i := range hvs.KVM {
		if hvs.KVM[i].Name == name {
			return "kvm", nil, nil, &hvs.KVM[i]
		}
	}
	return "", nil, nil, nil
}

// hypervisorSubnet retourne le CIDR configuré pour un hyperviseur ("" sinon).
func hypervisorSubnet(hvs *config.HypervisorsConfig, name string) string {
	kind, esxi, nutanix, kvm := findHypervisor(hvs, name)
	switch kind {
	case "esxi":
		return esxi.Subnet
	case "nutanix":
		return nutanix.Subnet
	case "kvm":
		return kvm.Subnet
	}
	return ""
}

// SuggestIP propose la première IPv4 libre du CIDR (hors réseau, passerelle
// .1 et broadcast), en excluant les IPs déjà utilisées (inventaire).
func SuggestIP(subnet string, used []string) (string, error) {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(subnet))
	if err != nil {
		return "", fmt.Errorf("subnet %q invalide: %w", subnet, err)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return "", fmt.Errorf("seul IPv4 est supporté")
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 || ones > 30 {
		return "", fmt.Errorf("subnet %q trop petit", subnet)
	}
	usedSet := map[string]bool{}
	for _, u := range used {
		if p := net.ParseIP(strings.TrimSpace(u)); p != nil {
			usedSet[p.String()] = true
		}
	}
	base := ip4.Mask(ipnet.Mask)
	n := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	hosts := uint32(1) << (32 - ones)
	for i := uint32(2); i < hosts-1; i++ {
		addr := n + i
		cand := net.IPv4(byte(addr>>24), byte(addr>>16), byte(addr>>8), byte(addr)).String()
		if !usedSet[cand] {
			return cand, nil
		}
	}
	return "", fmt.Errorf("aucune IP libre dans %s", subnet)
}

// shQuote protège un argument shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// isoByFile retrouve une entrée ISO par nom de fichier.
func isoByFile(cc *config.CreationConfig, file string) *config.ISOEntry {
	for i := range cc.ISOs {
		if cc.ISOs[i].File == file {
			return &cc.ISOs[i]
		}
	}
	return nil
}

// BuildPlan construit le scénario d'exécution (pur, sans effet de bord).
func BuildPlan(req Request, hvs *config.HypervisorsConfig, cc *config.CreationConfig, sshCfg config.SSHConfig) (Plan, error) {
	kind, esxi, nutanix, kvm := findHypervisor(hvs, req.Hypervisor)
	if kind == "" {
		return Plan{}, fmt.Errorf("hyperviseur %q inconnu", req.Hypervisor)
	}
	iso := isoByFile(cc, req.ISOFile)
	if iso == nil {
		return Plan{}, fmt.Errorf("iso %q inconnue", req.ISOFile)
	}
	switch kind {
	case "esxi":
		return buildESXiPlan(req, esxi, iso, cc, sshCfg)
	case "kvm":
		return buildKVMPlan(req, kvm, iso, cc, sshCfg)
	case "nutanix":
		return buildNutanixPlan(req, nutanix, iso, cc, sshCfg)
	}
	return Plan{}, fmt.Errorf("type %q non supporté", kind)
}

// buildESXiPlan crée la VM 100% sur l'hôte : vmdk + .vmx écrit, registre,
// power on. Équivalent sans dépendance à un flux ovftool (non installable
// sur ESXi).
func buildESXiPlan(req Request, hv *config.ESXiConfig, iso *config.ISOEntry, cc *config.CreationConfig, sshCfg config.SSHConfig) (Plan, error) {
	host := splitHost(hv.URL)
	if host == "" {
		return Plan{}, fmt.Errorf("esxi %q: url vide", hv.Name)
	}
	user := hv.Username
	if user == "" {
		user = "root"
	}
	dsPath := fmt.Sprintf("/vmfs/volumes/%s/%s", hv.Datastore, req.Name)
	isoPath := fmt.Sprintf("/vmfs/volumes/%s/%s/%s", hv.Datastore, strings.Trim(hv.IsoDir, "/"), iso.File)
	guestOS := iso.GuestOS
	if guestOS == "" {
		guestOS = "rhel9_64Guest"
	}
	vmx := esxiVMX(req.Name, guestOS, cc.ESXiHWVersion, cc.ESXiFirmware,
		req.CPU, req.RAMGB*1024, req.Network, dsPath+"/"+req.Name+".vmdk", isoPath)
	script := fmt.Sprintf(`set -e
mkdir -p %s
vmkfstools -c %dG %s
cat > %s <<'VMXEOF'
%sVMXEOF
VID=$(vim-cmd solo/registervm %s)
vim-cmd vmsvc/power.on "$VID"`,
		shQuote(dsPath), req.DiskGB, shQuote(dsPath+"/"+req.Name+".vmdk"),
		shQuote(dsPath+"/"+req.Name+".vmx"), vmx, shQuote(dsPath+"/"+req.Name+".vmx"))
	return Plan{
		Hypervisor: hv.Name, Kind: "esxi", Host: host,
		Commands: []string{script},
		Summary:  fmt.Sprintf("ESXi %s : VM %s (%d CPU, %d Go RAM, %d Go sur %s), ISO %s, power on", host, req.Name, req.CPU, req.RAMGB, req.DiskGB, hv.Datastore, iso.File),
	}, nil
}

// esxiVMX génère le descripteur .vmx (RHEL : pvscsi + vmxnet3 + cdrom ISO).
func esxiVMX(name, guestOS, hwVersion, firmware string, cpu, ramMB int, network, vmdk, iso string) string {
	if hwVersion == "" {
		hwVersion = "20"
	}
	if firmware == "" {
		firmware = "efi"
	}
	lines := []string{
		`config.version = "8"`,
		fmt.Sprintf(`virtualHW.version = %q`, hwVersion),
		fmt.Sprintf(`displayName = %q`, name),
		fmt.Sprintf(`guestOS = %q`, guestOS),
		fmt.Sprintf(`memSize = %q`, fmt.Sprint(ramMB)),
		fmt.Sprintf(`numvcpus = %q`, fmt.Sprint(cpu)),
		`cpuid.coresPerSocket = "1"`,
		fmt.Sprintf(`firmware = %q`, firmware),
		`scsi0.present = "TRUE"`,
		`scsi0.virtualDev = "pvscsi"`,
		`scsi0:0.present = "TRUE"`,
		`scsi0:0.deviceType = "scsi-hardDisk"`,
		fmt.Sprintf(`scsi0:0.fileName = %q`, vmdk),
		`ide1:0.present = "TRUE"`,
		`ide1:0.deviceType = "cdrom-image"`,
		fmt.Sprintf(`ide1:0.fileName = %q`, iso),
		`ide1:0.startConnected = "TRUE"`,
		`ethernet0.present = "TRUE"`,
		fmt.Sprintf(`ethernet0.networkName = %q`, network),
		`ethernet0.virtualDev = "vmxnet3"`,
		`ethernet0.addressType = "generated"`,
		`tools.syncTimeWithHost = "TRUE"`,
	}
	return strings.Join(lines, "\n") + "\n"
}

// buildKVMPlan construit l'appel virt-install (une commande, non bloquante).
func buildKVMPlan(req Request, hv *config.KVMConfig, iso *config.ISOEntry, cc *config.CreationConfig, sshCfg config.SSHConfig) (Plan, error) {
	if hv.Host == "" {
		return Plan{}, fmt.Errorf("kvm %q: host vide", hv.Name)
	}
	user := hv.User
	if user == "" {
		user = sshCfg.User
	}
	network := strings.TrimSpace(req.Network)
	if network == "" {
		network = hv.Network
	}
	if !strings.Contains(network, "=") {
		network = "bridge=" + network
	}
	isoPath := strings.TrimSuffix(hv.IsoDir, "/") + "/" + iso.File
	diskPath := strings.TrimSuffix(hv.PoolDir, "/") + "/" + req.Name + ".qcow2"
	variant := iso.OSVariant
	if variant == "" {
		variant = "rhel9.0"
	}
	graphics := cc.KVMGraphics
	if graphics == "" {
		graphics = "vnc"
	}
	cmd := fmt.Sprintf(
		"virt-install --name %s --vcpus %d --memory %d --disk path=%s,size=%d,format=qcow2 --cdrom %s --os-variant %s --network %s --graphics %s --noautoconsole",
		shQuote(req.Name), req.CPU, req.RAMGB*1024, shQuote(diskPath), req.DiskGB,
		shQuote(isoPath), shQuote(variant), network, shQuote(graphics))
	return Plan{
		Hypervisor: hv.Name, Kind: "kvm", Host: hv.Host,
		Commands: []string{cmd},
		Summary:  fmt.Sprintf("KVM %s : VM %s (%d CPU, %d Go RAM, %d Go), ISO %s, power on", hv.Host, req.Name, req.CPU, req.RAMGB, req.DiskGB, iso.File),
	}, nil
}

// buildNutanixPlan construit les appels acli sur la CVM (création, disques,
// ISO, réseau, power on).
func buildNutanixPlan(req Request, hv *config.NutanixConfig, iso *config.ISOEntry, cc *config.CreationConfig, sshCfg config.SSHConfig) (Plan, error) {
	image := iso.Image
	if image == "" {
		return Plan{}, fmt.Errorf("iso %q sans nom d'image Nutanix (champ image)", iso.File)
	}
	network := strings.TrimSpace(req.Network)
	if network == "" {
		network = hv.Network
	}
	cmds := []string{
		fmt.Sprintf("acli vm.create %s num_vcpus=%d num_cores_per_vcpu=1 memory=%dG",
			shQuote(req.Name), req.CPU, req.RAMGB),
		fmt.Sprintf("acli vm.disk_create %s container=%s create_size=%dG bus=scsi",
			shQuote(req.Name), shQuote(hv.Container), req.DiskGB),
		fmt.Sprintf("acli vm.disk_create %s cdrom=true clone_from_image=%s",
			shQuote(req.Name), shQuote(image)),
	}
	nic := fmt.Sprintf("acli vm.nic_create %s network=%s model=virtio", shQuote(req.Name), shQuote(network))
	if strings.TrimSpace(req.IP) != "" {
		nic += " ip=" + shQuote(strings.TrimSpace(req.IP))
	}
	cmds = append(cmds, nic, fmt.Sprintf("acli vm.on %s", shQuote(req.Name)))
	host := hv.SSHHost
	if host == "" {
		host = splitHost(hv.URL)
	}
	return Plan{
		Hypervisor: hv.Name, Kind: "nutanix", Host: host,
		Commands: cmds,
		Summary:  fmt.Sprintf("Nutanix %s : VM %s (%d CPU, %d Go RAM, %d Go sur %s), image %s, power on", host, req.Name, req.CPU, req.RAMGB, req.DiskGB, hv.Container, image),
	}, nil
}

// splitHost extrait l'hôte d'une URL (ou hôte nu, schéma/port ignorés).
func splitHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	rest := raw
	if i := strings.Index(raw, "://"); i >= 0 {
		rest = raw[i+3:]
	}
	if j := strings.Index(rest, "/"); j >= 0 {
		rest = rest[:j]
	}
	if k := strings.LastIndex(rest, ":"); k >= 0 {
		if _, err := strconv.Atoi(rest[k+1:]); err == nil {
			return rest[:k]
		}
	}
	return rest
}

// RunnerFor résout l'exécuteur SSH d'un hyperviseur (user/port par type,
// clé et mot de passe globaux).
func RunnerFor(hvs *config.HypervisorsConfig, name string, sshCfg config.SSHConfig) (*collector.SSHExecutor, string, error) {
	kind, esxi, nutanix, kvm := findHypervisor(hvs, name)
	if kind == "" {
		return nil, "", fmt.Errorf("hyperviseur %q inconnu", name)
	}
	timeout := time.Duration(sshCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ex := &collector.SSHExecutor{
		KeyPath: sshCfg.PrivateKeyPath, Password: sshCfg.Password, Timeout: timeout,
	}
	switch kind {
	case "esxi":
		ex.Host = splitHost(esxi.URL)
		ex.User = esxi.Username
		if ex.User == "" {
			ex.User = "root"
		}
		ex.Port = 22
	case "kvm":
		ex.Host = kvm.Host
		ex.User = kvm.User
		if ex.User == "" {
			ex.User = sshCfg.User
		}
		ex.Port = kvm.Port
		if ex.Port <= 0 {
			ex.Port = 22
		}
	case "nutanix":
		ex.Host = nutanix.SSHHost
		if ex.Host == "" {
			ex.Host = splitHost(nutanix.URL)
		}
		ex.User = nutanix.SSHUser
		if ex.User == "" {
			ex.User = sshCfg.User
		}
		if ex.User == "" {
			ex.User = "nutanix"
		}
		ex.Port = 22
	}
	if ex.Host == "" {
		return nil, "", fmt.Errorf("hyperviseur %q sans hôte SSH", name)
	}
	return ex, kind, nil
}

// Execute déroule le plan commande par commande (arrêt à la première erreur).
// Retourne le journal concaténé (tronqué à 4 Ko).
func Execute(ctx context.Context, plan Plan, ex *collector.SSHExecutor) (string, error) {
	var logs []string
	for i, cmd := range plan.Commands {
		out, err := ex.Run(ctx, "sh", "-c", cmd)
		logs = append(logs, fmt.Sprintf("$ [%d] …\n%s", i+1, strings.TrimSpace(out)))
		if err != nil {
			return strings.Join(logs, "\n"), fmt.Errorf("commande %d: %w", i+1, err)
		}
	}
	joined := strings.Join(logs, "\n")
	if len(joined) > 4096 {
		joined = joined[len(joined)-4096:]
	}
	return joined, nil
}

// SortedKeys retourne les clés triées (types stables pour l'UI).
func SortedKeys(m map[string]config.VMTypePreset) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
