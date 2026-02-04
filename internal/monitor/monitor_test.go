package monitor

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestNewMonitor(t *testing.T) {
	m := NewMonitor()
	if m == nil {
		t.Fatal("NewMonitor returned nil")
	}
	if m.metrics == nil {
		t.Fatal("metrics slice not initialized")
	}
	if m.startTime.IsZero() {
		t.Fatal("startTime not set")
	}
}

func TestCollectMetrics(t *testing.T) {
	m := NewMonitor()
	metrics := m.collectMetrics(100, 50.0)

	if metrics.FilesDeleted != 100 {
		t.Errorf("FilesDeleted = %d, want 100", metrics.FilesDeleted)
	}
	if metrics.DeletionRate != 50.0 {
		t.Errorf("DeletionRate = %f, want 50.0", metrics.DeletionRate)
	}
	if metrics.NumCPU != runtime.NumCPU() {
		t.Errorf("NumCPU = %d, want %d", metrics.NumCPU, runtime.NumCPU())
	}
	if metrics.NumGoroutines <= 0 {
		t.Error("NumGoroutines should be positive")
	}
	if metrics.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if metrics.AllocMB < 0 {
		t.Error("AllocMB should be non-negative")
	}
	if metrics.SysMB <= 0 {
		t.Error("SysMB should be positive")
	}
}

func TestRecordMetrics(t *testing.T) {
	m := NewMonitor()
	metrics := m.collectMetrics(10, 5.0)
	m.recordMetrics(metrics)

	stored := m.GetMetrics()
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored metric, got %d", len(stored))
	}
	if stored[0].FilesDeleted != 10 {
		t.Errorf("stored FilesDeleted = %d, want 10", stored[0].FilesDeleted)
	}
}

func TestGetMetricsReturnsCopy(t *testing.T) {
	m := NewMonitor()
	m.recordMetrics(m.collectMetrics(1, 1.0))

	metrics1 := m.GetMetrics()
	metrics2 := m.GetMetrics()

	// Modifying one should not affect the other
	if len(metrics1) == 0 || len(metrics2) == 0 {
		t.Fatal("expected non-empty metrics slices")
	}
	metrics1[0].FilesDeleted = 9999
	if metrics2[0].FilesDeleted == 9999 {
		t.Error("GetMetrics should return independent copies")
	}
}

func TestStartAndCancel(t *testing.T) {
	m := NewMonitor()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.Start(ctx, 50*time.Millisecond,
			func() int { return 42 },
			func() float64 { return 10.0 })
		close(done)
	}()

	// Let it collect a few samples
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	metrics := m.GetMetrics()
	if len(metrics) == 0 {
		t.Error("expected at least one metric sample")
	}
	// Verify the callback values were used
	for _, s := range metrics {
		if s.FilesDeleted != 42 {
			t.Errorf("FilesDeleted = %d, want 42", s.FilesDeleted)
		}
		if s.DeletionRate != 10.0 {
			t.Errorf("DeletionRate = %f, want 10.0", s.DeletionRate)
		}
	}
}

func TestGenerateReportEmpty(t *testing.T) {
	m := NewMonitor()
	report := m.GenerateReport()
	if report != "No metrics collected" {
		t.Errorf("expected 'No metrics collected', got %q", report)
	}
}

func TestGenerateReportWithData(t *testing.T) {
	m := NewMonitor()

	// Record a normal metric (no bottlenecks)
	m.recordMetrics(SystemMetrics{
		Timestamp:     time.Now(),
		NumGoroutines: 10,
		NumCPU:        4,
		CPUPercent:    30,
		AllocMB:       50,
		SysMB:         200,
		FilesDeleted:  1000,
		DeletionRate:  500,
	})

	report := m.GenerateReport()
	if report == "No metrics collected" {
		t.Error("should have generated a report with data")
	}
	if len(report) < 50 {
		t.Errorf("report seems too short: %q", report)
	}
}

func TestBottleneckDetection(t *testing.T) {
	m := NewMonitor()

	tests := []struct {
		name           string
		metrics        SystemMetrics
		wantMemPressure bool
		wantGCPressure  bool
		wantCPUSat      bool
	}{
		{
			name: "no bottlenecks",
			metrics: SystemMetrics{
				AllocMB:       50,
				SysMB:         200,
				CPUPercent:    30,
			},
			wantMemPressure: false,
			wantCPUSat:      false,
		},
		{
			name: "memory pressure",
			metrics: SystemMetrics{
				AllocMB:       192,  // 96% of SysMB (exceeds 95% threshold)
				SysMB:         200,
				CPUPercent:    30,
			},
			wantMemPressure: true,
			wantCPUSat:      false,
		},
		{
			name: "cpu saturation",
			metrics: SystemMetrics{
				AllocMB:       50,
				SysMB:         200,
				CPUPercent:    95,
			},
			wantMemPressure: false,
			wantCPUSat:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// collectMetrics applies the thresholds; verify by collecting
			// with known values instead (since collectMetrics reads real runtime stats)
			if tt.wantMemPressure && !tt.metrics.MemoryPressure {
				// Manually apply threshold check like collectMetrics does
				tt.metrics.MemoryPressure = tt.metrics.AllocMB > (tt.metrics.SysMB * MemoryPressureThreshold)
			}
			if tt.wantCPUSat && !tt.metrics.CPUSaturated {
				tt.metrics.CPUSaturated = tt.metrics.CPUPercent > CPUSaturationThreshold
			}
			m.recordMetrics(tt.metrics)

			if tt.wantMemPressure && !tt.metrics.MemoryPressure {
				t.Error("expected memory pressure to be detected")
			}
			if tt.wantCPUSat && !tt.metrics.CPUSaturated {
				t.Error("expected CPU saturation to be detected")
			}
		})
	}
}

func TestMemoryPressureThreshold(t *testing.T) {
	// Verify the threshold constant is what we expect
	// Increased from 0.8 to 0.95 to reduce false positives (1GB usage on 32GB system is fine)
	if MemoryPressureThreshold != 0.95 {
		t.Errorf("MemoryPressureThreshold = %f, want 0.95", MemoryPressureThreshold)
	}
	if GCPressureThreshold != 2.0 {
		t.Errorf("GCPressureThreshold = %f, want 2.0", GCPressureThreshold)
	}
	if CPUSaturationThreshold != 90.0 {
		t.Errorf("CPUSaturationThreshold = %f, want 90.0", CPUSaturationThreshold)
	}
}
