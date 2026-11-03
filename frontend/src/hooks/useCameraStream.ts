import { useCallback, useEffect, useRef, useState } from 'react';

type UseCameraStreamResult = {
  active: boolean;
  stream: MediaStream | null;
  error: string | null;
  /** Call from a click/tap handler so iOS grants camera access. */
  start: () => Promise<void>;
  stop: () => void;
  toggle: () => Promise<void>;
  clearError: () => void;
};

async function acquireCameraStream(): Promise<MediaStream> {
  const constraints: MediaStreamConstraints[] = [
    { video: { facingMode: { ideal: 'environment' } }, audio: false },
    { video: { facingMode: 'user' }, audio: false },
    { video: true, audio: false },
  ];

  let lastError: unknown;
  for (const constraint of constraints) {
    try {
      return await navigator.mediaDevices.getUserMedia(constraint);
    } catch (e) {
      lastError = e;
    }
  }

  throw lastError instanceof Error ? lastError : new Error('Could not access the camera.');
}

/**
 * Acquires a camera stream on user gesture (required on iOS / installed PWAs).
 * Tries rear camera first, then front-facing (typical on laptops), then any camera.
 */
export function useCameraStream(): UseCameraStreamResult {
  const streamRef = useRef<MediaStream | null>(null);
  const [active, setActive] = useState(false);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [error, setError] = useState<string | null>(null);

  const stop = useCallback(() => {
    streamRef.current?.getTracks().forEach((t) => t.stop());
    streamRef.current = null;
    setStream(null);
    setActive(false);
  }, []);

  const start = useCallback(async () => {
    setError(null);
    if (!navigator.mediaDevices?.getUserMedia) {
      setError('Camera not supported in this browser.');
      return;
    }
    stop();
    try {
      const next = await acquireCameraStream();
      streamRef.current = next;
      setStream(next);
      setActive(true);
    } catch (e) {
      const message =
        e instanceof Error
          ? e.message
          : 'Could not access the camera. Check permissions and try again.';
      setError(message);
      stop();
    }
  }, [stop]);

  const toggle = useCallback(async () => {
    if (active) {
      stop();
      return;
    }
    await start();
  }, [active, start, stop]);

  useEffect(() => () => stop(), [stop]);

  const clearError = useCallback(() => setError(null), []);

  return {
    active,
    stream,
    error,
    start,
    stop,
    toggle,
    clearError,
  };
}
