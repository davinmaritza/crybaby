package updater

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// MinBinarySizeBytes enforces minimum binary size (~1MB) to prevent writing HTML error pages as binaries.
const MinBinarySizeBytes = 1024 * 1024

// validateBinary checks file length and binary magic headers.
func validateBinary(data []byte) error {
	if len(data) < MinBinarySizeBytes {
		return fmt.Errorf("downloaded binary too small (%d bytes, minimum %d bytes expected)", len(data), MinBinarySizeBytes)
	}

	// Windows PE header starts with 'M' 'Z' (0x4D, 0x5A)
	if data[0] == 'M' && data[1] == 'Z' {
		return nil
	}

	// Linux ELF header starts with 0x7F 'E' 'L' 'F'
	if len(data) >= 4 && data[0] == 0x7F && data[1] == 'E' && data[2] == 'L' && data[3] == 'F' {
		return nil
	}

	return errors.New("downloaded payload is not a valid executable binary (unrecognized header)")
}

// ApplyUpdate downloads the new agent binary, validates its integrity, and triggers safe detachment replacement.
func ApplyUpdate(downloadURL, targetVersion, currentExePath string) error {
	if currentExePath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine current executable path: %w", err)
		}
		currentExePath = exe
	}

	// Clean path
	currentExePath = filepath.Clean(currentExePath)
	dir := filepath.Dir(currentExePath)
	exeName := filepath.Base(currentExePath)

	newExePath := filepath.Join(dir, exeName+".new")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update from %s: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned HTTP %d", resp.StatusCode)
	}

	// Read binary into memory for validation
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read binary payload: %w", err)
	}

	if err := validateBinary(data); err != nil {
		return fmt.Errorf("binary validation failed: %w", err)
	}

	// Write new binary
	if err := os.WriteFile(newExePath, data, 0755); err != nil {
		return fmt.Errorf("failed to write staging binary to %s: %w", newExePath, err)
	}

	// Execute detached replacement based on platform
	if runtime.GOOS == "windows" {
		scriptPath := filepath.Join(dir, "apply-update.bat")
		scriptContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
net stop CrybabyAgent >nul 2>&1
copy /y "%%~dp0%s.new" "%%~dp0%s" >nul 2>&1
del /f /q "%%~dp0%s.new" >nul 2>&1
net start CrybabyAgent >nul 2>&1
del "%%~f0" >nul 2>&1
`, exeName, exeName, exeName)

		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			_ = os.Remove(newExePath)
			return fmt.Errorf("failed to write updater batch script: %w", err)
		}

		// Detach script so it survives the service termination
		cmd := exec.Command("cmd.exe", "/c", "start", "", "/b", scriptPath)
		if err := cmd.Start(); err != nil {
			_ = os.Remove(newExePath)
			_ = os.Remove(scriptPath)
			return fmt.Errorf("failed to spawn updater script: %w", err)
		}
	} else {
		// Non-Windows / Test environment: atomic replace
		if err := os.Rename(newExePath, currentExePath); err != nil {
			_ = os.Remove(newExePath)
			return fmt.Errorf("failed to replace binary: %w", err)
		}
	}

	return nil
}
