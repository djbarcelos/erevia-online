import { Color3, Constants, Effect, MeshBuilder, ShaderMaterial, Vector4, type Mesh, type Scene, type Vector3 } from "@babylonjs/core";
import { TelegraphShape } from "../../gen/erevia/v1/game_pb";

/**
 * Avisos no chão das habilidades de monstro (tunnel#130): círculo, cone, linha e anel,
 * com o preenchimento correndo até o golpe cair (como o raid sempre fez).
 *
 * Custo fixo: um pool de MAX_DRAWN quadrados planos, cada um com o seu ShaderMaterial,
 * criados (e compilados) na carga. A forma sai do shader, então nenhum aviso cria malha,
 * material ou textura. Só os MAX_DRAWN avisos mais perto de você são desenhados; os outros
 * existem só no servidor (o golpe cai do mesmo jeito). Por quadro: uns uniforms por aviso
 * desenhado; a escolha dos mais próximos roda a cada PICK_MS ou quando chega um aviso.
 */

/** Avisos desenhados ao mesmo tempo (os mais próximos de você). */
export const MAX_DRAWN = 6;
/** De quanto em quanto tempo os mais próximos são escolhidos de novo (ms). */
const PICK_MS = 150;
/** O aviso some sozinho este tempo depois do fim, se o golpe não chegar (saiu da área). */
const LINGER_MS = 250;
/** Altura acima do chão (acima da praça e das estradas). */
const Y = 0.07;
/** Folga do quadrado em volta da forma, para a borda brilhante não ser cortada (m). */
const PAD = 0.3;

export interface WarningShape {
  shape: TelegraphShape;
  radius: number;
  /** Anel: raio do buraco seguro. */
  inner: number;
  /** Cone: abertura total (rad). */
  arc: number;
  length: number;
  width: number;
  /** Direção do cone e da linha (rad em Y: 0 = +Z, π/2 = +X; o rotation.y do Babylon). */
  angle: number;
}

export interface GroundWarning extends WarningShape {
  x: number;
  z: number;
  color: Color3;
  durationMs: number;
  /** Marca: o aviso anda junto com esta posição (null = parado onde está). */
  follow?: () => Vector3 | null;
}

interface Live {
  w: GroundWarning;
  start: number;
  end: number;
  slot: Slot | null;
  drawn: boolean;
}

/** Os uniforms ficam por referência no material: cada quadrado tem os seus vetores. */
interface Slot {
  mesh: Mesh;
  mat: ShaderMaterial;
  live: Live | null;
  box: Vector4;
  params: Vector4;
  line: Vector4;
  state: Vector4;
}

/** A borda mais distante da forma, contada da origem. */
export function reachOf(s: WarningShape): number {
  return s.shape === TelegraphShape.LINE ? Math.hypot(s.length, s.width / 2) : s.radius;
}

/** O ponto (x, z) está dentro da forma com origem em (ox, oz)? Igual ao servidor (zone/abilities.go). */
export function shapeContains(s: WarningShape, ox: number, oz: number, x: number, z: number): boolean {
  const dx = x - ox;
  const dz = z - oz;
  const d = Math.hypot(dx, dz);
  const fx = Math.sin(s.angle);
  const fz = Math.cos(s.angle);
  switch (s.shape) {
    case TelegraphShape.CONE:
      return d <= s.radius && (d < 1e-9 || dx * fx + dz * fz >= Math.cos(s.arc / 2) * d);
    case TelegraphShape.LINE: {
      const along = dx * fx + dz * fz;
      return along >= 0 && along <= s.length && Math.abs(dx * fz - dz * fx) <= s.width / 2;
    }
    case TelegraphShape.DONUT:
      return d >= s.inner && d <= s.radius;
    default:
      return d <= s.radius;
  }
}

/** Centro e raio aproximados da forma (onde o golpe "cai" no visual do impacto). */
export function shapeCenter(s: WarningShape, ox: number, oz: number): { x: number; z: number; radius: number } {
  const fx = Math.sin(s.angle);
  const fz = Math.cos(s.angle);
  switch (s.shape) {
    case TelegraphShape.CONE:
      return { x: ox + fx * s.radius * 0.55, z: oz + fz * s.radius * 0.55, radius: s.radius * 0.5 };
    case TelegraphShape.LINE:
      return { x: ox + (fx * s.length) / 2, z: oz + (fz * s.length) / 2, radius: Math.min(s.length / 2, Math.max(s.width, 2)) };
    default:
      return { x: ox, z: oz, radius: s.radius };
  }
}

