import { useMutation } from '@tanstack/react-query';
import { adminCollectionsApi } from '@/entities/curated-collection';

/** Read-only preview of a pasted list. */
export function usePastePreview(collectionId: string) {
  return useMutation({
    mutationFn: (text: string) => adminCollectionsApi.previewPaste(collectionId, text),
  });
}
