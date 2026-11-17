import "@babylonjs/loaders/glTF";
import {
  AbstractMesh,
  LoadAssetContainerAsync,
  Material,
  Matrix,
  Mesh,
  Quaternion,
  DirectionalLight,
  Scene,
  ShadowGenerator,
  Vector3,
  VertexData,
} from "@babylonjs/core";
import { CHUNKS_ON, CHUNK_MIN_TRIS, ShadowCuller, splitByChunk } from "./chunks";
import { isNatureKind, loadNatureKit, natureExtras, remapNature } from "./nature-remaster";
import { addRemasterPlaza } from "./plaza-remaster";
import { REMASTER } from "./remaster";
import { regionProps, townExtras, wallCorners } from "./town-layout";
import { loadTownProtos } from "./town-remaster";
import { monolithProtos } from "./teleport-stone";
import { heightAt, terrain } from "./terrain";
import { setWindKind, type WindKind } from "./wind";
import type { WorldData } from "./world-data";

/** Tipos que projetam sombra (grama, arbustos e adereços pequenos não, para economizar). */
const CASTS_SHADOW = /^(tree_|rock_3|rock_1_[hk]|rock_2_f|rock_l|cliff|monolith|tavern|church|blacksmith|market|home_|well|windmill|tower_|wall_|town_)/;
/** Tipos que recebem sombra (construções e muralhas ficam mais "assentadas" no chão). */
const RECEIVES_SHADOW = /^(tavern|church|blacksmith|market|home_|windmill|tower_|wall_|well|monolith|town_)/;
/** Árvores ficam translúcidas quando tapam o jogador ou o alvo. */
/** Peças que afundam um pouco no chão (m): a malha do Meshy traz uma película de poça azulada rente ao chão, sob a cabana. */
const SINK: Record<string, number> = { ferryman_hut: 0.15 };
const FADES = /^tree_/;

/** Itens que balançam ao vento (vertex shader, wind.ts): grama, arbustos e árvores. */
function windKindOf(kind: string): WindKind {
  if (kind.startsWith("grass_")) return "grass";
  if (kind.startsWith("bush_")) return "bush";
  if (kind.startsWith("tree_")) return "tree";
  return "off";
}

export interface Environment {
  drawn: number;
  fader: TreeFader;
  /** Remaster: recortes da natureza que o mapa de sombras enxerga (chamar a cada quadro, depois de mover a luz). */
  shadowCull: ShadowCuller | null;
}

/**
 * Cenário: carrega os kits (um .glb por pacote, um item = uma malha) e desenha cada
 * tipo de item com thin instances — milhares de objetos em poucas dezenas de draw calls.
 */
