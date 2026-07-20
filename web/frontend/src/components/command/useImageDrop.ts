// useImageDrop owns image drag-and-drop staging for a composer
// surface: dropped images upload immediately via the command upload
// endpoint, then wait in the composer as pending attachments so the
// operator can add a message before anything is sent. Shared by the
// canvas cards and focus mode so both surfaces accept drops.

import { useState } from 'react';
import { useUIStore } from '@/stores/ui.js';
import type { PendingAttachment } from './SteerComposer.js';

export interface ImageDropState {
  attachments: PendingAttachment[];
  dropActive: boolean;

  // Spread onto the drop surface element.
  dropHandlers: {
    onDragOver: (e: React.DragEvent) => void;
    onDragLeave: () => void;
    onDrop: (e: React.DragEvent) => void;
  };

  removeAttachment: (index: number) => void;
  clearAttachments: () => void;
}

export function useImageDrop(): ImageDropState {
  const [attachments, setAttachments] = useState<PendingAttachment[]>(
    [],
  );
  const [dropActive, setDropActive] = useState(false);
  const addToast = useUIStore((s) => s.addToast);

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDropActive(false);

    const images = Array.from(e.dataTransfer.files).filter((f) =>
      f.type.startsWith('image/'),
    );
    if (images.length === 0) {
      return;
    }

    try {
      const staged: PendingAttachment[] = [];
      for (const img of images) {
        const form = new FormData();
        form.append('file', img);
        const res = await fetch('/api/v1/command/upload', {
          method: 'POST',
          body: form,
        });
        if (!res.ok) {
          throw new Error(`upload failed (${res.status})`);
        }
        const data = (await res.json()) as {
          url: string;
          markdown: string;
        };
        staged.push({
          name: img.name, url: data.url, markdown: data.markdown,
        });
      }
      setAttachments((prev) => [...prev, ...staged]);
    } catch (err) {
      addToast({
        variant: 'error',
        title: 'Attachment failed',
        message:
          err instanceof Error ? err.message : 'Upload error',
      });
    }
  };

  return {
    attachments,
    dropActive,
    dropHandlers: {
      onDragOver: (e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setDropActive(true);
      },
      onDragLeave: () => setDropActive(false),
      onDrop: (e: React.DragEvent) => void handleDrop(e),
    },
    removeAttachment: (index: number) =>
      setAttachments((prev) => prev.filter((_, i) => i !== index)),
    clearAttachments: () => setAttachments([]),
  };
}
