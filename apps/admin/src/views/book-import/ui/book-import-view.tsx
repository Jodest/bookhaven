'use client';

import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import Link from 'next/link';
import { adminBooksApi } from '@/entities/book';
import { ApiError, ImportReport } from '@bookhaven/api-client';
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

export function BookImportView() {
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
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight">Импорт книг</h1>
        <Link href={'/catalog/books' as any}>
          <Button variant="outline">К списку книг</Button>
        </Link>
      </div>

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
        </div>
      )}
    </div>
  );
}
