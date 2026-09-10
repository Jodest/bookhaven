import type { Metadata } from 'next';
import { Inter, Literata } from 'next/font/google';
import { Header } from '@/widgets/header';
import { Footer } from '@/widgets/footer';
import { Providers } from './providers';
import './globals.css';

const inter = Inter({ subsets: ['latin', 'cyrillic'] });
const literata = Literata({
  subsets: ['latin', 'cyrillic'],
  weight: ['400', '500', '600', '700'],
  variable: '--font-display',
});

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
      <body className={`${inter.className} ${literata.variable} flex min-h-screen flex-col`}>
        <Providers>
          <Header />
          <div className="flex-1">{children}</div>
          <Footer />
        </Providers>
      </body>
    </html>
  );
}
