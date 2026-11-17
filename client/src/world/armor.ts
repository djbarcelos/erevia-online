/**
 * Armadura visível por raridade (tunnel#96, remaster). O servidor manda a raridade (Grade 1 a 5; 0 = vazio) de
 * cada peça equipada (`ArmorLook`); aqui ela vira o visual do herói:
 *
 * | Raridade (peito) | Corpo (textura da classe)                  | Ombreiras        | Capa especial      | Brilho  |
 * |------------------|--------------------------------------------|------------------|--------------------|---------|
 * | 1 comum          | a de sempre                                |                  |                    |         |
 * | 2 bom            | a de sempre, aço mais metálico e claro     |                  |                    |         |
 * | 3 raro           | retexture A (azul e prata)                 |                  |                    |         |
 * | 4 épico          | retexture B (negro, ouro e vermelho)       | ombreiras épicas |                    |         |
 * | 5 lendário       | retexture C (ouro radiante, runas ciano)   | ombreiras lendárias | capa lendária   | pulsa   |
 *
 * Elmo (peça `Hat`): raro e épico trocam o modelo por uma variante da classe; a capa equipada (raro, épico ou
 * lendário) é uma peça anexada nas costas. As peças de pernas, luvas e botas não têm malha nem região própria
 * (o corpo é uma textura só): contam só quando o peito está vazio. Tudo isso só existe no remaster (o padrão).
 */

export interface ArmorGrades {
  chest: number;
  helm: number;
  cape: number;
  legs: number;
  hands: number;
  feet: number;
}

export const NO_ARMOR: ArmorGrades = { chest: 0, helm: 0, cape: 0, legs: 0, hands: 0, feet: 0 };

export function toArmorGrades(a?: Partial<ArmorGrades> | null): ArmorGrades {
  const n = (v?: number) => Math.max(0, Math.min(5, Math.floor(v ?? 0)));
  return { chest: n(a?.chest), helm: n(a?.helm), cape: n(a?.cape), legs: n(a?.legs), hands: n(a?.hands), feet: n(a?.feet) };
}

/** Faixa de textura do corpo: 0 comum, 1 bom (mesma textura, mais metal), 2 raro, 3 épico, 4 lendário. */
export type BodyTier = 0 | 1 | 2 | 3 | 4;
export const TIER_FILES = ["", "", "rare", "epic", "legend"] as const;

/** Raridade que vale para o corpo: o peito; sem peito, a melhor das outras peças (no máximo rara). */
export function bodyGrade(a: ArmorGrades): number {
  if (a.chest > 0) return a.chest;
  return Math.min(3, Math.max(a.legs, a.hands, a.feet));
}

export function bodyTier(a: ArmorGrades): BodyTier {
  return Math.max(0, Math.min(4, bodyGrade(a) - 1)) as BodyTier;
}

/** Ombreiras: peito épico ou lendário. */
export function pauldronFile(a: ArmorGrades): string | null {
  const g = bodyGrade(a);
  return g >= 5 ? "pauldron_legend" : g === 4 ? "pauldron_epic" : null;
}

/** Capa: a capa equipada (raro, épico, lendário); o peito lendário traz a capa lendária se a capa for pior. */
export function capeFile(a: ArmorGrades): string | null {
  const g = Math.max(a.cape, bodyGrade(a) >= 5 ? 5 : 0);
  return g >= 5 ? "cape_legend" : g === 4 ? "cape_epic" : g === 3 ? "cape_rare" : null;
}

/** Variante de elmo por classe e raridade do elmo (null = o `Hat` de sempre). */
export function helmFile(model: string, a: ArmorGrades): string | null {
  const g = a.helm;
  if (model === "knight") return g >= 4 ? "helm_knight_epic" : g === 3 ? "helm_knight_rare" : null;
  if (model === "mage") return g >= 4 ? "hat_mage_epic" : null;
  if (model === "rogue") return g >= 4 ? "hood_rogue_epic" : null;
  if (model === "barbarian") return g >= 4 ? "helm_barbarian_epic" : null;
  return null;
}

/** Chave que muda quando algo do visual da armadura muda (para não refazer o que já está certo). */
export function armorKey(model: string, a: ArmorGrades): string {
  return [bodyTier(a), pauldronFile(a), capeFile(a), helmFile(model, a)].join("|");
}
