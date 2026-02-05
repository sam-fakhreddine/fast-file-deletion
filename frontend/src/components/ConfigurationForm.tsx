import { useState } from 'react';
import {
  Button,
  Input,
  Checkbox,
  Label,
  SpinButton,
  Dropdown,
  Option,
  MessageBar,
  MessageBarBody,
  MessageBarTitle,
  Accordion,
  AccordionItem,
  AccordionHeader,
  AccordionPanel,
  makeStyles,
  tokens,
} from '@fluentui/react-components';
import { FolderRegular } from '@fluentui/react-icons';
import { useAppContext } from '../context/AppContext';
import { Config, ValidationResult } from '../types/backend';
import { defaultConfig } from '../utils/config';

const useStyles = makeStyles({
  container: {
    padding: tokens.spacingVerticalXXL,
    maxWidth: '800px',
    margin: '0 auto',
  },
  field: {
    marginBottom: tokens.spacingVerticalL,
  },
  label: {
    display: 'block',
    marginBottom: tokens.spacingVerticalXS,
    fontWeight: tokens.fontWeightSemibold,
  },
  inputWithButton: {
    display: 'flex',
    gap: tokens.spacingHorizontalS,
  },
  helpText: {
    fontSize: tokens.fontSizeBase200,
    color: tokens.colorNeutralForeground3,
    marginTop: tokens.spacingVerticalXXS,
  },
  actions: {
    display: 'flex',
    gap: tokens.spacingHorizontalM,
    marginTop: tokens.spacingVerticalXL,
  },
});

