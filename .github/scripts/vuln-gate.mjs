#!/usr/bin/env node
// Vulnerability gate for CI (docs/features/dependency-scanning.md). No dependencies.
//
//   node .github/scripts/vuln-gate.mjs go  <govulncheck.json>   (govulncheck -format json ./...)
//   node .github/scripts/vuln-gate.mjs npm <pnpm-audit.json>    (pnpm audit --prod --json)
//
// Blocking: reachable (symbol-level) Go findings, high/critical npm advisories,
// expired or malformed allowlist entries, a missing/unparseable report.
// Everything else is reported (stdout and $GITHUB_STEP_SUMMARY) but does not block.
import { appendFileSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const MAX_EXPIRY_DAYS = 90;
const REQUIRED_FIELDS = ['id', 'package', 'reason', 'issue', 'expires'];
const DAY_MS = 24 * 60 * 60 * 1000;
const NON_BLOCKING_NPM = new Set(['info', 'low', 'moderate']);

function ecosystemOf(id) {
  if (/^GO-\d{4}-\d+$/.test(id)) return 'go';
  if (/^GHSA-/.test(id)) return 'npm';
  return null;
}

function parseDay(value) {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const ms = Date.parse(`${value}T00:00:00Z`);
  return Number.isNaN(ms) ? null : ms;
}

function todayMs(today) {
  const d = today ?? new Date();
  return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate());
}

/** Splits a stream of concatenated JSON objects (govulncheck output). Throws on garbage. */
export function parseJsonStream(text) {
  const out = [];
  let depth = 0;
  let start = -1;
  let inString = false;
  let escaped = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inString) {
      if (escaped) escaped = false;
      else if (c === '\\') escaped = true;
      else if (c === '"') inString = false;
      continue;
    }
    if (c === '"') {
      if (depth === 0) throw new Error('unexpected string outside a JSON object');
      inString = true;
    } else if (c === '{') {
      if (depth === 0) start = i;
      depth++;
    } else if (c === '}') {
      depth--;
      if (depth < 0) throw new Error('unbalanced braces');
      if (depth === 0) out.push(JSON.parse(text.slice(start, i + 1)));
    } else if (depth === 0 && !/\s/.test(c)) {
      throw new Error(`unexpected character "${c}" outside a JSON object`);
    }
  }
  if (depth !== 0 || inString) throw new Error('truncated JSON stream');
  return out;
}

/** Validates the allowlist shape and expiry. Returns a list of error strings. */
export function checkAllowlist(allowlist, today) {
  const errors = [];
  const now = todayMs(today);
  if (!allowlist || !Array.isArray(allowlist.entries)) {
    return ['allowlist must be an object with an "entries" array'];
  }
  for (const [i, e] of allowlist.entries.entries()) {
    const label = e && e.id ? `${e.id} (${e.package ?? '?'})` : `entry #${i}`;
    const missing = REQUIRED_FIELDS.filter((f) => !e || typeof e[f] !== 'string' || e[f].trim() === '');
    if (missing.length > 0) {
      errors.push(`allowlist ${label}: missing field(s) ${missing.join(', ')}`);
      continue;
    }
    if (ecosystemOf(e.id) === null) {
      errors.push(`allowlist ${label}: id must be GO-YYYY-NNNN or GHSA-...`);
    }
    const exp = parseDay(e.expires);
    if (exp === null) {
      errors.push(`allowlist ${label}: expires "${e.expires}" is not a YYYY-MM-DD date`);
    } else if (exp < now) {
      errors.push(`allowlist ${label}: EXPIRED on ${e.expires} - re-triage (bump the dependency or renew with a new reason)`);
    } else if ((exp - now) / DAY_MS > MAX_EXPIRY_DAYS) {
      errors.push(`allowlist ${label}: expires ${e.expires}, more than ${MAX_EXPIRY_DAYS} days out`);
    }
  }
  return errors;
}