export async function loadEnvironment(scene: Scene, world: WorldData, shadows: ShadowGenerator): Promise<Environment> {
  // Os kits do mapa (o principal usa os dois; os outros dizem quais no world.json).
  const kits = world.kits ?? ["/models/env-forest.glb", "/models/env-town.glb"];
  const containers = await Promise.all(kits.map((f) => LoadAssetContainerAsync(f, scene)));

  // nome do item → malha (já sem a hierarquia do glTF, com o transform aplicado nos vértices)
  const protos = new Map<string, Mesh>();
  for (const c of containers) {
    c.addAllToScene();
    for (const node of c.transformNodes) {
      if (!node.name.startsWith("item:")) continue;
      const mesh = node.getChildMeshes(false).find((m): m is Mesh => m instanceof Mesh && m.getTotalVertices() > 0);
      if (!mesh) continue;
      mesh.setParent(null); // mantém a posição no mundo (inclui a conversão de eixos do glTF)
      mesh.bakeCurrentTransformIntoVertices();
      protos.set(node.name.slice(5), mesh);
    }
    for (const root of c.rootNodes) if (root.name === "__root__") root.dispose();
  }
  // Monólitos rúnicos do vale: desenhados em código (mesma pedra da Pedra de Teleporte).
  if (world.objects.some((o) => o.k.startsWith("monolith"))) {
    for (const [kind, mesh] of monolithProtos(scene)) protos.set(kind, mesh);
  }

  // Remaster: a natureza do mapa principal vem do kit pintado à mão, nas mesmas posições (nature-remaster.ts),
  // e os prédios e adereços da cidade vêm do kit novo, no mesmo layout (tunnel#100). Lápides e cristais
  // das regiões são os da natureza; da cidade fica só o pilar dos Ermos (para não duplicar).
  let objects = world.objects;
  if (REMASTER && !world.kits) {
    for (const [name, mesh] of await loadNatureKit(scene)) protos.set(name, mesh);
    for (const [kind, mesh] of await loadTownProtos(scene)) {
      protos.get(kind)?.setEnabled(false);
      protos.set(kind, mesh);
    }
    objects = [
      ...remapNature(objects),
      ...natureExtras(world),
      ...townExtras(),
      ...regionProps(world).filter((o) => o.k === "town_pillar"),
      ...wallCorners(world),
    ];
  }

  // Agrupa as posições por tipo e cria um buffer de matrizes para cada um.
  const byKind = new Map<string, WorldData["objects"]>();
  // A lagoa do Brejo (tunnel#139) está no meio do pântano: árvores, juncos, o cenário de Vau Velho e a ilha ficam na água
  // (o servidor deixa pisar na rasa). Só as montanhas da borda, se a geração pôs alguma em cima, somem.
  const field = terrain();
  const lake = field.lake;
  for (const o of objects) {
    if (REMASTER && o.k === "well") continue; // a fonte do remaster ocupa o lugar do poço
    if (lake && /^(mountain|hills)/.test(o.k) && field.lakeU(o.x, o.z) < 1.5 && heightAt(o.x, o.z) < lake.level + 0.45) continue;
    let list = byKind.get(o.k);
    if (!list) byKind.set(o.k, (list = []));
    list.push(o);
  }

  const fader = new TreeFader(scene, shadows);
  // Remaster com ?chunks=1: a natureza vira recortes por célula (chunks.ts), cada um cortado pelo campo de visão e pelo volume da luz.
  const sun = shadows.getLight();
  const culler = REMASTER && !world.kits && sun instanceof DirectionalLight ? new ShadowCuller(shadows, sun) : null;
  const useChunks = CHUNKS_ON && culler !== null;
  const scale = new Vector3();
  const pos = new Vector3();
  const rot = new Quaternion();
  const m = new Matrix();
  let drawn = 0;
  for (const [kind, list] of byKind) {
    const mesh = protos.get(kind);
    if (!mesh) {
      console.warn(`cenário: item sem modelo: ${kind}`);
      continue;
    }
    // Antes das thin instances (que guardam buffers na geometria): molde limpo para as
    // cópias translúcidas e o tamanho do item, para estimar a copa.
    // Vento: o material é copiado por tipo (o glTF reparte um material entre vários itens).
    const wind = windKindOf(kind);
    if (wind !== "off" && mesh.material) {
      mesh.refreshBoundingInfo();
      mesh.material = mesh.material.clone(`${mesh.material.name}:${kind}`);
      setWindKind(mesh.material, wind, Math.max(mesh.getBoundingInfo().boundingBox.maximum.y, 0.05));
    }
    const fades = FADES.test(kind);
    if (fades) fader.addKind(kind, mesh);

    const chunked = useChunks && isNatureKind(kind) && (mesh.getTotalIndices() / 3) * list.length >= CHUNK_MIN_TRIS;
    const groups = chunked ? [...splitByChunk(list)] : [["", list] as const];
    // Cada recorte precisa da própria geometria: o buffer das thin instances entra como buffer de vértices dela
    // (um clone que divide a geometria com os outros sobrescreveria as matrizes dos vizinhos).
    for (const [cell, items] of groups) {
      let target = mesh;
      if (chunked) {
        // clone() mantém tudo do molde (material, orientação das faces, cores); a geometria troca por uma cópia.
        target = mesh.clone(`${mesh.name}@${cell}`, null, true)!;
        mesh.geometry!.copy(`${mesh.geometry!.id}@${cell}`).applyToMesh(target);
      }
      const buf = new Float32Array(items.length * 16);
      items.forEach((o, i) => {
        scale.set(o.s[0], o.s[1], o.s[2]);
        Quaternion.RotationYawPitchRollToRef(o.r, 0, 0, rot);
        pos.set(o.x, heightAt(o.x, o.z) - (SINK[kind] ?? 0), o.z);
        Matrix.ComposeToRef(scale, rot, pos, m);
        m.copyToArray(buf, i * 16);
        if (fades) fader.addTree(kind, i, o, m, target);
      });
      // Árvores podem ter instâncias escondidas/reveladas: buffer atualizável.
      target.thinInstanceSetBuffer("matrix", buf, 16, !fades);
      target.thinInstanceRefreshBoundingInfo(false);
      prepareStatic(target);
      if (CASTS_SHADOW.test(kind)) {
        if (chunked) culler!.add(target);
        else shadows.addShadowCaster(target, false);
      }
      if (RECEIVES_SHADOW.test(kind)) target.receiveShadows = true;
    }
    // O molde dos recortes não se desenha.
    if (chunked) mesh.setEnabled(false);
    drawn += list.length;
  }
  // Protótipos não usados no layout ficam invisíveis.
  for (const [kind, mesh] of protos) if (!byKind.has(kind)) mesh.setEnabled(false);
  if (REMASTER) await addRemasterPlaza(scene, shadows);
  return { drawn, fader, shadowCull: useChunks ? culler : null };
}

