/**
 * Linha de base da medição de FPS (?bench=1) no CI: o que entra na comparação, com que
 * tolerância, e a tabela do relatório. Usado por bench.spec.ts; a base fica em
 * e2e/bench-baseline.json e é atualizada com `npm run bench:baseline`.
 *
 * Só contadores que não mudam de uma rodada para outra no Chromium headless (SwiftShader).
 * Nunca FPS nem ms: dependem da máquina, que no CI é compartilhada.
 *
 * - Por região e hora (o maior valor entre os pontos da região, como na tabela do bench), só
 *   do CENÁRIO, num quadro com as entidades escondidas: draw calls, malhas ativas e malhas na
 *   cena. Tolerância: 10%.
 * - Censo do cenário, por grupo de nome: malhas, malhas soltas (sem freezeWorldMatrix) e nós.
 *   Tolerância: 10% ou +2, o que for maior (os grupos são pequenos: 1 → 2 malhas seria +100%).
 * - Entidades por tipo: malhas e nós de CADA uma (média). Tolerância: 10%. Quantas há não
 *   entra: os monstros andam e entram e saem da área de interesse do servidor (~70 m).
 *
 * Fica de fora por oscilar: draw calls e malhas ativas com as entidades (monstros perto do
 * ponto mudam de 3 a 9 por rodada nos Arredores e nos Campos dos Ossos), o total de malhas
 * da cena com as entidades (252–258) e a quantidade de entidades de cada tipo. O cenário,
 * medido assim, deu os mesmos números em todas as rodadas (client/src/ui/bench.ts: sceneOnly).
 */

export interface BenchSample {
  region: string;
  time: string;
  sceneDraws: number;
  sceneActive: number;
  sceneMeshes: number;
}

export interface BenchCensusRow {
  group: string;
  meshes: number;
  frozen: number;
  transformNodes?: number;
}

export interface BenchEntityRow {
  type: string;
  count: number;
  meshes: number;
  transformNodes: number;
}

export interface BenchResult {
  samples: BenchSample[];
  sceneCensus: BenchCensusRow[];
  entities: BenchEntityRow[];
  table: string;
}

type Numbers = Record<string, number>;

export interface Snapshot {
  _sobre?: string;
  regioes: Record<string, Numbers>;
  censo: Record<string, Numbers>;
  entidades: Record<string, Numbers>;
}

export const ABOUT =
  "Linha de base de e2e/tests/bench.spec.ts (contadores do ?bench=1 no CI). Não edite à mão: " +
  "rode `npm run bench:baseline` em e2e/ (com o servidor no ar) e commite o diff junto com a mudança.";

/** Tolerância de cada seção: piorar mais que isto reprova. */
export const TOLERANCE: Record<keyof Omit<Snapshot, "_sobre">, { pct: number; abs: number; why: string }> = {
  regioes: { pct: 0.1, abs: 0, why: "10%" },
  censo: { pct: 0.1, abs: 2, why: "10% ou +2, o que for maior" },
  entidades: { pct: 0.1, abs: 0, why: "10%" },
};

const LABELS: Record<string, string> = {
  draws: "draw calls",
  ativas: "malhas ativas",
  malhas: "malhas",
  soltas: "malhas soltas",
  nos: "nós",
};

const round1 = (v: number) => Math.round(v * 10) / 10;

/** Os números comparáveis de um resultado do bench (chaves em ordem fixa, para o diff do PR). */
export function snapshot(r: BenchResult): Snapshot {
  const regioes: Snapshot["regioes"] = {};
  for (const s of r.samples) {
    const k = `${s.region} · ${s.time}`;
    const cur = regioes[k] ?? { draws: 0, ativas: 0, malhas: 0 };
    regioes[k] = {
      draws: Math.max(cur.draws, s.sceneDraws),
      ativas: Math.max(cur.ativas, s.sceneActive),
      malhas: Math.max(cur.malhas, s.sceneMeshes),
    };
  }
  const censo: Snapshot["censo"] = {};
  for (const c of [...r.sceneCensus].sort((a, b) => a.group.localeCompare(b.group))) {
    censo[c.group] = { malhas: c.meshes, soltas: c.meshes - c.frozen, nos: c.transformNodes ?? 0 };
  }
  const entidades: Snapshot["entidades"] = {};
  for (const e of [...r.entities].sort((a, b) => a.type.localeCompare(b.type))) {
    if (!e.count) continue;
    entidades[e.type] = { malhas: round1(e.meshes / e.count), nos: round1(e.transformNodes / e.count) };
  }
  return { _sobre: ABOUT, regioes, censo, entidades };
}

export type Verdict = "igual" | "melhor" | "pior (na tolerância)" | "PIOR" | "novo" | "sumiu";

export interface Row {
  section: keyof typeof TOLERANCE;
  key: string;
  metric: string;
  base: number | null;
  now: number | null;
  verdict: Verdict;
}

export interface Report {
  rows: Row[];
  failed: Row[];
  improved: boolean;
  changed: boolean;
}

