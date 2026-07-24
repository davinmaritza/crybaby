package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

type Server struct {
	ID           string     `json:"id"`
	Hostname     string     `json:"hostname"`
	CustomName   *string    `json:"custom_name"`
	ClusterID    *string    `json:"cluster_id"`
	ClusterName  *string    `json:"cluster_name"`
	OSVersion    string     `json:"os_version"`
	CPUModel     string     `json:"cpu_model"`
	CPUCores     int        `json:"cpu_cores"`
	CPUThreads   int        `json:"cpu_threads"`
	RAMTotalMB   uint64     `json:"ram_total_mb"`
	DiskTotalMB  uint64     `json:"disk_total_mb"`
	AgentVersion string     `json:"agent_version"`
	Status       string     `json:"status"`
	FirstSeenAt  time.Time  `json:"first_seen_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	ApprovedAt   *time.Time `json:"approved_at"`
}

type Cluster struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ServerCount int    `json:"server_count"`
}

type CommandLog struct {
	ID          string     `json:"id"`
	ServerID    string     `json:"server_id"`
	ServerName  string     `json:"server_name"`
	IssuedBy    string     `json:"issued_by"`
	Command     string     `json:"command"`
	Result      *string    `json:"result"`
	IssuedAt    time.Time  `json:"issued_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type MetricSample struct {
	ServerID      string    `json:"server_id"`
	Timestamp     time.Time `json:"timestamp"`
	CPULoadPct    float64   `json:"cpu_load_pct"`
	RAMUsedMB     uint64    `json:"ram_used_mb"`
	DiskUsedMB    uint64    `json:"disk_used_mb"`
	UptimeSeconds uint64    `json:"uptime_seconds"`
}

func NewDB(dbPath string) (*DB, error) {
	// Ensure folder exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode & foreign keys
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, err
	}

	d := &DB{db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return d, nil
}

func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS clusters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT
		);`,

		`CREATE TABLE IF NOT EXISTS servers (
			id TEXT PRIMARY KEY,
			hostname TEXT NOT NULL,
			custom_name TEXT,
			cluster_id TEXT REFERENCES clusters(id) ON DELETE SET NULL,
			os_version TEXT,
			cpu_model TEXT,
			cpu_cores INTEGER,
			cpu_threads INTEGER,
			ram_total_mb INTEGER,
			disk_total_mb INTEGER,
			agent_version TEXT,
			status TEXT NOT NULL, -- 'pending_approval', 'approved', 'rejected', 'removed'
			auth_token TEXT,
			first_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			approved_at TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS metric_samples (
			server_id TEXT REFERENCES servers(id) ON DELETE CASCADE,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			cpu_load_pct REAL,
			ram_used_mb INTEGER,
			disk_used_mb INTEGER,
			uptime_seconds INTEGER
		);`,

		`CREATE TABLE IF NOT EXISTS command_logs (
			id TEXT PRIMARY KEY,
			server_id TEXT REFERENCES servers(id) ON DELETE CASCADE,
			issued_by TEXT,
			command TEXT,
			result TEXT,
			issued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return fmt.Errorf("migration query failed: %w", err)
		}
	}

	return nil
}

// GenerateToken generates a cryptographically secure token.
func GenerateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (d *DB) GetServer(id string) (*Server, error) {
	query := `
		SELECT s.id, s.hostname, s.custom_name, s.cluster_id, c.name, s.os_version, 
		       s.cpu_model, s.cpu_cores, s.cpu_threads, s.ram_total_mb, s.disk_total_mb, 
		       s.agent_version, s.status, s.first_seen_at, s.last_seen_at, s.approved_at
		FROM servers s
		LEFT JOIN clusters c ON s.cluster_id = c.id
		WHERE s.id = ? AND s.status != 'removed'`

	row := d.QueryRow(query, id)
	var s Server
	var firstSeenStr, lastSeenStr string
	var approvedStr sql.NullString

	err := row.Scan(
		&s.ID, &s.Hostname, &s.CustomName, &s.ClusterID, &s.ClusterName, &s.OSVersion,
		&s.CPUModel, &s.CPUCores, &s.CPUThreads, &s.RAMTotalMB, &s.DiskTotalMB,
		&s.AgentVersion, &s.Status, &firstSeenStr, &lastSeenStr, &approvedStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	s.FirstSeenAt = parseDBTime(firstSeenStr)
	s.LastSeenAt = parseDBTime(lastSeenStr)

	if approvedStr.Valid {
		t := parseDBTime(approvedStr.String)
		s.ApprovedAt = &t
	}

	return &s, nil
}

func parseDBTime(str string) time.Time {
	if str == "" {
		return time.Time{}
	}
	formats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, str); err == nil {
			return t
		}
	}
	return time.Time{}
}


