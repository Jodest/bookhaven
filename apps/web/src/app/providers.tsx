'use client';

import { SessionInitializer } from '@/entities/session';
import { createQueryClient } from '@/shared/api';
import { QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => createQueryClient());

  return (
    <QueryClientProvider client={queryClient}>
      <SessionInitializer />
      {children}
    </QueryClientProvider>
  );
}
