/**
 * Invariantes do cliente: o jogo percebe quando mostra algo impossível (vida acima do
 * máximo, monstro morto andando, câmera abaixo do chão…) e relata com
 * reportClientError("client_invariant", regra, contexto), que vai para client_errors.
 *
 * - Barato: nada roda por quadro. Uma checagem a cada 1 s (setInterval, fora do render)
 *   para câmera, FPS e entidades; o HUD é conferido quando o dado chega (SelfStats).
 * - Cada regra é relatada uma vez por sessão (página); a lista das quebradas vai no /bug.
 * - O que errors.ts e os carregadores já relatam não entra de novo: os .glb de
 *   personagens, armas e animações (models.ts) e o cenário (main.ts) saem como "model".
 *   Aqui ficam as texturas que falham (qualquer uma, inclusive as de dentro dos .glb) e os
 *   .glb que caíam em silêncio (o saco do saque, loot.ts).
 * - Em desenvolvimento, window.__erevia.invariants.provoke(regra) provoca cada uma
 *   (lista em window.__erevia.invariants.rules).
 */
import { LoadAssetContainerAsync, Texture, type AbstractEngine, type ArcRotateCamera, type Scene } from "@babylonjs/core";
import { create } from "@bufbuild/protobuf";
import { EntityKind, EntityStateSchema, SelfStatsSchema, type EntityState, type SelfStats } from "../gen/erevia/v1/game_pb";
import { settings } from "../ui/kit";
import { reportClientError } from "./errors";

export const INVARIANT_KIND = "client_invariant";

/** As regras (a mensagem do relatório). */
export const RULES = {
  /** HUD: vida acima do máximo, negativa ou não numérica. */
  hudHp: "hud_hp",
  /** HUD: mana acima do máximo, negativa ou não numérica. */
  hudMp: "hud_mp",
  /** HUD: XP fora da faixa do nível (antes do começo dele ou depois do próximo). */
  hudXp: "hud_xp",
  /** Spawn de um id que já está na tela (sem o despawn antes). */
  duplicateId: "entity_duplicate_id",
  /** Entidade sem nenhuma malha viva em duas checagens seguidas. */
  noMesh: "entity_no_mesh",
  /** Posição NaN ou Infinity. */
  badPosition: "entity_bad_position",
  /** Monstro morto que andou mais de 1 m entre duas checagens. */
  deadMoving: "monster_dead_moving",
  /** Entidade a mais de 6 m de onde o servidor a colocou, por mais de 2 s. */
  serverDrift: "entity_server_drift",
  /** Câmera com alvo, posição ou ângulos NaN/Infinity. */
  cameraBadTarget: "camera_bad_target",
  /** Alvo ou posição da câmera abaixo do chão. */
  cameraBelowGround: "camera_below_ground",
  /** FPS abaixo de 15 por mais de 10 s (com a região, as entidades e os draw calls). */
  lowFps: "low_fps",
  /** Textura que não carregou (arquivo que faltou ou não decodificou). */
  textureFailed: "texture_failed",
  /** .glb que não carregou (os que models.ts e o cenário não cobrem). */
  glbFailed: "glb_failed",
} as const;
export type Rule = (typeof RULES)[keyof typeof RULES];

const CHECK_MS = 1000;
const LOW_FPS = 15;
const LOW_FPS_MS = 10_000;
const DRIFT_M = 6;
const DRIFT_MS = 2000;
const DEAD_MOVE_M = 1;
/** Abaixo disto (m) é "embaixo do chão" (o chão do mapa é o plano y = 0). */
const GROUND_Y = -0.5;
/** Amostras de FPS guardadas (1 por segundo): o último minuto, para o /bug. */
const FPS_WINDOW = 60;

/** O mínimo de uma entidade que as checagens leem (a View de world/entities.ts serve). */
export interface EntityProbe {
  readonly id: number;
  readonly kind: EntityKind;
  readonly dead: boolean;
  readonly root: { readonly position: { readonly x: number; readonly y: number; readonly z: number }; isDisposed(): boolean };
  readonly meshes: readonly { isDisposed(): boolean }[];
}

export interface InvariantDeps {
  engine: AbstractEngine;
  scene: Scene;
  camera: ArcRotateCamera;
  localId: number;
  forEachEntity(cb: (e: EntityProbe) => void): void;
  /** Nome da região em (x, z). */
  regionAt(x: number, z: number): string;
  /** Onde o jogador local está (ou o foco da câmera). */
  focus(): { x: number; z: number } | undefined;
}

