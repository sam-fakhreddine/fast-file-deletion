package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/yourusername/fast-file-deletion/internal/backend"
	"github.com/yourusername/fast-file-deletion/internal/engine"
	"github.com/yourusername/fast-file-deletion/internal/logger"
	"github.com/yourusername/fast-file-deletion/internal/monitor"
	"github.com/yourusername/fast-file-deletion/internal/progress"
	"github.com/yourusername/fast-file-deletion/internal/safety"
	"github.com/yourusername/fast-file-deletion/internal/scanner"
)

// App is the main application service exposed to the frontend
type App struct {
	ctx                context.Context
	app                *application.App
	engine             *engine.Engine
	monitor            interface{}
	cancelFunc         context.CancelFunc
	backendInst        backend.Backend
	reporter           *progress.Reporter
	lastScanResult     *scanner.ScanResult
	deletionInProgress atomic.Bool // Prevents concurrent deletions
	mu                 sync.Mutex
}

// Config holds the configuration for deletion operations
type Config struct {
	TargetDir      string  `json:"targetDir"`
	Force          bool    `json:"force"`
	DryRun         bool    `json:"dryRun"`
	Verbose        bool    `json:"verbose"`
	LogFile        string  `json:"logFile"`
	KeepDays       *int    `json:"keepDays"`
	Workers        int     `json:"workers"`
	BufferSize     int     `json:"bufferSize"`
	DeletionMethod string  `json:"deletionMethod"`
	Benchmark      bool    `json:"benchmark"`
	Monitor        bool    `json:"monitor"`
}

// ValidationResult holds the result of path validation
type ValidationResult struct {
	IsValid bool   `json:"isValid"`
	Reason  string `json:"reason"`
}

// ScanResult holds the result of directory scanning
type ScanResult struct {
	TotalScanned  int   `json:"totalScanned"`
	TotalToDelete int   `json:"totalToDelete"`
	TotalRetained int   `json:"totalRetained"`
	TotalSizeBytes int64 `json:"totalSizeBytes"`
}

// LiveMetrics holds real-time deletion metrics
type LiveMetrics struct {
	FilesDeleted   int     `json:"filesDeleted"`
	DeletionRate   float64 `json:"deletionRate"`
	SystemMetrics  *SystemMetrics `json:"systemMetrics,omitempty"`
	ElapsedSeconds float64 `json:"elapsedSeconds"`
}

// SystemMetrics holds system resource usage data
type SystemMetrics struct {
	CPUPercent      float64 `json:"cpuPercent"`
	MemoryMB        float64 `json:"memoryMb"`
	IOOpsPerSec     float64 `json:"ioOpsPerSec"`
	GoroutineCount  int     `json:"goroutineCount"`
	MemoryPressure  bool    `json:"memoryPressure"`
	GCPressure      bool    `json:"gcPressure"`
	CPUSaturated    bool    `json:"cpuSaturated"`
}

