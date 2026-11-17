// Converte o CHANGELOG.md da raiz em client/public/novidades.json, que o jogo (janela
// "O que há de novo") e a página /novidades leem.
//
//   node scripts/build-changelog.mjs
//
// Roda sozinho no `npm run build`. Formato esperado no CHANGELOG.md:
//   ## AAAA-MM-DD · Título        (ou AAAA-MM-DD.2 para uma segunda versão no mesmo dia)
//   ### Novidades | Melhorias | Correções
//   - item (linhas seguidas sem "- " continuam o item)
// Comentários <!-- ... --> e o texto antes da primeira versão são ignorados.
//
// Sem o CHANGELOG.md (por exemplo, num build só com client/), mantém o novidades.json
// versionado e só avisa. Erro de formato derruba o build: melhor do que publicar errado.

import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const SRC = join(here, "..", "..", "CHANGELOG.md");
const OUT = join(here, "..", "public", "novidades.json");
const SECTIONS = ["Novidades", "Melhorias", "Correções"];

if (!existsSync(SRC)) {
  console.warn(`build-changelog: ${SRC} não encontrado; fica o novidades.json que já existe.`);
  process.exit(0);
}

const fail = (line, msg) => {
  console.error(`build-changelog: CHANGELOG.md, linha ${line}: ${msg}`);
  process.exit(1);
};

const text = readFileSync(SRC, "utf8").replace(/<!--[\s\S]*?-->/g, (c) => c.replace(/[^\n]/g, ""));
const versions = [];
let version = null;
let section = null;
let item = null;

text.split("\n").forEach((raw, i) => {
  const n = i + 1;
  const line = raw.trimEnd();
  let m;
  if ((m = /^## (\S+)\s+·\s+(.+)$/.exec(line))) {
    const [, id, title] = m;
    if (!/^\d{4}-\d{2}-\d{2}(\.\d+)?$/.test(id) || Number.isNaN(Date.parse(id.slice(0, 10)))) {
      fail(n, `versão "${id}" não é uma data AAAA-MM-DD (ou AAAA-MM-DD.2)`);
    }
    if (versions.some((v) => v.version === id)) fail(n, `versão ${id} repetida`);
    version = { version: id, date: id.slice(0, 10), title: title.trim(), sections: [] };
    versions.push(version);
    section = item = null;
  } else if (line.startsWith("## ")) {
    fail(n, `título de versão fora do formato "## AAAA-MM-DD · Título"`);
  } else if ((m = /^### (.+)$/.exec(line))) {
    if (!version) fail(n, "seção antes da primeira versão");
    const name = m[1].trim();
    if (!SECTIONS.includes(name)) fail(n, `seção "${name}" (use ${SECTIONS.join(", ")})`);
    if (version.sections.some((s) => s.title === name)) fail(n, `seção "${name}" repetida em ${version.version}`);
    section = { title: name, items: [] };
    version.sections.push(section);
    item = null;
  } else if ((m = /^\s*[-*] (.+)$/.exec(line))) {
    if (!version) return; // lista na introdução
    if (!section) fail(n, "item fora de uma seção (### Novidades, Melhorias ou Correções)");
    section.items.push(m[1].trim());
    item = section.items.length - 1;
  } else if (line.trim() === "") {
    item = null;
  } else if (version && section && item !== null) {
    section.items[item] += ` ${line.trim()}`;
  } else if (version) {
    fail(n, "texto solto dentro de uma versão (comece o item com \"- \")");
  }
});

if (!versions.length) fail(1, "nenhuma versão encontrada");
for (const v of versions) {
  v.sections = v.sections.filter((s) => s.items.length);
  if (!v.sections.length) fail(1, `a versão ${v.version} não tem itens`);
}
// Mais nova primeiro, mesmo que alguém cole fora de ordem.
versions.sort((a, b) => b.version.localeCompare(a.version, "en", { numeric: true }));

const json = `${JSON.stringify({ latest: versions[0].version, versions }, null, 2)}\n`;
const old = existsSync(OUT) ? readFileSync(OUT, "utf8") : "";
if (old !== json) writeFileSync(OUT, json);
const items = versions.reduce((n, v) => n + v.sections.reduce((m, s) => m + s.items.length, 0), 0);
console.log(`build-changelog: ${versions.length} versões, ${items} itens, mais nova ${versions[0].version}${old === json ? " (sem mudança)" : ""}`);
