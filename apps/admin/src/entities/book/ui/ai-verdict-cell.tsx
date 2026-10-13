import type { AdminBookAIVerdict } from '@bookhaven/api-client';
import { Badge, cn } from '@bookhaven/ui';
import { AI_VERDICT_LABELS, aiReasonLabel } from '../lib/ai-verdict';

const VERDICT_CLASSES: Record<AdminBookAIVerdict['verdict'], string> = {
  publish: 'border-transparent bg-green-100 text-green-900',
  reject: 'border-transparent bg-red-100 text-red-900',
  needs_human: 'border-transparent bg-amber-100 text-amber-900',
};

interface AiVerdictCellProps {
  verdict: AdminBookAIVerdict | null;
}

/**
 * The «ИИ» column of the book queue: verdict badge, confidence, reason-code
 * labels, the reason (plain text, clamped to 3 lines with the full text in
 * the title) and «устарел» for a stale verdict. The reason comes from the
 * model: never render it as HTML.
 */
export function AiVerdictCell({ verdict }: AiVerdictCellProps) {
  if (!verdict) return <span className="text-muted-foreground">—</span>;
  const label = AI_VERDICT_LABELS[verdict.verdict] ?? verdict.verdict;
  return (
    <div className="space-y-1">
      <div className="flex flex-wrap items-center gap-1">
        <Badge variant="outline" className={cn(VERDICT_CLASSES[verdict.verdict])}>
          {label}
        </Badge>
        <span className="text-xs text-muted-foreground">{Math.round(verdict.confidence * 100)}%</span>
        {verdict.stale && <span className="text-xs text-muted-foreground">устарел</span>}
      </div>
      {verdict.reason_codes.length > 0 && (
        <div className="text-xs">{verdict.reason_codes.map(aiReasonLabel).join(', ')}</div>
      )}
      {verdict.reason && (
        <p className="line-clamp-3 text-xs text-muted-foreground" title={verdict.reason}>
          {verdict.reason}
        </p>
      )}
    </div>
  );
}
