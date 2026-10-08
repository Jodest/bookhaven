'use client';

import type { AdminHarvestJob } from '@bookhaven/api-client';
import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@bookhaven/ui';
import Link from 'next/link';
import { useDashboardStats } from '@/entities/dashboard';
import {
  countInFlightHarvestJobs,
  harvestJobStateBadgeVariant,
  useHarvestJobs,
} from '@/entities/harvest';

interface Tile {
  label: string;
  value: number;
  href: string;
}

const RECENT_JOBS_LIMIT = 5;

function TileGrid({ tiles }: { tiles: Tile[] }) {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {tiles.map((tile) => (
        <Link key={tile.label} href={tile.href as any}>
          <Card className="transition-shadow hover:shadow-md">
            <CardHeader className="pb-2">
              <CardDescription>{tile.label}</CardDescription>
              <CardTitle className="text-4xl">{tile.value}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-primary">Перейти →</p>
            </CardContent>
          </Card>
        </Link>
      ))}
    </div>
  );
}

function SkeletonGrid({ count }: { count: number }) {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="h-32 animate-pulse rounded-lg bg-muted" />
      ))}
    </div>
  );
}

function DashboardGroup({
  id,
  title,
  children,
}: {
  id: string;
  title: string;
  children: React.ReactNode;
}) {
  const headingId = `dashboard-group-${id}`;
  return (
    <section aria-labelledby={headingId} className="space-y-3">
      <h2 id={headingId} className="text-xl font-semibold tracking-tight">
        {title}
      </h2>
      {children}
    </section>
  );
}

// One-line status of a job in the compact dashboard list.
function JobStatusLine({ job }: { job: AdminHarvestJob }) {
  if (job.state === 'active') {
    if (job.result && job.result.total > 0) {
      const processed = Math.min(job.result.processed, job.result.total);
      return (
        <span className="text-muted-foreground">
          {processed} / {job.result.total}
        </span>
      );
    }
    return <span className="text-muted-foreground">Выполняется…</span>;
  }
  if (job.state === 'completed' && job.result) {
    return <span>создано {job.result.created}</span>;
  }
  if (job.last_error) {
    return (
      <span className="block truncate text-muted-foreground" title={job.last_error}>
        {job.last_error}
      </span>
    );
  }
  return <span className="text-muted-foreground">—</span>;
}

/**
 * Compact harvest jobs block: the first jobs in server order (in-flight first,
 * then completed, then failed - the API has no chronology, hence no
 * "последние" wording). Full table lives on /import/jobs.
 */
function RecentHarvestJobs() {
  const { data, isLoading, error } = useHarvestJobs();
  const items = data?.items;
  const inFlight = countInFlightHarvestJobs(items);

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center gap-3">
          <CardTitle className="text-lg">Задачи харвеста</CardTitle>
          {inFlight > 0 && <Badge variant="secondary">в работе: {inFlight}</Badge>}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <p className="text-sm text-muted-foreground">Загрузка…</p>
        ) : error || !items ? (
          <p className="text-sm text-destructive">Не удалось загрузить задачи харвеста.</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">Нет задач</p>
        ) : (
          <ul className="divide-y text-sm" aria-label="Задачи харвеста">
            {items.slice(0, RECENT_JOBS_LIMIT).map((job) => (
              <li key={job.id} className="flex items-center gap-3 py-2">
                <span className="font-mono text-xs">{job.id.slice(0, 12)}</span>
                <Badge variant={harvestJobStateBadgeVariant(job.state)}>{job.state}</Badge>
                <div className="min-w-0 flex-1">
                  <JobStatusLine job={job} />
                </div>
              </li>
            ))}
          </ul>
        )}
        <div className="flex flex-wrap gap-4 text-sm">
          <Link href={'/import/jobs' as any} className="text-primary hover:underline">
            Все задачи →
          </Link>
          <Link href={'/import/wikidata' as any} className="text-primary hover:underline">
            Запустить харвест
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}

export function DashboardView() {
  const { data, isLoading, error } = useDashboardStats();

  let counters: React.ReactNode;
  if (isLoading) {
    counters = (
      <>
        <DashboardGroup id="moderation" title="Модерация">
          <SkeletonGrid count={4} />
        </DashboardGroup>
        <DashboardGroup id="catalog" title="Каталог">
          <SkeletonGrid count={4} />
        </DashboardGroup>
      </>
    );
  } else if (error || !data) {
    counters = (
      <div className="rounded-md bg-destructive/10 p-6 text-destructive">
        Не удалось загрузить данные панели управления.
      </div>
    );
  } else {
    const moderationTiles: Tile[] = [
      {
        label: 'Импортировано, на проверке',
        value: data.pending_import_review,
        href: '/moderation/books',
      },
      { label: 'Авторы на модерации', value: data.pending_authors, href: '/moderation/authors' },
      {
        // Until the genre moderation queue (#302) exists, the tile opens the
        // catalog list filtered to pending genres.
        label: 'Жанры на модерации',
        value: data.pending_genres,
        href: '/catalog/genres?status=pending',
      },
      {
        label: 'Скрытых отзывов',
        value: data.pending_reviews,
        href: '/moderation/reviews?visibility=hidden',
      },
    ];
    const catalogTiles: Tile[] = [
      { label: 'Всего книг', value: data.books, href: '/catalog/books' },
      { label: 'Скрытых книг', value: data.hidden_books, href: '/catalog/books?visibility=hidden' },
      { label: 'Авторов', value: data.authors, href: '/catalog/authors' },
      { label: 'Жанров', value: data.genres, href: '/catalog/genres' },
    ];
    counters = (
      <>
        <DashboardGroup id="moderation" title="Модерация">
          <TileGrid tiles={moderationTiles} />
        </DashboardGroup>
        <DashboardGroup id="catalog" title="Каталог">
          <TileGrid tiles={catalogTiles} />
        </DashboardGroup>
      </>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Обзор</h1>
      {counters}
      <DashboardGroup id="import" title="Импорт">
        <RecentHarvestJobs />
      </DashboardGroup>
    </div>
  );
}
