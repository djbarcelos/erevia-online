import { Effect } from "../gen/erevia/v1/game_pb";

/**
 * Ícones originais das skills e dos efeitos, em SVG (64×64), desenhados à mão aqui:
 * fundo com degradê radial na cor do tema + um glifo claro com sombra.
 */

export interface Theme {
  /** Centro e borda do fundo. */
  bg: [string, string];
  /** Cor do glifo. */
  ink: string;
  glyph: string;
}

export const STROKE = `stroke-linecap="round" stroke-linejoin="round"`;

const SKILL_ICONS: Record<string, Theme> = {
  // Escudo com marcas de impacto.
  shield_bash: {
    bg: ["#5f7fae", "#1c2a44"],
    ink: "#eef3ff",
    glyph: `
      <path d="M32 12 L48 18 V31 C48 41 41 48 32 52 C23 48 16 41 16 31 V18 Z" fill="currentColor" opacity=".95"/>
      <path d="M32 18 L42 22 V31 C42 37.5 38 42 32 45 Z" fill="#9db4dc"/>
      <path d="M9 14 L15 19 M6 27 H13 M9 40 L15 36" stroke="#ffd36b" stroke-width="3.2" ${STROKE}/>
      <circle cx="32" cy="30" r="4" fill="#ffd36b"/>`,
  },
  // Torre com ameias protegida por um escudo.
  fortress: {
    bg: ["#4d79c9", "#14234a"],
    ink: "#e9f1ff",
    glyph: `
      <path d="M18 52 V26 H22 V20 H27 V26 H30 V20 H34 V26 H37 V20 H42 V26 H46 V52 Z" fill="currentColor"/>
      <path d="M28 52 V42 C28 39 36 39 36 42 V52 Z" fill="#2a3f6e"/>
      <path d="M12 30 C20 26 44 26 52 30" stroke="#8fc0ff" stroke-width="2.5" fill="none" ${STROKE} opacity=".9"/>
      <path d="M10 22 C20 16 44 16 54 22" stroke="#8fc0ff" stroke-width="2" fill="none" ${STROKE} opacity=".55"/>`,
  },
  // Machado descendo num arco de corte.
  brutal_strike: {
    bg: ["#c3532f", "#3e130b"],
    ink: "#fff1e4",
    glyph: `
      <path d="M12 46 C18 26 34 13 52 12" stroke="#ffb27a" stroke-width="3" fill="none" ${STROKE} opacity=".8"/>
      <path d="M22 50 L44 20" stroke="#7a4a2a" stroke-width="5" ${STROKE}/>
      <path d="M38 14 C46 12 54 18 53 27 C49 25 45 25 41 28 C40 23 39 19 38 14 Z" fill="currentColor"/>
      <path d="M41 28 L47 20" stroke="#c9d0dc" stroke-width="2" ${STROKE}/>`,
  },
  // Chama dupla em forma de rosto zangado.
  fury: {
    bg: ["#d23b2b", "#3d0808"],
    ink: "#ffe3c4",
    glyph: `
      <path d="M32 54 C20 54 14 45 16 36 C18 28 24 26 23 16 C30 20 33 26 32 32 C35 28 36 24 35 18 C44 24 49 32 48 40 C47 48 41 54 32 54 Z" fill="#ff8a3a"/>
      <path d="M32 54 C25 54 21 49 22 43 C23 38 27 36 28 31 C33 35 35 39 34 44 C37 42 38 39 38 36 C42 40 43 45 42 48 C40 52 36 54 32 54 Z" fill="currentColor"/>
      <path d="M25 40 L30 42 M39 40 L34 42" stroke="#7a1505" stroke-width="2.4" ${STROKE}/>`,
  },
  // Bola de fogo com cauda.
  fireball: {
    bg: ["#e2731f", "#3d1505"],
    ink: "#fff6d6",
    glyph: `
      <path d="M10 54 C20 46 24 42 30 36" stroke="#ff9a3a" stroke-width="7" ${STROKE} opacity=".55"/>
      <path d="M14 48 C22 42 26 40 31 35" stroke="#ffc35a" stroke-width="4" ${STROKE} opacity=".8"/>
      <circle cx="38" cy="28" r="14" fill="#ff7a1a"/>
      <circle cx="39" cy="27" r="9.5" fill="#ffc94a"/>
      <circle cx="40.5" cy="25.5" r="5" fill="currentColor"/>`,
  },
  // Estrela de gelo de seis pontas.
  ice_nova: {
    bg: ["#4fb3e0", "#0c2a44"],
    ink: "#f1fbff",
    glyph: `
      <g stroke="currentColor" stroke-width="3.4" ${STROKE}>
        <path d="M32 10 V54 M13 21 L51 43 M13 43 L51 21"/>
        <path d="M27 14 L32 19 L37 14 M27 50 L32 45 L37 50" stroke-width="2.6"/>
        <path d="M13 27 L19 25 L17 19 M51 37 L45 39 L47 45 M13 37 L19 39 L17 45 M51 27 L45 25 L47 19" stroke-width="2.6"/>
      </g>
      <circle cx="32" cy="32" r="5" fill="#bfe9ff"/>`,
  },
  // Adaga com brilho na ponta.
  backstab: {
    bg: ["#7a46b8", "#1f0c33"],
    ink: "#f4ecff",
    glyph: `
      <path d="M46 12 L52 18 L30 40 L24 34 Z" fill="currentColor"/>
      <path d="M46 12 L52 18 L41 29 Z" fill="#cdb8f2"/>
      <path d="M20 32 L32 44" stroke="#c9a85a" stroke-width="4" ${STROKE}/>
      <path d="M24 40 L14 50" stroke="#5a3a24" stroke-width="5" ${STROKE}/>
      <path d="M50 8 L52 4 M55 12 L59 10 M54 7 L57 4" stroke="#ffe6a0" stroke-width="2" ${STROKE}/>`,
  },
  // Silhueta correndo com rastro de sombra.
  shadow_step: {
    bg: ["#4b3b7a", "#0d0a1c"],
    ink: "#e6defc",
    glyph: `
      <g fill="#8a76c8" opacity=".45" transform="translate(-9 2)">
        <circle cx="38" cy="15" r="5"/>
        <path d="M34 22 L44 24 L41 36 L48 46 L43 48 L36 39 L30 47 L25 44 L31 34 L28 28 L22 30 L21 25 Z"/>
      </g>
      <g fill="currentColor">
        <circle cx="40" cy="14" r="5"/>
        <path d="M36 21 L46 23 L43 35 L50 45 L45 47 L38 38 L32 46 L27 43 L33 33 L30 27 L24 29 L23 24 Z"/>
      </g>
      <path d="M8 50 H22 M12 44 H20" stroke="#8a76c8" stroke-width="2.6" ${STROKE}/>`,
  },
};

