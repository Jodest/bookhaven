import { LoginForm } from '@/features/auth';

export function LoginView() {
  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-8">
      <div className="w-full max-w-md space-y-6 rounded-lg border bg-card p-8 text-card-foreground shadow-sm">
        <div className="text-center">
          <h1 className="text-2xl font-bold tracking-tight">Вход в BookHaven</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Введите email и пароль, чтобы продолжить
          </p>
        </div>
        <LoginForm />
      </div>
    </main>
  );
}
