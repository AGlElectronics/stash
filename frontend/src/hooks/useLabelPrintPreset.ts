import { useEffect, useMemo, useState } from 'react';
import {
  DEFAULT_PRESET_ID,
  LABEL_PRESET_STORAGE_KEY,
  findPreset,
} from '@/lib/labelPresets';

function readStoredPresetId(): string {
  if (typeof window === 'undefined') return DEFAULT_PRESET_ID;
  return window.localStorage.getItem(LABEL_PRESET_STORAGE_KEY) ?? DEFAULT_PRESET_ID;
}

/** Label size + bar width state shared by inventory and L&F print pages. */
export function useLabelPrintPreset() {
  const [presetId, setPresetId] = useState(readStoredPresetId);
  const preset = useMemo(() => findPreset(presetId), [presetId]);
  const [moduleWidth, setModuleWidth] = useState(() => findPreset(readStoredPresetId()).defaultModuleWidthMm);

  useEffect(() => {
    try {
      window.localStorage.setItem(LABEL_PRESET_STORAGE_KEY, presetId);
    } catch {
      /* ignore */
    }
  }, [presetId]);

  useEffect(() => {
    setModuleWidth(preset.defaultModuleWidthMm);
  }, [presetId, preset.defaultModuleWidthMm]);

  return {
    presetId,
    setPresetId,
    preset,
    moduleWidth,
    setModuleWidth,
  };
}
