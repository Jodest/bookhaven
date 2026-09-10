'use client';

interface EmptyStateProps {
  message?: string;
}

export function EmptyState({
  message = 'По вашему запросу ничего не найдено. Попробуйте изменить фильтры.',
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="text-lg font-medium text-muted-foreground">{message}</div>
    </div>
  );
}
