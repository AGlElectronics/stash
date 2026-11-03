import { useLayoutEffect, useRef, useState } from 'react';
import { BrowserCodeReader, BrowserMultiFormatReader } from '@zxing/browser';
import { CameraOff, Loader2 } from 'lucide-react';

type Props = {
  /** Pre-acquired on a user gesture (see useCameraStream). Preferred on mobile. */
  stream?: MediaStream | null;
  onResult: (text: string) => void;
  /** Only called for fatal setup failures — not per-frame decode misses. */
  onError?: (err: Error) => void;
  /** Full-width preview without height cap or dark crop mask. */
  wide?: boolean;
};

function waitForVideoReady(video: HTMLVideoElement): Promise<void> {
  if (video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const onReady = () => {
      cleanup();
      resolve();
    };
    const onFail = () => {
      cleanup();
      reject(new Error('Camera preview failed to start'));
    };
    const cleanup = () => {
      video.removeEventListener('loadedmetadata', onReady);
      video.removeEventListener('canplay', onReady);
      video.removeEventListener('error', onFail);
    };
    video.addEventListener('loadedmetadata', onReady);
    video.addEventListener('canplay', onReady);
    video.addEventListener('error', onFail);
  });
}

/**
 * Live camera barcode scanner. Expects a MediaStream from useCameraStream (started
 * in a click handler). Uses zxing scan() directly so we keep stream ownership and
 * decodeFromStream does not stop our tracks on unmount.
 */
export function CameraScanner({ stream, onResult, onError, wide = false }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const firedRef = useRef(false);
  const onResultRef = useRef(onResult);
  const onErrorRef = useRef(onError);
  const [error, setError] = useState<string | null>(null);
  const [starting, setStarting] = useState(true);

  onResultRef.current = onResult;
  onErrorRef.current = onError;

  useLayoutEffect(() => {
    firedRef.current = false;
  }, [stream]);

  useLayoutEffect(() => {
    const video = videoRef.current;
    if (!video || !stream) {
      setStarting(!stream);
      return;
    }

    let cancelled = false;
    let stopScan: (() => void) | null = null;
    const reader = new BrowserMultiFormatReader(undefined, {
      delayBetweenScanAttempts: 150,
      delayBetweenScanSuccess: 400,
      tryPlayVideoTimeout: 10_000,
    });

    video.setAttribute('autoplay', 'true');
    video.setAttribute('muted', 'true');
    video.setAttribute('playsinline', 'true');
    video.muted = true;
    video.playsInline = true;

    async function start() {
      const el = videoRef.current;
      if (!el || !stream) return;

      setStarting(true);
      setError(null);
      try {
        BrowserCodeReader.addVideoSource(el, stream);
        await waitForVideoReady(el);
        if (el.paused) {
          await el.play();
        }
        if (cancelled) return;

        const controls = reader.scan(el, (result) => {
          if (firedRef.current) return;
          if (result) {
            firedRef.current = true;
            onResultRef.current(result.getText());
          }
          // Ignore decode errors (NotFound / Checksum / Format) — normal while scanning.
        });

        stopScan = () => controls.stop();
        setStarting(false);
      } catch (e) {
        if (cancelled) return;
        const message = e instanceof Error ? e.message : 'Could not start camera preview';
        setError(message);
        setStarting(false);
        onErrorRef.current?.(e instanceof Error ? e : new Error(message));
      }
    }

    void start();

    return () => {
      cancelled = true;
      stopScan?.();
      stopScan = null;
      BrowserCodeReader.cleanVideoSource(video);
    };
  }, [stream]);

  if (!stream) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 rounded-md border border-border bg-card p-6 text-sm text-muted-foreground">
        <CameraOff className="w-6 h-6" />
        <p className="text-center">Camera stream not ready. Tap the camera button again.</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 rounded-md border border-border bg-card p-6 text-sm text-muted-foreground">
        <CameraOff className="w-6 h-6" />
        <p className="text-center">{error}</p>
        <p className="text-xs text-center">
          Camera access requires HTTPS (or localhost) and permission. On iPhone, use Safari or
          reopen the app if the preview stays black. USB scanners work without the camera.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div
        className={
          wide
            ? 'relative w-full min-h-[min(52vh,28rem)] rounded-md border border-border bg-black'
            : 'relative aspect-[4/3] w-full overflow-hidden rounded-md border border-border bg-black'
        }
      >
        <video
          ref={videoRef}
          autoPlay
          muted
          playsInline
          className={
            wide
              ? 'block h-full w-full min-h-[min(52vh,28rem)] object-contain bg-black'
              : 'absolute inset-0 h-full w-full object-contain bg-black'
          }
        />
        {starting ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-black/70 text-white text-sm">
            <Loader2 className="w-6 h-6 animate-spin" />
            Starting camera…
          </div>
        ) : null}
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-4 sm:px-8">
          <div
            className={
              wide
                ? 'w-full h-[18%] min-h-[3rem] max-h-[5rem] border-2 border-awo-amaranth/90 rounded-sm'
                : 'w-full max-w-lg h-[22%] min-h-[2.75rem] max-h-[4.5rem] border-2 border-awo-amaranth/80 rounded-sm shadow-[0_0_0_9999px_rgba(0,0,0,0.35)]'
            }
            aria-hidden
          />
        </div>
      </div>
      <p className="text-xs text-center text-muted-foreground">
        Hold the barcode flat and horizontal inside the box
      </p>
    </div>
  );
}