function judge(section: keyof typeof TOLERANCE, base: number | null, now: number | null): Verdict {
  // Sem base: uma região nova do mapa (ou um tipo novo de entidade) não é regressão; um
  // grupo novo no censo do cenário é (ex.: malhas soltas que alguém esqueceu de juntar).
  if (base === null) return section !== "censo" ? "novo" : (now ?? 0) > TOLERANCE[section].abs ? "PIOR" : "novo";
  if (now === null) return "sumiu";
  if (now === base) return "igual";
  if (now < base) return "melhor";
  const t = TOLERANCE[section];
  return now - base > Math.max(base * t.pct, t.abs) ? "PIOR" : "pior (na tolerância)";
}

export function compare(base: Snapshot, now: Snapshot): Report {
  const rows: Row[] = [];
  for (const section of Object.keys(TOLERANCE) as (keyof typeof TOLERANCE)[]) {
    const b = base[section] ?? {};
    const n = now[section] ?? {};
    const keys = [...new Set([...Object.keys(n), ...Object.keys(b)])];
    for (const key of keys) {
      const metrics = [...new Set([...Object.keys(n[key] ?? {}), ...Object.keys(b[key] ?? {})])];
      for (const metric of metrics) {
        const bv = b[key]?.[metric] ?? null;
        const nv = n[key]?.[metric] ?? null;
        // Uma entidade que não apareceu nesta rodada (longe do personagem) não é melhora.
        if (section === "entidades" && nv === null) continue;
        rows.push({ section, key, metric, base: bv, now: nv, verdict: judge(section, bv, nv) });
      }
    }
  }
  const failed = rows.filter((r) => r.verdict === "PIOR");
  return {
    rows,
    failed,
    improved: rows.some((r) => r.verdict === "melhor" || (r.verdict === "sumiu" && r.section !== "entidades")),
    changed: rows.some((r) => r.verdict !== "igual"),
  };
}

function cell(r: Row | undefined): string {
  if (!r) return "–";
  const b = r.base ?? "–";
  const n = r.now ?? "–";
  if (r.verdict === "igual") return `${n}`;
  const pct = r.base && r.now !== null ? ` (${r.now > r.base ? "+" : ""}${Math.round(((r.now - r.base) / r.base) * 100)}%)` : "";
  const mark = r.verdict === "PIOR" ? " **PIOR**" : r.verdict === "melhor" ? " melhor" : "";
  return `${b} → ${n}${pct}${mark}`;
}

/** Relatório em Markdown: a tabela antes e depois (vai para o resumo do job no CI). */
export function markdown(rep: Report, extra: { title: string; seconds?: number; updated?: boolean }): string {
  const out: string[] = [`### ${extra.title}`, ""];
  if (extra.updated) out.push("Linha de base **atualizada** (e2e/bench-baseline.json): confira o diff antes de commitar.", "");
  else if (rep.failed.length) {
    out.push(
      `**${rep.failed.length} número(s) pioraram além da tolerância.** Se a piora é de propósito, ` +
        "atualize a base com `npm run bench:baseline` (em e2e/) e commite o diff junto.",
      "",
    );
  } else if (rep.improved) {
    out.push("Nada piorou além da tolerância, e alguns números **melhoraram**: atualize a base com `npm run bench:baseline` (em e2e/) para fixar o ganho.", "");
  } else if (rep.changed) out.push("Nada piorou além da tolerância.", "");
  else out.push("Tudo igual à linha de base.", "");

  // Regiões: todas, uma linha por região e hora.
  const reg = rep.rows.filter((r) => r.section === "regioes");
  out.push(
    `Cenário por região e hora (entidades escondidas; base → agora; tolerância ${TOLERANCE.regioes.why}):`,
    "",
    "| Região · hora | Draw calls | Malhas ativas | Malhas na cena |",
    "|---|---|---|---|",
  );
  for (const key of [...new Set(reg.map((r) => r.key))]) {
    const m = (metric: string) => reg.find((r) => r.key === key && r.metric === metric);
    out.push(`| ${key} | ${cell(m("draws"))} | ${cell(m("ativas"))} | ${cell(m("malhas"))} |`);
  }
  // Censo e entidades: só o que mudou.
  for (const [section, title] of [
    ["censo", `Censo do cenário, grupos que mudaram (tolerância ${TOLERANCE.censo.why})`],
    ["entidades", `Malhas e nós por entidade, tipos que mudaram (tolerância ${TOLERANCE.entidades.why})`],
  ] as const) {
    const changed = rep.rows.filter((r) => r.section === section && r.verdict !== "igual");
    out.push("", `${title}:`, "");
    if (!changed.length) {
      out.push("nenhum.");
      continue;
    }
    out.push("| Grupo | Métrica | Base → agora |", "|---|---|---|");
    for (const r of changed) out.push(`| ${r.key} | ${LABELS[r.metric] ?? r.metric} | ${cell(r)}${r.verdict === "novo" || r.verdict === "sumiu" ? ` (${r.verdict})` : ""} |`);
  }
  if (extra.seconds !== undefined) out.push("", `Medição: ${extra.seconds.toFixed(0)} s.`);
  out.push("");
  return out.join("\n");
}
