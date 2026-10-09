import type { ReactNode } from 'react';
import type { UseMutationResult } from '@tanstack/react-query';
import type { PaginatedResponse } from '@bookhaven/api-client';

export type CatalogStatus = 'pending' | 'published' | 'rejected';

export interface CatalogFeedback {
  type: 'success' | 'error';
  message: string;
}

export interface CatalogStatusChange {
  ids: string[];
  status: CatalogStatus;
}

export interface CatalogListState {
  /** undefined = all statuses. */
  status?: CatalogStatus;
  search?: string;
  page: number;
  limit: number;
  /** Keyed by filter param; raw URL values (the view's buildParams sanitises them). */
  filters: Record<string, string | undefined>;
}

export interface CatalogListFilter {
  /** URL key, e.g. 'author_id' | 'genre_id' | 'source'. */
  param: string;
  /** Select aria-label, e.g. 'Автор'. */
  label: string;
  /** The first option with value '' means "any". */
  options: { value: string; label: string }[];
}

export interface CatalogRowContext {
  notify: (feedback: CatalogFeedback | null) => void;
}

export interface CatalogListColumn<TItem> {
  id: string;
  header: ReactNode;
  headClassName?: string;
  cellClassName?: string;
  cell: (item: TItem) => ReactNode;
}

export interface CatalogListCopy {
  searchPlaceholder: string;
  loadError: string;
  empty: string;
  statusErrorFallback: string;
  unpublishConfirm: (name: string) => string;
  unpublishSuccess: (name: string) => string;
}

export interface CatalogListProps<TItem, TParams, TReport extends { errors: { message: string }[] }> {
  title: string;
  /** Rendered next to the h1, e.g. the «Очередь модерации» link. */
  headerExtra?: ReactNode;
  /** e.g. '/catalog/authors'; edit href = `${entityPath}/${id}/edit?back=…`. */
  entityPath: string;
  /** sessionStorage flash key consumed on mount, e.g. 'catalog.authors'. */
  flashKey: string;
  /** Query key prefix; the widget appends the built params. */
  queryKey: readonly unknown[];
  buildParams: (state: CatalogListState) => TParams;
  queryFn: (params: TParams) => Promise<PaginatedResponse<TItem>>;
  /** Result of the entity status-mutation hook (called in the view). */
  statusMutation: Pick<UseMutationResult<TReport, Error, CatalogStatusChange>, 'mutate' | 'isPending'>;
  getId: (item: TItem) => string;
  getName: (item: TItem) => string;
  getStatus: (item: TItem) => CatalogStatus;
  /** Status select labels (the entity's *_STATUS_LABELS). */
  statusLabels: Record<CatalogStatus, string>;
  filters?: CatalogListFilter[];
  columns: CatalogListColumn<TItem>[];
  /** Entity extras, rendered after «Редактировать» / «Снять с публикации». */
  renderRowActions?: (item: TItem, ctx: CatalogRowContext) => ReactNode;
  copy: CatalogListCopy;
  /** Page size; default 20. */
  limit?: number;
}
