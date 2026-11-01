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
	EnsureVirtualSwitch(switchName string) error
	CreateCheckpoint(vmID, name string) (*types.VMCheckpoint, error)
	ListCheckpoints(vmID string) ([]types.VMCheckpoint, error)
	RestoreCheckpoint(vmID, name string) error
	DeleteCheckpoint(vmID, name string) error
	ResizeVM(vmID string, cpuCount int, memoryMB uint64) error
	ExpandDisk(vmID string, newSizeGB uint64) error
	CloneVM(sourceVMID, newVMName string) (*types.VirtualMachine, error)
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
	script := `Get-VM | Select-Object Id, Name, State, CPUUsage, MemoryAssigned, MemoryDemand, Uptime, @{N='IPAddresses';E={$_.NetworkAdapters.IPAddresses -join ','}}, @{N='HardDrives';E={$_.HardDrives.Path -join ','}} | ConvertTo-Json -Compress`
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
		CPUUsage       int    `json:"CPUUsage"`
		MemoryAssigned int64  `json:"MemoryAssigned"`
		MemoryDemand   int64  `json:"MemoryDemand"`
		IPAddresses    string `json:"IPAddresses"`
		HardDrives     string `json:"HardDrives"`
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

		var ips []string
		if item.IPAddresses != "" {
			for _, ip := range strings.Split(item.IPAddresses, ",") {
				ip = strings.TrimSpace(ip)
				if ip != "" && !strings.HasPrefix(ip, "fe80:") {
					ips = append(ips, ip)
				}
			}
		}

		demandMB := uint64(0)
		if item.MemoryDemand > 0 {
			demandMB = uint64(item.MemoryDemand / (1024 * 1024))
		}

		vms = append(vms, types.VirtualMachine{
			ID:             item.Id,
			NodeID:         w.nodeID,
			Name:           item.Name,
			State:          state,
			CPUCount:       2,
			CPUUsagePct:    item.CPUUsage,
			MemoryMB:       uint64(item.MemoryAssigned / (1024 * 1024)),
			MemoryDemandMB: demandMB,
			DiskPath:       item.HardDrives,
			IPAddresses:    ips,
			CreatedAt:      time.Now(),
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
	isoCommand := ""
	if req.ISOPath != "" {
		if strings.HasPrefix(req.ISOPath, "http://") || strings.HasPrefix(req.ISOPath, "https://") {
			isoCommand = fmt.Sprintf(`
		$isoUrl = "%s"
		$isoDir = "C:\Crybaby\ISOs"
		if (!(Test-Path $isoDir)) { New-Item -ItemType Directory -Path $isoDir -Force | Out-Null }
		$rawName = [System.IO.Path]::GetFileName($isoUrl.Split('?')[0])
		if (!$rawName -or $rawName -eq "") { $rawName = "installer.iso" }
		$localIso = Join-Path $isoDir $rawName
		if (!(Test-Path $localIso)) {
			Invoke-WebRequest -Uri $isoUrl -OutFile $localIso -UseBasicParsing
		}
		Add-VMDvdDrive -VMName "%s" -Path $localIso
		$dvd = Get-VMDvdDrive -VMName "%s"
		Set-VMFirmware -VMName "%s" -FirstBootDevice $dvd -EnableSecureBoot $false
		`, req.ISOPath, req.Name, req.Name, req.Name)
		} else {
			isoCommand = fmt.Sprintf(`
		Add-VMDvdDrive -VMName "%s" -Path "%s"
		$dvd = Get-VMDvdDrive -VMName "%s"
		Set-VMFirmware -VMName "%s" -FirstBootDevice $dvd -EnableSecureBoot $false
		`, req.Name, req.ISOPath, req.Name, req.Name)
		}
	}

	script := fmt.Sprintf(`
		$vhdDir = "C:\Crybaby\Disks"
		if (!(Test-Path $vhdDir)) { New-Item -ItemType Directory -Path $vhdDir -Force | Out-Null }
		New-VM -Name "%s" -MemoryStartupBytes %dMB -NewVHDPath "%s" -NewVHDSizeBytes %dGB -Generation 2 -SwitchName "%s" | Select-Object -ExpandProperty Id
		Set-VMProcessor -VMName "%s" -Count %d
		Set-VM -Name "%s" -AutomaticStartAction StartIfRunning -AutomaticStopAction Save
		%s
	`, req.Name, req.MemoryMB, vhdPath, req.DiskSizeGB, req.SwitchName, req.Name, req.CPUCount, req.Name, isoCommand)

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

func (w *windowsEngine) EnsureVirtualSwitch(switchName string) error {
	script := fmt.Sprintf(`
		$ext = Get-VMSwitch -Name "%s" -ErrorAction SilentlyContinue
		if (-not $ext) {
			$eth = Get-NetAdapter -Physical -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq "Up" } | Select-Object -First 1
			if ($eth) {
				New-VMSwitch -Name "%s" -NetAdapterName $eth.Name -AllowManagementOS $true -ErrorAction SilentlyContinue | Out-Null
			}
		}
	`, switchName, switchName)
	_, err := runPowerShell(script)
	return err
}

func (w *windowsEngine) CreateCheckpoint(vmID, name string) (*types.VMCheckpoint, error) {
	if name == "" {
		name = fmt.Sprintf("Snapshot-%s", time.Now().Format("20060102-150405"))
	}
	script := fmt.Sprintf(`
		$snap = Checkpoint-VM -Id "%s" -SnapshotName "%s" -Passthru | Select-Object Id, Name, CreationTime | ConvertTo-Json -Compress
		$snap
	`, vmID, name)
	out, err := runPowerShell(script)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpoint: %w (%s)", err, out)
	}
	out = strings.TrimSpace(out)
	type psSnap struct {
		Id           string `json:"Id"`
		Name         string `json:"Name"`
		CreationTime string `json:"CreationTime"`
	}
	var s psSnap
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		return &types.VMCheckpoint{
			ID:        fmt.Sprintf("cp-%d", time.Now().UnixNano()%1000000),
			VMID:      vmID,
			Name:      name,
			CreatedAt: time.Now().UTC(),
		}, nil
	}
	return &types.VMCheckpoint{
		ID:        s.Id,
		VMID:      vmID,
		Name:      s.Name,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (w *windowsEngine) ListCheckpoints(vmID string) ([]types.VMCheckpoint, error) {
	script := fmt.Sprintf(`
		Get-VMSnapshot -VMId "%s" | Select-Object Id, Name, CreationTime | ConvertTo-Json -Compress
	`, vmID)
	out, err := runPowerShell(script)
	if err != nil {
		return []types.VMCheckpoint{}, nil
	}
	out = strings.TrimSpace(out)
	if out == "" || out == "null" {
		return []types.VMCheckpoint{}, nil
	}
	if strings.HasPrefix(out, "{") {
		out = "[" + out + "]"
	}
	type psSnap struct {
		Id           string `json:"Id"`
		Name         string `json:"Name"`
		CreationTime string `json:"CreationTime"`
	}
	var raw []psSnap
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return []types.VMCheckpoint{}, nil
	}
	res := make([]types.VMCheckpoint, 0, len(raw))
	for _, item := range raw {
		res = append(res, types.VMCheckpoint{
			ID:        item.Id,
			VMID:      vmID,
			Name:      item.Name,
			CreatedAt: time.Now().UTC(),
		})
	}
	return res, nil
}

func (w *windowsEngine) RestoreCheckpoint(vmID, name string) error {
	script := fmt.Sprintf(`
		Restore-VMSnapshot -VMId "%s" -Name "%s" -Confirm:$false
	`, vmID, name)
	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to restore checkpoint: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) DeleteCheckpoint(vmID, name string) error {
	script := fmt.Sprintf(`
		Remove-VMSnapshot -VMId "%s" -Name "%s" -Confirm:$false
	`, vmID, name)
	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to delete checkpoint: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) ResizeVM(vmID string, cpuCount int, memoryMB uint64) error {
	if cpuCount <= 0 {
		return errors.New("CPU count must be at least 1")
	}
	if memoryMB < 512 {
		return errors.New("memory must be at least 512 MB")
	}
	script := fmt.Sprintf(`
		$vm = Get-VM -Id "%s"
		if (!$vm) { throw "VM not found" }
		$wasRunning = ($vm.State -eq 'Running')
		if ($wasRunning) {
			Stop-VM -VM $vm -Force
		}
		Set-VMProcessor -VM $vm -Count %d
		Set-VMMemory -VM $vm -StartupBytes %dMB
		if ($wasRunning) {
			Start-VM -VM $vm
		}
	`, vmID, cpuCount, memoryMB)
	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to resize VM: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) ExpandDisk(vmID string, newSizeGB uint64) error {
	if newSizeGB == 0 {
		return errors.New("new disk size must be greater than 0")
	}
	script := fmt.Sprintf(`
		$vm = Get-VM -Id "%s"
		if (!$vm) { throw "VM not found" }
		$vhd = Get-VMHardDiskDrive -VM $vm | Select-Object -First 1
		if (!$vhd -or !$vhd.Path) { throw "No hard disk attached to VM" }
		Resize-VHD -Path $vhd.Path -SizeBytes %dGB
	`, vmID, newSizeGB)
	out, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("failed to expand VM disk: %w (%s)", err, out)
	}
	return nil
}

