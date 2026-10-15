'use client';

import type { BookCollectionPlace } from '@bookhaven/api-client';
import { Badge } from '@bookhaven/ui';
import Link from 'next/link';
import { useState } from 'react';
import { collectionPlaceHref } from '../lib/place-href';

const VISIBLE = 3;

interface CollectionPlaceBadgesProps {
  places: BookCollectionPlace[];
}

/** «№N в „…“» badges of a book: 3 visible, «Ещё N» reveals the rest. */
export function CollectionPlaceBadges({ places }: CollectionPlaceBadgesProps) {
  const [expanded, setExpanded] = useState(false);
  if (places.length === 0) return null;

  const shown = expanded ? places : places.slice(0, VISIBLE);
  const hidden = places.length - shown.length;

  return (
    <div aria-label="Места в подборках" className="flex flex-wrap items-center gap-2">
      {shown.map((p) => (
        <Link
          key={p.collection.id}
          href={collectionPlaceHref(p.collection.slug, p.place) as any}
        >
          <Badge variant="secondary" className="hover:bg-secondary/70">
            №{p.place} в „{p.collection.title}“
          </Badge>
        </Link>
      ))}
      {hidden > 0 && (
        <button
          type="button"
          onClick={() => setExpanded(true)}
          className="text-sm font-medium text-primary hover:underline"
        >
          Ещё {hidden}
        </button>
      )}
    </div>
  );
}
