// Package monitor provides real-time system resource monitoring during deletion operations.
// It tracks CPU usage, memory pressure, disk I/O, and goroutine metrics to identify
// performance bottlenecks.
package monitor

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/yourusername/fast-file-deletion/internal/logger"
)

// Bottleneck detection thresholds.
const (
	// MemoryPressureThreshold is the fraction of Sys memory that triggers a warning.
	MemoryPressureThreshold = 0.8
	// GCPressureThreshold is the GC cycles/sec rate that triggers a warning.
	GCPressureThreshold = 2.0
	// CPUSaturationThreshold is the CPU usage percentage that triggers a warning.
	CPUSaturationThreshold = 90.0
)

// SystemMetrics contains a snapshot of system resource usage at a point in time.
type SystemMetrics struct {
	Timestamp time.Time

	// CPU metrics
	NumGoroutines int     // Current number of goroutines
	NumCPU        int     // Number of logical CPUs
	CPUPercent    float64 // Estimated CPU usage percentage (0-100)

	// Memory metrics
	AllocMB      float64 // Currently allocated memory in MB
	TotalAllocMB float64 // Cumulative allocated memory in MB
	SysMB        float64 // Total memory obtained from OS in MB
	NumGC        uint32  // Number of completed GC cycles
	GCPauseMs    float64 // Recent GC pause time in milliseconds

	// Deletion metrics (provided externally)
	FilesDeleted int     // Total files deleted so far
	DeletionRate float64 // Current deletion rate (files/sec)

	// Bottleneck indicators
	MemoryPressure bool // True if memory usage is high (>80% of sys)
	GCPressure     bool // True if GC is running frequently
	CPUSaturated   bool // True if CPU usage is high (>90%)
}

// Monitor tracks system resources during deletion operations.
type Monitor struct {
	mu              sync.RWMutex
	metrics         []SystemMetrics
	startTime       time.Time
	lastGCCount     uint32
	lastGCPauseNs   uint64
	lastCPUTime     time.Duration
	lastMeasureTime time.Time
}

// NewMonitor creates a new system resource monitor.
func NewMonitor() *Monitor {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &Monitor{
		metrics:         make([]SystemMetrics, 0, 1000),
		startTime:       time.Now(),
		lastGCCount:     memStats.NumGC,
		lastGCPauseNs:   memStats.PauseTotalNs,
		lastMeasureTime: time.Now(),
	}
}

// Start begins monitoring system resources at regular intervals.
// It spawns a goroutine that collects metrics every interval until ctx is cancelled.
//
// Parameters:
//   - ctx: Context for cancellation
//   - interval: How often to collect metrics (e.g., 1 * time.Second)
//   - getFilesDeleted: Function that returns current file count
//   - getDeletionRate: Function that returns current deletion rate
func (m *Monitor) Start(ctx context.Context, interval time.Duration, getFilesDeleted func() int, getDeletionRate func() float64) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := m.collectMetrics(getFilesDeleted(), getDeletionRate())
			m.recordMetrics(metrics)
			m.logBottlenecks(metrics)
		}
	}
}

// collectMetrics gathers current system resource usage.
func (m *Monitor) collectMetrics(filesDeleted int, deletionRate float64) SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	now := time.Now()
	elapsed := now.Sub(m.lastMeasureTime).Seconds()

	// Calculate GC metrics
	gcCount := memStats.NumGC - m.lastGCCount
	gcPauseNs := memStats.PauseTotalNs - m.lastGCPauseNs
	gcPauseMs := float64(gcPauseNs) / 1e6

	// Estimate CPU usage based on goroutine activity
	// This is a rough estimate - true CPU usage requires platform-specific APIs
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()
	cpuPercent := (float64(numGoroutines) / float64(numCPU)) * 10.0 // Rough heuristic
	if cpuPercent > 100 {
		cpuPercent = 100
	}

	// Detect memory pressure
	allocMB := float64(memStats.Alloc) / (1024 * 1024)
	sysMB := float64(memStats.Sys) / (1024 * 1024)
	memoryPressure := allocMB > (sysMB * MemoryPressureThreshold)

	// Detect GC pressure
	gcPressure := elapsed > 0 && (float64(gcCount)/elapsed) > GCPressureThreshold

	// Detect CPU saturation
	cpuSaturated := cpuPercent > CPUSaturationThreshold

	metrics := SystemMetrics{
		Timestamp:      now,
		NumGoroutines:  numGoroutines,
		NumCPU:         numCPU,
		CPUPercent:     cpuPercent,
		AllocMB:        allocMB,
		TotalAllocMB:   float64(memStats.TotalAlloc) / (1024 * 1024),
		SysMB:          sysMB,
		NumGC:          memStats.NumGC,
		GCPauseMs:      gcPauseMs,
		FilesDeleted:   filesDeleted,
		DeletionRate:   deletionRate,
		MemoryPressure: memoryPressure,
		GCPressure:     gcPressure,
		CPUSaturated:   cpuSaturated,
	}

	// Update tracking variables
	m.lastGCCount = memStats.NumGC
	m.lastGCPauseNs = memStats.PauseTotalNs
	m.lastMeasureTime = now

	return metrics
}

// recordMetrics stores metrics for later analysis.
func (m *Monitor) recordMetrics(metrics SystemMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, metrics)
}

