/**
 * Regiões do mapa a partir das zonas de caça do servidor: anéis ao redor da cidade e
 * áreas próprias (biomas, como o Vale das Pedras Vivas no sul). O servidor manda o nome
 * dos monstros (e o da região, nas áreas); aqui cada anel ganha um nome de região, uma
 * cor pela dificuldade e um texto curto.
 */

import type { MapZone } from "../gen/erevia/v1/game_pb";

export interface Region {
  name: string;
  /** Monstros da região ("Esqueleto Lacaio, Esqueleto Ladino"). */
  monsters: string;
  rMin: number;
  rMax: number;
  levelMin: number;
  levelMax: number;
  safe: boolean;
  /** "r,g,b" */
  rgb: string;
  /** Área própria (disco), em vez de anel em volta da cidade. */
  area?: { x: number; z: number; r: number };
}

export interface BossPoint {
  name: string;
  level: number;
  x: number;
  z: number;
  /** Raid boss (covil próprio, renasce em horas). */
  raid?: boolean;
}

/** Nome e raio do covil do raid boss (server/internal/game: RaidLairRadius). */
export const RAID_LAIR = { name: "Cripta Esquecida", r: 24 };

const RING_NAMES = ["Arredores", "Campos dos Ossos", "Ermos Partidos", "Terras Mortas", "Confins"];

function tierRgb(levelMax: number): string {
  if (levelMax <= 7) return "120,200,110";
  if (levelMax <= 14) return "230,180,80";
  if (levelMax <= 22) return "230,90,70";
  return "200,90,220";
}

export function buildRegions(zones: MapZone[], safeRadius: number): { regions: Region[]; bosses: BossPoint[] } {
  const rings = zones.filter((z) => !z.fixed && !z.area).sort((a, b) => a.rMin - b.rMin);
  const regions: Region[] = [
    { name: "Cidade", monsters: "", rMin: 0, rMax: safeRadius, levelMin: 0, levelMax: 0, safe: true, rgb: "120,190,255" },
    ...rings.map((z, i) => ({
      name: RING_NAMES[i] ?? `Região ${i + 1}`,
      monsters: z.name,
      rMin: z.rMin,
      rMax: z.rMax,
      levelMin: z.levelMin,
      levelMax: z.levelMax,
      safe: false,
      rgb: tierRgb(z.levelMax),
    })),
    ...zones
      .filter((z) => z.area)
      .map((z) => ({
        name: z.region || z.name,
        monsters: z.name,
        rMin: 0,
        rMax: 0,
        levelMin: z.levelMin,
        levelMax: z.levelMax,
        safe: false,
        rgb: tierRgb(z.levelMax),
        area: { x: z.x, z: z.z, r: z.radius },
      })),
    // Covil do raid boss: uma região própria (aparece no minimapa ao entrar).
    ...zones
      .filter((z) => z.fixed && z.raid)
      .map((z) => ({
        name: RAID_LAIR.name,
        monsters: z.name,
        rMin: 0,
        rMax: 0,
        levelMin: z.levelMin,
        levelMax: z.levelMax,
        safe: false,
        rgb: "200,90,220",
        area: { x: z.x, z: z.z, r: RAID_LAIR.r },
      })),
  ];
  const bosses = zones.filter((z) => z.fixed).map((z) => ({ name: z.name, level: z.levelMin, x: z.x, z: z.z, raid: z.raid }));
  return { regions, bosses };
}

/** Folga em volta da área de caça que ainda conta como a região (o bioma é maior que o disco). */
const AREA_FRINGE = 1.6;

/**
 * Região onde está um ponto. As áreas (biomas) vêm primeiro; depois os anéis pela
 * distância da cidade. Fora de tudo: a área mais próxima, se ela estiver mais perto do
 * ponto do que a cidade (ex.: a transição floresta → vale); senão o último anel.
 */
export function regionAt(regions: Region[], x: number, z: number): Region {
  const areas = regions.filter((g) => g.area);
  const dist = (g: Region) => Math.hypot(x - g.area!.x, z - g.area!.z);
  const inArea = areas.find((g) => dist(g) < g.area!.r * AREA_FRINGE);
  if (inArea) return inArea;
  const rings = regions.filter((g) => !g.area);
  const r = Math.hypot(x, z);
  const ring = rings.find((g) => r >= g.rMin && r < g.rMax);
  if (ring) return ring;
  const near = areas.sort((a, b) => dist(a) - dist(b))[0];
  return near && dist(near) < r ? near : rings[rings.length - 1];
}

export function levelText(r: Region): string {
  return r.safe ? "Zona segura" : `Nv ${r.levelMin}–${r.levelMax}`;
}
