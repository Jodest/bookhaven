'use client';

import { useId, useState } from 'react';
import { Button, Label, Textarea } from '@bookhaven/ui';
import { MAX_EDITED_LENGTH } from '../lib/review-text';

interface ApproveEditorProps {
  initialText: string;
  isPending: boolean;
  onSubmit: (text: string) => void;
  onCancel: () => void;
}

/** «Изменить и одобрить»: an editable copy of the text with a counter. */
export function ApproveEditor({ initialText, isPending, onSubmit, onCancel }: ApproveEditorProps) {
  const id = useId();
  const [text, setText] = useState(initialText);
  const trimmed = text.trim();
  const canSubmit = !isPending && trimmed.length > 0 && text.length <= MAX_EDITED_LENGTH;

  return (
    <div className="space-y-2">
      <Label htmlFor={id}>Текст описания</Label>
      <Textarea
        id={id}
        value={text}
        maxLength={MAX_EDITED_LENGTH}
        rows={5}
        onChange={(e) => setText(e.target.value)}
      />
      <div className="text-right text-xs text-muted-foreground" aria-live="polite">
        {text.length} / {MAX_EDITED_LENGTH}
      </div>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" disabled={!canSubmit} onClick={() => onSubmit(trimmed)}>
          Одобрить с изменениями
        </Button>
        <Button size="sm" variant="outline" disabled={isPending} onClick={onCancel}>
          Отмена
        </Button>
      </div>
    </div>
  );
}
