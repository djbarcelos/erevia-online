/**
 * Natureza do remaster (?remaster=1): árvores, arbustos, mato, pedras e montanhas pintados à mão
 * (Meshy, Smart Topology; docs/arte/remaster-natureza.md) no lugar do kit KayKit, nas MESMAS posições
 * geradas. Cada item KayKit aponta para uma peça nova (nature-map.json, feito por
 * scripts/build-nature-kit.mjs) e a escala do mapa leva a peça ao tamanho do item antigo: mesma altura
 * (o vento, wind.ts, e o desvanecer de árvores contam com ela) e mesmos colisores.
 *
 * Peças são agrupadas por modelo: dezenas de itens KayKit viram uma dúzia de desenhos.
 * Sem o parâmetro, nada daqui roda.
 */
import { LoadAssetContainerAsync, Mesh, type Scene } from "@babylonjs/core";
import map from "./nature-map.json";
import type { WorldData } from "./world-data";

type Obj = WorldData["objects"][number];

const MODELS = map.models as string[];
const MODEL_SET = new Set(MODELS);
const KINDS = map.kinds as unknown as Record<string, { model: string; s: [number, number, number] }>;

/** O tipo é uma das peças da natureza do remaster (as que viram recortes, ver chunks.ts)? */
export function isNatureKind(kind: string): boolean {
  return MODEL_SET.has(kind);
}

/** Zona do Vale (z < isto): árvores de folha viram a árvore prateada do Vale. */
const VALE_Z = -150;
/** Itens de folha que, no Vale, trocam por ela (o resto fica carvalho/pinheiro, para variar). */
const VALE_SWAP = new Set(["tree_1_b", "tree_2_b", "tree_3_a", "tree_4_b"]);

/** Carrega as peças do kit novo: nome → malha (no espaço do mundo, sem __root__ nem LODs). */
export async function loadNatureKit(scene: Scene): Promise<Map<string, Mesh>> {
  const out = new Map<string, Mesh>();
  await Promise.all(
    MODELS.map(async (name) => {
      const c = await LoadAssetContainerAsync(`/models/nature_${name}_b.glb`, scene);
      c.addAllToScene();
      const main = c.meshes.find((m): m is Mesh => m instanceof Mesh && m.getTotalVertices() > 0 && !/_LOD[0-9]$/.test(m.name));
      if (!main) return;
      main.setParent(null);
      main.bakeCurrentTransformIntoVertices();
      // Os LODs (nós _LOD1/_LOD2) ficam no arquivo, mas thin instances não trocam de LOD por instância: saem com o __root__.
      for (const r of c.rootNodes) if (r.name === "__root__") r.dispose();
      out.set(name, main);
    }),
  );
  return out;
}

/** Troca os itens da natureza KayKit pelos do kit novo (mesma posição; escala e tipo novos). */
export function remapNature(objects: Obj[]): Obj[] {
  return objects.map((o) => {
    const e = KINDS[o.k];
    if (!e) return o;
    let model = e.model;
    if (model === "tree_oak" && o.z < VALE_Z && VALE_SWAP.has(o.k)) model = "tree_vale";
    return { ...o, k: model, s: [o.s[0] * e.s[0], o.s[1] * e.s[1], o.s[2] * e.s[2]] };
  });
}

function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/**
 * Peças só do remaster: lápides e ossos nos Campos dos Ossos (anel de 66 a 92 m do centro, fora do
 * Vale e da Cripta) e cristais no Vale das Pedras Vivas. Só enfeite: ficam longe dos colisores e
 * não bloqueiam nada. Determinístico (semente fixa).
 */
export function natureExtras(world: WorldData): Obj[] {
  const rand = rng(20240611);
  const out: Obj[] = [];
  const cripta = world.biomes?.find((b) => b.id === "cripta")?.bounds;
  const free = (x: number, z: number): boolean => {
    for (const c of world.colliders) if (Math.hypot(c.x - x, c.z - z) < c.r + 1.2) return false;
    for (const r of world.roads) {
      const dx = r.x2 - r.x1;
      const dz = r.z2 - r.z1;
      const t = Math.max(0, Math.min(1, ((x - r.x1) * dx + (z - r.z1) * dz) / (dx * dx + dz * dz || 1)));
      if (Math.hypot(x - (r.x1 + dx * t), z - (r.z1 + dz * t)) < r.w / 2 + 1.5) return false;
    }
    return true;
  };
  const add = (k: string, x: number, z: number, s: number): void => {
    out.push({ k, x: +x.toFixed(2), z: +z.toFixed(2), r: +(rand() * 6.28).toFixed(2), s: [s, s * (0.9 + rand() * 0.3), s] });
  };
  // Lápides: grupos pequenos de 2 a 4, tortas.
  for (let g = 0, tries = 0; g < 16 && tries < 400; tries++) {
    const a = rand() * Math.PI * 2;
    const rad = 66 + rand() * 26;
    const x = Math.cos(a) * rad;
    const z = Math.sin(a) * rad;
    if (z < -140 || (cripta && x > cripta.minX - 6 && z > cripta.minZ - 6) || !free(x, z)) continue;
    g++;
    const n = 2 + Math.floor(rand() * 3);
    for (let i = 0; i < n; i++) {
      const gx = x + (rand() - 0.5) * 5;
      const gz = z + (rand() - 0.5) * 5;
      if (free(gx, gz)) add("grave", gx, gz, 0.8 + rand() * 0.5);
    }
  }
  // Cristais: grupos no Vale (z de -330 a -150), longe das pedras grandes.
  for (let g = 0, tries = 0; g < 14 && tries < 600; tries++) {
    const x = -130 + rand() * 260;
    const z = -320 + rand() * 160;
    if (!free(x, z)) continue;
    g++;
    const n = 1 + Math.floor(rand() * 3);
    for (let i = 0; i < n; i++) add("crystal", x + (rand() - 0.5) * 3, z + (rand() - 0.5) * 3, 0.9 + rand() * 1.3);
  }
  return out;
}
