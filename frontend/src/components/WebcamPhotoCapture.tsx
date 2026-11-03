import { useEffect, useRef } from 'react';
import { Camera, CameraOff } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { useCameraStream } from '@/hooks/useCameraStream';

type Props = {
  onCapture: (file: File) => void;
  disabled?: boolean;
};

/** Live webcam preview with a capture button — for L&F report photos on laptops. */
export function WebcamPhotoCapture({ onCapture, disabled }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const { active, stream, error, start, stop, clearError } = useCameraStream();

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !stream) return;
    video.srcObject = stream;
    void video.play().catch(() => undefined);
    return () => {
      video.srcObject = null;
    };
  }, [stream]);

  function capture() {
    const video = videoRef.current;
    if (!video || video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) return;

    const canvas = document.createElement('canvas');
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.drawImage(video, 0, 0);

    canvas.toBlob(
      (blob) => {
        if (!blob) return;
        onCapture(new File([blob], `photo-${Date.now()}.jpg`, { type: 'image/jpeg' }));
        stop();
      },
      'image/jpeg',
      0.92
    );
  }

  if (!active) {
    return (
      <div className="space-y-2">
        <Button
          type="button"
          variant="outline"
          disabled={disabled}
          onClick={() => {
            clearError();
            void start();
          }}
        >
          <Camera className="w-4 h-4" />
          Use webcam
        </Button>
        {error ? (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="relative aspect-[4/3] w-full max-w-md overflow-hidden rounded-md border border-border bg-black">
        <video ref={videoRef} autoPlay muted playsInline className="h-full w-full object-contain" />
      </div>
      <div className="flex gap-2">
        <Button type="button" onClick={capture} disabled={disabled}>
          <Camera className="w-4 h-4" />
          Take photo
        </Button>
        <Button type="button" variant="ghost" onClick={stop} disabled={disabled}>
          <CameraOff className="w-4 h-4" />
          Cancel
        </Button>
      </div>
    </div>
  );
}
