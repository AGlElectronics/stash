import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchFileBlobURL } from '@/api/files';

/** Authenticated file preview URL. Refetches on each mount — blob URLs must not be cached across navigations. */
export function useFileBlobUrl(fileId: string | undefined | null) {
  const preview = useQuery({
    queryKey: ['file-blob', fileId],
    queryFn: () => fetchFileBlobURL(fileId!),
    enabled: !!fileId,
    staleTime: 0,
    gcTime: 0,
  });

  useEffect(() => {
    const url = preview.data;
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [preview.data]);

  return preview;
}
