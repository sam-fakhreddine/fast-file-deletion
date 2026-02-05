import { useState, useEffect } from 'react';
import {
  Button,
  ProgressBar,
  Card,
  makeStyles,
  tokens,
  Accordion,
  AccordionItem,
  AccordionHeader,
  AccordionPanel,
} from '@fluentui/react-components';
import { DismissRegular } from '@fluentui/react-icons';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { useAppContext } from '../context/AppContext';
import { useWailsEvent } from '../hooks/useWailsEvent';
import { LiveMetrics } from '../types/backend';
import { formatNumber, formatRate, formatDuration, calculateETA } from '../utils/config';

const useStyles = makeStyles({
  container: {
    padding: tokens.spacingVerticalXXL,
    maxWidth: '1200px',
    margin: '0 auto',
  },
  progressSection: {
    marginBottom: tokens.spacingVerticalXL,
  },
  statsGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
    gap: tokens.spacingHorizontalL,
    marginTop: tokens.spacingVerticalL,
  },
  statCard: {
    padding: tokens.spacingVerticalL,
  },
  statLabel: {
    fontSize: tokens.fontSizeBase200,
    color: tokens.colorNeutralForeground3,
    marginBottom: tokens.spacingVerticalXXS,
  },
  statValue: {
    fontSize: tokens.fontSizeHero800,
    fontWeight: tokens.fontWeightSemibold,
    color: tokens.colorBrandForeground1,
  },
  chartContainer: {
    marginTop: tokens.spacingVerticalL,
    height: '250px',
  },
  metricsGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: tokens.spacingHorizontalM,
    marginTop: tokens.spacingVerticalM,
  },
  metricCard: {
    padding: tokens.spacingVerticalM,
  },
  actions: {
    marginTop: tokens.spacingVerticalXL,
  },
});

interface ChartDataPoint {
  time: number;
  rate: number;
  cpu?: number;
  memory?: number;
}

