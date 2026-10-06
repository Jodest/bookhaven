'use client';

import { useState } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import Link from 'next/link';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAuthorsApi, AuthorStatusBadge, AUTHOR_STATUS_LABELS } from '@/entities/author';
import { AdminAuthor, AdminAuthorInput, AdminAuthorStatus } from '@bookhaven/api-client';
import {
  Input,
  Button,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { Search } from 'lucide-react';
import { Pagination } from '@/shared/ui';
import { AuthorForm } from '@/features/manage-author';
import { MergeAuthorDialog } from '@/features/merge-author';

const STATUS_FILTER_OPTIONS = [
  { value: '', label: 'Все статусы' },
  { value: 'pending', label: AUTHOR_STATUS_LABELS.pending },
  { value: 'published', label: AUTHOR_STATUS_LABELS.published },
  { value: 'rejected', label: AUTHOR_STATUS_LABELS.rejected },
];

function parseStatus(value: string | null): AdminAuthorStatus | undefined {
  return value === 'pending' || value === 'published' || value === 'rejected' ? value : undefined;
}

export function AuthorsView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [editing, setEditing] = useState<AdminAuthor | null>(null);
  const [formData, setFormData] = useState<AdminAuthorInput>({ name: '' });
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [mergingDuplicate, setMergingDuplicate] = useState<AdminAuthor | null>(null);
  const [mergeSuccessMessage, setMergeSuccessMessage] = useState<string | null>(null);

  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10));
  const status = parseStatus(searchParams.get('status'));
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'authors', { search, page, status }],
    queryFn: () =>
      adminAuthorsApi.list({
        search: search || undefined,
        page,
        limit,
        status,
      }),
  });

  const createMutation = useMutation({
    mutationFn: adminAuthorsApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      setFormData({ name: '' });
      setSuccessMessage('Автор создан');
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: AdminAuthorInput }) =>
      adminAuthorsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      setEditing(null);
      setFormData({ name: '' });
      setSuccessMessage('Автор обновлен');
    },
  });

  const updateParams = (updates: Record<string, string | undefined>) => {
    const params = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') {
        params.delete(key);
      } else {
        params.set(key, value);
      }
    });
    router.push(`${pathname}?${params.toString()}` as any);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    updateParams({ search, page: '1' });
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSuccessMessage(null);
    if (editing) {
      updateMutation.mutate({ id: editing.id, data: formData });
    } else {
      createMutation.mutate(formData);
    }
  };

  const startEdit = (author: AdminAuthor) => {
    setSuccessMessage(null);
    setEditing(author);
    setFormData({
      name: author.name,
      slug: author.slug,
      biography: author.biography ?? '',
      photo_url: author.photo_url ?? '',
      birth_date: author.birth_date ?? '',
      death_date: author.death_date ?? '',
      status: author.status,
    });
  };

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight">Авторы</h1>
        <Link href={'/authors/moderation' as any}>
          <Button variant="outline">Очередь модерации</Button>
        </Link>
      </div>

      {mergeSuccessMessage && (
        <div className="flex items-center justify-between rounded-md bg-green-100 p-3 text-sm text-green-900">
          {mergeSuccessMessage}
          <button
            type="button"
            onClick={() => setMergeSuccessMessage(null)}
            className="ml-4 underline"
          >
            Скрыть
          </button>
        </div>
      )}

      <AuthorForm
        formData={formData}
        onFieldChange={(patch) => setFormData((prev) => ({ ...prev, ...patch }))}
        editing={editing}
        onSubmit={handleSubmit}
        onCancel={() => {
          setEditing(null);
          setFormData({ name: '' });
          setSuccessMessage(null);
        }}
        pending={createMutation.isPending || updateMutation.isPending}
        successMessage={successMessage}
        onDismissSuccess={() => setSuccessMessage(null)}
        errorMessage={createMutation.error?.message || updateMutation.error?.message}
      />

      <form onSubmit={handleSearch} className="flex gap-2">
        <Input
          placeholder="Поиск по имени"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Button type="submit" variant="outline">
          <Search className="h-4 w-4" />
        </Button>
        <Select
          aria-label="Статус"
          options={STATUS_FILTER_OPTIONS}
          value={status ?? ''}
          onChange={(e) => updateParams({ status: e.target.value, page: '1' })}
          className="w-full md:w-48"
        />
      </form>

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить авторов.
        </div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      ) : data?.items.length === 0 ? (
        <div className="rounded-lg border p-8 text-center text-muted-foreground">
          Авторы не найдены.
        </div>
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Имя</TableHead>
                  <TableHead>Slug</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.items.map((author: AdminAuthor) => (
                  <TableRow key={author.id}>
                    <TableCell className="font-medium">{author.name}</TableCell>
                    <TableCell>{author.slug}</TableCell>
                    <TableCell>
                      <AuthorStatusBadge status={author.status} />
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => startEdit(author)}>
                        Редактировать
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setMergeSuccessMessage(null);
                          setMergingDuplicate(author);
                        }}
                      >
                        Слить
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => updateParams({ page: String(p) })}
          />
        </>
      )}

      {mergingDuplicate && (
        <MergeAuthorDialog
          duplicate={mergingDuplicate}
          onClose={() => setMergingDuplicate(null)}
          onMerged={(booksReassigned) => {
            setMergingDuplicate(null);
            setMergeSuccessMessage(
              `Автор «${mergingDuplicate.name}» слит с каноническим автором. Перенесено книг: ${booksReassigned}.`
            );
          }}
        />
      )}
    </div>
  );
}
