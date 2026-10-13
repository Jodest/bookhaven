'use client';

import { FormEvent, useState } from 'react';
import type { AdminCurationStatus } from '@bookhaven/api-client';
import { Button, Input, Label, Textarea } from '@bookhaven/ui';
import { createCurationErrorText } from '../lib/create-error-text';
import { useCreateCuration } from '../model/use-curation';

interface CurationPromptFormProps {
  status: AdminCurationStatus;
  /** Called with the new request id after a successful submit. */
  onCreated: (id: string) => void;
}

/**
 * The «Подбор ИИ» request form: a prompt with a character counter (limit
 * from status, 500), «Сколько авторов» (1–30, default 20) and «Подобрать
 * авторов». When AI curation is off it only says so (no submit); submit is
 * disabled while the admin has an active request.
 */
export function CurationPromptForm({ status, onCreated }: CurationPromptFormProps) {
  const { create, isPending } = useCreateCuration();
  const [prompt, setPrompt] = useState('');
  const [maxNames, setMaxNames] = useState(String(status.default_names));
  const [error, setError] = useState<string | null>(null);

  if (!status.enabled) {
    return (
      <div className="rounded-md border p-4 text-sm">
        <p className="font-medium">Подбор ИИ выключен</p>
        <p className="text-muted-foreground">
          Включите AI_CURATOR_ENABLED и задайте ANTHROPIC_API_KEY для api и воркера. Недавние подборки
          по-прежнему доступны ниже.
        </p>
      </div>
    );
  }

  const length = prompt.length;
  const trimmed = prompt.trim();
  const count = Number(maxNames);
  const countValid = Number.isInteger(count) && count >= 1 && count <= status.max_names;
  const active = status.active_request_id !== null;
  const limitReached = status.used_today >= status.daily_limit;
  const canSubmit =
    !isPending && !active && trimmed.length > 0 && length <= status.max_prompt_length && countValid;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setError(null);
    try {
      const id = await create({ prompt: trimmed, max_names: count });
      setPrompt('');
      onCreated(id);
    } catch (err) {
      setError(createCurationErrorText(err));
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="curation-prompt">Каких авторов не хватает в каталоге?</Label>
        <Textarea
          id="curation-prompt"
          value={prompt}
          maxLength={status.max_prompt_length}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="Например: русская классика XIX века или лауреаты премии «Хьюго»"
          rows={3}
        />
        <div className="text-right text-xs text-muted-foreground" aria-live="polite">
          {length} / {status.max_prompt_length}
        </div>
      </div>
      <div className="flex flex-wrap items-end gap-4">
        <div className="space-y-2">
          <Label htmlFor="curation-max-names">Сколько авторов</Label>
          <Input
            id="curation-max-names"
            type="number"
            min={1}
            max={status.max_names}
            value={maxNames}
            onChange={(e) => setMaxNames(e.target.value)}
            className="w-24"
          />
        </div>
        <Button type="submit" disabled={!canSubmit}>
          {isPending ? 'Отправка...' : 'Подобрать авторов'}
        </Button>
        <span className="text-xs text-muted-foreground">
          Сегодня: {status.used_today} из {status.daily_limit}
        </span>
      </div>
      {active && (
        <p className="text-sm text-muted-foreground">
          Ваша подборка ещё выполняется — новую можно создать после её завершения.
        </p>
      )}
      {!active && limitReached && (
        <p className="text-sm text-muted-foreground">Дневной лимит подборок исчерпан.</p>
      )}
      {error && (
        <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        ИИ предлагает только имена; каждое имя проверяется по Wikidata. Ничего не импортируется без вашего
        выбора, импортированные авторы ждут модерации.
      </p>
    </form>
  );
}
