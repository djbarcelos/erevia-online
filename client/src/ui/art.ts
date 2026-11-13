/**
 * Arte original do protótipo, em SVG 64×64 desenhado à mão:
 * - ícones de skills novas (mesmo estilo de src/ui/icons.ts: fundo radial + glifo claro);
 * - ícones de itens (objeto sobre fundo transparente; a raridade vem da moldura do slot).
 */

const S = `stroke-linecap="round" stroke-linejoin="round"`;
let uid = 0;

/**
 * Dá ids únicos aos <defs> de um SVG. Ícones repetidos com o mesmo id quebram quando a
 * primeira cópia está escondida (display:none): o degradê some e o ícone fica preto.
 */
export function uniqueIds(svg: string): string {
  const n = uid++;
  return svg.replace(/id="([^"]+)"/g, `id="$1-u${n}"`).replace(/url\(#([^)]+)\)/g, `url(#$1-u${n})`);
}

// ---------------------------------------------------------------------------------------
// Glifos das skills (usam currentColor = tinta clara, e `a` = cor de destaque)
// ---------------------------------------------------------------------------------------

type Glyph = (a: string) => string;

export const GLYPHS: Record<string, Glyph> = {
  shield: (a) => `
    <path d="M32 10 L50 17 V31 C50 42 42 50 32 54 C22 50 14 42 14 31 V17 Z" fill="currentColor"/>
    <path d="M32 17 L44 22 V31 C44 38 39 44 32 47 Z" fill="${a}" opacity=".8"/>`,
  sword: (a) => `
    <path d="M44 10 L54 10 L54 20 L28 46 L18 36 Z" fill="currentColor"/>
    <path d="M46 14 L50 14 L50 18 L27 41 L23 37 Z" fill="${a}" opacity=".55"/>
    <path d="M14 34 L30 50" stroke="#c9a24a" stroke-width="5" ${S}/>
    <path d="M18 46 L10 54" stroke="#6b4325" stroke-width="5" ${S}/>`,
  swirl: (a) => `
    <path d="M32 12 A20 20 0 1 1 13 38" stroke="currentColor" stroke-width="5" fill="none" ${S}/>
    <path d="M32 22 A10 10 0 1 1 23 36" stroke="${a}" stroke-width="4" fill="none" ${S}/>
    <path d="M8 34 L13 40 L19 34" stroke="currentColor" stroke-width="4" fill="none" ${S}/>`,
  dash: (a) => `
    <path d="M10 22 H30 M6 32 H26 M12 42 H28" stroke="${a}" stroke-width="3.5" ${S} opacity=".8"/>
    <path d="M30 14 L54 32 L30 50 L36 32 Z" fill="currentColor"/>`,
  crown: (a) => `
    <path d="M12 44 L14 20 L24 30 L32 14 L40 30 L50 20 L52 44 Z" fill="currentColor"/>
    <rect x="12" y="44" width="40" height="7" rx="2" fill="${a}"/>
    <circle cx="32" cy="34" r="4" fill="${a}"/>`,
  shout: (a) => `
    <path d="M10 26 H20 L34 14 V50 L20 38 H10 Z" fill="currentColor"/>
    <path d="M42 22 C46 26 46 38 42 42 M48 16 C55 24 55 40 48 48" stroke="${a}" stroke-width="3.5" fill="none" ${S}/>`,
  banner: (a) => `
    <path d="M18 8 V56" stroke="#7a5a35" stroke-width="4" ${S}/>
    <path d="M20 12 H50 L44 24 L50 36 H20 Z" fill="currentColor"/>
    <path d="M30 18 L34 24 L30 30 L26 24 Z" fill="${a}"/>`,
  heart: (a) => `
    <path d="M32 52 C14 40 10 30 12 22 C14 13 26 11 32 20 C38 11 50 13 52 22 C54 30 50 40 32 52 Z" fill="currentColor"/>
    <path d="M24 30 H30 L33 24 L36 36 L39 30 H44" stroke="${a}" stroke-width="3" fill="none" ${S}/>`,
  axe: (a) => `
    <path d="M20 54 L44 18" stroke="#7a4a2a" stroke-width="5" ${S}/>
    <path d="M36 10 C48 6 58 16 56 28 C50 25 44 25 38 30 C38 22 38 16 36 10 Z" fill="currentColor"/>
    <path d="M38 30 L46 18" stroke="${a}" stroke-width="2.5" ${S}/>`,
  crack: (a) => `
    <path d="M6 46 H58" stroke="currentColor" stroke-width="4" ${S}/>
    <path d="M32 46 L26 36 L34 30 L28 18 M32 46 L42 38 L38 28 L46 20" stroke="${a}" stroke-width="3.2" fill="none" ${S}/>
    <circle cx="14" cy="38" r="3" fill="currentColor"/><circle cx="50" cy="36" r="2.5" fill="currentColor"/>`,
  drop: (a) => `
    <path d="M32 8 C40 22 48 30 48 40 C48 50 40 56 32 56 C24 56 16 50 16 40 C16 30 24 22 32 8 Z" fill="currentColor"/>
    <path d="M24 40 C24 46 28 49 32 49" stroke="${a}" stroke-width="3" fill="none" ${S}/>`,
  fist: (a) => `
    <rect x="16" y="20" width="32" height="24" rx="8" fill="currentColor"/>
    <path d="M24 20 V30 M32 20 V30 M40 20 V30" stroke="${a}" stroke-width="2.5" ${S}/>
    <rect x="20" y="42" width="20" height="12" rx="3" fill="currentColor"/>
    <path d="M8 18 L14 22 M8 30 H14 M54 18 L50 22" stroke="${a}" stroke-width="3" ${S}/>`,
  paw: (a) => `
    <ellipse cx="32" cy="40" rx="13" ry="11" fill="currentColor"/>
    <ellipse cx="17" cy="26" rx="5" ry="7" fill="currentColor"/><ellipse cx="27" cy="18" rx="5" ry="7" fill="currentColor"/>
    <ellipse cx="38" cy="18" rx="5" ry="7" fill="currentColor"/><ellipse cx="48" cy="26" rx="5" ry="7" fill="currentColor"/>
    <path d="M26 42 L32 36 L38 42" stroke="${a}" stroke-width="2.5" fill="none" ${S}/>`,
  flame: (a) => `
    <path d="M32 56 C18 56 12 46 15 36 C18 27 26 24 25 12 C34 18 37 26 36 33 C40 29 41 24 40 18 C50 26 54 36 51 44 C48 51 41 56 32 56 Z" fill="${a}"/>
    <path d="M32 56 C25 56 21 51 22 45 C23 40 28 37 29 31 C34 36 36 41 35 46 C38 44 39 41 39 38 C43 42 44 47 42 50 C40 54 36 56 32 56 Z" fill="currentColor"/>`,
  meteor: (a) => `
    <path d="M10 10 L34 30 M18 6 L38 26 M6 20 L28 36" stroke="${a}" stroke-width="3.5" ${S} opacity=".8"/>
    <circle cx="40" cy="40" r="14" fill="currentColor"/>
    <circle cx="36" cy="36" r="4" fill="${a}" opacity=".7"/><circle cx="46" cy="44" r="3" fill="${a}" opacity=".6"/>`,
  snow: (a) => `
    <path d="M32 8 V56 M11 20 L53 44 M11 44 L53 20" stroke="currentColor" stroke-width="4" ${S}/>
    <path d="M26 12 L32 18 L38 12 M26 52 L32 46 L38 52" stroke="${a}" stroke-width="3" fill="none" ${S}/>
    <circle cx="32" cy="32" r="5" fill="${a}"/>`,
  spear: (a) => `
    <path d="M12 52 L40 24" stroke="currentColor" stroke-width="4" ${S}/>
    <path d="M38 14 L54 10 L50 26 L42 30 L34 22 Z" fill="currentColor"/>
    <path d="M42 18 L48 16 L46 22 Z" fill="${a}"/>
    <path d="M14 40 L8 46 M24 50 L18 56" stroke="${a}" stroke-width="2.5" ${S}/>`,
  crystal: (a) => `
    <path d="M32 6 L46 22 L40 52 H24 L18 22 Z" fill="currentColor"/>
    <path d="M32 6 L32 52 M18 22 H46" stroke="${a}" stroke-width="2" opacity=".8"/>
    <path d="M10 34 L16 30 M54 34 L48 30" stroke="${a}" stroke-width="3" ${S}/>`,
  cage: (a) => `
    <path d="M14 14 H50 M14 50 H50" stroke="currentColor" stroke-width="5" ${S}/>
    <path d="M20 14 V50 M30 14 V50 M40 14 V50 M48 14 V50" stroke="currentColor" stroke-width="3" ${S}/>
    <circle cx="34" cy="32" r="6" fill="${a}"/>`,
  eye: (a) => `
    <path d="M6 32 C16 16 48 16 58 32 C48 48 16 48 6 32 Z" fill="currentColor"/>
    <circle cx="32" cy="32" r="10" fill="${a}"/><circle cx="32" cy="32" r="4" fill="#111"/>`,
  burst: (a) => `
    <path d="M32 6 L37 24 L56 20 L42 33 L54 48 L36 42 L32 58 L27 42 L10 48 L22 33 L8 20 L27 24 Z" fill="currentColor"/>
    <circle cx="32" cy="32" r="7" fill="${a}"/>`,
  blink: (a) => `
    <circle cx="18" cy="40" r="8" fill="none" stroke="${a}" stroke-width="3" stroke-dasharray="4 4"/>
    <path d="M24 34 L40 22" stroke="${a}" stroke-width="3" ${S} stroke-dasharray="2 6"/>
    <circle cx="44" cy="22" r="10" fill="currentColor"/><path d="M44 14 V30 M36 22 H52" stroke="${a}" stroke-width="2.5" ${S}/>`,
  daggers: (a) => `
    <path d="M14 50 L40 16 L44 20 Z" fill="currentColor"/><path d="M50 50 L24 16 L20 20 Z" fill="currentColor"/>
    <path d="M14 42 L22 50 M50 42 L42 50" stroke="${a}" stroke-width="4" ${S}/>`,
  mark: (a) => `
    <circle cx="32" cy="32" r="20" fill="none" stroke="currentColor" stroke-width="4"/>
    <circle cx="32" cy="32" r="9" fill="none" stroke="${a}" stroke-width="3.5"/>
    <path d="M32 4 V16 M32 48 V60 M4 32 H16 M48 32 H60" stroke="currentColor" stroke-width="4" ${S}/>`,
  smoke: (a) => `
    <circle cx="22" cy="38" r="11" fill="currentColor" opacity=".9"/><circle cx="38" cy="32" r="14" fill="currentColor"/>
    <circle cx="44" cy="44" r="9" fill="currentColor" opacity=".85"/>
    <circle cx="20" cy="18" r="6" fill="${a}"/><path d="M24 14 L30 8" stroke="${a}" stroke-width="2.5" ${S}/>`,
  mask: (a) => `
    <path d="M8 24 C20 18 44 18 56 24 C56 36 46 44 32 40 C18 44 8 36 8 24 Z" fill="currentColor"/>
    <path d="M17 28 L26 30 M47 28 L38 30" stroke="${a}" stroke-width="3.5" ${S}/>`,
  flask: (a) => `
    <path d="M26 8 H38 M28 8 V22 L14 48 C12 53 15 56 20 56 H44 C49 56 52 53 50 48 L36 22 V8" fill="currentColor"/>
    <path d="M18 44 H46 L50 50 C51 53 49 54 46 54 H18 C15 54 13 53 14 50 Z" fill="${a}"/>
    <circle cx="28" cy="40" r="2.5" fill="${a}"/><circle cx="36" cy="34" r="2" fill="${a}"/>`,
  skull: (a) => `
    <path d="M32 8 C18 8 12 18 12 28 C12 36 16 40 20 42 V52 H44 V42 C48 40 52 36 52 28 C52 18 46 8 32 8 Z" fill="currentColor"/>
    <circle cx="24" cy="29" r="5.5" fill="${a}"/><circle cx="40" cy="29" r="5.5" fill="${a}"/>
    <path d="M28 52 V46 M36 52 V46" stroke="#111" stroke-width="2.5"/>`,
  star: (a) => `
    <path d="M32 6 L39 24 L58 25 L43 37 L48 56 L32 45 L16 56 L21 37 L6 25 L25 24 Z" fill="currentColor"/>
    <path d="M32 20 L35 29 L44 30 L37 36 L39 44 L32 39 L25 44 L27 36 L20 30 L29 29 Z" fill="${a}"/>`,
  book: (a) => `
    <path d="M8 16 C18 12 26 13 32 18 C38 13 46 12 56 16 V50 C46 46 38 47 32 52 C26 47 18 46 8 50 Z" fill="currentColor"/>
    <path d="M32 18 V52" stroke="${a}" stroke-width="2.5"/>
    <path d="M14 24 H26 M14 31 H26 M38 24 H50 M38 31 H50" stroke="${a}" stroke-width="2" ${S} opacity=".8"/>`,
  hourglass: (a) => `
    <path d="M16 8 H48 M16 56 H48" stroke="currentColor" stroke-width="4.5" ${S}/>
    <path d="M20 10 C20 24 28 28 32 32 C28 36 20 40 20 54 H44 C44 40 36 36 32 32 C36 28 44 24 44 10 Z" fill="currentColor"/>
    <path d="M24 52 C26 44 30 40 32 38 C34 40 38 44 40 52 Z" fill="${a}"/>`,
  bolt: (a) => `
    <path d="M36 6 L14 36 H30 L26 58 L50 26 H34 Z" fill="currentColor"/>
    <path d="M34 14 L22 32 H32" stroke="${a}" stroke-width="2.5" fill="none" ${S}/>`,
};