const EFFECT_ICONS: Record<number, Theme> = {
  [Effect.STUN]: {
    bg: ["#c9a227", "#3a2a05"],
    ink: "#fff7c8",
    glyph: `
      <path d="M32 12 L36 24 L49 24 L39 32 L43 45 L32 37 L21 45 L25 32 L15 24 L28 24 Z" fill="currentColor"/>
      <path d="M10 50 C22 56 42 56 54 50" stroke="#ffe27a" stroke-width="3" fill="none" ${STROKE}/>`,
  },
  [Effect.SLOW]: {
    bg: ["#5aa9d6", "#0c2638"],
    ink: "#eaf8ff",
    glyph: `
      <g stroke="currentColor" stroke-width="3.6" ${STROKE}>
        <path d="M32 12 V52 M15 22 L49 42 M15 42 L49 22"/>
      </g>
      <circle cx="32" cy="32" r="5" fill="#bfe9ff"/>`,
  },
  [Effect.BUFF_DEF]: {
    bg: ["#4d79c9", "#14234a"],
    ink: "#eef3ff",
    glyph: `
      <path d="M32 10 L50 17 V30 C50 41 42 49 32 54 C22 49 14 41 14 30 V17 Z" fill="currentColor"/>
      <path d="M32 22 V40 M24 31 H40" stroke="#2f5bb0" stroke-width="5" ${STROKE}/>`,
  },
  [Effect.BUFF_ATK]: {
    bg: ["#c8432f", "#3a0d07"],
    ink: "#fff0e6",
    glyph: `
      <path d="M44 10 L50 16 L28 38 L22 32 Z" fill="currentColor"/>
      <path d="M18 30 L30 42" stroke="#d6b46e" stroke-width="4" ${STROKE}/>
      <path d="M22 40 L14 48" stroke="#6a3a22" stroke-width="5" ${STROKE}/>
      <path d="M46 46 V30 M39 37 L46 30 L53 37" stroke="#ffd36b" stroke-width="3.4" fill="none" ${STROKE}/>`,
  },
  [Effect.BUFF_SPEED]: {
    bg: ["#3fae6a", "#0b2a16"],
    ink: "#eafff1",
    glyph: `
      <path d="M20 20 L34 32 L20 44 M32 20 L46 32 L32 44" stroke="currentColor" stroke-width="5" fill="none" ${STROKE}/>
      <path d="M8 26 H14 M6 32 H13 M8 38 H14" stroke="#a6f0c0" stroke-width="2.6" ${STROKE}/>`,
  },
};

