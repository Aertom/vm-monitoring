package kvm

import (
	"context"
	"fmt"
	"testing"
)

// mockExecutor simule l'exécution de commandes virsh pour les tests.
type mockExecutor struct {
	outputs map[string]string
	err     error
}

func (m *mockExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	key := name + " " + fmt.Sprint(args)
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return "", fmt.Errorf("mock: no output configured for %s", key)
}

func TestNewClient_NilExecutor(t *testing.T) {
	_, err := NewClient(nil)
	if err == nil {
		t.Fatal("expected error when executor is nil, got nil")
	}
}

func TestListVMs_Success(t *testing.T) {
	out := ` Id   Name      State
----------------------------
 1    web-01    running
 -    web-02    shut off
`
	exec := &mockExecutor{outputs: map[string]string{
		"virsh [list --all]": out,
	}}

	c, err := NewClient(exec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vms, err := c.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vms) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(vms))
	}
	if vms[0].Name != "web-01" || vms[0].State != "running" {
		t.Fatalf("unexpected first VM: %+v", vms[0])
	}
	if vms[1].Name != "web-02" || vms[1].State != "shut off" {
		t.Fatalf("unexpected second VM: %+v", vms[1])
	}
}

func TestListVMs_ExecutorError(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("connection refused")}
	c, _ := NewClient(exec)

	_, err := c.ListVMs(context.Background())
	if err == nil {
		t.Fatal("expected error when executor fails, got nil")
	}
}

func TestGetVMDetails_Success(t *testing.T) {
	out := `Id:             1
Name:           web-01
State:          running
CPU(s):         4
Used memory:    4194304 KiB
`
	exec := &mockExecutor{outputs: map[string]string{
		"virsh [dominfo web-01]": out,
	}}

	c, _ := NewClient(exec)
	vm, err := c.GetVMDetails(context.Background(), "web-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vm.VCPUs != 4 {
		t.Fatalf("expected 4 vCPUs, got %d", vm.VCPUs)
	}
	if vm.MemoryKB != 4194304 {
		t.Fatalf("expected 4194304 KiB, got %d", vm.MemoryKB)
	}
	if vm.State != "running" {
		t.Fatalf("expected state 'running', got %q", vm.State)
	}
}

func TestGetVMDetails_EmptyName(t *testing.T) {
	exec := &mockExecutor{}
	c, _ := NewClient(exec)

	_, err := c.GetVMDetails(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestParseVirshList_LocalizedHeader(t *testing.T) {
	// En-tête allemand + VM éteinte : seules les lignes Id/- sont gardées.
	out := "Kennung  Name      Status\n--------------------------\n 2    vm-de-01    running\n -    vm-de-02    shut off\n"
	vms := parseVirshList(out)
	if len(vms) != 2 || vms[0].Name != "vm-de-01" || vms[1].ID != "-" {
		t.Fatalf("parse localisé incorrect: %+v", vms)
	}
}

func TestListVMs_WithURI(t *testing.T) {
	out := "Id Name State\n---\n 1 a running\n"
	exec := &mockExecutor{outputs: map[string]string{
		"virsh [-c qemu:///system list --all]": out,
	}}
	c, _ := NewClient(exec)
	c.URI = "qemu:///system"
	vms, err := c.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vms) != 1 || vms[0].Name != "a" {
		t.Fatalf("unexpected VMs: %+v", vms)
	}
}

func TestParseDomIfAddr(t *testing.T) {
	out := `Name       MAC address          Protocol     Address
---------------------------------------------------
vnet0      52:54:00:12:34:56    ipv4         192.168.122.10/24
vnet1      52:54:00:12:34:57    ipv6         fe80::5054:ff:fe12:3457/64
`
	if got := ParseDomIfAddr(out); got != "192.168.122.10" {
		t.Fatalf("IP=%q", got)
	}
	if got := ParseDomIfAddr("Name MAC\n---\n"); got != "" {
		t.Fatalf("attendu vide, obtenu %q", got)
	}
	// Loopback ignorée au profit de la suivante.
	loop := "Name MAC Protocol Address\n---\nvnet0 m ipv4 127.0.0.2/8\nvnet1 m ipv4 10.0.0.3/24\n"
	if got := ParseDomIfAddr(loop); got != "10.0.0.3" {
		t.Fatalf("IP=%q", got)
	}
}

func TestGetPrimaryIP(t *testing.T) {
	exec := &mockExecutor{outputs: map[string]string{
		"virsh [domifaddr web-01]": "Name MAC Protocol Address\n---\nvnet0 m ipv4 10.0.0.8/24\n",
	}}
	c, _ := NewClient(exec)
	ip, err := c.GetPrimaryIP(context.Background(), "web-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "10.0.0.8" {
		t.Fatalf("IP=%q", ip)
	}
	if _, err := c.GetPrimaryIP(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}