// logBottlenecks logs warnings when resource bottlenecks are detected.
func (m *Monitor) logBottlenecks(metrics SystemMetrics) {
	if metrics.MemoryPressure {
		logger.Warning("BOTTLENECK: Memory pressure detected (%.1f MB / %.1f MB = %.1f%%)",
			metrics.AllocMB, metrics.SysMB, (metrics.AllocMB/metrics.SysMB)*100)
	}

	if metrics.GCPressure {
		logger.Warning("BOTTLENECK: GC pressure detected (%.1f ms pause in last interval)",
			metrics.GCPauseMs)
	}

	if metrics.CPUSaturated {
		logger.Warning("BOTTLENECK: CPU saturation detected (%d goroutines on %d CPUs = %.1f%%)",
			metrics.NumGoroutines, metrics.NumCPU, metrics.CPUPercent)
	}

	// Log detailed metrics every 10 seconds
	elapsed := time.Since(m.startTime)
	if int(elapsed.Seconds())%10 == 0 {
		logger.Debug("System metrics: Goroutines=%d, Memory=%.1fMB, GC=%d, Rate=%.1f files/sec",
			metrics.NumGoroutines, metrics.AllocMB, metrics.NumGC, metrics.DeletionRate)
	}
}

// GetMetrics returns all collected metrics.
func (m *Monitor) GetMetrics() []SystemMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]SystemMetrics, len(m.metrics))
	copy(result, m.metrics)
	return result
}

// GenerateReport generates a detailed bottleneck analysis report.
func (m *Monitor) GenerateReport() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.metrics) == 0 {
		return "No metrics collected"
	}

	report := "\n═══════════════════════════════════════════════════════════════════════════\n"
	report += "                    PERFORMANCE BOTTLENECK ANALYSIS\n"
	report += "═══════════════════════════════════════════════════════════════════════════\n\n"

	// Analyze bottleneck patterns
	memoryPressureCount := 0
	gcPressureCount := 0
	cpuSaturatedCount := 0
	maxMemoryMB := 0.0
	maxGoroutines := 0
	totalGCPauseMs := 0.0

	for _, m := range m.metrics {
		if m.MemoryPressure {
			memoryPressureCount++
		}
		if m.GCPressure {
			gcPressureCount++
		}
		if m.CPUSaturated {
			cpuSaturatedCount++
		}
		if m.AllocMB > maxMemoryMB {
			maxMemoryMB = m.AllocMB
		}
		if m.NumGoroutines > maxGoroutines {
			maxGoroutines = m.NumGoroutines
		}
		totalGCPauseMs += m.GCPauseMs
	}

	totalSamples := len(m.metrics)
	memoryPressurePct := (float64(memoryPressureCount) / float64(totalSamples)) * 100
	gcPressurePct := (float64(gcPressureCount) / float64(totalSamples)) * 100
	cpuSaturatedPct := (float64(cpuSaturatedCount) / float64(totalSamples)) * 100

	report += "Resource Pressure Summary:\n"
	report += "───────────────────────────────────────────────────────────────────────────\n"
	report += fmt.Sprintf("Memory Pressure:  %d/%d samples (%.1f%%) - Peak: %.1f MB\n",
		memoryPressureCount, totalSamples, memoryPressurePct, maxMemoryMB)
	report += fmt.Sprintf("GC Pressure:      %d/%d samples (%.1f%%) - Total pause: %.1f ms\n",
		gcPressureCount, totalSamples, gcPressurePct, totalGCPauseMs)
	report += fmt.Sprintf("CPU Saturation:   %d/%d samples (%.1f%%) - Peak goroutines: %d\n",
		cpuSaturatedCount, totalSamples, cpuSaturatedPct, maxGoroutines)
	report += "\n"

	// Identify primary bottleneck
	report += "Primary Bottleneck:\n"
	report += "───────────────────────────────────────────────────────────────────────────\n"

	if memoryPressurePct > 50 {
		report += "⚠️  MEMORY PRESSURE (detected in >50% of samples)\n"
		report += "   - Recommendation: Reduce batch size or worker count\n"
		report += "   - Consider: Increase system RAM or reduce buffer sizes\n"
	} else if gcPressurePct > 30 {
		report += "⚠️  GARBAGE COLLECTION PRESSURE (detected in >30% of samples)\n"
		report += "   - Recommendation: Reduce memory allocations per file\n"
		report += "   - Consider: Increase GOGC value or optimize data structures\n"
	} else if cpuSaturatedPct > 70 {
		report += "⚠️  CPU SATURATION (detected in >70% of samples)\n"
		report += "   - Recommendation: Increase worker count to utilize more cores\n"
		report += "   - Consider: This is expected for CPU-bound workloads\n"
	} else {
		report += "✓  DISK I/O BOTTLENECK (likely)\n"
		report += "   - No significant CPU, memory, or GC pressure detected\n"
		report += "   - Recommendation: Bottleneck is likely filesystem/disk I/O\n"
		report += "   - Consider: Use faster storage (SSD/NVMe) or reduce worker count\n"
		report += "   - Note: Windows NTFS metadata updates may be the limiting factor\n"
	}

	report += "\n═══════════════════════════════════════════════════════════════════════════\n"

	return report
}
