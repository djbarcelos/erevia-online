#!/usr/bin/env node
// Traduz a stack de um erro do cliente (tabela client_errors) para arquivo:linha:coluna do
// código-fonte, com os sourcemaps do build (vite build gera os .map ao lado dos .js, sem
// servir ao público).
//
// Uso (na pasta client/):
//   node scripts/resolve-stack.mjs --id 42              linha da tabela (pelo id)
//   node scripts/resolve-stack.mjs --hash 1a2b3c…       linha da tabela (pelo hash)
//   node scripts/resolve-stack.mjs < stack.txt          stack colada (stdin)
//   node scripts/resolve-stack.mjs --stack "at …"       stack na linha de comando
// Opções:
//   --maps <pasta>   build com os .map (padrão: dist). Ex.: os .map tirados da imagem
//                    Docker (ver README, "Erros do cliente").
// Banco (para --id/--hash): DATABASE_URL (padrão o do make db-up) e, se não houver psql
// instalado, PSQL com o comando inteiro, ex.:
//   PSQL="docker exec -i erevia-postgres-1 psql -U erevia -d erevia" node scripts/resolve-stack.mjs --id 42
//
// Os nomes dos .js têm o hash do conteúdo: se o arquivo da stack não existir na pasta dos
// maps, o build é outro (outra versão), e o script avisa e tenta o de mesmo nome base.

import { spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";

const args = process.argv.slice(2);
const opt = (name) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
};
const mapsDir = resolve(opt("--maps") ?? "dist");

// ---------- De onde vem a stack ----------

function fromDb(where) {
  const query = `SELECT json_build_object('id', id, 'hash', hash, 'version', version, 'kind', kind, 'message', message,
    'stack', stack, 'count', count, 'last_seen', last_seen, 'region', region, 'x', x, 'z', z) FROM client_errors WHERE ${where} ORDER BY last_seen DESC LIMIT 1`;
  const psql = process.env.PSQL?.trim();
  const [cmd, ...base] = psql
    ? psql.split(/\s+/)
    : ["psql", process.env.DATABASE_URL ?? "postgres://erevia:erevia@localhost:5432/erevia?sslmode=disable"];
  const r = spawnSync(cmd, [...base, "-At", "-c", query], { encoding: "utf8" });
  if (r.error || r.status !== 0) {
    console.error(`psql falhou: ${r.error?.message ?? r.stderr}`);
    process.exit(2);
  }
  const line = r.stdout.trim();
  if (!line) {
    console.error("nenhuma linha encontrada em client_errors");
    process.exit(1);
  }
  return JSON.parse(line);
}

let row = null;
let stack = opt("--stack");
const id = opt("--id");
const hash = opt("--hash");
if (id !== undefined) {
  if (!/^\d+$/.test(id)) throw new Error("--id deve ser um número");
  row = fromDb(`id = ${id}`);
} else if (hash !== undefined) {
  if (!/^[0-9a-f]{8,64}$/.test(hash)) throw new Error("--hash deve ser hexadecimal");
  row = fromDb(`hash = '${hash}'`);
}
if (row) {
  stack = row.stack;
  console.log(`#${row.id} ${row.kind} ×${row.count} · versão ${row.version} · última vez ${row.last_seen}${row.region ? ` · ${row.region} (${row.x}, ${row.z})` : ""}`);
  console.log(row.message);
  console.log("");
}
if (stack === undefined) stack = readFileSync(0, "utf8");

// ---------- Sourcemaps ----------

const B64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
const B64_VAL = new Map([...B64].map((c, i) => [c, i]));

