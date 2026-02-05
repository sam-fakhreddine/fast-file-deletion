import { Config } from '../types/backend';

export const defaultConfig: Config = {
  targetDir: '',
  force: false,
  dryRun: false,
  verbose: false,
  logFile: '',
  keepDays: null,
  workers: 0, // Auto-detect
  bufferSize: 0, // Auto-detect
  deletionMethod: 'auto',
  benchmark: false,
  monitor: false,
};

export function formatNumber(num: number): string {
  return num.toLocaleString();
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}

export function formatDuration(seconds: number): string {
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  if (minutes < 60) {
    return `${minutes}m ${secs}s`;
  }
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return `${hours}h ${mins}m ${secs}s`;
}

export function formatRate(rate: number): string {
  if (rate < 1000) {
    return `${rate.toFixed(0)} files/sec`;
  }
  return `${(rate / 1000).toFixed(1)}K files/sec`;
}

export function calculateETA(remaining: number, rate: number): string {
  if (rate === 0) return 'calculating...';
  const seconds = remaining / rate;
  if (seconds < 0 || !isFinite(seconds)) return 'calculating...';
  return formatDuration(seconds);
}
