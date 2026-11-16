/**
 * Regras da auto-caça para escolher e largar alvos (tunnel#129), sem navegador: os testes
 * rodam no Node (client/tests/autohunt.test.ts). Este arquivo não importa nada em tempo de
 * execução.
 *
 *   - Não mira o que não vê: monstro invisível (o emboscador escondido nem chega ao cliente,
 *     mas um efeito de invisibilidade também conta) e o que acabou de travar a caça.
 *   - Pula os neutros (só atacam quem bate neles), a não ser que o jogador marque "Atacar
 *     monstros neutros", que o neutro já esteja brigando com ele ou que o "Caçar aqui" da
 *     missão peça aquele monstro pelo nome.
 *   - Prefere quem já está brigando com você (o emboscador que apareceu, a matilha que veio).
 *   - Não fica parada: se o alvo não perde vida e você não chega mais perto dele por
 *     STALL_MS (o atirador recuou para onde não dá para ir, o alvo sumiu de vista), larga e
 *     não volta a ele por SKIP_MS.
 */

/** O que a auto-caça sabe de um monstro vivo na tela. */
export interface HuntCandidate {
  id: number;
  name: string;
  /** Distância até você (m). */
  dist: number;
  hp: number;
  /** Neutro agora (só ataca quem bateu nele). */
  neutral: boolean;
  /** Invisível: não dá para mirar. */
  hidden: boolean;
  /** Bateu em você ou você bateu nele há pouco. */
  fightingMe: boolean;
}

export interface HuntRules {
  /** "Caçar aqui": os nomes que valem (null ou vazio = qualquer). */
  names: readonly string[] | null;
  /** Configuração "Atacar monstros neutros". */
  neutrals: boolean;
  /** Alvos que travaram a caça há pouco. */
  skip: ReadonlySet<number>;
}

/** Sem progresso (vida do alvo e distância) por este tempo, a auto-caça larga o alvo. */
export const STALL_MS = 8000;
/** E não volta a ele por este tempo. */
export const SKIP_MS = 30000;
/** Bateu em você (ou você nele) há menos que isto: está brigando com você. */
export const FIGHT_MS = 8000;

/** O monstro vale como alvo da auto-caça. */
export function huntable(c: HuntCandidate, r: HuntRules): boolean {
  const named = !!r.names && r.names.length > 0;
  if (c.hidden || r.skip.has(c.id) || (named && !r.names!.includes(c.name))) return false; // o huntAllows do questgo
  return !c.neutral || r.neutrals || c.fightingMe || named;
}

/** O alvo novo dentro do raio: quem briga com você primeiro, depois o mais perto. */
export function pickTarget(list: Iterable<HuntCandidate>, r: HuntRules, radius: number): HuntCandidate | null {
  let best: HuntCandidate | null = null;
  for (const c of list) {
    if (c.dist > radius || !huntable(c, r)) continue;
    if (!best || (c.fightingMe && !best.fightingMe) || (c.fightingMe === best.fightingMe && c.dist < best.dist)) best = c;
  }
  return best;
}

/** Vigia o alvo atual: travou = não perde vida e você não chega mais perto. */
export class StallWatch {
  private id = 0;
  private hp = 0;
  private near = Infinity;
  private since = 0;
  private skipped = new Map<number, number>(); // id → até quando pular

  /** Chamado a cada tick com o alvo atual. true = travou: larga e pula por SKIP_MS. */
  check(id: number, hp: number, dist: number, now: number): boolean {
    if (id !== this.id) {
      this.id = id;
      this.hp = hp;
      this.near = dist;
      this.since = now;
      return false;
    }
    const progress = hp < this.hp || dist < this.near - 0.5;
    this.hp = hp;
    this.near = Math.min(this.near, dist);
    if (progress) {
      this.since = now;
      return false;
    }
    if (now - this.since < STALL_MS) return false;
    this.skipped.set(id, now + SKIP_MS);
    this.id = 0;
    return true;
  }

  /** Os alvos que ainda estão sendo pulados. */
  skips(now: number): ReadonlySet<number> {
    for (const [id, until] of this.skipped) if (until <= now) this.skipped.delete(id);
    return new Set(this.skipped.keys());
  }

  reset(): void {
    this.id = 0;
    this.skipped.clear();
  }
}
