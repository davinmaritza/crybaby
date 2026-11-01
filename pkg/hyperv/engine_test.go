package hyperv

import (
	"testing"

	"github.com/davinmaritza/crybaby/pkg/types"
)

func TestHyperVEngineLifecycle(t *testing.T) {
	nodeID := "node-test-win11"
	engine := NewEngine(nodeID)

	// 1. Verify initial list has default VM
	vms, err := engine.ListVMs()
	if err != nil {
		t.Fatalf("failed to list VMs: %v", err)
	}
	if len(vms) == 0 {
		t.Errorf("expected at least 1 VM in list")
	}

	// 2. Create a new VM
	createReq := types.CreateVMRequest{
		Name:       "windows-dev-01",
		CPUCount:   4,
		MemoryMB:   8192,
		DiskSizeGB: 100,
		SwitchName: "Default Switch",
	}

	newVM, err := engine.CreateVM(createReq)
	if err != nil {
		t.Fatalf("failed to create VM: %v", err)
	}

	if newVM.Name != "windows-dev-01" {
		t.Errorf("expected VM name 'windows-dev-01', got '%s'", newVM.Name)
	}
	if newVM.State != types.VMStateStopped {
		t.Errorf("expected newly created VM to be STOPPED, got '%s'", newVM.State)
	}

	// 3. Test duplicate VM name error
	_, err = engine.CreateVM(createReq)
	if err == nil {
		t.Errorf("expected error when creating duplicate VM name, got nil")
	}

	// 4. Start VM
	if err := engine.StartVM(newVM.ID); err != nil {
		t.Fatalf("failed to start VM: %v", err)
	}
	startedVM, err := engine.GetVM(newVM.ID)
	if err != nil || startedVM.State != types.VMStateRunning {
		t.Errorf("expected VM state RUNNING, got '%s'", startedVM.State)
	}

	// 5. Restart VM
	if err := engine.RestartVM(newVM.ID); err != nil {
		t.Fatalf("failed to restart VM: %v", err)
	}

	// 6. Stop VM
	if err := engine.StopVM(newVM.ID, false); err != nil {
		t.Fatalf("failed to stop VM: %v", err)
	}
	stoppedVM, err := engine.GetVM(newVM.ID)
	if err != nil || stoppedVM.State != types.VMStateStopped {
		t.Errorf("expected VM state STOPPED, got '%s'", stoppedVM.State)
	}

	// 7. Delete VM
	if err := engine.DeleteVM(newVM.ID, true); err != nil {
		t.Fatalf("failed to delete VM: %v", err)
	}
	_, err = engine.GetVM(newVM.ID)
	if err == nil {
		t.Errorf("expected error when fetching deleted VM, got nil")
	}

	// 8. Virtual Switches
	switches, err := engine.ListVirtualSwitches()
	if err != nil || len(switches) == 0 {
		t.Errorf("expected non-empty virtual switch list")
	}
}
