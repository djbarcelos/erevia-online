/**
 * Regiões do mapa a partir das zonas de caça do servidor (anéis ao redor da cidade).
 * O servidor manda o nome dos monstros; aqui cada anel ganha um nome de região, uma cor
 * pela dificuldade e um texto curto.
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
}

export interface BossPoint {
  name: string;
  level: number;
  x: number;
  z: number;
}

const RING_NAMES = ["Arredores", "Campos dos Ossos", "Ermos Partidos", "Terras Mortas", "Confins"];

function tierRgb(levelMax: number): string {
  if (levelMax <= 7) return "120,200,110";
  if (levelMax <= 14) return "230,180,80";
  return "230,90,70";
}

export function buildRegions(zones: MapZone[], safeRadius: number): { regions: Region[]; bosses: BossPoint[] } {
  const rings = zones.filter((z) => !z.fixed).sort((a, b) => a.rMin - b.rMin);
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
  ];
  const bosses = zones.filter((z) => z.fixed).map((z) => ({ name: z.name, level: z.levelMin, x: z.x, z: z.z }));
  return { regions, bosses };
}

/** Região onde está um ponto (fora de todas: a última). */
export function regionAt(regions: Region[], x: number, z: number): Region {
  const r = Math.hypot(x, z);
  return regions.find((g) => r >= g.rMin && r < g.rMax) ?? regions[regions.length - 1];
}

export function levelText(r: Region): string {
  return r.safe ? "Zona segura" : `Nv ${r.levelMin}–${r.levelMax}`;
}
