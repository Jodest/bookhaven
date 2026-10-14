import { Badge } from '@bookhaven/ui';

interface AiDescriptionTextProps {
  text: string;
  /** Prompt or book data changed since generation. */
  stale?: boolean;
  /** The book now has its own description. */
  superseded?: boolean;
}

/**
 * An AI description as plain text (React escapes it; never
 * dangerouslySetInnerHTML) with the «устарел» and «есть своё описание»
 * marks.
 */
export function AiDescriptionText({ text, stale = false, superseded = false }: AiDescriptionTextProps) {
  return (
    <div className="space-y-1">
      {(stale || superseded) && (
        <div className="flex flex-wrap gap-1">
          {stale && (
            <Badge variant="outline" title="С генерации изменились данные книги или версия промпта">
              устарел
            </Badge>
          )}
          {superseded && <Badge variant="secondary">есть своё описание</Badge>}
        </div>
      )}
      {text && <p className="whitespace-pre-line text-sm">{text}</p>}
    </div>
  );
}
