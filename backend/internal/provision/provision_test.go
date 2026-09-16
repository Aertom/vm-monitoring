package provision

import (
	"strings"
	"testing"

	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

func testSetup() (*config.HypervisorsConfig, *config.CreationConfig) {
	hvs := &config.HypervisorsConfig{
		ESXi: []config.ESXiConfig{{
			Name: "esx-08", URL: "https://192.168.0.10", Username: "root",
			Datastore: "datastore1", Network: "VM Network", IsoDir: "iso", Subnet: "192.168.1.0/24",
		}},
		KVM: []config.KVMConfig{{
			Name: "kvm-01", Host: "192.168.0.30", User: "qemu",
			PoolDir: "/var/lib/libvirt/images", Network: "bridge=br0", IsoDir: "/isos", Subnet: "192.168.2.0/24",
		}},
		Nutanix: []config.NutanixConfig{{
			Name: "ahv-01", URL: "https://192.168.0.20:9440",
			Container: "cont-01", Network: "vlan-10", Subnet: "192.168.3.0/24",
		}},
	}
	cc := &config.CreationConfig{
		ISOs: []config.ISOEntry{
			{Name: "RHEL 9.5", File: "rhel-9.5.iso", OSVariant: "rhel9.0", GuestOS: "rhel9_64Guest", Image: "rhel95"},
		},
		Types:         map[string]config.VMTypePreset{"serveur": {CPU: 4, RAMGB: 16, DiskGB: 100}},
		ESXiHWVersion: "20", ESXiFirmware: "efi", KVMGraphics: "vnc",
	}
	return hvs, cc
}

func testReq() Request {
	return Request{
		Hypervisor: "esx-08", Family: "sm", Name: "sm-test-01", Type: "serveur",
		ISOFile: "rhel-9.5.iso", Datastore: "datastore1", Network: "VM Network",
		CPU: 4, RAMGB: 16, DiskGB: 100, IP: "192.168.1.50",
	}
}

func TestValidateOK(t *testing.T) {
	hvs, cc := testSetup()
	set := model.DefaultFamilies()
	if err := Validate(testReq(), hvs, cc, set, []string{"192.168.1.10"}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	hvs, cc := testSetup()
	set := model.DefaultFamilies()
	cases := []struct {
		name string
		mut  func(*Request)
	}{
		{"hv inconnu", func(r *Request) { r.Hypervisor = "nope" }},
		{"nom invalide", func(r *Request) { r.Name = "bad name!" }},
		{"famille inconnue", func(r *Request) { r.Family = "zz" }},
		{"type inconnu", func(r *Request) { r.Type = "mainframe" }},
		{"iso inconnue", func(r *Request) { r.ISOFile = "win.iso" }},
		{"cpu hors bornes", func(r *Request) { r.CPU = 0 }},
		{"ip invalide", func(r *Request) { r.IP = "nope" }},
		{"ip hors subnet", func(r *Request) { r.IP = "10.9.9.9" }},
		{"ip utilisée", func(r *Request) { r.IP = "192.168.1.10" }},
		{"datastore vide", func(r *Request) { r.Datastore = "" }},
	}
	used := []string{"192.168.1.10"}
	for _, c := range cases {
		req := testReq()
		c.mut(&req)
		if err := Validate(req, hvs, cc, set, used); err == nil {
			t.Errorf("%s: erreur attendue", c.name)
		}
	}
}

func TestSuggestIP(t *testing.T) {
	ip, err := SuggestIP("192.168.1.0/24", []string{"192.168.1.2", "192.168.1.1"})
	if err != nil {
		t.Fatalf("SuggestIP: %v", err)
	}
	if ip != "192.168.1.3" {
		t.Fatalf("IP=%q (réseau .0 et passerelle .1 sautés)", ip)
	}
	if _, err := SuggestIP("nope", nil); err == nil {
		t.Fatal("CIDR invalide: erreur attendue")
	}
	if _, err := SuggestIP("192.168.1.0/31", nil); err == nil {
		t.Fatal("subnet trop petit: erreur attendue")
	}
}

func TestBuildESXiPlan(t *testing.T) {
	hvs, cc := testSetup()
	plan, err := BuildPlan(testReq(), hvs, cc, config.SSHConfig{})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.Kind != "esxi" || len(plan.Commands) != 1 {
		t.Fatalf("plan inattendu: %+v", plan)
	}
	cmd := plan.Commands[0]
	for _, want := range []string{
		"vmkfstools -c 100G", "guestOS = \"rhel9_64Guest\"",
		"memSize = \"16384\"", "ide1:0.fileName = \"/vmfs/volumes/datastore1/iso/rhel-9.5.iso\"",
		"ethernet0.networkName = \"VM Network\"", "vim-cmd solo/registervm", "vim-cmd vmsvc/power.on",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("commande sans %q:\n%s", want, cmd)
		}
	}
}

func TestBuildKVMPlan(t *testing.T) {
	hvs, cc := testSetup()
	req := testReq()
	req.Hypervisor = "kvm-01"
	req.Datastore = "default"
	req.Network = ""
	req.IP = "192.168.2.50"
	plan, err := BuildPlan(req, hvs, cc, config.SSHConfig{User: "monitor"})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	cmd := plan.Commands[0]
	for _, want := range []string{
		"virt-install", "--name 'sm-test-01'", "--vcpus 4", "--memory 16384",
		"--cdrom '/isos/rhel-9.5.iso'", "--os-variant 'rhel9.0'",
		"--network bridge=br0", "--graphics 'vnc'", "--noautoconsole",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("sans %q:\n%s", want, cmd)
		}
	}
}

func TestBuildNutanixPlan(t *testing.T) {
	hvs, cc := testSetup()
	req := testReq()
	req.Hypervisor = "ahv-01"
	req.Datastore = "cont-01"
	req.Network = "vlan-10"
	req.IP = "192.168.3.50"
	plan, err := BuildPlan(req, hvs, cc, config.SSHConfig{})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	joined := strings.Join(plan.Commands, "\n")
	for _, want := range []string{
		"acli vm.create", "acli vm.disk_create", "clone_from_image='rhel95'",
		"acli vm.nic_create", "network='vlan-10'", "ip='192.168.3.50'", "acli vm.on",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("sans %q:\n%s", want, joined)
		}
	}
}

func TestRunnerFor(t *testing.T) {
	hvs, _ := testSetup()
	ex, kind, err := RunnerFor(hvs, "esx-08", config.SSHConfig{})
	if err != nil {
		t.Fatalf("RunnerFor: %v", err)
	}
	if kind != "esxi" || ex.Host != "192.168.0.10" || ex.User != "root" {
		t.Fatalf("runner inattendu: %s %+v", kind, ex)
	}
	if _, _, err := RunnerFor(hvs, "nope", config.SSHConfig{}); err == nil {
		t.Fatal("hyperviseur inconnu: erreur attendue")
	}
}
