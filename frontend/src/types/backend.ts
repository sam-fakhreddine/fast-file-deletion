// TypeScript interfaces matching Go backend structs
// Generated from cmd/ffd-gui/app.go

export interface Config {
  targetDir: string;
  force: boolean;
  dryRun: boolean;
  verbose: boolean;
  logFile: string;
  keepDays: number | null;
  workers: number;
  bufferSize: number;
  deletionMethod: 'auto' | 'fileinfo' | 'deleteonclose' | 'ntapi' | 'deleteapi';
  benchmark: boolean;
  monitor: boolean;
}

export interface ValidationResult {
  isValid: boolean;
  reason: string;
}

export interface ScanResult {
  totalScanned: number;
  totalToDelete: number;
  totalRetained: number;
  totalSizeBytes: number;
}

export interface LiveMetrics {
  filesDeleted: number;
  deletionRate: number;
  elapsedSeconds: number;
  systemMetrics?: SystemMetrics;
}

export interface SystemMetrics {
  cpuPercent: number;
  memoryMb: number;
  ioOpsPerSec: number;
  goroutineCount: number;
  memoryPressure: boolean;
  gcPressure: boolean;
  cpuSaturated: boolean;
}

export interface DeletionResult {
  deletedCount: number;
  failedCount: number;
  retainedCount: number;
  durationMs: number;
  averageRate: number;
  peakRate: number;
  methodStats?: MethodStats;
  bottleneckReport?: string;
  errors?: string[];
}

export interface MethodStats {
  fileInfoCount: number;
  deleteOnCloseCount: number;
  ntApiCount: number;
  fallbackCount: number;
}

// State machine types
export type AppStage =
  | 'config'
  | 'scanning'
  | 'confirm'
  | 'progress'
  | 'results'
  | 'error';

export interface AppState {
  stage: AppStage;
  config: Config;
  scanResult?: ScanResult;
  liveMetrics?: LiveMetrics;
  deletionResult?: DeletionResult;
  error?: string;
}
