import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

import { evaluate, run } from './vuln-gate.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const fixture = (name) => readFileSync(join(here, 'fixtures', name), 'utf8');
const TODAY = new Date('2026-10-23T12:00:00Z');

const entry = (over = {}) => ({
  id: 'GO-2099-0001',
  package: 'example.com/vuln',
  reason: 'test',
  issue: '#1',
  expires: '2026-12-31',
  ...over,
});
// `report` is a fixture file name (*.json) or a literal report text.
const gate = (mode, report, entries = []) =>
  evaluate({
    mode,
    reportText: report.endsWith('.json') ? fixture(report) : report,
    allowlist: { entries },
    today: TODAY,
  });

test('go: a reachable finding fails', () => {
  const r = gate('go', 'govulncheck-reachable.json');
  assert.equal(r.ok, false);
  assert.deepEqual(r.blocking.map((f) => f.id), ['GO-2099-0001']);
  assert.equal(r.blocking.length, 1, 'symbol and module trace of the same OSV are deduplicated');
});

test('go: a module/package-only finding passes but is listed', () => {
  const r = gate('go', 'govulncheck-module-only.json');
  assert.equal(r.ok, true);
  assert.equal(r.info.length, 1);
});

test('go: an allowlisted reachable finding passes', () => {
  const r = gate('go', 'govulncheck-reachable.json', [entry()]);
  assert.equal(r.ok, true);
  assert.equal(r.allowed.length, 1);
  assert.deepEqual(r.notices, []);
});

test('go: an allowlist entry for another package does not match', () => {
  const r = gate('go', 'govulncheck-reachable.json', [entry({ package: 'example.com/other' })]);
  assert.equal(r.ok, false);
});

test('an expired allowlist entry fails', () => {
  const r = gate('go', 'govulncheck-reachable.json', [entry({ expires: '2026-10-22' })]);
  assert.equal(r.ok, false);
  assert.match(r.errors.join('\n'), /EXPIRED/);
});

test('an entry expiring today is still valid', () => {
  const r = gate('go', 'govulncheck-reachable.json', [entry({ expires: '2026-10-23' })]);
  assert.equal(r.ok, true);
});

test('an expiry more than 90 days out fails', () => {
  const r = gate('go', 'govulncheck-reachable.json', [entry({ expires: '2027-01-22' })]);
  assert.equal(r.ok, false);
  assert.match(r.errors.join('\n'), /more than 90 days/);
  assert.equal(gate('go', 'govulncheck-reachable.json', [entry({ expires: '2027-01-21' })]).ok, true);
});

test('a malformed entry fails', () => {
  const r = gate('go', 'govulncheck-module-only.json', [{ id: 'GO-2099-0001', package: 'x' }]);
  assert.equal(r.ok, false);
  assert.match(r.errors.join('\n'), /missing field/);
});

test('a stale entry is a notice only', () => {
  const r = gate('go', 'govulncheck-module-only.json', [entry({ id: 'GO-2099-0009' })]);
  assert.equal(r.ok, true);
  assert.equal(r.notices.length, 1);
});

test('an entry of the other ecosystem is not reported stale', () => {
  const r = gate('go', 'govulncheck-module-only.json', [
    entry({ id: 'GHSA-aaaa-bbbb-cccc', package: 'leftpad' }),
  ]);
  assert.deepEqual(r.notices, []);
});

test('npm: a high advisory fails', () => {
  const r = gate('npm', 'pnpm-audit-high.json');
  assert.equal(r.ok, false);
  assert.deepEqual(r.blocking.map((f) => f.id), ['GHSA-aaaa-bbbb-cccc']);
  assert.equal(r.info.length, 1);
});

test('npm: an allowlisted high advisory passes', () => {
  const r = gate('npm', 'pnpm-audit-high.json', [
    entry({ id: 'GHSA-aaaa-bbbb-cccc', package: 'leftpad' }),
  ]);
  assert.equal(r.ok, true);
});

test('npm: moderate and low pass but are listed', () => {
  const r = gate('npm', 'pnpm-audit-moderate.json');
  assert.equal(r.ok, true);
  assert.equal(r.info.length, 2);
});

test('npm: a missing or unknown severity fails closed', () => {
  for (const severity of [undefined, 'unknown']) {
    const report = JSON.stringify({
      advisories: {
        9: { id: 9, github_advisory_id: 'GHSA-jjjj-kkkk-llll', module_name: 'oddpad', severity, title: 'x' },
      },
      metadata: { vulnerabilities: { info: 0, low: 0, moderate: 0, high: 0, critical: 0 } },
    });
    const r = gate('npm', report);
    assert.equal(r.ok, false, `severity ${severity}`);
    assert.deepEqual(r.blocking.map((f) => f.id), ['GHSA-jjjj-kkkk-llll']);
  }
});

test('empty, garbage or truncated reports fail', () => {
  for (const mode of ['go', 'npm']) {
    for (const text of ['', '   ', 'not json at all', '{"config":', '[]']) {
      const r = gate(mode, text);
      assert.equal(r.ok, false, `${mode}: ${JSON.stringify(text)}`);
      assert.ok(r.errors.length > 0);
    }
  }
});

test('go: a stream without a config message fails', () => {
  assert.equal(gate('go', '{"progress":{"message":"x"}}').ok, false);
});

test('npm: a report without metadata fails', () => {
  const r = gate('npm', 'pnpm-audit-no-metadata.json');
  assert.equal(r.ok, false);
  assert.match(r.errors.join('\n'), /metadata/);
});

test('npm: a registry error report fails', () => {
  assert.equal(gate('npm', 'pnpm-audit-error.json').ok, false);
});

test('run: a missing report file fails', () => {
  const dir = mkdtempSync(join(tmpdir(), 'vuln-gate-'));
  try {
    const al = join(dir, 'al.json');
    writeFileSync(al, JSON.stringify({ entries: [] }));
    const out = run(['go', join(dir, 'nope.json')], { today: TODAY, allowlistPath: al });
    assert.equal(out.code, 1);
    assert.match(out.text, /missing or unreadable/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('run: bad usage exits 2', () => {
  assert.equal(run([]).code, 2);
});

test('the committed allowlist is well-formed and links an issue; no #594 (Next 15 migration) entries remain', () => {
  const al = JSON.parse(readFileSync(resolve(here, '..', 'vuln-allowlist.json'), 'utf8'));
  const r = evaluate({
    mode: 'go',
    reportText: fixture('govulncheck-module-only.json'),
    allowlist: al,
    today: new Date(),
  });
  assert.deepEqual(r.errors, []);
  for (const e of al.entries) {
    assert.match(e.issue, /^#\d+$/);
    assert.notEqual(e.issue, '#594');
  }
});
