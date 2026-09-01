'use client';

import { useState, useEffect } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { api } from '@bookhaven/api-client';
import { Checkbox } from '@/components/ui/checkbox';

interface Genre {
  id: string;
  name: string;
}

interface GenreFilterProps {
  selectedGenreIds: string[];
  onGenreChange: (genreIds: string[]) => void;
}

export function GenreFilter({ selectedGenreIds, onGenreChange }: GenreFilterProps) {
  const [genres, setGenres] = useState<Genre[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchGenres() {
      try {
        const result = await api.genres.list();
        setGenres(result.items);
      } catch (err) {
        console.error('Failed to fetch genres:', err);
      } finally {
        setLoading(false);
      }
    }
    fetchGenres();
  }, []);

  const handleToggle = (genreId: string) => {
    if (selectedGenreIds.includes(genreId)) {
      onGenreChange(selectedGenreIds.filter((id) => id !== genreId));
    } else {
      onGenreChange([...selectedGenreIds, genreId]);
    }
  };

  if (loading) {
    return <div className="space-y-2">Загрузка жанров...</div>;
  }

  return (
    <div className="space-y-2">
      {genres.map((genre) => (
        <Checkbox
          key={genre.id}
          label={genre.name}
          checked={selectedGenreIds.includes(genre.id)}
          onChange={() => handleToggle(genre.id)}
        />
      ))}
    </div>
  );
}
