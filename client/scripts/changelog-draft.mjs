// Rascunho do CHANGELOG.md: lista os PRs mergeados na main desde uma data (via gh) e
// imprime as versões em markdown, um dia por versão, para o humano editar e colar.
//
//   node scripts/changelog-draft.mjs              # desde a versão mais nova do CHANGELOG.md
//   node scripts/changelog-draft.mjs 2026-11-13   # desde esse dia (inclusive)
//
// Precisa do gh logado no repositório. A seção de cada PR é só um palpite pelo título
// (corrige → Correções; melhora, fluido, leve… → Melhorias; o resto → Novidades), e os
// que parecem internos (CI, testes, telemetria, docs) saem comentados. O texto final é
// sempre do humano: em linguagem de jogador, o que dá para fazer agora e onde.

import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const CHANGELOG = join(here, "..", "..", "CHANGELOG.md");
const TZ = "America/Sao_Paulo"; // os dias do CHANGELOG são no horário de Brasília

function lastVersionDate() {
  if (!existsSync(CHANGELOG)) return null;
  const m = /^## (\d{4}-\d{2}-\d{2})/m.exec(readFileSync(CHANGELOG, "utf8").replace(/<!--[\s\S]*?-->/g, ""));
  return m ? m[1] : null;
}

const since = process.argv[2] ?? lastVersionDate();
if (!since || !/^\d{4}-\d{2}-\d{2}$/.test(since)) {
  console.error("Uso: node scripts/changelog-draft.mjs AAAA-MM-DD");
  process.exit(1);
}

let prs;
try {
  const out = execFileSync(
    "gh",
    ["pr", "list", "--state", "merged", "--base", "main", "--limit", "200", "--search", `merged:>=${since}`, "--json", "number,title,body,mergedAt"],
    { encoding: "utf8", cwd: here, stdio: ["ignore", "pipe", "inherit"] },
  );
  prs = JSON.parse(out);
} catch (err) {
  console.error(`Não consegui listar os PRs com o gh (está instalado e logado?): ${err.message}`);
  process.exit(1);
}

const dayOf = (iso) => new Intl.DateTimeFormat("en-CA", { timeZone: TZ, year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date(iso));
// A busca do GitHub usa o dia em UTC: confere de novo no horário de Brasília.
prs = prs.filter((p) => dayOf(p.mergedAt) >= since).sort((a, b) => a.mergedAt.localeCompare(b.mergedAt));

const INTERNAL = /\b(ci|testes?|telemetria|carga|bench|medi[çc][ãa]o|readme|docs?|guia de arte|refatora\w*|gofmt|migra[çc](ão|ao|ões|oes))\b/i;
const FIX = /\b(corrige|conserta|fix|erro)|(?<!\/)\bbugs?\b/i;
const BETTER = /\b(melhora|ajuste|fluid|leve|r[áa]pid|desempenho|fps|otimiz|render|visual|interface|lgpd)/i;

function sectionOf(title) {
  if (FIX.test(title)) return "Correções";
  if (BETTER.test(title)) return "Melhorias";
  return "Novidades";
}

/** Primeira linha útil da descrição: ajuda a lembrar do que o PR trata. */
function hint(body) {
  for (const raw of (body ?? "").split("\n")) {
    const line = raw.replace(/[#*`>]/g, "").replace(/^\s*[-\d.]+\s*/, "").trim();
    if (line && !/^(o que (entra|entrou|muda)|como verificar|empilhado|closes|🤖)/i.test(line)) return line.slice(0, 160);
  }
  return "";
}

if (!prs.length) {
  console.error(`Nenhum PR mergeado desde ${since}.`);
  process.exit(0);
}

const byDay = new Map();
for (const p of prs) {
  const d = dayOf(p.mergedAt);
  if (!byDay.has(d)) byDay.set(d, { Novidades: [], Melhorias: [], Correções: [] });
  const s = byDay.get(d);
  const h = hint(p.body).replace(/--/g, "-");
  const base = `- ${p.title} (#${p.number})`;
  // O build-changelog ignora comentários, então colar com eles não quebra nada.
  s[sectionOf(p.title)].push(INTERNAL.test(p.title) ? `<!-- interno? ${base}${h ? `: ${h}` : ""} -->` : `${base}${h ? ` <!-- ${h} -->` : ""}`);
}

const out = [];
for (const [day, s] of [...byDay].reverse()) {
  out.push(`## ${day} · TÍTULO DA VERSÃO`, "");
  for (const name of ["Novidades", "Melhorias", "Correções"]) {
    if (!s[name].length) continue;
    out.push(`### ${name}`, ...s[name], "");
  }
}
console.log(out.join("\n").trimEnd());
console.error(`\n${prs.length} PRs desde ${since}. Reescreva cada item em linguagem de jogador, apague os internos e cole no topo do CHANGELOG.md.`);