func (d *DB) GetServerAuthToken(id string) (string, error) {
	var token sql.NullString
	err := d.QueryRow("SELECT auth_token FROM servers WHERE id = ?", id).Scan(&token)
	if err != nil {
		return "", err
	}
	return token.String, nil
}

func (d *DB) RegisterOrUpdateServer(id string, hostname string, osVer string, cpuModel string, cores int, threads int, ram uint64, disk uint64, agentVer string) (status string, token string, isNew bool, err error) {
	// Check if server exists
	var existingStatus sql.NullString
	var existingToken sql.NullString

	err = d.QueryRow("SELECT status, auth_token FROM servers WHERE id = ?", id).Scan(&existingStatus, &existingToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Brand new device
			status = "pending_approval"
			_, err = d.Exec(`
				INSERT INTO servers (id, hostname, os_version, cpu_model, cpu_cores, cpu_threads, ram_total_mb, disk_total_mb, agent_version, status, first_seen_at, last_seen_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending_approval', ?, ?)`,
				id, hostname, osVer, cpuModel, cores, threads, ram, disk, agentVer, time.Now(), time.Now())
			return status, "", true, err
		}
		return "", "", false, err
	}

	// Update system specs and last_seen_at on connect, but keep status
	status = existingStatus.String
	token = existingToken.String

	// If removed, reset status to pending_approval for re-installation
	if status == "removed" || status == "rejected" {
		status = "pending_approval"
		token = ""
		_, _ = d.Exec("UPDATE servers SET status = 'pending_approval', auth_token = NULL WHERE id = ?", id)
	}


	_, err = d.Exec(`
		UPDATE servers 
		SET hostname = ?, os_version = ?, cpu_model = ?, cpu_cores = ?, cpu_threads = ?, ram_total_mb = ?, disk_total_mb = ?, agent_version = ?, last_seen_at = ?
		WHERE id = ?`,
		hostname, osVer, cpuModel, cores, threads, ram, disk, agentVer, time.Now(), id)

	return status, token, false, err
}