export function render(t: Theme, id: string): string {
  return `<svg viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" style="color:${t.ink}">
    <defs>
      <radialGradient id="g-${id}" cx="40%" cy="35%" r="75%">
        <stop offset="0" stop-color="${t.bg[0]}"/><stop offset="1" stop-color="${t.bg[1]}"/>
      </radialGradient>
      <filter id="s-${id}" x="-20%" y="-20%" width="140%" height="140%">
        <feDropShadow dx="0" dy="1.5" stdDeviation="1.2" flood-color="#000" flood-opacity=".6"/>
      </filter>
    </defs>
    <rect width="64" height="64" fill="url(#g-${id})"/>
    <g filter="url(#s-${id})">${t.glyph}</g>
  </svg>`;
}

/** SVG do ícone da skill (id do fx). Skills desconhecidas usam um ícone neutro. */
export function skillIcon(fx: string): string {
  return render(SKILL_ICONS[fx] ?? SKILL_ICONS.brutal_strike, `sk-${fx || "x"}`);
}

export function effectIcon(effect: Effect): string {
  const t = EFFECT_ICONS[effect];
  return t ? render(t, `ef-${effect}`) : "";
}

/** Nome do efeito para a dica. */
export function effectName(effect: Effect): string {
  switch (effect) {
    case Effect.STUN:
      return "Atordoado";
    case Effect.SLOW:
      return "Lentidão";
    case Effect.BUFF_DEF:
      return "Defesa aumentada";
    case Effect.BUFF_ATK:
      return "Ataque aumentado";
    case Effect.BUFF_SPEED:
      return "Velocidade aumentada";
    default:
      return "Efeito";
  }
}

/** Descrição curta do efeito ("+50% de defesa por 10 s"). */
export function effectText(effect: Effect, value: number, ms: number): string {
  const pct = `${Math.round(value * 100)}%`;
  const dur = `${formatSeconds(ms)} s`;
  switch (effect) {
    case Effect.STUN:
      return `Atordoa por ${dur}`;
    case Effect.SLOW:
      return `Reduz a velocidade em ${pct} por ${dur}`;
    case Effect.BUFF_DEF:
      return `+${pct} de defesa por ${dur}`;
    case Effect.BUFF_ATK:
      return `+${pct} de ataque por ${dur}`;
    case Effect.BUFF_SPEED:
      return `+${pct} de velocidade por ${dur}`;
    default:
      return "";
  }
}

/** Efeitos ruins (bordas vermelhas). */
export function isDebuff(effect: Effect): boolean {
  return effect === Effect.STUN || effect === Effect.SLOW;
}

export function formatSeconds(ms: number): string {
  const s = ms / 1000;
  return Number.isInteger(s) ? String(s) : s.toFixed(1).replace(".", ",");
}
