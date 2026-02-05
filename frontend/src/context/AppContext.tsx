import React, { createContext, useContext, useReducer, ReactNode } from 'react';
import { AppState, Config, ScanResult, LiveMetrics, DeletionResult } from '../types/backend';
import { defaultConfig } from '../utils/config';

// Action types
type AppAction =
  | { type: 'SET_CONFIG'; payload: Config }
  | { type: 'START_SCAN' }
  | { type: 'SCAN_COMPLETE'; payload: ScanResult }
  | { type: 'SCAN_ERROR'; payload: string }
  | { type: 'CONFIRM_DELETION' }
  | { type: 'UPDATE_METRICS'; payload: LiveMetrics }
  | { type: 'DELETION_COMPLETE'; payload: DeletionResult }
  | { type: 'DELETION_ERROR'; payload: string }
  | { type: 'RESET' };

// Initial state
const initialState: AppState = {
  stage: 'config',
  config: defaultConfig,
};

// Reducer
function appReducer(state: AppState, action: AppAction): AppState {
  switch (action.type) {
    case 'SET_CONFIG':
      return {
        ...state,
        config: action.payload,
      };

    case 'START_SCAN':
      return {
        ...state,
        stage: 'scanning',
        scanResult: undefined,
        error: undefined,
      };

    case 'SCAN_COMPLETE':
      return {
        ...state,
        stage: 'confirm',
        scanResult: action.payload,
      };

    case 'SCAN_ERROR':
      return {
        ...state,
        stage: 'error',
        error: action.payload,
      };

    case 'CONFIRM_DELETION':
      return {
        ...state,
        stage: 'progress',
        liveMetrics: {
          filesDeleted: 0,
          deletionRate: 0,
          elapsedSeconds: 0,
        },
      };

    case 'UPDATE_METRICS':
      return {
        ...state,
        liveMetrics: action.payload,
      };

    case 'DELETION_COMPLETE':
      return {
        ...state,
        stage: 'results',
        deletionResult: action.payload,
      };

    case 'DELETION_ERROR':
      return {
        ...state,
        stage: 'error',
        error: action.payload,
      };

    case 'RESET':
      return initialState;

    default:
      return state;
  }
}

// Context
interface AppContextType {
  state: AppState;
  dispatch: React.Dispatch<AppAction>;
}

const AppContext = createContext<AppContextType | undefined>(undefined);

// Provider
export function AppProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(appReducer, initialState);

  return (
    <AppContext.Provider value={{ state, dispatch }}>
      {children}
    </AppContext.Provider>
  );
}

// Hook
export function useAppContext() {
  const context = useContext(AppContext);
  if (!context) {
    throw new Error('useAppContext must be used within AppProvider');
  }
  return context;
}
