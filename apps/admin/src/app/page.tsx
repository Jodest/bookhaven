'use client';

import { api } from '@bookhaven/api-client';
import { useQuery } from '@tanstack/react-query';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@bookhaven/ui';
import Link from 'next/link';

export default function DashboardPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'dashboard'],
    queryFn: () => api.admin.dashboard(),
  });

  if (isLoading) {
    return (
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="h-32 animate-pulse rounded-lg bg-muted" />
        ))}
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="rounded-md bg-destructive/10 p-6 text-destructive">
        Не удалось загрузить данные панели управления.
      </div>
    );
  }

  const cards = [
    { label: 'Всего книг', value: data.books, href: '/books' },
    { label: 'Скрытых книг', value: data.hidden_books, href: '/books?visibility=hidden' },
    { label: 'Авторов', value: data.authors, href: '/authors' },
    { label: 'Жанров', value: data.genres, href: '/genres' },
    { label: 'Отзывов на модерации', value: data.pending_reviews, href: '/reviews?visibility=hidden' },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Обзор</h1>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {cards.map((card) => (
          <Link key={card.label} href={card.href as any}>
            <Card className="transition-shadow hover:shadow-md">
              <CardHeader className="pb-2">
                <CardDescription>{card.label}</CardDescription>
                <CardTitle className="text-4xl">{card.value}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-primary">Перейти →</p>
              </CardContent>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