Effect.ShadersStore["groundWarningVertexShader"] = `
precision highp float;
attribute vec3 position;
uniform mat4 world;
uniform mat4 viewProjection;
uniform vec4 uBox;
varying vec2 vP;
void main(void) {
  // O quadrado 1×1 vira a caixa da forma, em metros no espaço do aviso (frente = +Z).
  vec2 p = mix(uBox.xy, uBox.zw, position.xz + 0.5);
  vP = p;
  gl_Position = viewProjection * world * vec4(p.x, 0.0, p.y, 1.0);
}`;

Effect.ShadersStore["groundWarningFragmentShader"] = `
precision highp float;
varying vec2 vP;
uniform float uShape;
uniform vec4 uParams; // raio, raio interno, meia abertura do cone (rad), 0
uniform vec4 uLine;   // comprimento, meia largura, 0, 0
uniform vec4 uState;  // progresso 0–1, tempo (s), opacidade, 0
uniform vec3 uColor;
void main(void) {
  float d = length(vP);
  float R = uParams.x;
  float inside;   // distância até a borda (positiva dentro)
  float fillAt;   // até onde o preenchimento chegou nesta coordenada
  float fillMax;
  if (uShape < 0.5) {
    inside = R - d;
    fillAt = d; fillMax = R;
  } else if (uShape < 1.5) {
    float h = uParams.z;
    float ang = atan(abs(vP.x), vP.y);
    float side = ang <= h ? d * sin(min(h - ang, 1.5708)) : -d * sin(min(ang - h, 1.5708));
    inside = min(R - d, side);
    fillAt = d; fillMax = R;
  } else if (uShape < 2.5) {
    inside = min(min(vP.y, uLine.x - vP.y), uLine.y - abs(vP.x));
    fillAt = vP.y; fillMax = uLine.x;
  } else {
    inside = min(R - d, d - uParams.y);
    fillAt = R - d; fillMax = R - uParams.y; // fecha de fora para dentro, até o buraco seguro
  }
  if (inside < 0.0) discard;
  float k = uState.x;
  float reach = k * fillMax;
  float filled = step(fillAt, reach);
  float front = filled * (1.0 - smoothstep(0.0, 0.3, reach - fillAt));
  float border = 1.0 - smoothstep(0.0, 0.32, inside);
  float pulse = 0.75 + 0.25 * sin(uState.y * (6.0 + 10.0 * k));
  float a = 0.18 + 0.3 * filled + 0.3 * front + 0.7 * border * pulse;
  vec3 col = uColor * (0.85 + 0.5 * border + 0.3 * front);
  gl_FragColor = vec4(col, clamp(a, 0.0, 0.92) * uState.z);
}`;

const SHAPE_ID: Record<number, number> = {
  [TelegraphShape.CIRCLE]: 0,
  [TelegraphShape.CONE]: 1,
  [TelegraphShape.LINE]: 2,
  [TelegraphShape.DONUT]: 3,
};

export class GroundWarnings {
  private slots: Slot[] = [];
  private live = new Map<number, Live>();
  private nextPick = 0;

  constructor(
    scene: Scene,
    private me: () => Vector3 | null,
  ) {
    for (let i = 0; i < MAX_DRAWN; i++) {
      const mesh = MeshBuilder.CreateGround(`fx-warning-${i}`, { width: 1, height: 1 }, scene);
      const mat = new ShaderMaterial(
        `fx-warning-${i}`,
        scene,
        { vertex: "groundWarning", fragment: "groundWarning" },
        {
          attributes: ["position"],
          uniforms: ["world", "viewProjection", "uBox", "uShape", "uParams", "uLine", "uState", "uColor"],
          needAlphaBlending: true,
        },
      );
      mat.alphaMode = Constants.ALPHA_COMBINE;
      mat.backFaceCulling = false;
      mat.disableDepthWrite = true;
      mesh.material = mat;
      mesh.isPickable = false;
      // A caixa sai do shader: a do quadrado 1×1 cortaria o aviso na borda da tela.
      mesh.alwaysSelectAsActiveMesh = true;
      mesh.setEnabled(false);
      // Compila na carga, não no primeiro golpe.
      if (i === 0) mat.forceCompilation(mesh);
      const slot: Slot = { mesh, mat, live: null, box: new Vector4(), params: new Vector4(), line: new Vector4(), state: new Vector4() };
      mat.setVector4("uBox", slot.box);
      mat.setVector4("uParams", slot.params);
      mat.setVector4("uLine", slot.line);
      mat.setVector4("uState", slot.state);
      this.slots.push(slot);
    }
  }

