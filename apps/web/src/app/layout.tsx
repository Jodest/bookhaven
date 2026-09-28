import type { Metadata } from 'next';
import localFont from 'next/font/local';
import { Header } from '@/widgets/header';
import { Footer } from '@/widgets/footer';
import { Providers } from './providers';
import './globals.css';

const inter = localFont({
  src: '../fonts/inter/Inter-Variable.woff2',
  weight: '100 900',
});
const literata = localFont({
  src: '../fonts/literata/Literata-Variable.woff2',
  weight: '400 700',
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
