import { RegisterForm } from '@/features/auth';

export function RegisterView() {
  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-6 rounded-xl border bg-card p-8 text-card-foreground shadow-sm">
        <div className="text-center">
          <h1 className="font-display text-2xl font-semibold tracking-tight">Регистрация</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Создайте аккаунт, чтобы отслеживать книги и оставлять рецензии
          </p>
        </div>
        <RegisterForm />
      </div>
    </main>
  );
}
