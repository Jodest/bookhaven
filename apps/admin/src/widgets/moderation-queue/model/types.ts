import type { ReactNode } from 'react';
import type { UseMutationResult } from '@tanstack/react-query';
import type { PaginatedResponse } from '@bookhaven/api-client';

export type ModerationStatus = 'pending' | 'published' | 'rejected';

export interface ModerationFeedback {
  type: 'success' | 'error';
  message: string;
}

export interface ModerationStatusChange {
  ids: string[];
  status: ModerationStatus;
}

export interface ModerationStatusReport {
  processed: number;
  updated: number;
  errors: { message: string }[];
}

export interface ModerationQueueState {
  status: ModerationStatus;
  search?: string;
  page: number;
  limit: number;
  /** Keyed by filter param; values are validated against the filter options. */
  filters: Record<string, string | undefined>;
}

export interface ModerationQueueFilter {
  /** URL key, e.g. 'qid' | 'books' | 'source'. */
  param: string;
  /** Select aria-label, e.g. 'QID', 'Книги'. */
  label: string;
  /** The first option with value '' means "any". */
  options: { value: string; label: string }[];
}

export interface ModerationQueueControls {
  notify: (feedback: ModerationFeedback | null) => void;
  clearSelection: () => void;
}

export interface ModerationRowContext extends ModerationQueueControls {
  /** Row status change incl. confirm (the widget-owned flow). */
  changeStatus: (next: ModerationStatus) => void;
  statusPending: boolean;
}

export interface ModerationQueueColumn<TItem> {
  id: string;
  header: ReactNode;
  headClassName?: string;
  /** Full TableCell className, e.g. 'align-top text-sm'. */
  cellClassName?: string;
  cell: (item: TItem, ctx: ModerationRowContext) => ReactNode;
}

export interface ModerationQueueCopy<TItem, TReport> {
  title: string;
  searchPlaceholder: string;
  loadError: string;
  emptyPending: string;
  emptyFiltered: string;
  statusErrorFallback: string;
  rowConfirm: (next: ModerationStatus, item: TItem) => string;
  bulkConfirm: (next: ModerationStatus, items: TItem[]) => string;
  rowSuccess: (next: ModerationStatus, name: string) => string;
  bulkSummary: (report: TReport, names: Record<string, string>) => string;
}

export interface ModerationQueueProps<TItem, TParams, TReport extends ModerationStatusReport> {
  /** Query key prefix; the widget appends the built params. */
  queryKey: readonly unknown[];
  buildParams: (state: ModerationQueueState) => TParams;
  queryFn: (params: TParams) => Promise<PaginatedResponse<TItem>>;
  /** Result of the entity status-mutation hook (called in the view). */
  statusMutation: Pick<UseMutationResult<TReport, Error, ModerationStatusChange>, 'mutate' | 'isPending'>;
  getId: (item: TItem) => string;
  getName: (item: TItem) => string;
  getStatus: (item: TItem) => ModerationStatus;
  filters?: ModerationQueueFilter[];
  columns: ModerationQueueColumn<TItem>[];
  renderRowActions?: (item: TItem, ctx: ModerationRowContext) => ReactNode;
  renderBulkActions?: (selected: TItem[], controls: ModerationQueueControls) => ReactNode;
  renderOverlay?: (controls: ModerationQueueControls) => ReactNode;
  copy: ModerationQueueCopy<TItem, TReport>;
  /** Page size; default 20. */
  limit?: number;
}
