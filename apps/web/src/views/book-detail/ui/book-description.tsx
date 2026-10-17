'use client';

import { cn } from '@bookhaven/ui';
import { useState } from 'react';

interface BookDescriptionProps {
  text: string;
  isAI: boolean;
}

// Deterministic (SSR-safe) "long text" heuristic: no layout measurement.
function isLongText(text: string): boolean {
  return text.length > 400 || (text.match(/\n/g)?.length ?? 0) > 4;
}

export function BookDescription({ text, isAI }: BookDescriptionProps) {
  const [expanded, setExpanded] = useState(false);
  const long = isLongText(text);

  return (
    <div>
      <p
        className={cn(
          'whitespace-pre-line text-sm leading-relaxed text-muted-foreground md:text-base',
          long && !expanded && 'line-clamp-6'
        )}
      >
        {text}
      </p>
      {long && (
        <button
          type="button"
          className="mt-1 text-sm text-primary hover:underline"
          aria-expanded={expanded}
          onClick={() => setExpanded((v) => !v)}
        >
          {expanded ? 'Свернуть' : 'Ещё'}
        </button>
      )}
      {isAI && (
        <p className="mt-2 text-xs text-muted-foreground">
          Описание создано ИИ и может содержать неточности.
        </p>
      )}
    </div>
  );
}