// ---------- Relatório (uma vez por regra na sessão) ----------

const reported = new Set<string>();

/** Relata a regra quebrada, uma vez por sessão. Devolve true se relatou agora. */
export function invariantBroken(rule: Rule, extra: Record<string, unknown>): boolean {
  if (reported.has(rule)) return false;
  reported.add(rule);
  reportClientError(INVARIANT_KIND, rule, extra);
  return true;
}

/** As regras quebradas nesta sessão (para o /bug). */
export function brokenInvariants(): string[] {
  return [...reported];
}

/** Um recurso que não carregou e que nenhum outro relatório cobre (ex.: o saco do saque). */
export function resourceFailed(kind: "glb" | "texture", url: string, err?: unknown): void {
  const error = err === undefined ? undefined : String((err as Error)?.message ?? err).slice(0, 300);
  invariantBroken(kind === "glb" ? RULES.glbFailed : RULES.textureFailed, { url: shortUrl(url), error });
}

let texturesWatched = false;
/** Toda textura do Babylon que falha (inclusive as embutidas nos .glb) avisa aqui. */
function watchTextures(): void {
  if (texturesWatched) return;
  texturesWatched = true;
  Texture.OnTextureLoadErrorObservable.add((t) => {
    // Só olha a textura quando ela falha: nada roda nas que carregam.
    const err = (t as unknown as { _errorObject?: { message?: string } })._errorObject;
    const url = t instanceof Texture ? t.url : null;
    resourceFailed("texture", url || t.name, err?.message);
  });
}

function shortUrl(url: string): string {
  // data: e blob: (texturas embutidas) não interessam inteiros.
  if (url.startsWith("data:")) return url.slice(0, 40) + "…";
  return url.slice(0, 200);
}

const finite = (...v: number[]) => v.every(Number.isFinite);
const r1 = (v: number) => (Number.isFinite(v) ? Math.round(v * 10) / 10 : String(v));

// ---------- As checagens ----------

export class ClientInvariants {
  private timer: number;
  private fps: number[] = [];
  private lowSince = 0;
  /** FPS baixo esperando o próximo quadro para contar os draw calls. */
  private lowFpsPending = false;
  /** Último estado de cada entidade vindo do servidor e quando chegou. */
  private serverState = new Map<number, EntityState>();
  private serverAt = new Map<number, number>();
  private driftSince = new Map<number, number>();
  private deadAt = new Map<number, { x: number; z: number }>();
  private meshless = new Set<number>();
  /** Desenvolvimento: entidades de mentira e FPS forçado (provoke). */
  private fakes: (EntityProbe & { step?(): void; until: number })[] = [];
  private fpsOverride: { value: number; until: number } | null = null;

  constructor(private d: InvariantDeps) {
    watchTextures();
    this.timer = window.setInterval(() => this.check(), CHECK_MS);
  }

  dispose(): void {
    window.clearInterval(this.timer);
  }

  /** Snapshot: guarda a última posição do servidor de cada entidade (sem copiar nada). */
  noteStates(states: readonly EntityState[]): void {
    if (!states.length) return;
    const now = performance.now();
    for (const s of states) {
      this.serverState.set(s.id, s);
      this.serverAt.set(s.id, now);
    }
  }

  /** Spawn chegando: `known` = o id já está na tela. */
  noteSpawn(id: number, known: boolean, kind?: EntityKind): void {
    if (known) invariantBroken(RULES.duplicateId, { id, kind, local: id === this.d.localId });
    this.forget(id); // posição nova: o último estado era da vida anterior
  }

  noteDespawn(id: number): void {
    this.forget(id);
  }

  /** HUD: confere o que chegou para as barras de vida, mana e XP. */
  stats(s: SelfStats): void {
    const base = { level: s.level, dead: s.dead };
    if (!finite(s.hp, s.maxHp) || s.hp < 0 || s.hp > s.maxHp) invariantBroken(RULES.hudHp, { ...base, hp: s.hp, maxHp: s.maxHp });
    if (!finite(s.mp, s.maxMp) || s.mp < 0 || s.mp > s.maxMp) invariantBroken(RULES.hudMp, { ...base, mp: s.mp, maxMp: s.maxMp });
    const next = s.xpNextLevel;
    if (s.xp < s.xpLevelStart || (next > 0n && (s.xp >= next || next <= s.xpLevelStart))) {
      invariantBroken(RULES.hudXp, { ...base, xp: String(s.xp), levelStart: String(s.xpLevelStart), next: String(next) });
    }
  }

