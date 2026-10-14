import { formatSource } from '../lib/format-source';

interface SourceLineProps {
  source: string | null;
  url: string | null;
  className?: string;
}

/** «Источник: …», linked only when the collection has a source URL. */
export function SourceLine({ source, url, className }: SourceLineProps) {
  const text = formatSource(source);
  if (!text) return null;
  if (!url) return <p className={className}>{text}</p>;
  return (
    <p className={className}>
      <a href={url} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">
        {text}
      </a>
    </p>
  );
}
