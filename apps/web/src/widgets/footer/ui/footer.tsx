'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { genresApi } from '@/entities/genre';
import { useAuthStore } from '@/entities/session';
import { Genre } from '@bookhaven/api-client';

const GENRE_LIMIT = 6;

export function Footer() {
  const { user, isAuthenticated } = useAuthStore();
  const [genres, setGenres] = useState<Genre[]>([]);

  useEffect(() => {
    genresApi
      .list()
      .then((res) => setGenres(res.items.slice(0, GENRE_LIMIT)))
      .catch(() => setGenres([]));
  }, []);

  return (
    <footer className="mt-16 bg-foreground text-background">
      <div className="container mx-auto grid grid-cols-1 gap-8 px-4 py-12 sm:grid-cols-2 sm:px-6 md:py-16 lg:grid-cols-3 lg:px-8">
        <div>
          <h2 className="font-display text-lg font-semibold">Каталог</h2>
          <ul className="mt-4 space-y-2 text-sm text-background/80">
            <li>
              <Link href="/books" className="hover:text-background hover:underline">
                Все книги
              </Link>
            </li>
            {genres.map((genre) => (
              <li key={genre.id}>
                <Link
                  href={`/books?genre_id=${genre.id}`}
                  className="hover:text-background hover:underline"
                >
                  {genre.name}
                </Link>
              </li>
            ))}
          </ul>
        </div>

        <div>
          <h2 className="font-display text-lg font-semibold">Аккаунт</h2>
          <ul className="mt-4 space-y-2 text-sm text-background/80">
            {isAuthenticated && user ? (
              <>
                <li>
                  <Link href="/profile" className="hover:text-background hover:underline">
                    Профиль
                  </Link>
                </li>
                <li>
                  <Link href="/shelves" className="hover:text-background hover:underline">
                    Мои полки
                  </Link>
                </li>
                <li>
                  <Link href="/lists" className="hover:text-background hover:underline">
                    Мои списки
                  </Link>
                </li>
                <li>
                  <Link href="/challenge" className="hover:text-background hover:underline">
                    Читательский вызов
                  </Link>
                </li>
              </>
            ) : (
              <>
                <li>
                  <Link href="/login" className="hover:text-background hover:underline">
                    Войти
                  </Link>
                </li>
                <li>
                  <Link href="/register" className="hover:text-background hover:underline">
                    Регистрация
                  </Link>
                </li>
              </>
            )}
          </ul>
        </div>

        <div>
          <h2 className="font-display text-lg font-semibold">О проекте</h2>
          <p className="mt-4 text-sm leading-relaxed text-background/80">
            BookHaven — социальный каталог книг: находите книги по жанру и автору,
            ставьте оценки, пишите рецензии, ведите полки чтения и собственные
            подборки, следите за читательским вызовом.
          </p>
        </div>
      </div>

      <div className="border-t border-background/10">
        <div className="container mx-auto px-4 py-4 text-center text-xs text-background/60 sm:px-6 lg:px-8">
          © {new Date().getFullYear()} BookHaven
        </div>
      </div>
    </footer>
  );
}
