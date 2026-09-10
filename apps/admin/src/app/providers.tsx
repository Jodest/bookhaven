'use client';

import { SessionInitializer, createAdminQueryClient } from '@/entities/session';
import { QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => createAdminQueryClient());

  return (
    <QueryClientProvider client={queryClient}>
      <SessionInitializer />
      {children}
    </QueryClientProvider>
  );
}