export interface SkillTheme {
  glyph: keyof typeof GLYPHS;
  bg: [string, string];
  ink?: string;
  accent?: string;
}

/** Ícone de skill no estilo dos ícones atuais do jogo. */
export function skillArt(t: SkillTheme): string {
  const id = `pa${uid++}`;
  const ink = t.ink ?? "#f4efe4";
  const accent = t.accent ?? t.bg[0];
  return `<svg viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" style="color:${ink}">
    <defs>
      <radialGradient id="g-${id}" cx="40%" cy="35%" r="75%">
        <stop offset="0" stop-color="${t.bg[0]}"/><stop offset="1" stop-color="${t.bg[1]}"/>
      </radialGradient>
      <filter id="s-${id}" x="-20%" y="-20%" width="140%" height="140%">
        <feDropShadow dx="0" dy="1.5" stdDeviation="1.2" flood-color="#000" flood-opacity=".6"/>
      </filter>
    </defs>
    <rect width="64" height="64" fill="url(#g-${id})"/>
    <g filter="url(#s-${id})">${GLYPHS[t.glyph](accent)}</g>
  </svg>`;
}

// ---------------------------------------------------------------------------------------
// Itens
// ---------------------------------------------------------------------------------------

const METAL = ["#eef2f8", "#9aa6b8", "#5c6678"];

