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

	// 8. Virtual Switches & EnsureVirtualSwitch
	switches, err := engine.ListVirtualSwitches()
	if err != nil || len(switches) == 0 {
		t.Errorf("expected non-empty virtual switch list")
	}
	if err := engine.EnsureVirtualSwitch("Crybaby-LAN"); err != nil {
		t.Fatalf("failed to ensure virtual switch: %v", err)
	}

	// 9. Test Checkpoint / Snapshot Lifecycle
	demoVM, err := engine.GetVM("c1000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("failed to get demo VM: %v", err)
	}
	if demoVM.CPUUsagePct < 0 {
		t.Errorf("expected valid CPUUsagePct, got %d", demoVM.CPUUsagePct)
	}

	cp, err := engine.CreateCheckpoint(demoVM.ID, "clean-install")
	if err != nil {
		t.Fatalf("failed to create checkpoint: %v", err)
	}
	if cp.Name != "clean-install" {
		t.Errorf("expected checkpoint name 'clean-install', got '%s'", cp.Name)
	}

	cps, err := engine.ListCheckpoints(demoVM.ID)
	if err != nil || len(cps) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d (err: %v)", len(cps), err)
	}

	if err := engine.RestoreCheckpoint(demoVM.ID, "clean-install"); err != nil {
		t.Fatalf("failed to restore checkpoint: %v", err)
	}

	if err := engine.DeleteCheckpoint(demoVM.ID, "clean-install"); err != nil {
		t.Fatalf("failed to delete checkpoint: %v", err)
	}

	cpsAfter, err := engine.ListCheckpoints(demoVM.ID)
	if err != nil || len(cpsAfter) != 0 {
		t.Fatalf("expected 0 checkpoints after deletion, got %d", len(cpsAfter))
	}
}
