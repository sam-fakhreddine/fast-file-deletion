import { useState, useEffect } from 'react';
import {
  FluentProvider,
  webLightTheme,
  webDarkTheme,
  Spinner,
  Dialog,
  DialogSurface,
  DialogTitle,
  DialogBody,
  DialogActions,
  DialogContent,
  Button,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
  makeStyles,
  tokens,
} from '@fluentui/react-components';
import { AppProvider, useAppContext } from './context/AppContext';
import { useWailsEvent } from './hooks/useWailsEvent';
import { ConfigurationForm } from './components/ConfigurationForm';
import { ProgressView } from './components/ProgressView';
import { ResultsView } from './components/ResultsView';
import { DeletionResult } from './types/backend';
import { formatNumber, formatBytes } from './utils/config';

const useStyles = makeStyles({
  app: {
    minHeight: '100vh',
    backgroundColor: tokens.colorNeutralBackground1,
  },
  loadingContainer: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '100vh',
    gap: tokens.spacingVerticalL,
  },
  errorContainer: {
    padding: tokens.spacingVerticalXXL,
    maxWidth: '800px',
    margin: '0 auto',
  },
});

function AppContent() {
  const classes = useStyles();
  const { state, dispatch } = useAppContext();
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);

  // Listen for deletion complete event
  useWailsEvent<DeletionResult>('deletion:complete', (result) => {
    dispatch({ type: 'DELETION_COMPLETE', payload: result });
  });

  // Listen for deletion error event
  useWailsEvent<{ error: string }>('deletion:error', (data) => {
    dispatch({ type: 'DELETION_ERROR', payload: data.error });
  });

  // Show confirmation dialog when scan completes
  useEffect(() => {
    if (state.stage === 'confirm') {
      setConfirmDialogOpen(true);
    }
  }, [state.stage]);

  const handleConfirmDeletion = async () => {
    setConfirmDialogOpen(false);
    dispatch({ type: 'CONFIRM_DELETION' });

    try {
      await (window as any).ffd.StartDeletion(state.config);
    } catch (err: any) {
      dispatch({ type: 'DELETION_ERROR', payload: err.toString() });
    }
  };

  const handleCancelConfirmation = () => {
    setConfirmDialogOpen(false);
    dispatch({ type: 'RESET' });
  };

  // Render current stage
  const renderStage = () => {
    switch (state.stage) {
      case 'config':
        return <ConfigurationForm />;

      case 'scanning':
        return (
          <div className={classes.loadingContainer}>
            <Spinner size="huge" label="Scanning directory..." />
            <p>Please wait while we scan the target directory...</p>
          </div>
        );

      case 'confirm':
        return <ConfigurationForm />; // Keep form visible in background

      case 'progress':
        return <ProgressView />;

      case 'results':
        return <ResultsView />;

      case 'error':
        return (
          <div className={classes.errorContainer}>
            <MessageBar intent="error">
              <MessageBarBody>
                <MessageBarTitle>Error</MessageBarTitle>
                {state.error}
              </MessageBarBody>
            </MessageBar>
            <Button
              appearance="primary"
              onClick={() => dispatch({ type: 'RESET' })}
              style={{ marginTop: tokens.spacingVerticalL }}
            >
              Back to Configuration
            </Button>
          </div>
        );

      default:
        return null;
    }
  };

  const scanResult = state.scanResult;
  const isDryRun = state.config.dryRun;

  return (
    <div className={classes.app}>
      {renderStage()}

      {/* Confirmation Dialog */}
      <Dialog open={confirmDialogOpen} onOpenChange={(e, data) => setConfirmDialogOpen(data.open)}>
        <DialogSurface>
          <DialogBody>
            <DialogTitle>{isDryRun ? 'Preview Deletion' : 'Confirm Deletion'}</DialogTitle>
            <DialogContent>
              {scanResult && (
                <div>
                  <p style={{ marginBottom: tokens.spacingVerticalM }}>
                    <strong>Target Directory:</strong><br />
                    {state.config.targetDir}
                  </p>

                  <p style={{ marginBottom: tokens.spacingVerticalM }}>
                    <strong>Files to Delete:</strong> {formatNumber(scanResult.totalToDelete)}
                  </p>

                  {scanResult.totalRetained > 0 && (
                    <p style={{ marginBottom: tokens.spacingVerticalM }}>
                      <strong>Files to Retain:</strong> {formatNumber(scanResult.totalRetained)} (age filter active)
                    </p>
                  )}

                  <p style={{ marginBottom: tokens.spacingVerticalM }}>
                    <strong>Total Size:</strong> {formatBytes(scanResult.totalSizeBytes)}
                  </p>

                  {isDryRun ? (
                    <MessageBar intent="info" style={{ marginTop: tokens.spacingVerticalL }}>
                      <MessageBarBody>
                        <strong>Preview Mode:</strong> No files will actually be deleted
                      </MessageBarBody>
                    </MessageBar>
                  ) : (
                    <MessageBar intent="warning" style={{ marginTop: tokens.spacingVerticalL }}>
                      <MessageBarBody>
                        <MessageBarTitle>Warning</MessageBarTitle>
                        This action cannot be undone. {formatNumber(scanResult.totalToDelete)} files will be permanently deleted.
                      </MessageBarBody>
                    </MessageBar>
                  )}
                </div>
              )}
            </DialogContent>
            <DialogActions>
              <Button appearance="secondary" onClick={handleCancelConfirmation}>
                Cancel
              </Button>
              <Button appearance="primary" onClick={handleConfirmDeletion}>
                {isDryRun ? 'Start Preview' : 'Delete Files'}
              </Button>
            </DialogActions>
          </DialogBody>
        </DialogSurface>
      </Dialog>
    </div>
  );
}

export default function App() {
  const [theme, setTheme] = useState(webDarkTheme);

  // Detect system theme on mount
  useEffect(() => {
    // Check if system prefers dark mode
    const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    setTheme(prefersDark ? webDarkTheme : webLightTheme);

    // Listen for theme changes
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handleChange = (e: MediaQueryListEvent) => {
      setTheme(e.matches ? webDarkTheme : webLightTheme);
    };

    mediaQuery.addEventListener('change', handleChange);
    return () => mediaQuery.removeEventListener('change', handleChange);
  }, []);

  return (
    <FluentProvider theme={theme}>
      <AppProvider>
        <AppContent />
      </AppProvider>
    </FluentProvider>
  );
}