export function ConfigurationForm() {
  const classes = useStyles();
  const { state, dispatch } = useAppContext();
  const [config, setConfig] = useState<Config>(defaultConfig);
  const [validationError, setValidationError] = useState<string>('');
  const [isValidating, setIsValidating] = useState(false);

  const disabled = state.stage !== 'config';

  const handleDirectoryPicker = async () => {
    try {
      // Wails directory picker
      const result = await (window as any).runtime.OpenDirectoryDialog();
      if (result) {
        setConfig({ ...config, targetDir: result });
        validatePath(result);
      }
    } catch (err) {
      console.error('Failed to open directory picker:', err);
    }
  };

  const validatePath = async (path: string) => {
    if (!path) {
      setValidationError('');
      return;
    }

    setIsValidating(true);
    try {
      const result: ValidationResult = await (window as any).ffd.ValidatePath(path);
      if (!result.isValid) {
        setValidationError(result.reason);
      } else {
        setValidationError('');
      }
    } catch (err) {
      setValidationError(`Validation error: ${err}`);
    } finally {
      setIsValidating(false);
    }
  };

  const handleScan = async () => {
    // Validate config
    if (!config.targetDir) {
      setValidationError('Target directory is required');
      return;
    }

    if (config.benchmark && config.dryRun) {
      setValidationError('Benchmark and dry-run cannot be used together');
      return;
    }

    if (config.benchmark && config.keepDays !== null) {
      setValidationError('Benchmark and keep-days cannot be used together');
      return;
    }

    dispatch({ type: 'SET_CONFIG', payload: config });
    dispatch({ type: 'START_SCAN' });

    try {
      const result = await (window as any).ffd.ScanDirectory(config);
      dispatch({ type: 'SCAN_COMPLETE', payload: result });
    } catch (err: any) {
      dispatch({ type: 'SCAN_ERROR', payload: err.toString() });
    }
  };

  return (
    <div className={classes.container}>
      <h1>Fast File Deletion</h1>
      <p className={classes.helpText}>
        Configure deletion options and scan the target directory
      </p>

      {validationError && (
        <MessageBar intent="error" style={{ marginBottom: tokens.spacingVerticalL }}>
          <MessageBarBody>
            <MessageBarTitle>Validation Error</MessageBarTitle>
            {validationError}
          </MessageBarBody>
        </MessageBar>
      )}

      {/* Target Directory */}
      <div className={classes.field}>
        <Label className={classes.label} htmlFor="targetDir">
          Target Directory *
        </Label>
        <div className={classes.inputWithButton}>
          <Input
            id="targetDir"
            value={config.targetDir}
            onChange={(e) => {
              const value = e.target.value;
              setConfig({ ...config, targetDir: value });
              validatePath(value);
            }}
            contentBefore={<FolderRegular />}
            placeholder="C:\temp\old-logs"
            disabled={disabled || isValidating}
            style={{ flex: 1 }}
          />
          <Button onClick={handleDirectoryPicker} disabled={disabled}>
            Browse...
          </Button>
        </div>
        <div className={classes.helpText}>
          Directory to scan and delete
        </div>
      </div>

      {/* Basic Options */}
      <div className={classes.field}>
        <Checkbox
          label="Dry Run (preview mode - no files deleted)"
          checked={config.dryRun}
          onChange={(e, data) => setConfig({ ...config, dryRun: data.checked === true })}
          disabled={disabled}
        />
      </div>

      <div className={classes.field}>
        <Checkbox
          label="Force (skip confirmation prompts)"
          checked={config.force}
          onChange={(e, data) => setConfig({ ...config, force: data.checked === true })}
          disabled={disabled}
        />
      </div>

      <div className={classes.field}>
        <Checkbox
          label="Verbose Logging"
          checked={config.verbose}
          onChange={(e, data) => setConfig({ ...config, verbose: data.checked === true })}
          disabled={disabled}
        />
      </div>

      <div className={classes.field}>
        <Checkbox
          label="Enable System Monitoring"
          checked={config.monitor}
          onChange={(e, data) => setConfig({ ...config, monitor: data.checked === true })}
          disabled={disabled}
        />
        <div className={classes.helpText}>
          Monitor CPU, memory, and I/O during deletion
        </div>
      </div>

      {/* Advanced Options */}
      <Accordion collapsible>
        <AccordionItem value="advanced">
          <AccordionHeader>Advanced Options</AccordionHeader>
          <AccordionPanel>
            {/* Keep Days */}
            <div className={classes.field}>
              <Label className={classes.label} htmlFor="keepDays">
                Keep Days (age filter)
              </Label>
              <SpinButton
                id="keepDays"
                value={config.keepDays ?? undefined}
                onChange={(e, data) => {
                  const value = data.value;
                  setConfig({ ...config, keepDays: value !== undefined ? value : null });
                }}
                min={0}
                max={365}
                disabled={disabled}
              />
              <div className={classes.helpText}>
                Only delete files older than N days (leave empty to delete all)
              </div>
            </div>

            {/* Workers */}
            <div className={classes.field}>
              <Label className={classes.label} htmlFor="workers">
                Worker Count
              </Label>
              <SpinButton
                id="workers"
                value={config.workers || undefined}
                onChange={(e, data) => {
                  setConfig({ ...config, workers: data.value ?? 0 });
                }}
                min={0}
                max={1000}
                disabled={disabled}
              />
              <div className={classes.helpText}>
                Parallel workers (0 = auto-detect: CPU × 4)
              </div>
            </div>

            {/* Buffer Size */}
            <div className={classes.field}>
              <Label className={classes.label} htmlFor="bufferSize">
                Buffer Size
              </Label>
              <SpinButton
                id="bufferSize"
                value={config.bufferSize || undefined}
                onChange={(e, data) => {
                  setConfig({ ...config, bufferSize: data.value ?? 0 });
                }}
                min={0}
                max={100000}
                step={1000}
                disabled={disabled}
              />
              <div className={classes.helpText}>
                Work queue buffer size (0 = auto-detect)
              </div>
            </div>

            {/* Deletion Method */}
            <div className={classes.field}>
              <Label className={classes.label} htmlFor="deletionMethod">
                Deletion Method
              </Label>
              <Dropdown
                id="deletionMethod"
                value={config.deletionMethod}
                onOptionSelect={(e, data) => {
                  setConfig({ ...config, deletionMethod: data.optionValue as any });
                }}
                disabled={disabled}
              >
                <Option value="auto">Automatic (recommended)</Option>
                <Option value="fileinfo">FileInfo (fastest on Windows 10+)</Option>
                <Option value="deleteonclose">DeleteOnClose</Option>
                <Option value="ntapi">NtAPI (native)</Option>
                <Option value="deleteapi">DeleteAPI (baseline)</Option>
              </Dropdown>
              <div className={classes.helpText}>
                Automatic selection chooses the fastest method for your system
              </div>
            </div>

            {/* Log File */}
            <div className={classes.field}>
              <Label className={classes.label} htmlFor="logFile">
                Log File (optional)
              </Label>
              <Input
                id="logFile"
                value={config.logFile}
                onChange={(e) => setConfig({ ...config, logFile: e.target.value })}
                placeholder="deletion.log"
                disabled={disabled}
              />
              <div className={classes.helpText}>
                Write detailed logs to file
              </div>
            </div>

            {/* Benchmark */}
            <div className={classes.field}>
              <Checkbox
                label="Benchmark Mode (Windows only)"
                checked={config.benchmark}
                onChange={(e, data) => setConfig({ ...config, benchmark: data.checked === true })}
                disabled={disabled}
              />
              <div className={classes.helpText}>
                Compare all deletion methods
              </div>
            </div>
          </AccordionPanel>
        </AccordionItem>
      </Accordion>

      {/* Actions */}
      <div className={classes.actions}>
        <Button
          appearance="primary"
          onClick={handleScan}
          disabled={disabled || !config.targetDir || !!validationError || isValidating}
        >
          {isValidating ? 'Validating...' : 'Scan Directory'}
        </Button>
        <Button
          onClick={() => {
            setConfig(defaultConfig);
            setValidationError('');
          }}
          disabled={disabled}
        >
          Reset
        </Button>
      </div>
    </div>
  );
}