/** Normalised Go findings, deduplicated by OSV ID + module. */
export function collectGo(reportText) {
  let msgs;
  try {
    msgs = parseJsonStream(reportText);
  } catch (err) {
    throw new Error(`govulncheck report is not valid JSON: ${err.message}`);
  }
  if (msgs.length === 0) throw new Error('govulncheck report is empty');
  if (!msgs.some((m) => m.config)) {
    throw new Error('govulncheck report has no "config" message (garbage or incomplete run)');
  }
  const summaries = new Map();
  for (const m of msgs) if (m.osv) summaries.set(m.osv.id, m.osv.summary ?? '');
  const rank = { symbol: 3, package: 2, module: 1 };
  const byKey = new Map();
  for (const m of msgs) {
    const f = m.finding;
    if (!f) continue;
    const frame = f.trace?.[0] ?? {};
    const pkg = frame.module ?? '(unknown module)';
    const level = frame.function ? 'symbol' : frame.package ? 'package' : 'module';
    const key = `${f.osv}|${pkg}`;
    const prev = byKey.get(key);
    if (!prev || rank[level] > rank[prev.level]) {
      byKey.set(key, {
        id: f.osv,
        package: pkg,
        level,
        reachable: level === 'symbol',
        fixed: f.fixed_version ?? '',
        summary: summaries.get(f.osv) ?? '',
      });
    }
  }
  return [...byKey.values()];
}

/** Normalised npm advisories from `pnpm audit --json`, deduplicated by GHSA + package. */
export function collectNpm(reportText) {
  let report;
  try {
    report = JSON.parse(reportText);
  } catch (err) {
    throw new Error(`pnpm audit report is not valid JSON: ${err.message}`);
  }
  if (!report || typeof report !== 'object' || Array.isArray(report)) {
    throw new Error('pnpm audit report is not a JSON object');
  }
  if (report.error) {
    throw new Error(`pnpm audit failed: ${report.error.message ?? JSON.stringify(report.error)}`);
  }
  if (!report.metadata || typeof report.metadata !== 'object') {
    throw new Error('pnpm audit report has no "metadata" (registry error or truncated output)');
  }
  const byKey = new Map();
  for (const adv of Object.values(report.advisories ?? {})) {
    const id = adv.github_advisory_id ?? `npm-${adv.id}`;
    const key = `${id}|${adv.module_name}`;
    if (byKey.has(key)) continue;
    byKey.set(key, {
      id,
      package: adv.module_name,
      severity: adv.severity,
      fixed: adv.patched_versions ?? '',
      summary: adv.title ?? '',
    });
  }
  return [...byKey.values()];
}

/**
 * Pure gate. mode: 'go' | 'npm'. Returns {ok, errors, blocking, allowed, info, notices}.
 * An unusable report or allowlist becomes an error (never a pass).
 */
export function evaluate({ mode, reportText, allowlist, today }) {
  const result = { ok: true, errors: [], blocking: [], allowed: [], info: [], notices: [] };
  result.errors.push(...checkAllowlist(allowlist, today));

  let findings = [];
  try {
    findings = mode === 'go' ? collectGo(reportText) : collectNpm(reportText);
  } catch (err) {
    result.errors.push(err.message);
  }

  const entries = Array.isArray(allowlist?.entries) ? allowlist.entries : [];
  const used = new Set();
  for (const f of findings) {
    const entry = entries.find((e) => e && e.id === f.id && e.package === f.package);
    if (entry) used.add(entry);
    // npm: anything not known to be info/low/moderate blocks (fail closed on
    // a missing or unexpected severity).
    const blocking = mode === 'go' ? f.reachable : !NON_BLOCKING_NPM.has(f.severity);
    if (!blocking) result.info.push(f);
    else if (entry) result.allowed.push({ ...f, entry });
    else result.blocking.push(f);
  }

  for (const e of entries) {
    if (!e || used.has(e)) continue;
    if (ecosystemOf(e.id ?? '') === mode) {
      result.notices.push(`allowlist ${e.id} (${e.package}) matches no finding - stale, remove it`);
    }
  }

  result.ok = result.errors.length === 0 && result.blocking.length === 0;
  return result;
}

