'use client';

import { useState, useEffect } from 'react';
import { api } from '@bookhaven/api-client';
import { Select } from '@/components/ui/select';

interface Language {
  code: string;
  name: string;
}

interface LanguageFilterProps {
  selectedLanguage: string | null;
  onLanguageChange: (language: string | null) => void;
}

export function LanguageFilter({
  selectedLanguage,
  onLanguageChange,
}: LanguageFilterProps) {
  const [languages, setLanguages] = useState<Language[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchLanguages() {
      try {
        const result = await api.languages.list();
        setLanguages(result.items);
      } catch (err) {
        console.error('Failed to fetch languages:', err);
      } finally {
        setLoading(false);
      }
    }
    fetchLanguages();
  }, []);

  const options = [
    { value: '', label: 'Все языки' },
    ...languages.map((lang) => ({
      value: lang.code,
      label: lang.name,
    })),
  ];

  if (loading) {
    return <div className="text-sm text-muted-foreground">Загрузка...</div>;
  }

  return (
    <Select
      options={options}
      value={selectedLanguage ?? ''}
      onChange={(e) => onLanguageChange(e.target.value || null)}
    />
  );
}