  /** FPS do último minuto (média e mínimo das amostras de 1 s), para o /bug. */
  fpsSummary(): { avg: number; min: number; samples: number } {
    const n = this.fps.length;
    if (!n) return { avg: 0, min: 0, samples: 0 };
    let sum = 0;
    let min = Infinity;
    for (const f of this.fps) {
      sum += f;
      if (f < min) min = f;
    }
    return { avg: Math.round((sum / n) * 10) / 10, min: Math.round(min * 10) / 10, samples: n };
  }

  private forget(id: number): void {
    this.serverState.delete(id);
    this.serverAt.delete(id);
    this.driftSince.delete(id);
    this.deadAt.delete(id);
    this.meshless.delete(id);
  }

  private check(): void {
    // Aba escondida: o navegador quase não desenha, nada aqui faz sentido.
    if (document.hidden) {
      this.lowSince = 0;
      return;
    }
    const now = performance.now();
    try {
      this.checkFps(now);
      this.checkCamera();
      this.checkEntities(now);
    } catch (err) {
      // A checagem nunca pode derrubar o jogo; o erro em si vale um relatório.
      reportClientError("error", `invariantes: ${(err as Error)?.message ?? err}`, undefined, err);
    }
  }

  private checkFps(now: number): void {
    let f = this.d.engine.getFps();
    if (this.fpsOverride) {
      if (now < this.fpsOverride.until) f = this.fpsOverride.value;
      else this.fpsOverride = null;
    }
    if (!Number.isFinite(f) || f <= 0) return;
    this.fps.push(f);
    if (this.fps.length > FPS_WINDOW) this.fps.shift();
    if (f >= LOW_FPS) {
      this.lowSince = 0;
      return;
    }
    if (!this.lowSince) this.lowSince = now;
    else if (now - this.lowSince >= LOW_FPS_MS && !reported.has(RULES.lowFps) && !this.lowFpsPending) this.reportLowFps(now);
  }

  /** FPS baixo: conta os draw calls de um quadro (só agora, sem instrumentação contínua) e relata. */
  private reportLowFps(now: number): void {
    const { scene, engine } = this.d;
    const seconds = Math.round((now - this.lowSince) / 1000);
    const fps = this.fpsSummary();
    let players = 0;
    let monsters = 0;
    let total = 0;
    this.d.forEachEntity((e) => {
      total++;
      if (e.kind === EntityKind.PLAYER) players++;
      else if (e.kind === EntityKind.MONSTER) monsters++;
    });
    const at = this.d.focus();
    const counter = (engine as unknown as { _drawCalls?: { current: number } })._drawCalls;
    this.lowFpsPending = true; // o relatório sai no fim do próximo quadro
    let start = 0;
    scene.onBeforeRenderObservable.addOnce(() => (start = counter?.current ?? 0));
    scene.onAfterRenderObservable.addOnce(() => {
      const cur = counter?.current ?? 0;
      // A instrumentação (F3, ?bench) zera o contador no começo do quadro.
      const draws = cur >= start ? cur - start : cur;
      this.lowFpsPending = false;
      invariantBroken(RULES.lowFps, {
        fps: Math.round(engine.getFps() * 10) / 10,
        seconds,
        lastMinute: fps,
        region: at ? this.d.regionAt(at.x, at.z) : undefined,
        x: at && r1(at.x),
        z: at && r1(at.z),
        entities: total,
        players,
        monsters,
        draws: counter ? draws : undefined,
        activeMeshes: scene.getActiveMeshes().length,
        meshes: scene.meshes.length,
        quality: settings.get().quality,
        scale: engine.getHardwareScalingLevel(),
        size: `${engine.getRenderWidth()}x${engine.getRenderHeight()}`,
      });
    });
  }

  private checkCamera(): void {
    const c = this.d.camera;
    const t = c.target;
    const p = c.position;
    if (!finite(t.x, t.y, t.z, p.x, p.y, p.z, c.alpha, c.beta, c.radius)) {
      invariantBroken(RULES.cameraBadTarget, {
        target: [r1(t.x), r1(t.y), r1(t.z)],
        position: [r1(p.x), r1(p.y), r1(p.z)],
        alpha: r1(c.alpha),
        beta: r1(c.beta),
        radius: r1(c.radius),
      });
      return;
    }
    if (t.y < GROUND_Y || p.y < GROUND_Y) {
      invariantBroken(RULES.cameraBelowGround, { target: [r1(t.x), r1(t.y), r1(t.z)], position: [r1(p.x), r1(p.y), r1(p.z)], beta: r1(c.beta) });
    }
  }

