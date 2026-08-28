import { Button } from '@bookhaven/ui';

export default function HomePage() {
  return (
    <main className="container mx-auto px-4 py-16">
      <div className="mx-auto max-w-2xl text-center">
        <h1 className="mb-6 text-4xl font-bold tracking-tight sm:text-6xl">
          BookHaven
        </h1>
        <p className="mb-8 text-lg text-muted-foreground">
          Социальный каталог книг с оценками, ревью и подборками
        </p>
        <div className="flex justify-center gap-4">
          <Button size="lg">Найти книгу</Button>
          <Button size="lg" variant="outline">
            Мои полки
          </Button>
        </div>
      </div>
    </main>
  );
}
