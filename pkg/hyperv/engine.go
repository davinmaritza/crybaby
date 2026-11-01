package hyperv

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/davinmaritza/crybaby/pkg/types"
)

// Engine defines the operational interface for Hyper-V virtualization on Windows 11.
type Engine interface {
	ListVMs() ([]types.VirtualMachine, error)
	GetVM(nameOrID string) (*types.VirtualMachine, error)
	CreateVM(req types.CreateVMRequest) (*types.VirtualMachine, error)
	StartVM(vmID string) error
	StopVM(vmID string, force bool) error
	RestartVM(vmID string) error
	DeleteVM(vmID string, deleteDisk bool) error
	ListVirtualSwitches() ([]string, error)
}

// NewEngine returns a platform-appropriate Hyper-V engine.
// On Windows, it invokes native PowerShell Hyper-V cmdlets; on non-Windows it provides a simulated store.
func NewEngine(nodeID string) Engine {
	if runtime.GOOS == "windows" {
		return &windowsEngine{nodeID: nodeID}
	}
	return newSimulatedEngine(nodeID)
}

// windowsEngine executes native PowerShell cmdlets against the local Hyper-V management service.
type windowsEngine struct {
	nodeID string
}

func (w *windowsEngine) ListVMs() ([]types.VirtualMachine, error) {
	script := `Get-VM | Select-Object Id, Name, State, CPUUsage, MemoryAssigned, Uptime | ConvertTo-Json -Compress`
	out, err := runPowerShell(script)
	if err != nil {
		return nil, fmt.Errorf("failed to query Hyper-V VMs: %w", err)
	}

	out = strings.TrimSpace(out)
	if out == "" || out == "null" {
		return []types.VirtualMachine{}, nil
	}

	// Handle both single object and array output from PowerShell JSON
	if strings.HasPrefix(out, "{") {
		out = "[" + out + "]"
	}

	type psVM struct {
		Id             string `json:"Id"`
		Name           string `json:"Name"`
		State          int    `json:"State"`
		MemoryAssigned int64  `json:"MemoryAssigned"`
	}

	var raw []psVM
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse Hyper-V JSON output: %w", err)
	}

	vms := make([]types.VirtualMachine, 0, len(raw))
	for _, item := range raw {
		state := types.VMStateStopped
		if item.State == 2 { // Hyper-V 2 = Running
			state = types.VMStateRunning
		} else if item.State == 3 {
			state = types.VMStateStopped
		} else if item.State == 6 {
			state = types.VMStateSaved
		}

		vms = append(vms, types.VirtualMachine{
			ID:        item.Id,
			NodeID:    w.nodeID,
			Name:      item.Name,
			State:     state,
			MemoryMB:  uint64(item.MemoryAssigned / (1024 * 1024)),
			CreatedAt: time.Now(),
		})
	}
	return vms, nil
}

func (w *windowsEngine) GetVM(nameOrID string) (*types.VirtualMachine, error) {
	vms, err := w.ListVMs()
	if err != nil {
		return nil, err
	}
	for _, vm := range vms {
		if vm.ID == nameOrID || strings.EqualFold(vm.Name, nameOrID) {
			return &vm, nil
		}
	}
	return nil, fmt.Errorf("virtual machine '%s' not found", nameOrID)
}