  /** Um aviso começou. */
  add(id: number, w: GroundWarning): void {
    const now = performance.now();
    this.live.set(id, { w, start: now, end: now + w.durationMs + LINGER_MS, slot: null, drawn: false });
    this.nextPick = 0;
  }

  /** O golpe caiu ou foi cancelado. Devolve se o aviso chegou a ser desenhado. */
  remove(id: number): boolean {
    const l = this.live.get(id);
    if (!l) return false;
    this.free(l);
    this.live.delete(id);
    this.nextPick = 0;
    return l.drawn;
  }

  /** Por quadro. */
  update(): void {
    const now = performance.now();
    if (now >= this.nextPick) this.pick(now);
    const t = now / 1000;
    for (const s of this.slots) {
      const l = s.live;
      if (!l) continue;
      const w = l.w;
      if (w.follow) {
        const p = w.follow();
        if (p) s.mesh.position.set(p.x, Y, p.z);
      }
      const secs = Math.max(0.05, w.durationMs / 1000);
      const age = (now - l.start) / 1000;
      s.state.set(Math.min(1, age / secs), t, Math.min(1, age / 0.15), 0);
    }
  }

  dispose(): void {
    for (const s of this.slots) {
      s.mesh.dispose();
      s.mat.dispose();
    }
    this.slots = [];
    this.live.clear();
  }

  /** Escolhe os MAX_DRAWN mais próximos de você (quem já está desenhado fica no mesmo lugar). */
  private pick(now: number): void {
    this.nextPick = now + PICK_MS;
    if (this.live.size === 0) return; // remove() já soltou os quadrados
    const me = this.me();
    const ranked: { l: Live; d: number }[] = [];
    for (const [id, l] of this.live) {
      if (now >= l.end) {
        this.free(l);
        this.live.delete(id);
        continue;
      }
      const p = l.w.follow?.() ?? null;
      const x = p ? p.x : l.w.x;
      const z = p ? p.z : l.w.z;
      const d = me ? Math.max(0, Math.hypot(me.x - x, me.z - z) - reachOf(l.w)) : 0;
      ranked.push({ l, d });
    }
    ranked.sort((a, b) => a.d - b.d);
    const chosen = new Set(ranked.slice(0, MAX_DRAWN).map((r) => r.l));
    for (const s of this.slots) if (s.live && !chosen.has(s.live)) this.free(s.live);
    for (const l of chosen) {
      if (l.slot) continue;
      const s = this.slots.find((x) => !x.live);
      if (s) this.assign(s, l);
    }
  }

  private assign(s: Slot, l: Live): void {
    const w = l.w;
    s.live = l;
    l.slot = s;
    l.drawn = true;
    const R = w.radius;
    switch (w.shape) {
      case TelegraphShape.CONE: {
        const h = w.arc / 2;
        const side = h >= Math.PI / 2 ? R : R * Math.sin(h);
        s.box.set(-side - PAD, Math.min(0, R * Math.cos(h)) - PAD, side + PAD, R + PAD);
        break;
      }
      case TelegraphShape.LINE:
        s.box.set(-w.width / 2 - PAD, -PAD, w.width / 2 + PAD, w.length + PAD);
        break;
      default:
        s.box.set(-R - PAD, -R - PAD, R + PAD, R + PAD);
    }
    s.params.set(R, w.inner, w.arc / 2, 0);
    s.line.set(w.length, w.width / 2, 0, 0);
    s.state.set(0, 0, 0, 0);
    s.mat.setFloat("uShape", SHAPE_ID[w.shape] ?? 0);
    s.mat.setColor3("uColor", w.color);
    s.mesh.position.set(w.x, Y, w.z);
    s.mesh.rotation.y = w.angle;
    s.mesh.setEnabled(true);
  }

  private free(l: Live): void {
    const s = l.slot;
    if (!s) return;
    s.live = null;
    s.mesh.setEnabled(false);
    l.slot = null;
  }
}
