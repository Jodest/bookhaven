interface PaginationProps {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

export function Pagination({ page, totalPages, onPageChange }: PaginationProps) {
  if (totalPages <= 1) return null;

  return (
    <div className="flex items-center justify-center gap-2">
      <button
        onClick={() => onPageChange(page - 1)}
        disabled={page <= 1}
        className="rounded-md border px-4 py-2 text-sm hover:bg-accent disabled:opacity-50"
      >
        Назад
      </button>
      <span className="px-4 py-2 text-sm">
        Страница {page} из {totalPages}
      </span>
      <button
        onClick={() => onPageChange(page + 1)}
        disabled={page >= totalPages}
        className="rounded-md border px-4 py-2 text-sm hover:bg-accent disabled:opacity-50"
      >
        Вперед
      </button>
    </div>
  );
}