function prepareStatic(mesh: AbstractMesh): void {
  mesh.isPickable = false; // o clique atravessa o cenário e chega ao chão
  mesh.freezeWorldMatrix();
  mesh.doNotSyncBoundingInfo = true;
}

// ---------------------------------------------------------------------------------------

/** Translucidez de uma árvore que tapa a visão. */
const FADED_VISIBILITY = 0.28;
/** Velocidade do esmaecer (1/s). */
const FADE_RATE = 8;
/** Lado da célula da grade espacial de árvores (m). */
const CELL = 10;
const ZERO = Matrix.Scaling(0, 0, 0);

interface KindInfo {
  proto: Mesh;
  /** Cópia com geometria própria (sem os buffers das thin instances), para clonar. */
  template: Mesh;
  /** Largura (maior lado no chão) e altura do item em escala 1. */
  width: number;
  height: number;
  pool: Mesh[];
}

interface Tree {
  kind: KindInfo;
  /** Malha com as instâncias (o molde, ou o recorte da célula, no remaster) e o índice dentro dela. */
  mesh: Mesh;
  index: number;
  matrix: Matrix;
  /** Copa como esfera. */
  cx: number;
  cy: number;
  cz: number;
  r: number;
}

/**
 * Árvores que ficam entre a câmera e o jogador (ou o alvo) ficam translúcidas.
 * Como as árvores são thin instances (todas de um tipo num só desenho), a árvore que tapa
 * a visão é escondida no buffer e trocada por uma cópia comum com visibility < 1.
 */
export class TreeFader {
  private kinds = new Map<string, KindInfo>();
  private grid = new Map<string, Tree[]>();
  /** Árvore escondida no buffer → cópia translúcida que está no lugar dela. */
  private faded = new Map<Tree, Mesh>();
  private seen = new Set<Tree>();
  /** Material original → cópia com mistura de transparência (o glTF marca OPAQUE, e aí
   *  o Babylon ignora a visibility da malha). */
  private fadeMats = new Map<Material, Material>();

  constructor(
    private scene: Scene,
    private shadows: ShadowGenerator,
  ) {}

  addKind(kind: string, proto: Mesh): void {
    const template = new Mesh(`fade-tpl:${kind}`, this.scene);
    VertexData.ExtractFromMesh(proto).applyToMesh(template);
    template.material = this.fadeMaterial(proto.material);
    template.isPickable = false;
    template.setEnabled(false);
    proto.refreshBoundingInfo();
    const b = proto.getBoundingInfo().boundingBox;
    const width = Math.max(b.maximum.x - b.minimum.x, b.maximum.z - b.minimum.z);
    this.kinds.set(kind, { proto, template, width, height: b.maximum.y - b.minimum.y, pool: [] });
  }

  private fadeMaterial(mat: Material | null): Material | null {
    if (!mat) return null;
    let fade = this.fadeMats.get(mat);
    if (!fade) {
      fade = mat.clone(`${mat.name}-fade`)!;
      fade.transparencyMode = Material.MATERIAL_ALPHABLEND;
      this.fadeMats.set(mat, fade);
    }
    return fade;
  }

  addTree(kind: string, index: number, o: WorldData["objects"][number], matrix: Matrix, mesh?: Mesh): void {
    const info = this.kinds.get(kind)!;
    const h = info.height * o.s[1];
    const w = info.width * Math.max(o.s[0], o.s[2]);
    // Copa: esfera um pouco acima do meio da árvore, sem passar da metade da largura.
    const tree: Tree = { kind: info, mesh: mesh ?? info.proto, index, matrix: matrix.clone(), cx: o.x, cy: heightAt(o.x, o.z) + h * 0.62, cz: o.z, r: Math.min(w / 2, h * 0.4) };
    const key = cellKey(Math.floor(o.x / CELL), Math.floor(o.z / CELL));
    let list = this.grid.get(key);
    if (!list) this.grid.set(key, (list = []));
    list.push(tree);
  }