const ITEMS: Record<string, (c: string) => string> = {
  sword: (c) => `
    <path d="M46 8 L56 8 L56 18 L26 48 L16 38 Z" fill="${METAL[0]}" stroke="#1a1410" stroke-width="1.5"/>
    <path d="M51 11 L22 40" stroke="${METAL[2]}" stroke-width="1.6"/>
    <path d="M12 34 L30 52" stroke="#1a1410" stroke-width="7" ${S}/><path d="M12 34 L30 52" stroke="${c}" stroke-width="4.2" ${S}/>
    <path d="M18 46 L9 55" stroke="#1a1410" stroke-width="6.5" ${S}/><path d="M18 46 L9 55" stroke="#7a4a2a" stroke-width="3.8" ${S}/>
    <circle cx="8" cy="56" r="3.4" fill="${c}" stroke="#1a1410" stroke-width="1.4"/>`,
  axe: (c) => `
    <path d="M16 56 L44 16" stroke="#1a1410" stroke-width="7" ${S}/><path d="M16 56 L44 16" stroke="#8a5a32" stroke-width="4" ${S}/>
    <path d="M34 8 C48 2 62 14 58 30 C52 26 44 26 38 32 C38 22 37 14 34 8 Z" fill="${METAL[0]}" stroke="#1a1410" stroke-width="1.6"/>
    <path d="M40 28 C44 24 50 24 55 26" stroke="${METAL[2]}" stroke-width="1.6" fill="none"/>
    <rect x="38" y="18" width="8" height="6" rx="1" transform="rotate(-56 42 21)" fill="${c}" stroke="#1a1410" stroke-width="1.2"/>`,
  dagger: (c) => `
    <path d="M40 10 L48 16 L28 40 L22 36 Z" fill="${METAL[0]}" stroke="#1a1410" stroke-width="1.5"/>
    <path d="M44 13 L25 38" stroke="${METAL[2]}" stroke-width="1.3"/>
    <path d="M16 32 L30 46" stroke="#1a1410" stroke-width="6" ${S}/><path d="M16 32 L30 46" stroke="${c}" stroke-width="3.4" ${S}/>
    <path d="M22 42 L12 54" stroke="#1a1410" stroke-width="6" ${S}/><path d="M22 42 L12 54" stroke="#5a3a24" stroke-width="3.4" ${S}/>`,
  staff: (c) => `
    <path d="M14 58 L42 18" stroke="#1a1410" stroke-width="6.5" ${S}/><path d="M14 58 L42 18" stroke="#8a6a42" stroke-width="3.6" ${S}/>
    <path d="M38 22 C34 12 42 4 50 8 C58 12 56 24 46 24" fill="none" stroke="#1a1410" stroke-width="5.5" ${S}/>
    <path d="M38 22 C34 12 42 4 50 8 C58 12 56 24 46 24" fill="none" stroke="#8a6a42" stroke-width="2.8" ${S}/>
    <circle cx="47" cy="15" r="6.5" fill="${c}" stroke="#1a1410" stroke-width="1.5"/>
    <circle cx="45" cy="13" r="2" fill="#fff" opacity=".8"/>`,
  shield: (c) => `
    <path d="M32 6 L54 14 V30 C54 44 44 53 32 58 C20 53 10 44 10 30 V14 Z" fill="${c}" stroke="#1a1410" stroke-width="2"/>
    <path d="M32 12 L48 18 V30 C48 40 41 47 32 51 C23 47 16 40 16 30 V18 Z" fill="none" stroke="#f3d995" stroke-width="2"/>
    <path d="M32 18 V46 M20 28 H44" stroke="#f3d995" stroke-width="3" ${S}/>`,
  buckler: (c) => `
    <circle cx="32" cy="32" r="24" fill="#7a4e2c" stroke="#1a1410" stroke-width="2"/>
    <circle cx="32" cy="32" r="24" fill="none" stroke="${METAL[1]}" stroke-width="4"/>
    <path d="M14 22 L50 42 M14 42 L50 22" stroke="#5a3a20" stroke-width="2"/>
    <circle cx="32" cy="32" r="7" fill="${c}" stroke="#1a1410" stroke-width="1.5"/>`,
  helmet: (c) => `
    <path d="M12 40 C12 20 22 10 32 10 C42 10 52 20 52 40 V48 H40 V36 H24 V48 H12 Z" fill="${METAL[1]}" stroke="#1a1410" stroke-width="2"/>
    <path d="M32 10 V34" stroke="${c}" stroke-width="5"/>
    <path d="M18 26 C24 22 40 22 46 26" stroke="${METAL[0]}" stroke-width="2" fill="none"/>
    <path d="M32 4 C30 8 30 12 32 14 C34 12 34 8 32 4 Z" fill="${c}"/>`,
  chest: (c) => `
    <path d="M18 10 L26 8 C28 12 36 12 38 8 L46 10 L56 20 L50 30 L46 26 V54 H18 V26 L14 30 L8 20 Z" fill="${c}" stroke="#1a1410" stroke-width="2"/>
    <path d="M24 22 H40 M24 32 H40 M24 42 H40" stroke="rgba(0,0,0,.35)" stroke-width="2"/>
    <path d="M32 12 V54" stroke="#f3d995" stroke-width="2" opacity=".7"/>`,
  legs: (c) => `
    <path d="M18 8 H46 L48 30 L44 58 H34 L32 30 L30 58 H20 L16 30 Z" fill="${c}" stroke="#1a1410" stroke-width="2"/>
    <path d="M18 14 H46" stroke="#f3d995" stroke-width="3"/>`,
  gloves: (c) => `
    <path d="M18 56 V34 L14 22 C13 18 18 16 20 20 L24 30 V12 C24 8 30 8 30 12 V28 V10 C30 6 36 6 36 10 V28 V14 C36 10 42 10 42 14 V32 L46 26 C48 22 54 24 52 28 L44 44 V56 Z" fill="${c}" stroke="#1a1410" stroke-width="1.8" ${S}/>
    <path d="M18 48 H44" stroke="#f3d995" stroke-width="3"/>`,
  boots: (c) => `
    <path d="M18 8 H34 V40 L52 46 C56 47 56 56 52 56 H14 L16 40 Z" fill="${c}" stroke="#1a1410" stroke-width="2" ${S}/>
    <path d="M16 14 H36" stroke="#f3d995" stroke-width="3"/><path d="M14 50 H54" stroke="rgba(0,0,0,.45)" stroke-width="3"/>`,
  cape: (c) => `
    <path d="M22 8 H42 C44 20 52 36 56 56 C46 52 40 56 32 52 C24 56 18 52 8 56 C12 36 20 20 22 8 Z" fill="${c}" stroke="#1a1410" stroke-width="2"/>
    <path d="M22 10 H42" stroke="#f3d995" stroke-width="4"/>
    <path d="M26 20 C24 32 20 42 18 50 M38 20 C40 32 44 42 46 50" stroke="rgba(0,0,0,.3)" stroke-width="2" fill="none"/>`,
  ring: (c) => `
    <ellipse cx="32" cy="40" rx="17" ry="14" fill="none" stroke="#1a1410" stroke-width="8"/>
    <ellipse cx="32" cy="40" rx="17" ry="14" fill="none" stroke="#e2c070" stroke-width="5"/>
    <path d="M24 22 L32 12 L40 22 L32 30 Z" fill="${c}" stroke="#1a1410" stroke-width="1.8"/>
    <path d="M29 20 L32 16" stroke="#fff" stroke-width="1.6" opacity=".8"/>`,
  necklace: (c) => `
    <path d="M12 8 C12 30 22 40 32 40 C42 40 52 30 52 8" fill="none" stroke="#e2c070" stroke-width="3" stroke-dasharray="3 2"/>
    <path d="M32 38 L42 48 L32 60 L22 48 Z" fill="${c}" stroke="#1a1410" stroke-width="2"/>
    <path d="M30 44 L32 42" stroke="#fff" stroke-width="2" opacity=".8"/>`,
  earring: (c) => `
    <circle cx="32" cy="14" r="6" fill="none" stroke="#e2c070" stroke-width="3"/>
    <path d="M32 20 V28" stroke="#e2c070" stroke-width="3"/>
    <path d="M32 28 C40 36 42 44 32 54 C22 44 24 36 32 28 Z" fill="${c}" stroke="#1a1410" stroke-width="2"/>`,
  potion: (c) => `
    <rect x="26" y="6" width="12" height="7" rx="2" fill="#8a5a32" stroke="#1a1410" stroke-width="1.5"/>
    <path d="M28 13 V22 C18 26 14 34 14 42 C14 52 22 58 32 58 C42 58 50 52 50 42 C50 34 46 26 36 22 V13 Z" fill="rgba(220,235,255,.25)" stroke="#1a1410" stroke-width="2"/>
    <path d="M17 40 C17 50 24 55 32 55 C40 55 47 50 47 40 Z" fill="${c}"/>
    <path d="M22 30 C20 34 19 38 20 42" stroke="#fff" stroke-width="2.5" opacity=".6" ${S}/>`,
  scroll: (c) => `
    <rect x="14" y="14" width="36" height="36" fill="#e8d8b0" stroke="#1a1410" stroke-width="2"/>
    <rect x="10" y="10" width="44" height="8" rx="4" fill="#c9b07a" stroke="#1a1410" stroke-width="2"/>
    <rect x="10" y="46" width="44" height="8" rx="4" fill="#c9b07a" stroke="#1a1410" stroke-width="2"/>
    <path d="M20 26 H44 M20 32 H40 M20 38 H42" stroke="#7a6a4a" stroke-width="2"/>
    <circle cx="40" cy="44" r="5" fill="${c}" stroke="#1a1410" stroke-width="1.5"/>`,
  letter: (c) => `
    <rect x="8" y="16" width="48" height="34" rx="2" fill="#e8d8b0" stroke="#1a1410" stroke-width="2"/>
    <path d="M8 18 L32 36 L56 18" fill="none" stroke="#7a6a4a" stroke-width="2"/>
    <circle cx="32" cy="36" r="7" fill="${c}" stroke="#1a1410" stroke-width="1.5"/>`,
  ore: (c) => `
    <path d="M10 44 L18 24 L32 16 L48 22 L56 40 L44 54 H20 Z" fill="#5c5560" stroke="#1a1410" stroke-width="2"/>
    <path d="M22 30 L28 26 L32 34 Z M38 28 L46 32 L40 40 Z M26 44 L32 40 L36 48 Z" fill="${c}"/>`,
  gem: (c) => `
    <path d="M16 22 L26 10 H38 L48 22 L32 56 Z" fill="${c}" stroke="#1a1410" stroke-width="2"/>
    <path d="M16 22 H48 M26 10 L32 22 L38 10 M32 22 V56" stroke="rgba(255,255,255,.45)" stroke-width="1.5" fill="none"/>
    <path d="M22 18 L26 14" stroke="#fff" stroke-width="2" opacity=".85"/>`,
  bone: (c) => `
    <path d="M16 46 L46 16" stroke="#1a1410" stroke-width="10" ${S}/><path d="M16 46 L46 16" stroke="${c}" stroke-width="7" ${S}/>
    <circle cx="12" cy="44" r="6" fill="${c}" stroke="#1a1410" stroke-width="1.8"/><circle cx="18" cy="52" r="6" fill="${c}" stroke="#1a1410" stroke-width="1.8"/>
    <circle cx="46" cy="12" r="6" fill="${c}" stroke="#1a1410" stroke-width="1.8"/><circle cx="52" cy="20" r="6" fill="${c}" stroke="#1a1410" stroke-width="1.8"/>`,
  coin: (c) => `
    <ellipse cx="32" cy="38" rx="20" ry="9" fill="#a77a30" stroke="#1a1410" stroke-width="2"/>
    <ellipse cx="32" cy="32" rx="20" ry="9" fill="${c}" stroke="#1a1410" stroke-width="2"/>
    <ellipse cx="32" cy="32" rx="11" ry="4.5" fill="none" stroke="#a77a30" stroke-width="2"/>`,
};