  private checkEntities(now: number): void {
    const visit = (e: EntityProbe) => this.checkEntity(e, now);
    this.d.forEachEntity(visit);
    if (this.fakes.length) {
      this.fakes = this.fakes.filter((f) => f.until > now);
      for (const f of this.fakes) {
        f.step?.();
        visit(f);
      }
    }
  }

  private checkEntity(e: EntityProbe, now: number): void {
    const { id, kind } = e;
    const pos = e.root.position;
    if (!finite(pos.x, pos.y, pos.z)) {
      invariantBroken(RULES.badPosition, { id, kind, x: String(pos.x), y: String(pos.y), z: String(pos.z) });
      return;
    }
    // Sem malha viva: o modelo sumiu e a cápsula também. Duas checagens seguidas (a troca
    // de modelo descarta o velho antes de pôr o novo).
    const alive = !e.root.isDisposed() && e.meshes.some((m) => !m.isDisposed());
    if (!alive) {
      if (this.meshless.has(id)) invariantBroken(RULES.noMesh, { id, kind, meshes: e.meshes.length, rootDisposed: e.root.isDisposed() });
      else this.meshless.add(id);
    } else if (this.meshless.size) this.meshless.delete(id);

    if (kind === EntityKind.MONSTER && e.dead) {
      const last = this.deadAt.get(id);
      if (last) {
        const moved = Math.hypot(pos.x - last.x, pos.z - last.z);
        if (moved > DEAD_MOVE_M) invariantBroken(RULES.deadMoving, { id, moved: r1(moved), from: [r1(last.x), r1(last.z)], to: [r1(pos.x), r1(pos.z)] });
        last.x = pos.x;
        last.z = pos.z;
      } else this.deadAt.set(id, { x: pos.x, z: pos.z });
    } else if (this.deadAt.size) this.deadAt.delete(id);

    // Longe do servidor: compara com onde o servidor diz que ela está agora (o último
    // estado, andando em linha reta até o destino na velocidade dele, como no Go).
    const s = this.serverState.get(id);
    if (!s || e.dead) return;
    const elapsed = (now - (this.serverAt.get(id) ?? now)) / 1000;
    const dx = s.targetX - s.x;
    const dz = s.targetZ - s.z;
    const len = Math.hypot(dx, dz);
    const step = len > 0 ? Math.min(len, Math.max(0, s.speed) * elapsed) / len : 0;
    const ex = s.x + dx * step;
    const ez = s.z + dz * step;
    const gap = Math.hypot(pos.x - ex, pos.z - ez);
    if (gap <= DRIFT_M) {
      if (this.driftSince.size) this.driftSince.delete(id);
      return;
    }
    const since = this.driftSince.get(id);
    if (since === undefined) this.driftSince.set(id, now);
    else if (now - since >= DRIFT_MS) {
      invariantBroken(RULES.serverDrift, {
        id,
        kind,
        local: id === this.d.localId,
        gap: r1(gap),
        seconds: r1((now - since) / 1000),
        drawn: [r1(pos.x), r1(pos.z)],
        server: [r1(ex), r1(ez)],
        snapshotAgeS: r1(elapsed),
      });
    }
  }

  // ---------- Desenvolvimento: provocar cada regra ----------

  /**
   * Ganchos de window.__erevia.invariants (só em import.meta.env.DEV). Cada provoke passa
   * pelo mesmo caminho da checagem de verdade: dados de mentira na entrada (SelfStats,
   * spawn, entidade, FPS) ou uma falha real (textura e .glb que não existem; câmera com
   * NaN só durante a checagem, sem chegar a desenhar).
   */
  devHooks() {
    const rules = Object.values(RULES);
    return {
      rules,
      /** As regras já relatadas nesta sessão. */
      broken: () => brokenInvariants(),
      provoke: (rule: Rule): string => this.provoke(rule),
      /** Provoca todas (cada uma uma vez; FPS baixo leva ~11 s e o drift ~3 s). */
      provokeAll: () => rules.map((r) => this.provoke(r)),
      /** Custo de uma checagem completa (ms), na média de n seguidas, com as entidades de agora. */
      checkCostMs: (n = 1000): { ms: number; entities: number } => {
        const keep = [...this.fps];
        let entities = 0;
        this.d.forEachEntity(() => entities++);
        const t0 = performance.now();
        for (let i = 0; i < n; i++) this.check();
        const ms = (performance.now() - t0) / n;
        this.fps = keep; // as amostras repetidas não contam no FPS do /bug
        return { ms, entities };
      },
    };
  }

