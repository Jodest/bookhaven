import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { Header } from '@/widgets/header';
import { Providers } from './providers';
import './globals.css';

const inter = Inter({ subsets: ['latin', 'cyrillic'] });

export const metadata: Metadata = {
  title: 'BookHaven — каталог книг',
  description: 'Социальный каталог книг с оценками, ревью и подборками',
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
          <Header />
          {children}
        </Providers>
      </body>
    </html>
  );
}
