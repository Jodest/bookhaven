import { Button } from '@bookhaven/ui';

export default function AdminPage() {
  return (
    <main className="container mx-auto px-4 py-16">
      <div className="mx-auto max-w-2xl text-center">
        <h1 className="mb-6 text-4xl font-bold tracking-tight">
          BookHaven Admin
        </h1>
        <p className="mb-8 text-lg text-muted-foreground">
          Панель управления каталогом книг
        </p>
        <div className="flex justify-center gap-4">
          <Button>Управление книгами</Button>
          <Button variant="outline">Модерация</Button>
        </div>
      </div>
    </main>
  );
}
