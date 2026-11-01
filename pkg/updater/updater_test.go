package updater

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBinary(t *testing.T) {
	// 1. Too small
	err := validateBinary([]byte("small"))
	if err == nil {
		t.Errorf("expected error for small binary")
	}

	// 2. Large enough but invalid header
	largeGarbage := make([]byte, 2*1024*1024)
	err = validateBinary(largeGarbage)
	if err == nil {
		t.Errorf("expected error for invalid magic header")
	}

	// 3. Valid Windows PE header (MZ)
	peBinary := make([]byte, 2*1024*1024)
	peBinary[0] = 'M'
	peBinary[1] = 'Z'
	if err := validateBinary(peBinary); err != nil {
		t.Errorf("unexpected error for PE binary: %v", err)
	}

	// 4. Valid ELF header (\x7fELF)
	elfBinary := make([]byte, 2*1024*1024)
	copy(elfBinary[:4], []byte{0x7f, 'E', 'L', 'F'})
	if err := validateBinary(elfBinary); err != nil {
		t.Errorf("unexpected error for ELF binary: %v", err)
	}
}

func TestDownloadAndApplyUpdate(t *testing.T) {
	// Create mock binary server
	mockBinary := make([]byte, 2*1024*1024)
	mockBinary[0] = 'M'
	mockBinary[1] = 'Z'

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(mockBinary)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	currentExe := filepath.Join(tmpDir, "crybaby-agent.exe")
	if err := os.WriteFile(currentExe, []byte("old-binary-content"), 0755); err != nil {
		t.Fatalf("failed to create dummy current exe: %v", err)
	}

	err := ApplyUpdate(server.URL, "0.5.0", currentExe)
	if err != nil {
		t.Fatalf("ApplyUpdate failed: %v", err)
	}
}
