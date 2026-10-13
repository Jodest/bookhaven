import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { AdminBookAIVerdict } from '@bookhaven/api-client';
import { AiVerdictCell } from './ai-verdict-cell';

const base: AdminBookAIVerdict = {
  verdict: 'reject',
  reason_codes: ['collection', 'model_truncated', 'something_new'],
  reason: 'Это сборник рассказов.',
  confidence: 0.876,
  model: 'claude-haiku-5-5',
  prompt_version: 1,
  created_at: '2026-10-13T10:00:00Z',
  stale: false,
};

describe('AiVerdictCell', () => {
  it('renders «—» without a verdict', () => {
    render(<AiVerdictCell verdict={null} />);
    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('renders the badge, confidence, code labels (unknown raw) and the reason', () => {
    render(<AiVerdictCell verdict={base} />);
    expect(screen.getByText('Отклонить')).toBeInTheDocument();
    expect(screen.getByText('88%')).toBeInTheDocument();
    expect(screen.getByText('сборник, ответ обрезан, something_new')).toBeInTheDocument();
    const reason = screen.getByText('Это сборник рассказов.');
    expect(reason).toHaveAttribute('title', 'Это сборник рассказов.');
    expect(screen.queryByText('устарел')).not.toBeInTheDocument();
  });

  it('labels every verdict and marks a stale one', () => {
    const { rerender } = render(<AiVerdictCell verdict={{ ...base, verdict: 'publish', reason_codes: [], stale: true }} />);
    expect(screen.getByText('Опубликовать')).toBeInTheDocument();
    expect(screen.getByText('устарел')).toBeInTheDocument();
    rerender(<AiVerdictCell verdict={{ ...base, verdict: 'needs_human' }} />);
    expect(screen.getByText('Нужен человек')).toBeInTheDocument();
  });

  it('renders the reason as plain text, never as HTML', () => {
    const { container } = render(<AiVerdictCell verdict={{ ...base, reason: '<b>жирный</b><script>x()</script>' }} />);
    expect(container.querySelector('b')).toBeNull();
    expect(container.querySelector('script')).toBeNull();
    expect(screen.getByText('<b>жирный</b><script>x()</script>')).toBeInTheDocument();
  });
});