export function ProgressView() {
  const classes = useStyles();
  const { state, dispatch } = useAppContext();
  const [chartData, setChartData] = useState<ChartDataPoint[]>([]);
  const [showCancelDialog, setShowCancelDialog] = useState(false);

  const metrics = state.liveMetrics;
  const scanResult = state.scanResult!;
  const totalFiles = scanResult.totalToDelete;
  const filesDeleted = metrics?.filesDeleted || 0;
  const rate = metrics?.deletionRate || 0;
  const elapsed = metrics?.elapsedSeconds || 0;
  const progress = totalFiles > 0 ? (filesDeleted / totalFiles) * 100 : 0;
  const remaining = totalFiles - filesDeleted;
  const eta = calculateETA(remaining, rate);

  // Listen to progress updates (throttled to 100ms by hook)
  useWailsEvent<LiveMetrics>('progress:update', (data) => {
    dispatch({ type: 'UPDATE_METRICS', payload: data });
  }, 100);

  // Update chart data
  useEffect(() => {
    if (!metrics) return;

    setChartData((prev) => {
      const newPoint: ChartDataPoint = {
        time: metrics.elapsedSeconds,
        rate: metrics.deletionRate,
        cpu: metrics.systemMetrics?.cpuPercent,
        memory: metrics.systemMetrics?.memoryMb,
      };

      // Keep last 60 points (1 minute of history at 100ms throttle = ~600 points, but we sample every update)
      const updated = [...prev, newPoint];
      return updated.slice(-60);
    });

    // Update taskbar progress (Windows only)
    try {
      if ((window as any).runtime?.Window?.SetProgressBar) {
        (window as any).runtime.Window.SetProgressBar(progress / 100);
      }
    } catch (err) {
      // Ignore if not available
    }
  }, [metrics, progress]);

  const handleCancel = async () => {
    try {
      await (window as any).ffd.CancelDeletion();
      setShowCancelDialog(false);
    } catch (err) {
      console.error('Failed to cancel deletion:', err);
    }
  };

  const systemMetrics = metrics?.systemMetrics;
  const hasMonitoring = state.config.monitor && systemMetrics;

  return (
    <div className={classes.container}>
      <h1>{state.config.dryRun ? 'Preview Mode' : 'Deleting Files'}</h1>

      {/* Progress Bar */}
      <div className={classes.progressSection}>
        <ProgressBar value={progress / 100} />
        <div style={{ textAlign: 'center', marginTop: tokens.spacingVerticalS }}>
          {progress.toFixed(1)}% complete
        </div>
      </div>

      {/* Stats Grid */}
      <div className={classes.statsGrid}>
        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Files Deleted</div>
          <div className={classes.statValue}>{formatNumber(filesDeleted)} / {formatNumber(totalFiles)}</div>
        </Card>

        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Deletion Rate</div>
          <div className={classes.statValue}>{formatRate(rate)}</div>
        </Card>

        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Elapsed Time</div>
          <div className={classes.statValue}>{formatDuration(elapsed)}</div>
        </Card>

        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Estimated Time</div>
          <div className={classes.statValue}>{eta}</div>
        </Card>
      </div>

      {/* Deletion Rate Chart */}
      {chartData.length > 1 && (
        <div className={classes.chartContainer}>
          <h3>Deletion Rate (last 60 seconds)</h3>
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis
                dataKey="time"
                label={{ value: 'Time (s)', position: 'insideBottom', offset: -5 }}
                tickFormatter={(value) => value.toFixed(0)}
              />
              <YAxis
                label={{ value: 'Files/sec', angle: -90, position: 'insideLeft' }}
              />
              <Tooltip
                formatter={(value: number) => formatRate(value)}
                labelFormatter={(label) => `${label.toFixed(1)}s`}
              />
              <Line
                type="monotone"
                dataKey="rate"
                stroke={tokens.colorBrandForeground1}
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* System Monitoring (Collapsible) */}
      {hasMonitoring && (
        <Accordion collapsible style={{ marginTop: tokens.spacingVerticalXL }}>
          <AccordionItem value="monitoring">
            <AccordionHeader>System Resource Monitoring</AccordionHeader>
            <AccordionPanel>
              <div className={classes.metricsGrid}>
                <Card className={classes.metricCard}>
                  <div className={classes.statLabel}>CPU Usage</div>
                  <div className={classes.statValue}>
                    {systemMetrics.cpuPercent.toFixed(1)}%
                    {systemMetrics.cpuSaturated && <span style={{ color: tokens.colorPaletteRedForeground1 }}> (saturated)</span>}
                  </div>
                </Card>

                <Card className={classes.metricCard}>
                  <div className={classes.statLabel}>Memory Usage</div>
                  <div className={classes.statValue}>
                    {systemMetrics.memoryMb.toFixed(0)} MB
                    {systemMetrics.memoryPressure && <span style={{ color: tokens.colorPaletteRedForeground1 }}> (pressure)</span>}
                  </div>
                </Card>

                <Card className={classes.metricCard}>
                  <div className={classes.statLabel}>I/O Operations</div>
                  <div className={classes.statValue}>{systemMetrics.ioOpsPerSec.toFixed(0)} ops/sec</div>
                </Card>

                <Card className={classes.metricCard}>
                  <div className={classes.statLabel}>Goroutines</div>
                  <div className={classes.statValue}>
                    {systemMetrics.goroutineCount}
                    {systemMetrics.gcPressure && <span style={{ color: tokens.colorPaletteRedForeground1 }}> (GC pressure)</span>}
                  </div>
                </Card>
              </div>

              {/* System Metrics Chart */}
              {chartData.some(d => d.cpu !== undefined) && (
                <div className={classes.chartContainer}>
                  <h4>System Metrics History</h4>
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" tickFormatter={(value) => value.toFixed(0)} />
                      <YAxis yAxisId="left" />
                      <YAxis yAxisId="right" orientation="right" />
                      <Tooltip />
                      <Line
                        yAxisId="left"
                        type="monotone"
                        dataKey="cpu"
                        stroke={tokens.colorBrandForeground1}
                        name="CPU %"
                        dot={false}
                        isAnimationActive={false}
                      />
                      <Line
                        yAxisId="right"
                        type="monotone"
                        dataKey="memory"
                        stroke={tokens.colorPaletteGreenForeground1}
                        name="Memory MB"
                        dot={false}
                        isAnimationActive={false}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              )}
            </AccordionPanel>
          </AccordionItem>
        </Accordion>
      )}

      {/* Cancel Button */}
      <div className={classes.actions}>
        <Button
          appearance="secondary"
          icon={<DismissRegular />}
          onClick={() => setShowCancelDialog(true)}
        >
          Cancel Deletion
        </Button>
      </div>

      {/* Cancel Confirmation (Simple version - could be enhanced with Dialog component) */}
      {showCancelDialog && (
        <Card style={{ marginTop: tokens.spacingVerticalL, padding: tokens.spacingVerticalL }}>
          <h3>Cancel Deletion?</h3>
          <p>Are you sure you want to stop the deletion? Progress will be lost.</p>
          <div style={{ display: 'flex', gap: tokens.spacingHorizontalM, marginTop: tokens.spacingVerticalM }}>
            <Button appearance="primary" onClick={handleCancel}>
              Yes, Cancel
            </Button>
            <Button onClick={() => setShowCancelDialog(false)}>
              No, Continue
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
}
