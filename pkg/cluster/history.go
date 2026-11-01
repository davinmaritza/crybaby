package cluster

import (
	"sync"
	"time"

	"github.com/davinmaritza/crybaby/pkg/types"
)

// HistoryBuffer stores a rolling window of cluster telemetry data points.
type HistoryBuffer struct {
	mu       sync.RWMutex
	capacity int
	points   []types.TelemetryPoint
}

// NewHistoryBuffer creates a buffer with the specified max capacity.
func NewHistoryBuffer(capacity int) *HistoryBuffer {
	if capacity <= 0 {
		capacity = 30
	}
	return &HistoryBuffer{
		capacity: capacity,
		points:   make([]types.TelemetryPoint, 0, capacity),
	}
}

// Add appends a new point, keeping the slice within capacity.
func (h *HistoryBuffer) Add(cpu, ramPct float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	point := types.TelemetryPoint{
		Timestamp: time.Now().Unix(),
		CPU:       cpu,
		RAMPct:    ramPct,
	}

	if len(h.points) >= h.capacity {
		h.points = append(h.points[1:], point)
	} else {
		h.points = append(h.points, point)
	}
}

// GetPoints returns a copy of the current history points.
func (h *HistoryBuffer) GetPoints() []types.TelemetryPoint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	res := make([]types.TelemetryPoint, len(h.points))
	copy(res, h.points)
	return res
}
