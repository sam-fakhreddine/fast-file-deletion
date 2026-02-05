import { useState } from 'react';
import {
  Button,
  Card,
  makeStyles,
  tokens,
  Accordion,
  AccordionItem,
  AccordionHeader,
  AccordionPanel,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
} from '@fluentui/react-components';
import { CheckmarkCircleRegular, ErrorCircleRegular } from '@fluentui/react-icons';
import { useAppContext } from '../context/AppContext';
import { formatNumber, formatRate, formatDuration } from '../utils/config';

const useStyles = makeStyles({
  container: {
    padding: tokens.spacingVerticalXXL,
    maxWidth: '1000px',
    margin: '0 auto',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalM,
    marginBottom: tokens.spacingVerticalXL,
  },
  icon: {
    fontSize: '48px',
    color: tokens.colorPaletteGreenForeground1,
  },
  statsGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
    gap: tokens.spacingHorizontalL,
    marginBottom: tokens.spacingVerticalXL,
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
    fontSize: tokens.fontSizeHero700,
    fontWeight: tokens.fontWeightSemibold,
  },
  section: {
    marginBottom: tokens.spacingVerticalXL,
  },
  methodTable: {
    width: '100%',
    borderCollapse: 'collapse',
    marginTop: tokens.spacingVerticalM,
    '& th': {
      textAlign: 'left',
      padding: tokens.spacingVerticalM,
      borderBottom: `1px solid ${tokens.colorNeutralStroke1}`,
      fontWeight: tokens.fontWeightSemibold,
    },
    '& td': {
      padding: tokens.spacingVerticalM,
      borderBottom: `1px solid ${tokens.colorNeutralStroke1}`,
    },
  },
  reportText: {
    whiteSpace: 'pre-wrap',
    fontFamily: tokens.fontFamilyMonospace,
    fontSize: tokens.fontSizeBase200,
    padding: tokens.spacingVerticalM,
    backgroundColor: tokens.colorNeutralBackground3,
    borderRadius: tokens.borderRadiusMedium,
    marginTop: tokens.spacingVerticalM,
  },
  errorList: {
    maxHeight: '300px',
    overflowY: 'auto',
    marginTop: tokens.spacingVerticalM,
  },
  errorItem: {
    padding: tokens.spacingVerticalS,
    borderBottom: `1px solid ${tokens.colorNeutralStroke1}`,
    fontFamily: tokens.fontFamilyMonospace,
    fontSize: tokens.fontSizeBase200,
  },
  actions: {
    display: 'flex',
    gap: tokens.spacingHorizontalM,
    marginTop: tokens.spacingVerticalXL,
  },
});

