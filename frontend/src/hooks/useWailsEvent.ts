import { useEffect, useRef } from 'react';
import { Events } from '@wailsio/runtime';

/**
 * Custom hook for Wails events with throttling and cleanup
 * Prevents memory leaks and excessive re-renders
 */
export function useWailsEvent<T>(
  eventName: string,
  handler: (data: T) => void,
  throttleMs: number = 100
) {
  const handlerRef = useRef(handler);
  const lastCallRef = useRef<number>(0);
  const pendingRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Update handler ref on each render
  useEffect(() => {
    handlerRef.current = handler;
  }, [handler]);

  useEffect(() => {
    const throttledHandler = (event: any) => {
      const now = Date.now();
      const timeSinceLastCall = now - lastCallRef.current;

      // Clear any pending call
      if (pendingRef.current) {
        clearTimeout(pendingRef.current);
        pendingRef.current = null;
      }

      if (timeSinceLastCall >= throttleMs) {
        // Leading edge: call immediately
        lastCallRef.current = now;
        handlerRef.current(event.data);
      } else {
        // Trailing edge: schedule call
        pendingRef.current = setTimeout(() => {
          lastCallRef.current = Date.now();
          handlerRef.current(event.data);
          pendingRef.current = null;
        }, throttleMs - timeSinceLastCall);
      }
    };

    // Subscribe to event
    const unsubscribe = Events.On(eventName, throttledHandler);

    // Cleanup on unmount
    return () => {
      if (pendingRef.current) {
        clearTimeout(pendingRef.current);
      }
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, [eventName, throttleMs]);
}