/** Decodifica o campo "mappings" (VLQ base64) em linhas de segmentos [col, src, linha, coluna, nome]. */
function decodeMappings(mappings) {
  const lines = [];
  let src = 0, srcLine = 0, srcCol = 0, name = 0;
  for (const text of mappings.split(";")) {
    const segs = [];
    let genCol = 0;
    for (const seg of text.split(",")) {
      if (!seg) continue;
      const vals = [];
      let value = 0, shift = 0;
      for (const ch of seg) {
        const d = B64_VAL.get(ch);
        value += (d & 31) << shift;
        if (d & 32) {
          shift += 5;
        } else {
          vals.push(value & 1 ? -(value >>> 1) : value >>> 1);
          value = 0;
          shift = 0;
        }
      }
      genCol += vals[0];
      if (vals.length >= 4) {
        src += vals[1];
        srcLine += vals[2];
        srcCol += vals[3];
        if (vals.length >= 5) name += vals[4];
        segs.push([genCol, src, srcLine, srcCol, vals.length >= 5 ? name : -1]);
      }
    }
    lines.push(segs);
  }
  return lines;
}

const maps = new Map();
function loadMap(file) {
  if (!maps.has(file)) {
    const m = JSON.parse(readFileSync(file, "utf8"));
    maps.set(file, { sources: m.sources, names: m.names ?? [], root: m.sourceRoot ?? "", lines: decodeMappings(m.mappings) });
  }
  return maps.get(file);
}

/** Acha o .map do .js da stack (pelo nome exato ou, se o build for outro, pelo nome base). */
function mapFor(jsPath) {
  const exact = join(mapsDir, jsPath + ".map");
  if (existsSync(exact)) return { file: exact, exact: true };
  const dir = join(mapsDir, dirname(jsPath));
  const stem = basename(jsPath).replace(/-[\w-]{6,}\.js$/, "");
  const alt = existsSync(dir) ? readdirSync(dir).find((f) => f.endsWith(".js.map") && f.replace(/-[\w-]{6,}\.js\.map$/, "") === stem) : undefined;
  return alt ? { file: join(dir, alt), exact: false } : null;
}

/** Linha e coluna do build (1 e 1, como na stack do navegador) → posição no fonte. */
function lookup(map, line, col) {
  const segs = map.lines[line - 1];
  if (!segs?.length) return null;
  let best = null;
  for (const s of segs) {
    if (s[0] <= col - 1) best = s;
    else break;
  }
  if (!best) return null;
  const source = (map.root + map.sources[best[1]]).replace(/^(\.\.\/)+/, "").replace(/^\/?app\/client\//, "");
  return { source, line: best[2] + 1, col: best[3] + 1, name: best[4] >= 0 ? map.names[best[4]] : "" };
}

// ---------- Tradução ----------

// "…/assets/index-AbC123.js:1:23456" (V8: "at f (url:l:c)"; Firefox/Safari: "f@url:l:c").
const FRAME = /(?:[a-z]+:\/\/[^/\s)]+)?\/((?:[\w.-]+\/)*[\w.-]+\.js):(\d+):(\d+)/i;
let resolved = 0, missing = 0;
const warned = new Set();
for (const raw of stack.split("\n")) {
  const line = raw.trimEnd();
  const m = FRAME.exec(line);
  if (!m) {
    if (line.trim()) console.log(line);
    continue;
  }
  const [, jsPath, l, c] = m;
  const found = mapFor(jsPath);
  if (!found) {
    missing++;
    console.log(`${line}\n      → (sem .map para ${jsPath} em ${mapsDir})`);
    continue;
  }
  if (!found.exact && !warned.has(jsPath)) {
    warned.add(jsPath);
    console.error(`aviso: ${jsPath} não está em ${mapsDir}; usando ${basename(found.file)} (build diferente: as linhas podem não bater)`);
  }
  const pos = lookup(loadMap(found.file), Number(l), Number(c));
  if (!pos) {
    missing++;
    console.log(`${line}\n      → (posição fora do mapa)`);
    continue;
  }
  resolved++;
  console.log(`${line}\n      → ${pos.source}:${pos.line}:${pos.col}${pos.name ? ` (${pos.name})` : ""}`);
}
if (!resolved && !missing) console.error("nenhum quadro de /assets/*.js na stack");
process.exit(resolved || !missing ? 0 : 1);