export function ResultsView() {
  const classes = useStyles();
  const { state, dispatch } = useAppContext();
  const [showAllErrors, setShowAllErrors] = useState(false);

  const result = state.deletionResult!;
  const config = state.config;
  const isDryRun = config.dryRun;

  const success = result.failedCount === 0;
  const duration = result.durationMs / 1000;

  const errors = result.errors || [];
  const displayedErrors = showAllErrors ? errors : errors.slice(0, 50);

  const handleReset = () => {
    // Clear taskbar progress
    try {
      if ((window as any).runtime?.Window?.SetProgressBar) {
        (window as any).runtime.Window.SetProgressBar(-1);
      }
    } catch (err) {
      // Ignore
    }

    dispatch({ type: 'RESET' });
  };

  return (
    <div className={classes.container}>
      {/* Header */}
      <div className={classes.header}>
        {success ? (
          <CheckmarkCircleRegular className={classes.icon} />
        ) : (
          <ErrorCircleRegular className={classes.icon} style={{ color: tokens.colorPaletteYellowForeground1 }} />
        )}
        <div>
          <h1>{isDryRun ? 'Preview Complete' : success ? 'Deletion Complete' : 'Deletion Completed with Errors'}</h1>
          {isDryRun && <p>No files were actually deleted (dry run mode)</p>}
        </div>
      </div>

      {/* Stats Grid */}
      <div className={classes.statsGrid}>
        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Files Deleted</div>
          <div className={classes.statValue} style={{ color: tokens.colorPaletteGreenForeground1 }}>
            {formatNumber(result.deletedCount)}
          </div>
        </Card>

        {result.failedCount > 0 && (
          <Card className={classes.statCard}>
            <div className={classes.statLabel}>Failed</div>
            <div className={classes.statValue} style={{ color: tokens.colorPaletteRedForeground1 }}>
              {formatNumber(result.failedCount)}
            </div>
          </Card>
        )}

        {result.retainedCount > 0 && (
          <Card className={classes.statCard}>
            <div className={classes.statLabel}>Retained</div>
            <div className={classes.statValue} style={{ color: tokens.colorNeutralForeground3 }}>
              {formatNumber(result.retainedCount)}
            </div>
          </Card>
        )}

        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Duration</div>
          <div className={classes.statValue}>{formatDuration(duration)}</div>
        </Card>

        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Average Rate</div>
          <div className={classes.statValue}>{formatRate(result.averageRate)}</div>
        </Card>

        <Card className={classes.statCard}>
          <div className={classes.statLabel}>Peak Rate</div>
          <div className={classes.statValue}>{formatRate(result.peakRate)}</div>
        </Card>
      </div>

      {/* Method Statistics */}
      {result.methodStats && (
        <div className={classes.section}>
          <h2>Deletion Method Breakdown</h2>
          <table className={classes.methodTable}>
            <thead>
              <tr>
                <th>Method</th>
                <th>Files Deleted</th>
                <th>Percentage</th>
              </tr>
            </thead>
            <tbody>
              {result.methodStats.fileInfoCount > 0 && (
                <tr>
                  <td>FileInfo (SetFileInformationByHandle)</td>
                  <td>{formatNumber(result.methodStats.fileInfoCount)}</td>
                  <td>{((result.methodStats.fileInfoCount / result.deletedCount) * 100).toFixed(1)}%</td>
                </tr>
              )}
              {result.methodStats.deleteOnCloseCount > 0 && (
                <tr>
                  <td>DeleteOnClose (FILE_FLAG_DELETE_ON_CLOSE)</td>
                  <td>{formatNumber(result.methodStats.deleteOnCloseCount)}</td>
                  <td>{((result.methodStats.deleteOnCloseCount / result.deletedCount) * 100).toFixed(1)}%</td>
                </tr>
              )}
              {result.methodStats.ntApiCount > 0 && (
                <tr>
                  <td>NtAPI (NtDeleteFile)</td>
                  <td>{formatNumber(result.methodStats.ntApiCount)}</td>
                  <td>{((result.methodStats.ntApiCount / result.deletedCount) * 100).toFixed(1)}%</td>
                </tr>
              )}
              {result.methodStats.fallbackCount > 0 && (
                <tr>
                  <td>Fallback (DeleteFile)</td>
                  <td>{formatNumber(result.methodStats.fallbackCount)}</td>
                  <td>{((result.methodStats.fallbackCount / result.deletedCount) * 100).toFixed(1)}%</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Bottleneck Report */}
      {result.bottleneckReport && (
        <div className={classes.section}>
          <h2>Performance Bottleneck Analysis</h2>
          <div className={classes.reportText}>{result.bottleneckReport}</div>
        </div>
      )}

      {/* Errors */}
      {errors.length > 0 && (
        <div className={classes.section}>
          <Accordion collapsible defaultOpenItems={errors.length < 10 ? ['errors'] : []}>
            <AccordionItem value="errors">
              <AccordionHeader>
                <MessageBar intent="error">
                  <MessageBarBody>
                    <MessageBarTitle>{errors.length} Error{errors.length !== 1 ? 's' : ''} Occurred</MessageBarTitle>
                    Some files could not be deleted
                  </MessageBarBody>
                </MessageBar>
              </AccordionHeader>
              <AccordionPanel>
                <div className={classes.errorList}>
                  {displayedErrors.map((error, index) => (
                    <div key={index} className={classes.errorItem}>
                      {error}
                    </div>
                  ))}
                  {errors.length > 50 && !showAllErrors && (
                    <Button
                      appearance="subtle"
                      onClick={() => setShowAllErrors(true)}
                      style={{ marginTop: tokens.spacingVerticalM }}
                    >
                      Show All {errors.length} Errors
                    </Button>
                  )}
                </div>
              </AccordionPanel>
            </AccordionItem>
          </Accordion>
        </div>
      )}

      {/* Actions */}
      <div className={classes.actions}>
        <Button appearance="primary" onClick={handleReset}>
          Delete More Files
        </Button>
      </div>
    </div>
  );
}
