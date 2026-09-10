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
