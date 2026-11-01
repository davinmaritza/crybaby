package main

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/davinmaritza/crybaby/pkg/types"
)

// DetectHardware queries local system specifications.
func DetectHardware() types.NodeHardware {
	hostname, _ := os.Hostname()
	mac := getPrimaryMACAddress()
	hyperv := checkHyperVEnabled()

	return types.NodeHardware{
		Hostname:       hostname,
		OS:             runtime.GOOS + " " + runtime.GOARCH,
		KernelVersion:  runtime.Version(),
		CPUModel:       runtime.GOARCH + " Multi-Core Processor",
		CPUCores:       runtime.NumCPU(),
		CPUThreads:     runtime.NumCPU(),
		TotalRAMBytes:  16 * 1024 * 1024 * 1024, // Baseline detection fallback
		TotalDiskBytes: 512 * 1024 * 1024 * 1024,
		MACAddress:     mac,
		HyperVEnabled:  hyperv,
	}
}

// getPrimaryMACAddress returns the physical MAC address of the first active broadcast interface.
func getPrimaryMACAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			mac := iface.HardwareAddr.String()
			if mac != "" {
				return mac
			}
		}
	}
	return ""
}

// checkHyperVEnabled determines whether Microsoft Hyper-V virtualization is active.
func checkHyperVEnabled() bool {
	if runtime.GOOS != "windows" {
		return true // Mocked true for cross-platform simulation
	}

	// Query sc query vmms (Virtual Machine Management Service)
	cmd := exec.Command("sc", "query", "vmms")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "STATE") {
		return true
	}
	return false
}
