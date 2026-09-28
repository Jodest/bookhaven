import type { Metadata } from 'next';
import localFont from 'next/font/local';
import { Providers } from './providers';
import { Shell } from '@/widgets/admin-shell';
import './globals.css';

const inter = localFont({
  src: '../fonts/inter/Inter-Variable.woff2',
  weight: '100 900',
});

export const metadata: Metadata = {
  title: 'BookHaven Admin',
  description: 'Панель управления BookHaven',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru">
      <body className={inter.className}>
        <Providers>
          <Shell>{children}</Shell>
        </Providers>
      </body>
    </html>
  );
}