func (d *DB) ApproveServer(id string) (string, error) {
	token := GenerateToken()
	_, err := d.Exec(`
		UPDATE servers 
		SET status = 'approved', auth_token = ?, approved_at = ?
		WHERE id = ?`,
		token, time.Now(), id)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (d *DB) RejectServer(id string) error {
	_, err := d.Exec("UPDATE servers SET status = 'rejected', auth_token = NULL WHERE id = ?", id)
	return err
}

func (d *DB) DecommissionServer(id string) error {
	// Soft delete
	_, err := d.Exec("UPDATE servers SET status = 'removed', auth_token = NULL WHERE id = ?", id)
	return err
}

func (d *DB) UpdateServerLastSeen(id string) error {
	_, err := d.Exec("UPDATE servers SET last_seen_at = ? WHERE id = ?", time.Now(), id)
	return err
}

func (d *DB) UpdateServerCustomName(id string, name *string) error {
	_, err := d.Exec("UPDATE servers SET custom_name = ? WHERE id = ?", name, id)
	return err
}

func (d *DB) SetServerCluster(id string, clusterID *string) error {
	_, err := d.Exec("UPDATE servers SET cluster_id = ? WHERE id = ?", clusterID, id)
	return err
}

func (d *DB) GetServers() ([]*Server, error) {
	query := `
		SELECT s.id, s.hostname, s.custom_name, s.cluster_id, c.name, s.os_version, 
		       s.cpu_model, s.cpu_cores, s.cpu_threads, s.ram_total_mb, s.disk_total_mb, 
		       s.agent_version, s.status, s.first_seen_at, s.last_seen_at, s.approved_at
		FROM servers s
		LEFT JOIN clusters c ON s.cluster_id = c.id
		WHERE s.status != 'removed'
		ORDER BY s.hostname ASC`

	rows, err := d.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Server
	for rows.Next() {
		var s Server
		var firstSeenStr, lastSeenStr string
		var approvedStr sql.NullString

		errScan := rows.Scan(
			&s.ID, &s.Hostname, &s.CustomName, &s.ClusterID, &s.ClusterName, &s.OSVersion,
			&s.CPUModel, &s.CPUCores, &s.CPUThreads, &s.RAMTotalMB, &s.DiskTotalMB,
			&s.AgentVersion, &s.Status, &firstSeenStr, &lastSeenStr, &approvedStr,
		)
		if errScan != nil {
			return nil, errScan
		}

		s.FirstSeenAt = parseDBTime(firstSeenStr)
		s.LastSeenAt = parseDBTime(lastSeenStr)

		if approvedStr.Valid {
			t := parseDBTime(approvedStr.String)
			s.ApprovedAt = &t
		}


		list = append(list, &s)
	}

	return list, nil
}

func (d *DB) GetClusters() ([]*Cluster, error) {
	query := `
		SELECT c.id, c.name, c.description, COUNT(s.id) as server_count
		FROM clusters c
		LEFT JOIN servers s ON s.cluster_id = c.id AND s.status != 'removed'
		GROUP BY c.id
		ORDER BY c.name ASC`

	rows, err := d.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*Cluster
	for rows.Next() {
		var c Cluster
		var desc sql.NullString
		if errScan := rows.Scan(&c.ID, &c.Name, &desc, &c.ServerCount); errScan != nil {
			return nil, errScan
		}
		c.Description = desc.String
		list = append(list, &c)
	}
	return list, nil
}

func (d *DB) CreateCluster(id string, name string, description string) error {
	_, err := d.Exec("INSERT INTO clusters (id, name, description) VALUES (?, ?, ?)", id, name, description)
	return err
}

func (d *DB) UpdateCluster(id string, name string, description string) error {
	_, err := d.Exec("UPDATE clusters SET name = ?, description = ? WHERE id = ?", name, description, id)
	return err
}

func (d *DB) DeleteCluster(id string) error {
	_, err := d.Exec("DELETE FROM clusters WHERE id = ?", id)
	return err
}

func (d *DB) SaveMetricSample(serverID string, cpu float64, ram uint64, disk uint64, uptime uint64) error {
	_, err := d.Exec(`
		INSERT INTO metric_samples (server_id, timestamp, cpu_load_pct, ram_used_mb, disk_used_mb, uptime_seconds)
		VALUES (?, ?, ?, ?, ?, ?)`,
		serverID, time.Now(), cpu, ram, disk, uptime)
	return err
}

func (d *DB) PruneMetricSamples(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	res, err := d.Exec("DELETE FROM metric_samples WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (d *DB) LogCommandStart(id string, serverID string, issuedBy string, command string) error {
	_, err := d.Exec(`
		INSERT INTO command_logs (id, server_id, issued_by, command, issued_at)
		VALUES (?, ?, ?, ?, ?)`,
		id, serverID, issuedBy, command, time.Now())
	return err
}

func (d *DB) LogCommandComplete(id string, result string) error {
	_, err := d.Exec(`
		UPDATE command_logs
		SET result = ?, completed_at = ?
		WHERE id = ?`,
		result, time.Now(), id)
	return err
}

func (d *DB) GetCommandLogs(limit int) ([]*CommandLog, error) {
	query := `
		SELECT l.id, l.server_id, COALESCE(s.custom_name, s.hostname) as server_name, 
		       l.issued_by, l.command, l.result, l.issued_at, l.completed_at
		FROM command_logs l
		LEFT JOIN servers s ON l.server_id = s.id
		ORDER BY l.issued_at DESC
		LIMIT ?`

	rows, err := d.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*CommandLog
	for rows.Next() {
		var l CommandLog
		var issuedAtStr string
		var completedAtStr sql.NullString

		errScan := rows.Scan(
			&l.ID, &l.ServerID, &l.ServerName, &l.IssuedBy, &l.Command, &l.Result,
			&issuedAtStr, &completedAtStr,
		)
		if errScan != nil {
			return nil, errScan
		}

		l.IssuedAt, _ = time.Parse("2006-01-02 15:04:05", issuedAtStr)
		if l.IssuedAt.IsZero() {
			l.IssuedAt, _ = time.Parse(time.RFC3339, issuedAtStr)
		}

		if completedAtStr.Valid {
			t, errVal := time.Parse("2006-01-02 15:04:05", completedAtStr.String)
			if errVal != nil {
				t, _ = time.Parse(time.RFC3339, completedAtStr.String)
			}
			l.CompletedAt = &t
		}

		list = append(list, &l)
	}

	return list, nil
}

func (d *DB) GetRecentMetricSamples(serverID string, limit int) ([]*MetricSample, error) {
	query := `
		SELECT server_id, timestamp, cpu_load_pct, ram_used_mb, disk_used_mb, uptime_seconds
		FROM metric_samples
		WHERE server_id = ?
		ORDER BY timestamp DESC
		LIMIT ?`

	rows, err := d.Query(query, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*MetricSample
	for rows.Next() {
		var m MetricSample
		var tStr string
		if errScan := rows.Scan(&m.ServerID, &tStr, &m.CPULoadPct, &m.RAMUsedMB, &m.DiskUsedMB, &m.UptimeSeconds); errScan != nil {
			return nil, errScan
		}
		m.Timestamp, _ = time.Parse("2006-01-02 15:04:05", tStr)
		if m.Timestamp.IsZero() {
			m.Timestamp, _ = time.Parse(time.RFC3339, tStr)
		}
		list = append(list, &m)
	}
	return list, nil
}
