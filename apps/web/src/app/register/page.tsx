import { RegisterForm } from '@/components/register-form';

export const metadata = {
  title: 'Регистрация — BookHaven',
};

export default function RegisterPage() {
  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-8">
      <div className="w-full max-w-md space-y-6 rounded-lg border bg-card p-8 text-card-foreground shadow-sm">
        <div className="text-center">
          <h1 className="text-2xl font-bold tracking-tight">Регистрация</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Создайте аккаунт, чтобы отслеживать книги и оставлять рецензии
          </p>
        </div>
        <RegisterForm />
      </div>
    </main>
  );
}