function classOf(f, mode) {
  return mode === 'go' ? (f.reachable ? 'reachable' : f.level) : f.severity;
}

function describe(f, mode) {
  const fix = f.fixed ? `fixed in ${f.fixed}` : 'no fix';
  return `${f.id} ${f.package} [${classOf(f, mode)}] ${fix}${f.summary ? ` - ${f.summary}` : ''}`;
}

export function render(result, mode) {
  const title = mode === 'go' ? 'Go (govulncheck)' : 'npm (pnpm audit --prod)';
  const verdict = result.ok ? 'PASS' : 'FAIL';
  const infoHeading = mode === 'go' ? 'Not reachable (information only)' : 'Moderate/low (information only)';

  const lines = [`Vulnerability gate: ${title} -> ${verdict}`];
  const section = (heading, items) => {
    if (items.length === 0) return;
    lines.push('', `${heading} (${items.length})`, ...items.map((s) => `  - ${s}`));
  };
  section('ERRORS', result.errors);
  section('BLOCKING findings', result.blocking.map((f) => describe(f, mode)));
  section(
    'Allowlisted (blocking class, accepted until expiry)',
    result.allowed.map((f) => `${describe(f, mode)} | until ${f.entry.expires}, ${f.entry.issue}`),
  );
  section(infoHeading, result.info.map((f) => describe(f, mode)));
  section('NOTICES', result.notices);

  const md = [`### Vulnerability gate: ${title} - ${verdict}`, ''];
  const table = (heading, rows) => {
    if (rows.length === 0) return;
    md.push(`**${heading} (${rows.length})**`, '', '| ID | Package | Class | Fix | Note |', '|---|---|---|---|---|');
    for (const r of rows) {
      md.push(`| ${r.id} | ${r.package} | ${classOf(r, mode)} | ${r.fixed || 'none'} | ${r.note.replace(/\|/g, '\\|')} |`);
    }
    md.push('');
  };
  const list = (heading, items) => {
    if (items.length === 0) return;
    md.push(`**${heading} (${items.length})**`, '', ...items.map((s) => `- ${s}`), '');
  };
  list('Errors', result.errors);
  table('Blocking', result.blocking.map((f) => ({ ...f, note: f.summary })));
  table('Allowlisted', result.allowed.map((f) => ({ ...f, note: `until ${f.entry.expires}, ${f.entry.issue}` })));
  table(infoHeading, result.info.map((f) => ({ ...f, note: f.summary })));
  list('Notices', result.notices);
  return { text: lines.join('\n'), markdown: md.join('\n') };
}

export function run(argv, { today, allowlistPath } = {}) {
  const [mode, reportPath] = argv;
  if ((mode !== 'go' && mode !== 'npm') || !reportPath) {
    return { code: 2, text: 'usage: vuln-gate.mjs <go|npm> <report.json>', markdown: '' };
  }
  const here = dirname(fileURLToPath(import.meta.url));
  const alPath = allowlistPath ?? resolve(here, '..', 'vuln-allowlist.json');
  const preErrors = [];
  let allowlist = { entries: [] };
  try {
    allowlist = JSON.parse(readFileSync(alPath, 'utf8'));
  } catch (err) {
    preErrors.push(`cannot read allowlist ${alPath}: ${err.message}`);
  }
  let reportText = '';
  try {
    reportText = readFileSync(reportPath, 'utf8');
  } catch (err) {
    preErrors.push(`scanner report ${reportPath} is missing or unreadable: ${err.message}`);
  }
  const result = evaluate({ mode, reportText, allowlist, today });
  result.errors.unshift(...preErrors);
  result.ok = result.errors.length === 0 && result.blocking.length === 0;
  const { text, markdown } = render(result, mode);
  return { code: result.ok ? 0 : 1, text, markdown };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const { code, text, markdown } = run(process.argv.slice(2));
  console.log(text);
  if (process.env.GITHUB_STEP_SUMMARY && markdown) {
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, `${markdown}\n`);
  }
  process.exit(code);
}
