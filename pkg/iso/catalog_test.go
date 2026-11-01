package iso

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListISOs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dummy files
	f1 := filepath.Join(tmpDir, "ubuntu-24.04.iso")
	_ = os.WriteFile(f1, []byte("dummy-iso-content-1"), 0644)

	f2 := filepath.Join(tmpDir, "debian-12.img")
	_ = os.WriteFile(f2, []byte("dummy-img-content-2"), 0644)

	f3 := filepath.Join(tmpDir, "not-an-iso.txt")
	_ = os.WriteFile(f3, []byte("text-content"), 0644)

	list, err := ListISOs(tmpDir)
	if err != nil {
		t.Fatalf("ListISOs failed: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 ISO files, got %d", len(list))
	}

	foundUbuntu := false
	for _, iso := range list {
		if iso.Name == "ubuntu-24.04.iso" {
			foundUbuntu = true
			if iso.SizeBytes != int64(len("dummy-iso-content-1")) {
				t.Errorf("expected size %d, got %d", len("dummy-iso-content-1"), iso.SizeBytes)
			}
		}
	}
	if !foundUbuntu {
		t.Errorf("ubuntu-24.04.iso not found in list")
	}
}

func TestDownloadManager(t *testing.T) {
	testData := []byte("mock-iso-payload-for-download")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "29")
		_, _ = w.Write(testData)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	job, err := mgr.StartDownload(server.URL, "test-download.iso")
	if err != nil {
		t.Fatalf("StartDownload failed: %v", err)
	}

	// Wait for background download to finish
	var finishedJob *DownloadJob
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		j, found := mgr.GetJob(job.ID)
		if found && j.Status == "completed" {
			finishedJob = j
			break
		}
	}

	if finishedJob == nil {
		t.Fatalf("download job did not complete in time")
	}

	if finishedJob.Downloaded != int64(len(testData)) {
		t.Errorf("expected downloaded bytes %d, got %d", len(testData), finishedJob.Downloaded)
	}

	destFile := filepath.Join(tmpDir, "test-download.iso")
	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("file content mismatch: expected '%s', got '%s'", testData, content)
	}
}
