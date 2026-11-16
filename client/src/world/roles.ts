/**
 * Papel do monstro na luta (arquétipo de IA, tunnel#129): o ícone e a dica do quadro do
 * alvo, e se ele está neutro agora (a auto-caça pula os neutros).
 *
 * Os números da dica são os padrões do servidor (server/internal/game/archetypes.go).
 * Este arquivo não importa nada em tempo de execução (só tipos), para os testes rodarem no
 * Node (client/tests/autohunt.test.ts); os valores do enum MonsterArchetype vão escritos
 * à mão, na ordem do protocolo.
 */

import type { MonsterArchetype } from "../gen/erevia/v1/game_pb";

/** Valores de MonsterArchetype (proto/erevia/v1/game.proto), sem importar o enum. */
export const ARCH = {
  brute: 0,
  archer: 1,
  healer: 2,
  summoner: 3,
  ambusher: 4,
  pack: 5,
  shield: 6,
  splitter: 7,
  exploder: 8,
} as const satisfies Record<string, MonsterArchetype>;

/** O que o cliente sabe do papel de um monstro (Entity.role). */
export interface RoleFields {
  role?: MonsterArchetype;
  neutral?: boolean;
  neutralByDay?: boolean;
}

export interface RoleInfo {
  /** Classe CSS do selo (t-role-<key>). */
  key: string;
  label: string;
  hint: string;
  /** Ícone 16×16 em SVG (currentColor). */
  icon: string;
}

const svg = (body: string) =>
  `<svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${body}</svg>`;

const ROLES: Record<number, RoleInfo> = {
  [ARCH.archer]: {
    key: "archer",
    label: "Atirador",
    hint: "Ataca de longe e recua quando você encosta (no máximo uma vez a cada 6 s). Chegue por trás das pedras, atordoe ou prenda.",
    icon: svg(`<path d="M3 13 13 3"/><path d="M9 3h4v4"/><path d="M3 10v3h3"/>`),
  },
  [ARCH.healer]: {
    key: "healer",
    label: "Curandeiro",
    hint: "Cura um aliado ferido (abaixo da metade da vida) a cada 8 s. Derrube-o primeiro, ou atordoe.",
    icon: svg(`<path d="M6.2 2.5h3.6v3.7h3.7v3.6H9.8v3.7H6.2V9.8H2.5V6.2h3.7z"/>`),
  },
  [ARCH.summoner]: {
    key: "summoner",
    label: "Invocador",
    hint: "Chama 2 lacaios a cada 20 s de luta (até 4 de uma vez). Os lacaios caem com ele.",
    icon: svg(`<circle cx="8" cy="8" r="5.5"/><path d="M8 2.5 9.6 6.4h-3.2zM3.2 10.7l4-1.4-1.2 3.2zM12.8 10.7l-4-1.4 1.2 3.2z"/>`),
  },
  [ARCH.ambusher]: {
    key: "ambusher",
    label: "Emboscador",
    hint: "Fica escondido até você chegar a uns 4 m. O primeiro golpe dele dói 50% a mais.",
    icon: svg(`<path d="M1.5 8s2.4-4.5 6.5-4.5S14.5 8 14.5 8 12.1 12.5 8 12.5 1.5 8 1.5 8z"/><circle cx="8" cy="8" r="2"/>`),
  },
  [ARCH.pack]: {
    key: "pack",
    label: "Matilha",
    hint: "Andam em bando: bater em um traz todos. O último que sobra fica mais rápido.",
    icon: svg(`<circle cx="4" cy="6" r="1.3"/><circle cx="7" cy="3.6" r="1.3"/><circle cx="10.6" cy="4.2" r="1.3"/><circle cx="12.6" cy="7.4" r="1.3"/><path d="M5.5 12.4c0-2.4 1.5-4.4 3.5-4.4s2.8 1.6 2.8 3.2-1.2 1.8-3 1.8-3.3.6-3.3-.6z"/>`),
  },
  [ARCH.shield]: {
    key: "shield",
    label: "Escudo frontal",
    hint: "O escudo segura 60% do dano físico que vem de frente, e ele vira devagar. Ataque pelos lados, pelas costas ou com magia.",
    icon: svg(`<path d="M8 1.8 13.2 3.6v4.1c0 3.2-2.2 5.5-5.2 6.5-3-1-5.2-3.3-5.2-6.5V3.6z"/>`),
  },
  [ARCH.splitter]: {
    key: "splitter",
    label: "Divide",
    hint: "Ao morrer, vira dois menores. Golpes em área ajudam.",
    icon: svg(`<circle cx="5.2" cy="9.5" r="3"/><circle cx="11" cy="6.5" r="3"/><path d="M8 1.5v2.5M8 12v2.5"/>`),
  },
  [ARCH.exploder]: {
    key: "exploder",
    label: "Explosivo",
    hint: "Explode quando chega perto de você ou quando morre, depois de 1 s de aviso no chão. Saia do círculo.",
    icon: svg(`<path d="M8 1.5 9.3 5.6 13.6 4 11 7.6 14.5 10 10.2 10.2 10.6 14.5 8 11.3 5.4 14.5 5.8 10.2 1.5 10 5 7.6 2.4 4 6.7 5.6z"/>`),
  },
};

const NEUTRAL: RoleInfo = {
  key: "neutral",
  label: "Neutro",
  hint: "Só ataca quem bate nele.",
  icon: svg(`<circle cx="8" cy="8" r="5.5"/><path d="M5 8h6"/>`),
};

/** O papel para o quadro do alvo (null = bruto e não neutro: sem selo). */
export function roleInfo(f: RoleFields): RoleInfo | null {
  const r = f.role ? ROLES[f.role] : undefined;
  if (r) return r;
  return f.neutral || f.neutralByDay ? NEUTRAL : null;
}

/** O monstro está neutro agora (só ataca quem bateu nele). night = a hora do mundo. */
export function neutralNow(f: RoleFields, night: boolean): boolean {
  return !!f.neutral || (!!f.neutralByDay && !night);
}

/** A linha de neutralidade da dica ("" = não é neutro). */
export function neutralText(f: RoleFields, night: boolean): string {
  if (f.neutral) return "Neutro: só ataca quem bate nele.";
  if (f.neutralByDay) return night ? "Neutro de dia, agressivo à noite (agora: agressivo)." : "Neutro de dia, agressivo à noite (agora: neutro).";
  return "";
}
