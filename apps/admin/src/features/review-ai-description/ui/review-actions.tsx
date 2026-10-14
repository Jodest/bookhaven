'use client';

import { useState } from 'react';
import type { AIDescriptionStatus } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { APPROVED_TEXT, DISCARDED_TEXT, reviewErrorText } from '../lib/review-text';
import { useApproveAiDescription, useDiscardAiDescription } from '../model/use-review';
import { ApproveEditor } from './approve-editor';

/** Structurally the feedback banners of the panel and the review tab. */
export interface ReviewFeedback {
  type: 'success' | 'error';
  message: string;
}

interface ReviewActionsProps {
  bookId: string;
  status: AIDescriptionStatus;
  text: string;
  notify: (feedback: ReviewFeedback | null) => void;
}

/**
 * Review actions for one AI description: «Одобрить», «Изменить и
 * одобрить» and «Отклонить» on a draft; «Изменить и одобрить» and
 * «Отклонить» (takes it off the site) on an approved one. Nothing for the
 * other statuses. There is deliberately no bulk approval.
 */
export function ReviewActions({ bookId, status, text, notify }: ReviewActionsProps) {
  const approve = useApproveAiDescription();
  const discard = useDiscardAiDescription();
  const [editing, setEditing] = useState(false);
  const pending = approve.isPending || discard.isPending;

  if (status !== 'draft' && status !== 'approved') return null;

  const doApprove = (edited?: string) => {
    notify(null);
    approve.mutate(
      { bookId, text: edited },
      {
        onSuccess: () => {
          setEditing(false);
          notify({ type: 'success', message: APPROVED_TEXT });
        },
        onError: (err) => notify({ type: 'error', message: reviewErrorText(err) }),
      }
    );
  };

  const doDiscard = () => {
    notify(null);
    discard.mutate(bookId, {
      onSuccess: () => notify({ type: 'success', message: DISCARDED_TEXT }),
      onError: (err) => notify({ type: 'error', message: reviewErrorText(err) }),
    });
  };

  if (editing) {
    return (
      <ApproveEditor
        initialText={text}
        isPending={pending}
        onSubmit={(edited) => doApprove(edited)}
        onCancel={() => setEditing(false)}
      />
    );
  }

  return (
    <div className="flex flex-wrap gap-2" role="group" aria-label="Проверка описания ИИ">
      {status === 'draft' && (
        <Button size="sm" disabled={pending} onClick={() => doApprove()}>
          Одобрить
        </Button>
      )}
      <Button size="sm" variant="outline" disabled={pending} onClick={() => setEditing(true)}>
        Изменить и одобрить
      </Button>
      <Button size="sm" variant="outline" disabled={pending} onClick={doDiscard}>
        Отклонить
      </Button>
    </div>
  );
}
