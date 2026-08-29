# AI Feature Development Artifacts

Эта директория содержит артефакты, создаваемые мультиагентным процессом разработки BookHaven.

Каждая фича имеет собственную директорию:

`docs/ai/features/<feature-slug>/`

## Файлы

### business-analysis.md

Бизнес-, продуктовый и UX-анализ фичи.

### architecture.md

Техническая архитектура и план реализации.

### implementation.md

Описание фактической реализации.

### review.md

Результат Code Review.

### qa.md

Результат QA.

### documentation.md

Отчёт об обновлении документации.

### git.md

Результат создания Git commit.

### state.md

Текущее состояние pipeline разработки.

## Процесс

Стандартный жизненный цикл:

Business Analysis
→ Architecture
→ Development
→ Code Review
→ QA
→ Documentation
→ Git

Если Code Review завершился FAIL:

Development
→ Code Review

Если QA завершился FAIL:

Development
→ Code Review
→ QA

Фича считается завершённой только после успешного QA, обновления документации и Git commit.