func (w *windowsEngine) CreateVM(req types.CreateVMRequest) (*types.VirtualMachine, error) {
	if req.Name == "" {
		return nil, errors.New("VM name cannot be empty")
	}
	if req.MemoryMB == 0 {
		req.MemoryMB = 2048
	}
	if req.CPUCount == 0 {
		req.CPUCount = 2
	}
	if req.DiskSizeGB == 0 {
		req.DiskSizeGB = 32
	}
	if req.SwitchName == "" {
		req.SwitchName = "Default Switch"
	}

	vhdPath := fmt.Sprintf(`C:\Crybaby\Disks\%s.vhdx`, req.Name)
	script := fmt.Sprintf(`
		$vhdDir = "C:\Crybaby\Disks"
		if (!(Test-Path $vhdDir)) { New-Item -ItemType Directory -Path $vhdDir -Force | Out-Null }
		New-VM -Name "%s" -MemoryStartupBytes %dMB -NewVHDPath "%s" -NewVHDSizeBytes %dGB -Generation 2 -SwitchName "%s" | Select-Object -ExpandProperty Id
		Set-VMProcessor -VMName "%s" -Count %d
	`, req.Name, req.MemoryMB, vhdPath, req.DiskSizeGB, req.SwitchName, req.Name, req.CPUCount)

	out, err := runPowerShell(script)
	if err != nil {
		return nil, fmt.Errorf("failed to create Hyper-V VM: %w (%s)", err, out)
	}

	vmID := strings.TrimSpace(out)
	return &types.VirtualMachine{
		ID:            vmID,
		NodeID:        w.nodeID,
		Name:          req.Name,
		State:         types.VMStateStopped,
		CPUCount:      req.CPUCount,
		MemoryMB:      req.MemoryMB,
		DiskPath:      vhdPath,
		DiskSizeBytes: req.DiskSizeGB * 1024 * 1024 * 1024,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (w *windowsEngine) StartVM(vmID string) error {
	script := fmt.Sprintf(`Start-VM -Id "%s"`, vmID)
	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) StopVM(vmID string, force bool) error {
	mode := "-ShutDown"
	if force {
		mode = "-TurnOff"
	}
	script := fmt.Sprintf(`Stop-VM -Id "%s" %s`, vmID, mode)
	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to stop VM: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) RestartVM(vmID string) error {
	script := fmt.Sprintf(`Restart-VM -Id "%s" -Force`, vmID)
	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to restart VM: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) DeleteVM(vmID string, deleteDisk bool) error {
	script := fmt.Sprintf(`
		$vm = Get-VM -Id "%s"
		$vhdPath = (Get-VMHardDiskDrive -VM $vm).Path
		Stop-VM -VM $vm -TurnOff -ErrorAction SilentlyContinue
		Remove-VM -VM $vm -Force
	`, vmID)

	if deleteDisk {
		script += `
		if ($vhdPath -and (Test-Path $vhdPath)) { Remove-Item -Path $vhdPath -Force }
		`
	}

	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) ListVirtualSwitches() ([]string, error) {
	script := `Get-VMSwitch | Select-Object -ExpandProperty Name | ConvertTo-Json -Compress`
	out, err := runPowerShell(script)
	if err != nil {
		return []string{"Default Switch"}, nil
	}
	out = strings.TrimSpace(out)
	if out == "" || out == "null" {
		return []string{"Default Switch"}, nil
	}

	if strings.HasPrefix(out, "[") {
		var list []string
		_ = json.Unmarshal([]byte(out), &list)
		if len(list) > 0 {
			return list, nil
		}
	} else if strings.HasPrefix(out, "\"") {
		var single string
		_ = json.Unmarshal([]byte(out), &single)
		if single != "" {
			return []string{single}, nil
		}
	}
	return []string{"Default Switch"}, nil
}

func runPowerShell(command string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// simulatedEngine provides an in-memory Hyper-V simulation for development and non-Windows hosts.
type simulatedEngine struct {
	mu     sync.RWMutex
	nodeID string
	vms    map[string]*types.VirtualMachine
}

func newSimulatedEngine(nodeID string) *simulatedEngine {
	s := &simulatedEngine{
		nodeID: nodeID,
		vms:    make(map[string]*types.VirtualMachine),
	}
	// Seed 1 default demo VM
	demoID := "c1000000-0000-0000-0000-000000000001"
	s.vms[demoID] = &types.VirtualMachine{
		ID:            demoID,
		NodeID:        nodeID,
		Name:          "ubuntu-srv-01",
		State:         types.VMStateRunning,
		CPUCount:      2,
		MemoryMB:      4096,
		DiskPath:      `C:\Crybaby\Disks\ubuntu-srv-01.vhdx`,
		DiskSizeBytes: 40 * 1024 * 1024 * 1024,
		UptimeSeconds: 7200,
		IPAddresses:   []string{"10.42.100.15"},
		CreatedAt:     time.Now().Add(-2 * time.Hour),
	}
	return s
}

func (s *simulatedEngine) ListVMs() ([]types.VirtualMachine, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]types.VirtualMachine, 0, len(s.vms))
	for _, vm := range s.vms {
		list = append(list, *vm)
	}
	return list, nil
}

func (s *simulatedEngine) GetVM(nameOrID string) (*types.VirtualMachine, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, vm := range s.vms {
		if vm.ID == nameOrID || strings.EqualFold(vm.Name, nameOrID) {
			cp := *vm
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("virtual machine '%s' not found", nameOrID)
}

func (s *simulatedEngine) CreateVM(req types.CreateVMRequest) (*types.VirtualMachine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.vms {
		if strings.EqualFold(existing.Name, req.Name) {
			return nil, fmt.Errorf("a VM named '%s' already exists", req.Name)
		}
	}

	newID := fmt.Sprintf("vm-%d", time.Now().UnixNano()%1000000)
	vm := &types.VirtualMachine{
		ID:            newID,
		NodeID:        s.nodeID,
		Name:          req.Name,
		State:         types.VMStateStopped,
		CPUCount:      req.CPUCount,
		MemoryMB:      req.MemoryMB,
		DiskPath:      fmt.Sprintf(`C:\Crybaby\Disks\%s.vhdx`, req.Name),
		DiskSizeBytes: req.DiskSizeGB * 1024 * 1024 * 1024,
		CreatedAt:     time.Now().UTC(),
	}
	s.vms[newID] = vm
	cp := *vm
	return &cp, nil
}

func (s *simulatedEngine) StartVM(vmID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vm, exists := s.vms[vmID]
	if !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	vm.State = types.VMStateRunning
	vm.UptimeSeconds = 0
	return nil
}

func (s *simulatedEngine) StopVM(vmID string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vm, exists := s.vms[vmID]
	if !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	vm.State = types.VMStateStopped
	vm.UptimeSeconds = 0
	return nil
}

func (s *simulatedEngine) RestartVM(vmID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vm, exists := s.vms[vmID]
	if !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	vm.State = types.VMStateRunning
	vm.UptimeSeconds = 0
	return nil
}

func (s *simulatedEngine) DeleteVM(vmID string, deleteDisk bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.vms[vmID]; !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	delete(s.vms, vmID)
	return nil
}

func (s *simulatedEngine) ListVirtualSwitches() ([]string, error) {
	return []string{"Default Switch", "Internal-VMSwitch", "External-Bridge"}, nil
}
