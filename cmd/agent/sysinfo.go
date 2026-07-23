package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// SysInfo represents the system specification.
type SysInfo struct {
	Hostname     string
	OSVersion    string
	CPUModel     string
	CPUCores     int
	CPUThreads   int
	RAMTotalMB   uint64
	DiskTotalMB  uint64
	AgentVersion string
}

// LiveMetrics represents the dynamic metrics.
type LiveMetrics struct {
	CPULoadPct    float64
	RAMUsedMB     uint64
	DiskUsedMB    uint64
	UptimeSeconds uint64
}

// runPowerShell executes a PowerShell command and returns the trimmed output.
func runPowerShell(cmd string) (string, error) {
	c := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", cmd)
	var out bytes.Buffer
	var errOut bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errOut
	err := c.Run()
	if err != nil {
		return "", fmt.Errorf("err: %v, stderr: %s", err, errOut.String())
	}
	return strings.TrimSpace(out.String()), nil
}

func GetSystemInfo(agentVersion string) (*SysInfo, error) {
	info := &SysInfo{
		AgentVersion: agentVersion,
	}

	// 1. Hostname
	hn, err := runPowerShell("[System.Net.Dns]::GetHostName()")
	if err != nil {
		hn = "Unknown-Host"
	}
	info.Hostname = hn

	// 2. OS Version
	osName, err := runPowerShell("(Get-CimInstance Win32_OperatingSystem).Caption")
	if err != nil {
		osName = "Windows"
	}
	osVer, err := runPowerShell("(Get-CimInstance Win32_OperatingSystem).Version")
	if err != nil {
		osVer = "Unknown"
	}
	info.OSVersion = fmt.Sprintf("%s (Build %s)", osName, osVer)

	// 3. CPU Name
	cpuName, err := runPowerShell("(Get-CimInstance Win32_Processor).Name")
	if err != nil {
		cpuName = "Unknown CPU"
	}
	// If multi-CPU, get the first one
	if idx := strings.Index(cpuName, "\n"); idx != -1 {
		cpuName = cpuName[:idx]
	}
	info.CPUModel = strings.TrimSpace(cpuName)

	// 4. CPU Cores & Threads
	coresStr, err := runPowerShell("(Get-CimInstance Win32_Processor).NumberOfCores")
	if err == nil {
		info.CPUCores, _ = strconv.Atoi(coresStr)
	}
	if info.CPUCores == 0 {
		info.CPUCores = 1
	}

	threadsStr, err := runPowerShell("(Get-CimInstance Win32_Processor).NumberOfLogicalProcessors")
	if err == nil {
		info.CPUThreads, _ = strconv.Atoi(threadsStr)
	}
	if info.CPUThreads == 0 {
		info.CPUThreads = 1
	}

	// 5. Total RAM (MB)
	ramStr, err := runPowerShell("[math]::round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1MB)")
	if err == nil {
		ramVal, _ := strconv.ParseUint(ramStr, 10, 64)
		info.RAMTotalMB = ramVal
	}

	// 6. Total Disk C: (MB)
	diskStr, err := runPowerShell("[math]::round((Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='C:'\").Size / 1MB)")
	if err == nil {
		diskVal, _ := strconv.ParseUint(diskStr, 10, 64)
		info.DiskTotalMB = diskVal
	}

	return info, nil
}

func GetLiveMetrics() (*LiveMetrics, error) {
	metrics := &LiveMetrics{}

	// 1. CPU Load
	cpuStr, err := runPowerShell("(Get-CimInstance Win32_PerfFormattedData_PerfOS_Processor -Filter \"Name='_Total'\").PercentProcessorTime")
	if err == nil {
		cpuVal, _ := strconv.ParseFloat(cpuStr, 64)
		metrics.CPULoadPct = cpuVal
	}

	// 2. RAM Used
	totalRamStr, err := runPowerShell("(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory")
	freeRamStr, err := runPowerShell("(Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory * 1KB") // FreePhysicalMemory is in KB
	if err == nil {
		totalRam, _ := strconv.ParseUint(totalRamStr, 10, 64)
		freeRam, _ := strconv.ParseUint(freeRamStr, 10, 64)
		if totalRam > freeRam {
			metrics.RAMUsedMB = (totalRam - freeRam) / 1024 / 1024
		}
	}

	// 3. Disk Used
	totalDiskStr, err := runPowerShell("(Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='C:'\").Size")
	freeDiskStr, err := runPowerShell("(Get-CimInstance Win32_LogicalDisk -Filter \"DeviceID='C:'\").FreeSpace")
	if err == nil {
		totalDisk, _ := strconv.ParseUint(totalDiskStr, 10, 64)
		freeDisk, _ := strconv.ParseUint(freeDiskStr, 10, 64)
		if totalDisk > freeDisk {
			metrics.DiskUsedMB = (totalDisk - freeDisk) / 1024 / 1024
		}
	}

	// 4. Uptime
	uptimeStr, err := runPowerShell("[int]([DateTime]::Now - (Get-CimInstance Win32_OperatingSystem).LastBootUpTime).TotalSeconds")
	if err == nil {
		uptimeVal, _ := strconv.ParseUint(uptimeStr, 10, 64)
		metrics.UptimeSeconds = uptimeVal
	}

	return metrics, nil
}