  /**
   * A cada quadro: árvores cujas copas cortam a linha câmera→foco ficam translúcidas;
   * as que deixaram de tapar voltam ao normal aos poucos.
   */
  update(dt: number, cam: Vector3, focus: Vector3[]): void {
    this.seen.clear();
    for (const f of focus) this.collect(cam, f);

    for (const t of this.seen) {
      if (this.faded.has(t)) continue;
      this.faded.set(t, this.takeCopy(t));
      t.mesh.thinInstanceSetMatrixAt(t.index, ZERO, true);
    }

    const k = 1 - Math.exp(-FADE_RATE * dt);
    for (const [t, copy] of this.faded) {
      const occluding = this.seen.has(t);
      copy.visibility += ((occluding ? FADED_VISIBILITY : 1) - copy.visibility) * k;
      if (!occluding && copy.visibility > 0.98) this.restore(t, copy);
    }
  }

  /** Voltou ao normal: devolve a árvore ao buffer e a cópia ao estoque. */
  private restore(t: Tree, copy: Mesh): void {
    t.mesh.thinInstanceSetMatrixAt(t.index, t.matrix, true);
    copy.setEnabled(false);
    t.kind.pool.push(copy);
    this.faded.delete(t);
  }

  /**
   * Termina na hora as transições do último update (medição de FPS, ?bench=1): sem isso, quantas
   * cópias ainda estão voltando ao normal depende do FPS, e o número de malhas ativas também.
   */
  settle(): void {
    for (const [t, copy] of this.faded) {
      if (this.seen.has(t)) copy.visibility = FADED_VISIBILITY;
      else this.restore(t, copy);
    }
  }

  /** Cópias guardadas no estoque (desligadas): quantas são depende de quantas já se precisou. */
  pooled(): Mesh[] {
    const out: Mesh[] = [];
    for (const k of this.kinds.values()) out.push(...k.pool);
    return out;
  }

  private collect(a: Vector3, b: Vector3): void {
    const x0 = Math.floor((Math.min(a.x, b.x) - CELL) / CELL);
    const x1 = Math.floor((Math.max(a.x, b.x) + CELL) / CELL);
    const z0 = Math.floor((Math.min(a.z, b.z) - CELL) / CELL);
    const z1 = Math.floor((Math.max(a.z, b.z) + CELL) / CELL);
    for (let cx = x0; cx <= x1; cx++) {
      for (let cz = z0; cz <= z1; cz++) {
        const list = this.grid.get(cellKey(cx, cz));
        if (!list) continue;
        for (const t of list) if (segmentHitsSphere(a, b, t)) this.seen.add(t);
      }
    }
  }

  private takeCopy(t: Tree): Mesh {
    let copy = t.kind.pool.pop();
    if (!copy) {
      copy = t.kind.template.clone(`${t.kind.template.name}#copy`)!;
      copy.isPickable = false;
      copy.rotationQuaternion = new Quaternion();
      this.shadows.addShadowCaster(copy, false);
    }
    t.matrix.decompose(copy.scaling, copy.rotationQuaternion!, copy.position);
    copy.visibility = 1;
    copy.setEnabled(true);
    return copy;
  }
}

function cellKey(x: number, z: number): string {
  return `${x},${z}`;
}

/** O segmento a→b passa pela copa (com uma pequena folga para não piscar na borda)? */
function segmentHitsSphere(a: Vector3, b: Vector3, t: Tree): boolean {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const dz = b.z - a.z;
  const len2 = dx * dx + dy * dy + dz * dz;
  if (len2 < 1e-6) return false;
  let s = ((t.cx - a.x) * dx + (t.cy - a.y) * dy + (t.cz - a.z) * dz) / len2;
  s = Math.max(0, Math.min(1, s));
  const px = a.x + dx * s - t.cx;
  const py = a.y + dy * s - t.cy;
  const pz = a.z + dz * s - t.cz;
  const r = t.r * 0.9;
  return px * px + py * py + pz * pz < r * r;
}
