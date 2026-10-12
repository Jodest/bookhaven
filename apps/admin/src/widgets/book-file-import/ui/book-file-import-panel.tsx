'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation } from '@tanstack/react-query';
import { adminBooksApi } from '@/entities/book';
import { ApiError, ImportCreatedEntity, ImportReport } from '@bookhaven/api-client';
import {
  Badge,
  Button,
  Input,
  Label,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';

/**
 * CSV/JSON book import, the /import/books/file sub-page
 * (docs/features/import-by-entity.md, docs/features/catalog-manual-import.md).
 */
export function BookFileImportPanel() {
  const [file, setFile] = useState<File | null>(null);
  const [status, setStatus] = useState<'published' | 'pending'>('published');
  const [report, setReport] = useState<ImportReport | null>(null);
  const [error, setError] = useState<string | null>(null);

  const importMutation = useMutation({
    mutationFn: (selectedFile: File) => adminBooksApi.import(selectedFile, { status }),
    onSuccess: (data) => {
      setReport(data);
      setError(null);
    },
    onError: (err) => {
      setReport(null);
      let message = 'Не удалось выполнить импорт. Проверьте файл и попробуйте снова.';
      if (err instanceof ApiError) {
        if (err.status === 413) {
          message = 'Файл слишком большой. Разбейте его на части или используйте cmd/harvest / cmd/seed.';
        } else if (err.status === 422) {
          message =
            err.message ||
            'Слишком много строк в файле. Используйте cmd/harvest / cmd/seed для больших партий.';
        } else if (err.status === 400) {
          message = err.message || 'Не удалось разобрать файл. Проверьте формат (.csv/.json).';
        }
      }
      setError(message);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) {
      setError('Выберите файл .csv или .json для импорта.');
      return;
    }
    setError(null);
    importMutation.mutate(file);
  };

  return (
    <div className="space-y-6">
      <form onSubmit={handleSubmit} className="max-w-xl space-y-4 rounded-lg border p-6">
        <div className="space-y-2">
          <Label htmlFor="import-file">Файл (.csv или .json)</Label>
          <Input
            id="import-file"
            type="file"
            accept=".csv,.json"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="import-status">Статус партии</Label>
          <Select
            id="import-status"
            options={[
              { value: 'published', label: 'Опубликованы' },
              { value: 'pending', label: 'На модерации' },
            ]}
            value={status}
            onChange={(e) => setStatus(e.target.value === 'pending' ? 'pending' : 'published')}
          />
        </div>

        <Button type="submit" disabled={importMutation.isPending}>
          {importMutation.isPending ? 'Импорт...' : 'Импортировать'}
        </Button>
      </form>

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-sm text-destructive">{error}</div>
      )}

      {report && (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2">
            <Badge variant="default">Обработано: {report.processed}</Badge>
            <Badge variant="default">Создано: {report.created}</Badge>
            <Badge variant="secondary">Пропущено (уже существуют): {report.skipped_existing}</Badge>
            <Badge variant={report.errors.length > 0 ? 'destructive' : 'outline'}>
              Ошибок: {report.errors.length}
            </Badge>
          </div>

          {report.errors.length > 0 && (
            <div className="rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Строка</TableHead>
                    <TableHead>Название</TableHead>
                    <TableHead>Ошибка</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {report.errors.map((rowError) => (
                    <TableRow key={`${rowError.row}-${rowError.message}`}>
                      <TableCell>{rowError.row}</TableCell>
                      <TableCell>{rowError.title || '—'}</TableCell>
                      <TableCell>{rowError.message}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          <CreatedEntitiesList
            title="Созданные авторы"
            entities={report.created_authors ?? []}
            hrefBase="/catalog/authors"
          />
          <CreatedEntitiesList
            title="Созданные жанры"
            entities={report.created_genres ?? []}
            hrefBase="/catalog/genres"
            note="Новые жанры созданы на модерации и не видны на сайте до публикации."
          />
        </div>
      )}
    </div>
  );
}

interface CreatedEntitiesListProps {
  title: string;
  entities: ImportCreatedEntity[];
  hrefBase: '/catalog/authors' | '/catalog/genres';
  note?: string;
}

// CreatedEntitiesList shows the authors/genres the import created, each
// linked to its edit page. Nothing is rendered for an empty list.
function CreatedEntitiesList({ title, entities, hrefBase, note }: CreatedEntitiesListProps) {
  if (entities.length === 0) {
    return null;
  }
  return (
    <section className="space-y-2 rounded-lg border p-4">
      <h3 className="text-sm font-semibold">
        {title} ({entities.length})
      </h3>
      {note && <p className="text-sm text-muted-foreground">{note}</p>}
      <ul className="space-y-1 text-sm">
        {entities.map((entity) => (
          <li key={entity.id}>
            <Link
              href={`${hrefBase}/${encodeURIComponent(entity.id)}/edit` as any}
              className="underline"
            >
              {entity.name}
            </Link>{' '}
            <span className="text-muted-foreground">строка {entity.row}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