export type ItemArt = keyof typeof ITEMS;

/** Ícone de item: objeto com sombra suave. */
export function itemArt(kind: ItemArt, color: string): string {
  const id = `ia${uid++}`;
  return `<svg viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <defs><filter id="${id}" x="-20%" y="-20%" width="140%" height="140%">
      <feDropShadow dx="0" dy="2" stdDeviation="1.6" flood-color="#000" flood-opacity=".75"/>
    </filter></defs>
    <g filter="url(#${id})">${ITEMS[kind](color)}</g>
  </svg>`;
}

// ---------------------------------------------------------------------------------------
// Ícones de interface (traço simples, herdam a cor do texto)
// ---------------------------------------------------------------------------------------

const UI: Record<string, string> = {
  bag: `<path d="M6 9h12l-1 11H7L6 9Z"/><path d="M9 9V7a3 3 0 0 1 6 0v2"/>`,
  book: `<path d="M4 5c3-1 6-1 8 1 2-2 5-2 8-1v14c-3-1-6-1-8 1-2-2-5-2-8-1V5Z"/><path d="M12 6v14"/>`,
  user: `<circle cx="12" cy="8" r="4"/><path d="M4 21c1-5 5-7 8-7s7 2 8 7"/>`,
  map: `<path d="M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2V6Z"/><path d="M9 4v14M15 6v14"/>`,
  gear: `<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1 7 17M17 7l2.1-2.1"/>`,
  menu: `<path d="M4 7h16M4 12h16M4 17h16"/>`,
  chat: `<path d="M4 5h16v11H9l-5 4V5Z"/>`,
  sword: `<path d="M14 4h6v6L9 21l-6-6L14 4Z"/><path d="M6 13l5 5"/>`,
  target: `<circle cx="12" cy="12" r="8"/><circle cx="12" cy="12" r="3"/><path d="M12 1v4M12 19v4M1 12h4M19 12h4"/>`,
  auto: `<path d="M20 12a8 8 0 1 1-2.3-5.7"/><path d="M20 4v5h-5"/>`,
  sort: `<path d="M7 4v16M3 16l4 4 4-4M17 20V4M13 8l4-4 4 4"/>`,
  search: `<circle cx="11" cy="11" r="7"/><path d="M20 20l-4-4"/>`,
  clan: `<path d="M6 3v18"/><path d="M6 4h12l-3 4.5 3 4.5H6"/>`,
  users: `<circle cx="9" cy="8" r="3.5"/><path d="M2 20c1-4 4-6 7-6s6 2 7 6"/><circle cx="17" cy="7" r="2.5"/><path d="M17 12c2 0 4 1.5 5 5"/>`,
  scroll: `<path d="M6 4h11a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M9 9h7M9 13h7"/>`,
  shop: `<path d="M4 9l2-5h12l2 5"/><path d="M5 9v11h14V9"/><path d="M10 20v-6h4v6"/>`,
  mail: `<rect x="3" y="6" width="18" height="13" rx="1"/><path d="M3 7l9 7 9-7"/>`,
  trophy: `<path d="M8 4h8v5a4 4 0 0 1-8 0V4Z"/><path d="M8 6H4a3 3 0 0 0 4 4M16 6h4a3 3 0 0 1-4 4M12 13v4M8 20h8"/>`,
  logout: `<path d="M15 4h4v16h-4"/><path d="M10 8l-4 4 4 4M6 12h10"/>`,
  rotate: `<path d="M4 12a8 8 0 0 1 14-5l2 2"/><path d="M20 4v5h-5"/><path d="M20 12a8 8 0 0 1-14 5l-2-2"/><path d="M4 20v-5h5"/>`,
  phone: `<rect x="7" y="2" width="10" height="20" rx="2"/><path d="M11 18h2"/>`,
  desktop: `<rect x="3" y="4" width="18" height="12" rx="1"/><path d="M8 20h8M12 16v4"/>`,
  tablet: `<rect x="4" y="2" width="16" height="20" rx="2"/><path d="M11 18h2"/>`,
  eye: `<path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>`,
  sun: `<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5 19 19M5 19l1.5-1.5M17.5 6.5 19 5"/>`,
  moon: `<path d="M20 14A8 8 0 0 1 10 4a8 8 0 1 0 10 10Z"/>`,
  wifi: `<path d="M2 9a15 15 0 0 1 20 0M5 13a10 10 0 0 1 14 0M8.5 16.5a5 5 0 0 1 7 0"/><circle cx="12" cy="20" r="1"/>`,
  google: `<path d="M21 12.2c0-.7-.1-1.3-.2-1.9H12v3.6h5.1a4.4 4.4 0 0 1-1.9 2.9v2.4h3.1c1.8-1.7 2.7-4.1 2.7-7Z" fill="#4285F4" stroke="none"/><path d="M12 21.5c2.6 0 4.7-.9 6.3-2.3l-3.1-2.4c-.9.6-2 .9-3.2.9-2.5 0-4.6-1.7-5.3-3.9H3.5v2.5a9.5 9.5 0 0 0 8.5 5.2Z" fill="#34A853" stroke="none"/><path d="M6.7 13.8a5.7 5.7 0 0 1 0-3.6V7.7H3.5a9.5 9.5 0 0 0 0 8.6l3.2-2.5Z" fill="#FBBC05" stroke="none"/><path d="M12 6.3c1.4 0 2.6.5 3.6 1.4l2.7-2.7A9.5 9.5 0 0 0 3.5 7.7l3.2 2.5C7.4 8 9.5 6.3 12 6.3Z" fill="#EA4335" stroke="none"/>`,
  discord: `<path d="M19.3 5.3A16.5 16.5 0 0 0 15.2 4l-.5 1a15.3 15.3 0 0 0-5.4 0l-.5-1a16.5 16.5 0 0 0-4.1 1.3C2.1 9.2 1.4 13 1.7 16.7a16.6 16.6 0 0 0 5 2.6l1.1-1.7c-.6-.2-1.2-.5-1.7-.9l.4-.3a11.8 11.8 0 0 0 11 0l.4.3c-.5.4-1.1.7-1.7.9l1.1 1.7a16.6 16.6 0 0 0 5-2.6c.4-4.3-.7-8-3-11.4ZM8.7 14.5c-1 0-1.8-.9-1.8-2s.8-2 1.8-2 1.8.9 1.8 2-.8 2-1.8 2Zm6.6 0c-1 0-1.8-.9-1.8-2s.8-2 1.8-2 1.8.9 1.8 2-.8 2-1.8 2Z" fill="currentColor" stroke="none"/>`,
  lock: `<rect x="5" y="11" width="14" height="10" rx="1"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>`,
  plus: `<path d="M12 5v14M5 12h14"/>`,
  trash: `<path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13"/>`,
  undo: `<path d="M9 14 4 9l5-5"/><path d="M4 9h10a6 6 0 0 1 0 12h-3"/>`,
  check: `<path d="M5 12l5 5 9-10"/>`,
  sparkle: `<path d="M12 3l2 6 6 2-6 2-2 6-2-6-6-2 6-2 2-6Z"/>`,
  potion: `<path d="M10 3h4M10 3v5L6 15a4 4 0 0 0 4 6h4a4 4 0 0 0 4-6l-4-7V3"/>`,
  swap: `<path d="M4 8h14l-4-4M20 16H6l4 4"/>`,
};

export function ui(name: keyof typeof UI | string, size = 20): string {
  return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${UI[name] ?? ""}</svg>`;
}
