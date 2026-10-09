interface WikidataLinkProps {
  qid: string | null;
}

/** Wikidata entity link (opens in a new tab); renders a muted dash when there is no QID. */
export function WikidataLink({ qid }: WikidataLinkProps) {
  if (!qid) return <span className="text-muted-foreground">—</span>;
  return (
    <a
      href={`https://www.wikidata.org/wiki/${qid}`}
      target="_blank"
      rel="noopener noreferrer"
      className="font-mono text-xs text-primary underline"
    >
      {qid}
    </a>
  );
}
