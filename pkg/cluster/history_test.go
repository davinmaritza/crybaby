package cluster

import (
	"testing"
)

func TestHistoryBuffer(t *testing.T) {
	buf := NewHistoryBuffer(3)

	if len(buf.GetPoints()) != 0 {
		t.Fatalf("expected 0 points, got %d", len(buf.GetPoints()))
	}

	buf.Add(10.5, 45.0)
	buf.Add(20.0, 50.0)

	points := buf.GetPoints()
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].CPU != 10.5 || points[0].RAMPct != 45.0 {
		t.Errorf("unexpected point 0: %+v", points[0])
	}
	if points[1].CPU != 20.0 || points[1].RAMPct != 50.0 {
		t.Errorf("unexpected point 1: %+v", points[1])
	}

	// Add 3rd point (at capacity)
	buf.Add(30.0, 55.0)
	if len(buf.GetPoints()) != 3 {
		t.Fatalf("expected 3 points, got %d", len(buf.GetPoints()))
	}

	// Add 4th point (exceeds capacity, should drop oldest point)
	buf.Add(40.0, 60.0)
	points = buf.GetPoints()
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	if points[0].CPU != 20.0 || points[2].CPU != 40.0 {
		t.Errorf("expected sliding window, got %+v", points)
	}
}