// DeletionResult holds the final deletion results
type DeletionResult struct {
	DeletedCount   int     `json:"deletedCount"`
	FailedCount    int     `json:"failedCount"`
	RetainedCount  int     `json:"retainedCount"`
	DurationMs     int64   `json:"durationMs"`
	AverageRate    float64 `json:"averageRate"`
	PeakRate       float64 `json:"peakRate"`
	MethodStats    *MethodStats `json:"methodStats,omitempty"`
	BottleneckReport string `json:"bottleneckReport,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

// MethodStats holds statistics about deletion methods used
type MethodStats struct {
	FileInfoCount       int `json:"fileInfoCount"`
	DeleteOnCloseCount  int `json:"deleteOnCloseCount"`
	NtAPICount          int `json:"ntApiCount"`
	FallbackCount       int `json:"fallbackCount"`
}

const (
	// MaxScanResults limits the number of files that can be scanned
	// to prevent memory exhaustion attacks
	MaxScanResults = 10_000_000 // 10 million files
)

var (
	startTime time.Time
)

// NewApp creates a new App instance
func NewApp() *App {
	return &App{}
}

// Name returns the service name
func (a *App) Name() string {
	return "ffd"
}

// OnStartup is called when the app is starting up
func (a *App) OnStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx

	// Initialize memory limit same as CLI
	initializeMemoryLimit()

	return nil
}

// OnShutdown is called when the app is shutting down
// Security: Edge case handling for window close during deletion
func (a *App) OnShutdown() error {
	a.mu.Lock()
	cancel := a.cancelFunc
	a.mu.Unlock()

	if cancel != nil {
		cancel() // Cancel ongoing deletion
		// Give goroutine time to clean up
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// SetApp sets the application instance
func (a *App) SetApp(app *application.App) {
	a.app = app
}

// ValidatePath validates if a path is safe to delete
func (a *App) ValidatePath(path string) ValidationResult {
	if path == "" {
		return ValidationResult{
			IsValid: false,
			Reason:  "Path cannot be empty",
		}
	}

	isSafe, reason := safety.IsSafePath(path)
	return ValidationResult{
		IsValid: isSafe,
		Reason:  reason,
	}
}

// ScanDirectory scans a directory and returns file counts
func (a *App) ScanDirectory(config Config) (ScanResult, error) {
	// Validate configuration
	if err := a.validateConfig(config); err != nil {
		return ScanResult{}, err
	}

	// Setup logging
	if err := logger.SetupLogging(config.Verbose, config.LogFile); err != nil {
		return ScanResult{}, fmt.Errorf("failed to setup logging: %w", err)
	}

	// Validate path safety
	isSafe, reason := safety.IsSafePath(config.TargetDir)
	if !isSafe {
		return ScanResult{}, fmt.Errorf("path is not safe: %s", reason)
	}

	// Scan directory
	s := scanner.NewScanner(config.TargetDir, config.KeepDays)
	scanResult, err := s.Scan()
	if err != nil {
		return ScanResult{}, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Security: Enforce scan size limit to prevent memory exhaustion
	if scanResult.TotalScanned > MaxScanResults {
		return ScanResult{}, fmt.Errorf("directory too large: %d files (maximum: %d)",
			scanResult.TotalScanned, MaxScanResults)
	}

	// Store scan result for later use
	a.mu.Lock()
	a.lastScanResult = scanResult
	a.mu.Unlock()

	return ScanResult{
		TotalScanned:   scanResult.TotalScanned,
		TotalToDelete:  scanResult.TotalToDelete,
		TotalRetained:  scanResult.TotalRetained,
		TotalSizeBytes: scanResult.TotalSizeBytes,
	}, nil
}

// StartDeletion begins the deletion process
func (a *App) StartDeletion(config Config) error {
	// Security Fix #2: Atomic check-and-set to prevent concurrent deletions
	if !a.deletionInProgress.CompareAndSwap(false, true) {
		return fmt.Errorf("deletion already in progress")
	}
	// Note: deletionInProgress will be cleared in the deferred cleanup handler

	// Security Fix #2: Consume scan result (single-use pattern)
	a.mu.Lock()
	scanResult := a.lastScanResult
	a.lastScanResult = nil // Consume the scan result
	a.mu.Unlock()

	if scanResult == nil {
		a.deletionInProgress.Store(false) // Clear lock before error return
		return fmt.Errorf("no scan result available - please scan directory first")
	}

	if scanResult.TotalToDelete == 0 {
		a.deletionInProgress.Store(false) // Clear lock before error return
		return fmt.Errorf("no files to delete")
	}

	// Security Fix #1: Verify path matches scan (TOCTOU protection)
	absPath, err := filepath.Abs(config.TargetDir)
	if err != nil {
		a.deletionInProgress.Store(false)
		return fmt.Errorf("cannot get absolute path: %w", err)
	}
	if scanResult.ScannedPath != absPath {
		a.deletionInProgress.Store(false)
		return fmt.Errorf("path mismatch: scanned %s, deletion requested for %s",
			scanResult.ScannedPath, absPath)
	}

	// Security Fix #4: Re-validate path safety (defense in depth)
	isSafe, reason := safety.IsSafePath(config.TargetDir)
	if !isSafe {
		a.deletionInProgress.Store(false)
		return fmt.Errorf("path no longer safe: %s", reason)
	}

	// Initialize engine and backend
	workerCount := config.Workers
	if workerCount == 0 {
		workerCount = runtime.NumCPU() * engine.DefaultWorkerMultiplier
	}

	bufferSize := config.BufferSize
	if bufferSize == 0 {
		bufferSize = min(scanResult.TotalToDelete, 10000)
	}

	backendInstance := backend.NewBackend()

	// Set deletion method if specified
	if config.DeletionMethod != "auto" {
		if advBackend, ok := backendInstance.(backend.AdvancedBackend); ok {
			var method backend.DeletionMethod
			switch config.DeletionMethod {
			case "fileinfo":
				method = backend.MethodFileInfo
			case "deleteonclose":
				method = backend.MethodDeleteOnClose
			case "ntapi":
				method = backend.MethodNtAPI
			case "deleteapi":
				method = backend.MethodDeleteAPI
			}
			advBackend.SetDeletionMethod(method)
		}
	}

	reporter := progress.NewReporter(scanResult.TotalToDelete, scanResult.TotalSizeBytes)

	var eng *engine.Engine
	eng = engine.NewEngineWithBufferSize(backendInstance, config.Workers, config.BufferSize, func(deletedCount int) {
		reporter.Update(deletedCount)

		// Emit progress event (throttled by frontend)
		if a.app != nil {
			a.app.Event.Emit("progress:update", LiveMetrics{
				FilesDeleted:   eng.FilesDeleted(),
				DeletionRate:   eng.DeletionRate(),
				ElapsedSeconds: time.Since(startTime).Seconds(),
			})
		}
	})

	a.mu.Lock()
	a.engine = eng
	a.backendInst = backendInstance
	a.reporter = reporter
	a.mu.Unlock()

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancelFunc = cancel
	a.mu.Unlock()

	// Start monitor if enabled
	if config.Monitor {
		getFilesDeleted := func() int { return eng.FilesDeleted() }
		getDeletionRate := func() float64 { return eng.DeletionRate() }

		if runtime.GOOS == "windows" {
			winMon := monitor.NewWindowsMonitor()
			a.mu.Lock()
			a.monitor = winMon
			a.mu.Unlock()
			go winMon.Start(ctx, 500*time.Millisecond, getFilesDeleted, getDeletionRate)
		} else {
			genMon := monitor.NewMonitor()
			a.mu.Lock()
			a.monitor = genMon
			a.mu.Unlock()
			go genMon.Start(ctx, 500*time.Millisecond, getFilesDeleted, getDeletionRate)
		}
	}

	// Start deletion in goroutine
	go func() {
		// Security Fix #3: Deferred cleanup handler (resource leak protection)
		defer func() {
			cancel() // Cancel context to stop monitor

			a.mu.Lock()
			a.cancelFunc = nil
			a.monitor = nil
			a.engine = nil
			a.reporter = nil
			a.backendInst = nil
			a.mu.Unlock()

			a.deletionInProgress.Store(false) // Allow new deletions
		}()

		startTime = time.Now()

		result, err := eng.DeleteWithUTF16(ctx, scanResult.Files, scanResult.FilesUTF16, scanResult.IsDirectory, config.DryRun)

		duration := time.Since(startTime)

		if err != nil && err != context.Canceled {
			a.app.Event.Emit("deletion:error", map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Build final result
		finalResult := DeletionResult{
			DeletedCount:  result.DeletedCount,
			FailedCount:   result.FailedCount,
			RetainedCount: scanResult.TotalRetained,
			DurationMs:    duration.Milliseconds(),
		}

		// Calculate rates
		if duration.Seconds() > 0 {
			finalResult.AverageRate = float64(result.DeletedCount) / duration.Seconds()
		}
		finalResult.PeakRate = result.PeakRate

		// Get method stats if available
		if advBackend, ok := backendInstance.(backend.AdvancedBackend); ok {
			stats := advBackend.GetDeletionStats()
			finalResult.MethodStats = &MethodStats{
				FileInfoCount:      stats.FileInfoSuccesses,
				DeleteOnCloseCount: stats.DeleteOnCloseSuccesses,
				NtAPICount:         stats.NtAPISuccesses,
				FallbackCount:      stats.FallbackSuccesses,
			}
		}

		// Get monitor report if available
		a.mu.Lock()
		mon := a.monitor
		a.mu.Unlock()

		if mon != nil {
			if winMon, ok := mon.(*monitor.WindowsMonitor); ok {
				finalResult.BottleneckReport = winMon.GenerateReport()
			} else if genMon, ok := mon.(*monitor.Monitor); ok {
				finalResult.BottleneckReport = genMon.GenerateReport()
			}
		}

		// Emit completion event
		a.app.Event.Emit("deletion:complete", finalResult)
	}()

	return nil
}

// CancelDeletion cancels an ongoing deletion
func (a *App) CancelDeletion() error {
	a.mu.Lock()
	cancel := a.cancelFunc
	a.mu.Unlock()

	if cancel == nil {
		return fmt.Errorf("no deletion in progress")
	}

	cancel()
	return nil
}

// GetLiveMetrics returns current deletion metrics
func (a *App) GetLiveMetrics() LiveMetrics {
	a.mu.Lock()
	eng := a.engine
	mon := a.monitor
	a.mu.Unlock()

	if eng == nil {
		return LiveMetrics{}
	}

	metrics := LiveMetrics{
		FilesDeleted:   eng.FilesDeleted(),
		DeletionRate:   eng.DeletionRate(),
		ElapsedSeconds: time.Since(startTime).Seconds(),
	}

	// Add system metrics if monitoring is enabled
	if mon != nil {
		var sysMetricsSlice []monitor.SystemMetrics
		if winMon, ok := mon.(*monitor.WindowsMonitor); ok {
			sysMetricsSlice = winMon.GetMetrics()
		} else if genMon, ok := mon.(*monitor.Monitor); ok {
			sysMetricsSlice = genMon.GetMetrics()
		}

		if len(sysMetricsSlice) > 0 {
			latest := sysMetricsSlice[len(sysMetricsSlice)-1]
			metrics.SystemMetrics = &SystemMetrics{
				CPUPercent:     latest.CPUPercent,
				MemoryMB:       latest.AllocMB,
				IOOpsPerSec:    latest.DeletionRate, // Use deletion rate as proxy for I/O
				GoroutineCount: latest.NumGoroutines,
				MemoryPressure: latest.MemoryPressure,
				GCPressure:     latest.GCPressure,
				CPUSaturated:   latest.CPUSaturated,
			}
		}
	}

	return metrics
}

// validateConfig validates the configuration
func (a *App) validateConfig(config Config) error {
	if config.TargetDir == "" {
		return fmt.Errorf("target directory is required")
	}

	if config.Workers < 0 {
		return fmt.Errorf("workers must be >= 0")
	}

	if config.Workers > 1000 {
		return fmt.Errorf("workers must be <= 1000")
	}

	if config.BufferSize < 0 {
		return fmt.Errorf("buffer size must be >= 0")
	}

	if config.BufferSize > 100000 {
		return fmt.Errorf("buffer size must be <= 100000")
	}

	validMethods := map[string]bool{
		"auto":          true,
		"fileinfo":      true,
		"deleteonclose": true,
		"ntapi":         true,
		"deleteapi":     true,
	}
	if !validMethods[config.DeletionMethod] {
		return fmt.Errorf("invalid deletion method: %s", config.DeletionMethod)
	}

	if config.Benchmark && config.DryRun {
		return fmt.Errorf("benchmark and dry-run cannot be used together")
	}

	if config.Benchmark && config.KeepDays != nil {
		return fmt.Errorf("benchmark and keep-days cannot be used together")
	}

	if config.Benchmark && runtime.GOOS != "windows" {
		return fmt.Errorf("benchmark mode is only available on Windows")
	}

	return nil
}

// initializeMemoryLimit sets Go's memory limit to 25% of system RAM
func initializeMemoryLimit() {
	// Same logic as CLI
	if os.Getenv("GOMEMLIMIT") != "" {
		return
	}

	totalRAM := getTotalSystemMemory()
	if totalRAM <= 0 {
		return
	}

	percentLimit := int64(float64(totalRAM) * 0.25)
	const maxLimit = 6 * 1024 * 1024 * 1024 // 6GB
	memLimit := percentLimit
	if memLimit > maxLimit {
		memLimit = maxLimit
	}

	const minLimit = 512 * 1024 * 1024 // 512MB
	if memLimit < minLimit {
		memLimit = minLimit
	}

	debug.SetMemoryLimit(memLimit)
}