  private provoke(rule: Rule): string {
    const now = performance.now();
    const fakeId = 900_000 + Math.floor(Math.random() * 99_999);
    const fake = (over: Partial<EntityProbe> & { step?(): void }) =>
      this.fakes.push({
        id: fakeId,
        kind: EntityKind.MONSTER,
        dead: false,
        root: { position: { x: 0, y: 0, z: 0 }, isDisposed: () => false },
        meshes: [{ isDisposed: () => false }],
        until: now + 15_000,
        ...over,
      });
    switch (rule) {
      case RULES.hudHp:
        this.stats(create(SelfStatsSchema, { hp: 150, maxHp: 100, mp: 10, maxMp: 50, level: 2, xp: 120n, xpLevelStart: 100n, xpNextLevel: 300n }));
        return "SelfStats com vida 150/100";
      case RULES.hudMp:
        this.stats(create(SelfStatsSchema, { hp: 50, maxHp: 100, mp: 80, maxMp: 50, level: 2, xp: 120n, xpLevelStart: 100n, xpNextLevel: 300n }));
        return "SelfStats com mana 80/50";
      case RULES.hudXp:
        this.stats(create(SelfStatsSchema, { hp: 50, maxHp: 100, mp: 10, maxMp: 50, level: 3, xp: 10n, xpLevelStart: 100n, xpNextLevel: 300n }));
        return "SelfStats com XP 10 no nível que começa em 100";
      case RULES.duplicateId:
        this.noteSpawn(this.d.localId, true, EntityKind.PLAYER);
        return `spawn do id ${this.d.localId}, que já está na tela`;
      case RULES.noMesh:
        fake({ meshes: [] });
        return `entidade ${fakeId} sem malha (relata na 2ª checagem)`;
      case RULES.badPosition:
        fake({ root: { position: { x: NaN, y: 0, z: Infinity }, isDisposed: () => false } });
        return `entidade ${fakeId} em (NaN, Infinity)`;
      case RULES.deadMoving: {
        const position = { x: 0, y: 0, z: 0 };
        fake({ dead: true, root: { position, isDisposed: () => false }, step: () => void (position.x += 3) });
        return `monstro morto ${fakeId} andando 3 m por checagem`;
      }
      case RULES.serverDrift:
        this.serverState.set(fakeId, create(EntityStateSchema, { id: fakeId, x: 0, z: 0, targetX: 0, targetZ: 0, speed: 0 }));
        this.serverAt.set(fakeId, now);
        fake({ root: { position: { x: 40, y: 0, z: 30 }, isDisposed: () => false } });
        return `entidade ${fakeId} a 50 m do servidor (relata depois de 2 s)`;
      case RULES.cameraBadTarget:
      case RULES.cameraBelowGround: {
        const t = this.d.camera.target;
        const saved = rule === RULES.cameraBadTarget ? t.x : t.y;
        if (rule === RULES.cameraBadTarget) t.x = NaN;
        else t.y = -5;
        try {
          this.checkCamera();
        } finally {
          if (rule === RULES.cameraBadTarget) t.x = saved;
          else t.y = saved;
        }
        return rule === RULES.cameraBadTarget ? "alvo da câmera com x = NaN" : "alvo da câmera em y = -5";
      }
      case RULES.lowFps:
        this.fpsOverride = { value: 5, until: now + LOW_FPS_MS + 3000 };
        return "FPS lido como 5 por 13 s (relata depois de 10 s)";
      case RULES.textureFailed: {
        const tex = new Texture(`/textures/nao-existe-${Date.now()}.png`, this.d.scene);
        tex.onLoadObservable.addOnce(() => tex.dispose());
        window.setTimeout(() => tex.dispose(), 10_000);
        return "textura que não existe";
      }
      case RULES.glbFailed: {
        const url = `/models/nao-existe-${Date.now()}.glb`;
        LoadAssetContainerAsync(url, this.d.scene).then(
          (c) => c.dispose(),
          (err: unknown) => resourceFailed("glb", url, err),
        );
        return ".glb que não existe";
      }
    }
  }
}
