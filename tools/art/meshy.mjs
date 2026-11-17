#!/usr/bin/env node
// CLI da API do Meshy (https://docs.meshy.ai), reutilizável para os próximos modelos.
//
// A chave vem da variável MESHY do .env do checkout principal (ou de --env <arquivo>). Ela nunca
// é impressa nem gravada: nenhum log leva o header de Authorization.
//
// Uso (da raiz do repositório):
//   node tools/art/meshy.mjs balance
//   node tools/art/meshy.mjs spent
//   node tools/art/meshy.mjs create <endpoint> --est <créditos> [--name n] (--json '{...}' | --json-file f)
//        endpoints: text-to-image, image-to-3d, rigging, animations, retexture, remesh ...
//        um valor "@arquivo.png" dentro do JSON vira data URI (imagem local)
//   node tools/art/meshy.mjs get <endpoint> <id> [--download]   (consulta e baixa sem pagar de novo)
//   node tools/art/meshy.mjs list-tasks
// Cada task é gravada em assets-src/meshy/tasks.jsonl (id, tipo, parâmetros, créditos) e o resultado é
// baixado para assets-src/meshy/<endpoint>-<id8>/. O teto da rodada é MESHY_BUDGET (padrão 250): se o gasto
// registrado + a estimativa passar do teto, o CLI recusa a chamada.
import { appendFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { basename, extname, resolve } from 'node:path';

const BASE = 'https://api.meshy.ai/openapi';
const ROOT = resolve(import.meta.dirname, '../..');
const OUT = resolve(ROOT, 'assets-src/meshy');
const LOG = resolve(OUT, 'tasks.jsonl');
const DEFAULT_ENV = 'C:/Users/DanielTeofilo/OneDrive - Info Tecnologia/Documentos/GitHub/erevia-online/.env';
const BUDGET = Number(process.env.MESHY_BUDGET ?? 250);

const argv = process.argv.slice(2);
const BOOL = new Set(['download']);
const flags = {};
const pos = [];
for (let i = 0; i < argv.length; i++) {
  if (argv[i].startsWith('--')) {
    const k = argv[i].slice(2);
    flags[k] = BOOL.has(k) ? true : argv[++i];
  } else pos.push(argv[i]);
}

function apiKey() {
  const f = flags.env ?? DEFAULT_ENV;
  const m = readFileSync(f, 'utf8').match(/^\s*MESHY\s*=\s*["']?([^"'\r\n]+)["']?\s*$/m);
  if (!m) throw new Error(`variável MESHY não encontrada em ${f}`);
  return m[1].trim();
}

async function api(method, path, body) {
  const res = await fetch(BASE + path, {
    method,
    headers: { Authorization: `Bearer ${apiKey()}`, ...(body ? { 'Content-Type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json;
  try { json = JSON.parse(text); } catch { json = { raw: text.slice(0, 500) }; }
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status}: ${JSON.stringify(json).slice(0, 600)}`);
  return json;
}

function readLog() {
  if (!existsSync(LOG)) return [];
  return readFileSync(LOG, 'utf8').split(/\r?\n/).filter(Boolean).map((l) => JSON.parse(l));
}
function log(rec) {
  mkdirSync(OUT, { recursive: true });
  appendFileSync(LOG, JSON.stringify({ ts: new Date().toISOString(), ...rec }) + '\n');
}
/** Gasto por id: o último valor real (consumed_credits) ou, na falta, a estimativa. */
function spent() {
  const byId = new Map();
  for (const r of readLog()) {
    const cur = byId.get(r.id) ?? { est: 0, real: null };
    if (r.est != null) cur.est = r.est;
    if (r.consumed_credits != null) cur.real = r.consumed_credits;
    byId.set(r.id, cur);
  }
  let total = 0;
  for (const v of byId.values()) total += v.real ?? v.est;
  return { total, byId };
}

/** "@arquivo" no JSON vira data URI. */
function inline(v) {
  if (typeof v === 'string' && v.startsWith('@')) {
    const f = resolve(v.slice(1));
    const ext = extname(f).slice(1).toLowerCase().replace('jpg', 'jpeg');
    return `data:image/${ext};base64,${readFileSync(f).toString('base64')}`;
  }
  if (Array.isArray(v)) return v.map(inline);
  if (v && typeof v === 'object') return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, inline(x)]));
  return v;
}
const redact = (v) => JSON.parse(JSON.stringify(v, (k, x) => (typeof x === 'string' && x.startsWith('data:') ? `${x.slice(0, 20)}...(${x.length} bytes)` : x)));

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function wait(endpoint, id) {
  for (let i = 0; i < 240; i++) {
    const t = await api('GET', `/v1/${endpoint}/${id}`);
    process.stdout.write(`\r  ${endpoint} ${id.slice(0, 8)} ${t.status} ${t.progress ?? 0}%   `);
    if (['SUCCEEDED', 'FAILED', 'CANCELED'].includes(t.status)) { console.log(); return t; }
    await sleep(5000);
  }
  throw new Error('tempo esgotado esperando a task (retome com: get <endpoint> <id>)');
}

/** Baixa as URLs de arquivo do resultado para assets-src/meshy/<endpoint>-<id8>/. */
async function download(endpoint, t) {
  const dir = resolve(OUT, `${endpoint}-${t.id.slice(-8)}`);
  mkdirSync(dir, { recursive: true });
  const urls = [];
  const walk = (k, v) => {
    if (typeof v === 'string' && /^https:\/\//.test(v) && /\.(glb|fbx|png|jpg|jpeg|obj|mtl|usdz)$/i.test(v.split('?')[0])) urls.push([k, v]);
    else if (Array.isArray(v)) v.forEach((x, i) => walk(`${k}${i}`, x));
    else if (v && typeof v === 'object') for (const [kk, x] of Object.entries(v)) walk(kk, x);
  };
  walk('', { image_urls: t.image_urls, model_urls: t.model_urls, texture_urls: t.texture_urls, result: t.result, thumbnail_url: t.thumbnail_url });
  const saved = [];
  for (const [k, u] of urls) {
    const name = `${k || 'file'}-${basename(u.split('?')[0])}`.replace(/[^\w.-]/g, '_').slice(0, 90);
    const res = await fetch(u);
    if (!res.ok) continue;
    writeFileSync(resolve(dir, name), Buffer.from(await res.arrayBuffer()));
    saved.push(name);
  }
  writeFileSync(resolve(dir, 'task.json'), JSON.stringify(t, null, 2));
  console.log(`  baixado em ${dir}: ${saved.join(', ') || '(nada)'}`);
}

async function main() {
  const cmd = pos[0];
  if (cmd === 'balance') { console.log(JSON.stringify(await api('GET', '/v1/balance'))); return; }
  if (cmd === 'library') {
    // biblioteca de ações do Meshy (grátis): id e nome, para escolher as ações; grava em assets-src/meshy/library.json
    const lib = await api('GET', '/v1/animations/library');
    mkdirSync(OUT, { recursive: true });
    writeFileSync(resolve(OUT, 'library.json'), JSON.stringify(lib));
    console.log(`biblioteca: ${(Array.isArray(lib) ? lib : lib.result ?? lib.data ?? []).length} ações`);
    return;
  }
  if (cmd === 'spent') { console.log(`gasto registrado: ${spent().total} de ${BUDGET}`); return; }
  if (cmd === 'list-tasks') {
    for (const r of readLog()) console.log(r.ts, r.endpoint, r.id, r.status ?? '', r.est ?? '', r.consumed_credits ?? '', r.name ?? '', r.note ?? '');
    return;
  }
  if (cmd === 'create') {
    const endpoint = pos[1];
    const est = Number(flags.est);
    if (!endpoint || !Number.isFinite(est)) throw new Error('create <endpoint> --est <créditos> --json ...');
    const raw = flags['json-file'] ? readFileSync(flags['json-file'], 'utf8') : flags.json;
    const body = inline(JSON.parse(raw));
    const { total } = spent();
    if (total + est > BUDGET) throw new Error(`ORÇAMENTO: gasto ${total} + ${est} passa de ${BUDGET}. Parado.`);
    const r = await api('POST', `/${endpoint === 'text-to-3d' ? 'v2' : 'v1'}/${endpoint}`, body);
    const id = r.result;
    log({ endpoint, id, est, name: flags.name ?? '', params: redact(body), status: 'CREATED' });
    console.log(`  task ${id} criada (${endpoint}, estimativa ${est})`);
    const t = await wait(endpoint, id);
    log({ endpoint, id, status: t.status, consumed_credits: t.consumed_credits ?? null, note: t.task_error?.message });
    if (t.status !== 'SUCCEEDED') throw new Error(`task ${t.status}: ${t.task_error?.message ?? ''}`);
    console.log(`  créditos consumidos: ${t.consumed_credits ?? '?'}`);
    await download(endpoint, t);
    return;
  }
  if (cmd === 'get') {
    const t = await wait(pos[1], pos[2]);
    log({ endpoint: pos[1], id: pos[2], status: t.status, consumed_credits: t.consumed_credits ?? null, note: 'get' });
    if (t.status === 'SUCCEEDED' && flags.download) await download(pos[1], t);
    else console.log(JSON.stringify({ status: t.status, consumed: t.consumed_credits }));
    return;
  }
  console.error('comandos: balance | spent | list-tasks | create | get  (veja o cabeçalho do arquivo)');
  process.exit(2);
}
main().catch((e) => { console.error(String(e.message ?? e)); process.exit(1); });