func (w *windowsEngine) CloneVM(sourceVMID, newVMName string) (*types.VirtualMachine, error) {
	if newVMName == "" {
		return nil, errors.New("new VM name cannot be empty")
	}
	script := fmt.Sprintf(`
		$src = Get-VM -Id "%s"
		if (!$src) { throw "Source VM not found" }
		$existing = Get-VM -Name "%s" -ErrorAction SilentlyContinue
		if ($existing) { throw "A VM named '%s' already exists" }
		$srcVhd = (Get-VMHardDiskDrive -VM $src | Select-Object -First 1).Path
		$vhdDir = "C:\Crybaby\Disks"
		if (!(Test-Path $vhdDir)) { New-Item -ItemType Directory -Path $vhdDir -Force | Out-Null }
		$newVhd = Join-Path $vhdDir "%s.vhdx"
		if ($srcVhd -and (Test-Path $srcVhd)) {
			Copy-Item -Path $srcVhd -Destination $newVhd -Force
		} else {
			New-VHD -Path $newVhd -SizeBytes 32GB -Dynamic | Out-Null
		}
		$switch = (Get-VMNetworkAdapter -VM $src | Select-Object -First 1).SwitchName
		if (!$switch) { $switch = "Crybaby-LAN" }
		$newVM = New-VM -Name "%s" -MemoryStartupBytes $src.MemoryStartup -VHDPath $newVhd -Generation $src.Generation -SwitchName $switch
		Set-VMProcessor -VM $newVM -Count $src.ProcessorCount
		Set-VM -VM $newVM -AutomaticStartAction StartIfRunning -AutomaticStopAction Save
		$newVM | Select-Object -ExpandProperty Id
	`, sourceVMID, newVMName, newVMName, newVMName, newVMName)
	out, err := runPowerShell(script)
	if err != nil {
		return nil, fmt.Errorf("failed to clone VM: %w (%s)", err, out)
	}
	newID := strings.TrimSpace(out)
	return &types.VirtualMachine{
		ID:            newID,
		NodeID:        w.nodeID,
		Name:          newVMName,
		State:         types.VMStateStopped,
		DiskPath:      fmt.Sprintf(`C:\Crybaby\Disks\%s.vhdx`, newVMName),
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func runPowerShell(command string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// simulatedEngine provides an in-memory Hyper-V simulation for development and non-Windows hosts.
type simulatedEngine struct {
	mu          sync.RWMutex
	nodeID      string
	vms         map[string]*types.VirtualMachine
	checkpoints map[string][]types.VMCheckpoint
	switches    []string
}

func newSimulatedEngine(nodeID string) *simulatedEngine {
	s := &simulatedEngine{
		nodeID:      nodeID,
		vms:         make(map[string]*types.VirtualMachine),
		checkpoints: make(map[string][]types.VMCheckpoint),
		switches:    []string{"Default Switch", "Internal-VMSwitch", "External-Bridge", "Crybaby-LAN"},
	}
	// Seed 1 default demo VM
	demoID := "c1000000-0000-0000-0000-000000000001"
	s.vms[demoID] = &types.VirtualMachine{
		ID:             demoID,
		NodeID:         nodeID,
		Name:           "ubuntu-srv-01",
		State:          types.VMStateRunning,
		CPUCount:       2,
		CPUUsagePct:    12,
		MemoryMB:       4096,
		MemoryDemandMB: 1536,
		DiskPath:       `C:\Crybaby\Disks\ubuntu-srv-01.vhdx`,
		DiskSizeBytes:  40 * 1024 * 1024 * 1024,
		UptimeSeconds:  7200,
		IPAddresses:    []string{"10.42.100.15"},
		CreatedAt:      time.Now().Add(-2 * time.Hour),
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
		CPUUsagePct:   0,
		MemoryMB:      req.MemoryMB,
		MemoryDemandMB: 0,
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
	vm.IPAddresses = []string{"192.168.100.192"}
	vm.CPUUsagePct = 8
	vm.MemoryDemandMB = vm.MemoryMB / 2
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
	vm.IPAddresses = nil
	vm.CPUUsagePct = 0
	vm.MemoryDemandMB = 0
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
	vm.CPUUsagePct = 10
	vm.MemoryDemandMB = vm.MemoryMB / 2
	return nil
}

func (s *simulatedEngine) DeleteVM(vmID string, deleteDisk bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.vms[vmID]; !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	delete(s.vms, vmID)
	delete(s.checkpoints, vmID)
	return nil
}

func (s *simulatedEngine) ListVirtualSwitches() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]string, len(s.switches))
	copy(list, s.switches)
	return list, nil
}

func (s *simulatedEngine) EnsureVirtualSwitch(switchName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sw := range s.switches {
		if sw == switchName {
			return nil
		}
	}
	s.switches = append(s.switches, switchName)
	return nil
}

func (s *simulatedEngine) CreateCheckpoint(vmID, name string) (*types.VMCheckpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vm, exists := s.vms[vmID]
	if !exists {
		return nil, fmt.Errorf("VM '%s' not found", vmID)
	}
	if name == "" {
		name = fmt.Sprintf("Snapshot-%s", time.Now().Format("20060102-150405"))
	}
	cp := types.VMCheckpoint{
		ID:        fmt.Sprintf("cp-%d", time.Now().UnixNano()%1000000),
		VMID:      vm.ID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	s.checkpoints[vmID] = append(s.checkpoints[vmID], cp)
	return &cp, nil
}

func (s *simulatedEngine) ListCheckpoints(vmID string) ([]types.VMCheckpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.vms[vmID]; !exists {
		return nil, fmt.Errorf("VM '%s' not found", vmID)
	}
	list := s.checkpoints[vmID]
	if list == nil {
		return []types.VMCheckpoint{}, nil
	}
	res := make([]types.VMCheckpoint, len(list))
	copy(res, list)
	return res, nil
}

func (s *simulatedEngine) RestoreCheckpoint(vmID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.vms[vmID]; !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	for _, cp := range s.checkpoints[vmID] {
		if cp.Name == name || cp.ID == name {
			return nil
		}
	}
	return fmt.Errorf("checkpoint '%s' not found", name)
}

func (s *simulatedEngine) DeleteCheckpoint(vmID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.vms[vmID]; !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	list := s.checkpoints[vmID]
	for i, cp := range list {
		if cp.Name == name || cp.ID == name {
			s.checkpoints[vmID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("checkpoint '%s' not found", name)
}

func (s *simulatedEngine) ResizeVM(vmID string, cpuCount int, memoryMB uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vm, exists := s.vms[vmID]
	if !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	if cpuCount <= 0 {
		return errors.New("CPU count must be at least 1")
	}
	if memoryMB < 512 {
		return errors.New("memory must be at least 512 MB")
	}
	vm.CPUCount = cpuCount
	vm.MemoryMB = memoryMB
	return nil
}

func (s *simulatedEngine) ExpandDisk(vmID string, newSizeGB uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vm, exists := s.vms[vmID]
	if !exists {
		return fmt.Errorf("VM '%s' not found", vmID)
	}
	if newSizeGB == 0 {
		return errors.New("new disk size must be greater than 0")
	}
	newBytes := newSizeGB * 1024 * 1024 * 1024
	if newBytes <= vm.DiskSizeBytes {
		return fmt.Errorf("new size (%d GB) must be larger than current size (%d GB)", newSizeGB, vm.DiskSizeBytes/(1024*1024*1024))
	}
	vm.DiskSizeBytes = newBytes
	return nil
}

func (s *simulatedEngine) CloneVM(sourceVMID, newVMName string) (*types.VirtualMachine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, exists := s.vms[sourceVMID]
	if !exists {
		return nil, fmt.Errorf("source VM '%s' not found", sourceVMID)
	}
	if newVMName == "" {
		return nil, errors.New("new VM name cannot be empty")
	}
	for _, existing := range s.vms {
		if strings.EqualFold(existing.Name, newVMName) {
			return nil, fmt.Errorf("a VM named '%s' already exists", newVMName)
		}
	}
	newID := fmt.Sprintf("vm-%d", time.Now().UnixNano()%1000000)
	clone := &types.VirtualMachine{
		ID:            newID,
		NodeID:        s.nodeID,
		Name:          newVMName,
		State:         types.VMStateStopped,
		CPUCount:      src.CPUCount,
		MemoryMB:      src.MemoryMB,
		DiskPath:      fmt.Sprintf(`C:\Crybaby\Disks\%s.vhdx`, newVMName),
		DiskSizeBytes: src.DiskSizeBytes,
		CreatedAt:     time.Now().UTC(),
	}
	s.vms[newID] = clone
	cp := *clone
	return &cp, nil
}
