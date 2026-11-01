package iso

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ISOFile represents a discovered installer image in the repository.
type ISOFile struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"size_bytes"`
	SizeStr   string    `json:"size_str"`
	Modified  time.Time `json:"modified"`
}

// DownloadJob tracks an ongoing or completed ISO background download.
type DownloadJob struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Filename    string    `json:"filename"`
	TotalBytes  int64     `json:"total_bytes"`
	Downloaded  int64     `json:"downloaded_bytes"`
	ProgressPct float64   `json:"progress_pct"`
	Status      string    `json:"status"` // "downloading", "completed", "failed"
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
}

// Manager orchestrates ISO catalog discovery and background URL streaming.
type Manager struct {
	mu      sync.RWMutex
	dir     string
	jobs    map[string]*DownloadJob
	client  *http.Client
}

// NewManager initializes the ISO manager for a target directory.
func NewManager(dir string) *Manager {
	_ = os.MkdirAll(dir, 0755)
	return &Manager{
		dir:  dir,
		jobs: make(map[string]*DownloadJob),
		client: &http.Client{
			Timeout: 0, // No global timeout for large ISO streams
		},
	}
}

// ListISOs scans the repository directory for valid ISO or disk image files.
func ListISOs(dir string) ([]ISOFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ISOFile{}, nil
		}
		return nil, err
	}

	var results []ISOFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".iso") || strings.HasSuffix(lower, ".img") || strings.HasSuffix(lower, ".vhdx") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			results = append(results, ISOFile{
				Name:      name,
				SizeBytes: info.Size(),
				SizeStr:   FormatBytes(info.Size()),
				Modified:  info.ModTime().UTC(),
			})
		}
	}
	return results, nil
}

// List returns ISOs discovered in the manager's directory.
func (m *Manager) List() ([]ISOFile, error) {
	return ListISOs(m.dir)
}

// StartDownload starts a non-blocking background HTTP stream into the ISO repository.
func (m *Manager) StartDownload(rawURL, filename string) (*DownloadJob, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("invalid HTTP/HTTPS download URL")
	}

	if filename == "" {
		filename = filepath.Base(parsed.Path)
	}
	if filename == "" || filename == "." || filename == "/" {
		filename = "downloaded-image.iso"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".iso") && !strings.HasSuffix(strings.ToLower(filename), ".img") {
		filename += ".iso"
	}

	// Sanitize filename to prevent directory traversal
	filename = filepath.Base(filename)

	id := generateRandomID(8)
	job := &DownloadJob{
		ID:        id,
		URL:       rawURL,
		Filename:  filename,
		Status:    "downloading",
		StartedAt: time.Now().UTC(),
	}

	m.mu.Lock()
	m.jobs[id] = job
	m.mu.Unlock()

	go m.executeDownload(job)
	return job, nil
}

func (m *Manager) executeDownload(job *DownloadJob) {
	destPath := filepath.Join(m.dir, job.Filename)
	partPath := destPath + ".part"

	resp, err := m.client.Get(job.URL)
	if err != nil {
		m.failJob(job, fmt.Sprintf("HTTP connection failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		m.failJob(job, fmt.Sprintf("server returned HTTP %d", resp.StatusCode))
		return
	}

	total := resp.ContentLength
	job.TotalBytes = total

	out, err := os.Create(partPath)
	if err != nil {
		m.failJob(job, fmt.Sprintf("failed to create destination file: %v", err))
		return
	}
	defer out.Close()

	buf := make([]byte, 64*1024) // 64KB buffer
	var downloaded int64

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				m.failJob(job, fmt.Sprintf("write error: %v", wErr))
				_ = os.Remove(partPath)
				return
			}
			downloaded += int64(n)

			m.mu.Lock()
			job.Downloaded = downloaded
			if total > 0 {
				job.ProgressPct = float64(downloaded) / float64(total) * 100.0
			}
			m.mu.Unlock()
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			m.failJob(job, fmt.Sprintf("read error: %v", rErr))
			_ = os.Remove(partPath)
			return
		}
	}

	_ = out.Close()
	if err := os.Rename(partPath, destPath); err != nil {
		m.failJob(job, fmt.Sprintf("failed to finalize file: %v", err))
		_ = os.Remove(partPath)
		return
	}

	m.mu.Lock()
	job.Status = "completed"
	job.ProgressPct = 100.0
	m.mu.Unlock()
}

func (m *Manager) failJob(job *DownloadJob, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job.Status = "failed"
	job.Error = errMsg
}

// GetJob returns a copy of a specific job by ID.
func (m *Manager) GetJob(id string) (*DownloadJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, found := m.jobs[id]
	if !found {
		return nil, false
	}
	cp := *j
	return &cp, true
}

// ListJobs returns all active and recent download jobs.
func (m *Manager) ListJobs() []*DownloadJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]*DownloadJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		cp := *j
		results = append(results, &cp)
	}
	return results
}

// FormatBytes formats byte counts into human-readable strings.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func generateRandomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
