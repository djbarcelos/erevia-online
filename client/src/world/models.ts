import "@babylonjs/loaders/glTF";
import {
  AbstractMesh,
  AnimationGroup,
  AssetContainer,
  Color3,
  LoadAssetContainerAsync,
  Material,
  Node,
  PBRMaterial,
  RawTexture,
  StandardMaterial,
  Texture,
  Quaternion,
  Scene,
  TransformNode,
  Vector3,
} from "@babylonjs/core";
import { bodyPalettes, colorKey, needsRepaint, normalLook, ORIGINAL_LOOK, paintTexture, type BodyLook } from "./body-look";

/**
 * Personagens e monstros KayKit (CC0). Todos usam o mesmo esqueleto ("Rig_Medium"),
 * então as animações vêm de arquivos separados e são reaplicadas pelo nome dos ossos.
 */

const BASE = "/models/";
/** Altura desejada do personagem em metros (o modelo é escalado para isso). */
const TARGET_HEIGHT = 1.8;

const ANIM_FILES = [
  "anims-general.glb",
  "anims-movementbasic.glb",
  "anims-combatmelee.glb",
  "anims-combatranged.glb",
  "anims-special.glb",
];

export type AnimRole = "idle" | "run" | "attack" | "skill" | "buff" | "hit" | "death";
type AnimSet = Record<AnimRole, string>;

const HERO_BASE = { idle: "Idle_A", run: "Running_A", buff: "Ranged_Magic_Raise", hit: "Hit_A", death: "Death_A" };
const SKELETON_BASE = {
  idle: "Skeletons_Idle",
  run: "Skeletons_Walking",
  buff: "Ranged_Magic_Raise",
  hit: "Hit_A",
  death: "Skeletons_Death",
};

/** Quais animações cada modelo usa em cada papel. */
const ANIM_SETS: Record<string, AnimSet> = {
  knight: { ...HERO_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_1H_Attack_Stab" },
  barbarian: { ...HERO_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_2H_Attack_Chop" },
  mage: { ...HERO_BASE, attack: "Ranged_Magic_Shoot", skill: "Ranged_Magic_Spellcasting" },
  rogue: { ...HERO_BASE, attack: "Melee_Dualwield_Attack_Stab", skill: "Melee_1H_Attack_Stab" },
  skeleton_minion: { ...SKELETON_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_1H_Attack_Chop" },
  skeleton_warrior: { ...SKELETON_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_2H_Attack_Chop" },
  skeleton_rogue: { ...SKELETON_BASE, attack: "Melee_Dualwield_Attack_Stab", skill: "Melee_1H_Attack_Stab" },
  skeleton_mage: { ...SKELETON_BASE, attack: "Ranged_Magic_Shoot", skill: "Ranged_Magic_Spellcasting" },
  npc_merchant: { ...HERO_BASE, attack: "Melee_1H_Attack_Chop", skill: "Ranged_Magic_Raise" },
  npc_smith: { ...HERO_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_1H_Attack_Chop" },
};

export type Slot = "handslot.r" | "handslot.l";
/** Armas padrão de cada modelo. Os heróis usam o que o servidor manda (equipamento). */
const WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  knight: {},
  barbarian: {},
  mage: {},
  rogue: {},
  npc_merchant: {},
  npc_smith: { "handslot.r": "axe_1handed" },
};
/** Todas as armas que os itens podem mostrar na mão. */
const ITEM_WEAPONS = [
  "sword_1handed", "sword_2handed", "sword_2handed_color", "axe_1handed", "axe_2handed", "staff", "wand", "dagger",
  "shield_badge_color", "shield_round_barbarian", "shield_square", "shield_spikes", "shield_spikes_color",
  // Opções da criação de personagem (mesmos arquivos das armas dos esqueletos).
  "skeleton_blade", "skeleton_axe", "skeleton_staff", "skeleton_shield_large_a",
];
/** Brilho das armas raras (3) e épicas (4). */
const GRADE_GLOW: Record<number, Color3> = { 3: new Color3(0.15, 0.35, 0.9), 4: new Color3(0.6, 0.2, 0.95) };
/** Encantamento: +4 a +6 brilha dourado; +7 ou mais, dourado forte e pulsando. */
const ENCHANT_GLOW = { light: new Color3(0.45, 0.33, 0.08), strong: new Color3(1, 0.72, 0.22) };

/** Qual brilho a arma usa: o do encantamento vence o da raridade. */
function glowKey(grade: number, enchant: number): string | null {
  if (enchant >= 7) return "e2";
  if (enchant >= 4) return "e1";
  return GRADE_GLOW[grade] ? `g${grade}` : null;
}

/** Modelos de monstro (arquivos opcionais: se faltarem, o jogo usa cápsulas). */
const MONSTER_MODELS = ["skeleton_minion", "skeleton_warrior", "skeleton_rogue", "skeleton_mage"];
const MONSTER_WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  skeleton_minion: { "handslot.r": "skeleton_blade" },
  skeleton_warrior: { "handslot.r": "skeleton_axe", "handslot.l": "skeleton_shield_large_a" },
  skeleton_rogue: { "handslot.r": "skeleton_blade", "handslot.l": "skeleton_blade" },
  skeleton_mage: { "handslot.r": "skeleton_staff" },
};

export interface CharacterInstance {
  root: TransformNode;
  anims: Map<AnimRole, AnimationGroup>;
  /**
   * Devolve os ossos à pose de descanso. As animações de ação (algumas feitas para os
   * heróis) movem canais que a pose parada dos esqueletos não anima — como a posição do
   * quadril —, e sem isso o corpo ficava enterrado no chão depois de um golpe.
   */
  resetPose(): void;
  /** Troca o que está nas mãos (null/"" = mão vazia). grade 3–4 e encantamento +4 fazem a arma brilhar. */
  setWeapons(right: string | null, left: string | null, grade?: number, enchant?: number): void;
  /** Troca o visual do corpo (pele, cabelo, roupa, rosto e chapéu). */
  setLook(look: BodyLook): void;
  /** Malhas atuais (corpo + armas), para sombra e visibilidade. */
  meshes(): AbstractMesh[];
  dispose(): void;
}

interface ModelAssets {
  body: AssetContainer;
  weapons: Partial<Record<Slot, AssetContainer>>;
  scale: number;
}

export class CharacterModels {
  private constructor(
    private models: Map<string, ModelAssets>,
    private animTemplates: Map<string, AnimationGroup>,
    private weaponFiles: Map<string, AssetContainer | null>,
  ) {}

  /** Materiais com brilho, por material original + raridade (compartilhados, nunca descartados). */
  private glowMats = new Map<string, Material>();
  /** Materiais com a textura repintada, por classe + cores; descartados quando ninguém usa. */
  private paintMats = new Map<string, { mat: Promise<Material | null>; refs: number }>();

  /** Pega (criando se preciso) o material repintado; devolva com releasePaint. */
  private acquirePaint(model: string, base: Material, look: BodyLook): Promise<Material | null> {
    const key = colorKey(model, look);
    let e = this.paintMats.get(key);
    if (!e) {
      e = { mat: this.paintMaterial(base, model, look), refs: 0 };
      this.paintMats.set(key, e);
    }
    e.refs++;
    return e.mat;
  }

  private releasePaint(model: string, look: BodyLook): void {
    const key = colorKey(model, look);
    const e = this.paintMats.get(key);
    if (!e || --e.refs > 0) return;
    this.paintMats.delete(key);
    void e.mat.then((m) => {
      if (!m) return;
      const tex = m instanceof PBRMaterial ? m.albedoTexture : m instanceof StandardMaterial ? m.diffuseTexture : null;
      tex?.dispose();
      m.dispose();
    });
  }

  /** Cópia do material com as faixas de pele, cabelo e roupa repintadas. */
  private async paintMaterial(base: Material, model: string, look: BodyLook): Promise<Material | null> {
    const pal = await bodyPalettes();
    const src = base instanceof PBRMaterial ? base.albedoTexture : base instanceof StandardMaterial ? base.diffuseTexture : null;
    if (!pal || !(src instanceof Texture)) return null;
    await new Promise<void>((ok) => (src.isReady() ? ok() : src.onLoadObservable.addOnce(() => ok())));
    const { width, height } = src.getSize();
    const pixels = await src.readPixels()?.catch(() => null);
    if (!pixels || !width) return null;
    const data = new Uint8Array(pixels.buffer, pixels.byteOffset, width * height * 4).slice();
    paintTexture(data, width, height, model, look, pal);
    const scene = base.getScene();
    const tex = RawTexture.CreateRGBATexture(data, width, height, scene, true, src.invertY, Texture.TRILINEAR_SAMPLINGMODE);
    tex.name = `${src.name}-${colorKey(model, look)}`;
    tex.gammaSpace = src.gammaSpace;
    tex.wrapU = src.wrapU;
    tex.wrapV = src.wrapV;
    const mat = base.clone(`${base.name}-${colorKey(model, look)}`);
    if (mat instanceof PBRMaterial) mat.albedoTexture = tex;
    else if (mat instanceof StandardMaterial) mat.diffuseTexture = tex;
    return mat;
  }

  /** Baixa modelos, armas e animações em paralelo. Arquivos que faltarem são ignorados. */
  static async load(scene: Scene): Promise<CharacterModels> {
    const load = (file: string) => LoadAssetContainerAsync(BASE + file, scene);
    const tryLoad = (file: string) => load(file).catch(() => null);

    const allWeapons = { ...WEAPONS, ...MONSTER_WEAPONS };
    const weaponFiles = [...new Set([...Object.values(allWeapons).flatMap((w) => Object.values(w)), ...ITEM_WEAPONS])];
    const modelIds = [...Object.keys(WEAPONS), ...MONSTER_MODELS];

    const [animContainers, weaponList, bodies] = await Promise.all([
      Promise.all(ANIM_FILES.map(load)),
      Promise.all(weaponFiles.map((w) => tryLoad(`weapons/${w}.glb`).then((c) => [w, c] as const))),
      Promise.all(modelIds.map((id) => tryLoad(`${id}.glb`).then((c) => [id, c] as const))),
    ]);
    const weaponByFile = new Map(weaponList);

    const models = new Map<string, ModelAssets>();
    for (const [id, body] of bodies) {
      if (!body) continue;
      const weapons: ModelAssets["weapons"] = {};
      for (const [slot, file] of Object.entries(allWeapons[id] ?? {}) as [Slot, string][]) {
        const w = weaponByFile.get(file);
        if (w) weapons[slot] = w;
      }
      models.set(id, { body, weapons, scale: measureScale(body, id) });
    }

    // Os grupos de animação servem de molde; não tocam sozinhos.
    const templates = new Map<string, AnimationGroup>();
    for (const c of animContainers) {
      for (const g of c.animationGroups) {
        g.stop();
        templates.set(g.name, g);
      }
    }
    return new CharacterModels(models, templates, weaponByFile);
  }

  /** Cópia do material com brilho (uma por material + tipo de brilho; "e2" pulsa). */
  private glowMaterial(base: Material, glow: string): Material {
    const key = `${base.uniqueId}:${glow}`;
    let mat = this.glowMats.get(key);
    if (!mat) {
      mat = base.clone(`${base.name}-${glow}`) ?? base;
      const c =
        glow === "e2" ? ENCHANT_GLOW.strong.clone() : glow === "e1" ? ENCHANT_GLOW.light : GRADE_GLOW[Number(glow.slice(1))];
      const target = mat instanceof PBRMaterial || mat instanceof StandardMaterial ? mat : null;
      if (target) target.emissiveColor = c;
      if (glow === "e2" && target) {
        // Um pulso por material (todas as armas +7 pulsam juntas).
        const scene = mat.getScene();
        scene.onBeforeRenderObservable.add(() => {
          const k = 0.55 + 0.45 * Math.sin(performance.now() / 260);
          target.emissiveColor.copyFromFloats(ENCHANT_GLOW.strong.r * k, ENCHANT_GLOW.strong.g * k, ENCHANT_GLOW.strong.b * k);
        });
      }
      this.glowMats.set(key, mat);
    }
    return mat;
  }

  has(model: string): boolean {
    return this.models.has(model);
  }

  /** Arquivos de arma carregados (podem ser usados em `weapons` do instantiate). */
  weaponNames(): string[] {
    return [...this.weaponFiles].filter(([, c]) => c).map(([w]) => w);
  }

  /**
   * Cria uma cópia animável do modelo (ou null se ele não existir).
   * `weapons` troca o que vai em cada mão (null = mão vazia); sem ele, usa as armas padrão.
   */
  instantiate(model: string, id: string, weapons?: Partial<Record<Slot, string | null>>, look?: BodyLook): CharacterInstance | null {
    const assets = this.models.get(model);
    const set = ANIM_SETS[model];
    if (!assets || !set) return null;
    let handItems = assets.weapons;
    if (weapons) {
      handItems = { ...assets.weapons };
      for (const [slot, file] of Object.entries(weapons) as [Slot, string | null][]) {
        const c = file ? this.weaponFiles.get(file) : null;
        if (c) handItems[slot] = c;
        else delete handItems[slot];
      }
    }

    // Malhas com esqueleto precisam ser clonadas (não instanciadas).
    const entries = assets.body.instantiateModelsToScene((n) => `${n}#${id}`, false, { doNotInstantiate: true });
    const gltfRoot = entries.rootNodes[0] as TransformNode;

    const root = new TransformNode(`model#${id}`, gltfRoot.getScene());
    gltfRoot.parent = root;
    root.scaling.setAll(assets.scale);

    const byName = new Map<string, Node>();
    for (const n of gltfRoot.getDescendants(false)) byName.set(n.name.split("#")[0], n);

    // Armas presas nos encaixes das mãos (trocadas quando o equipamento muda).
    const held = new Map<Slot, TransformNode[]>();
    const attach = (slot: Slot, container: AssetContainer | null | undefined, grade: number, enchant = 0) => {
      for (const n of held.get(slot) ?? []) n.dispose(false, false);
      held.delete(slot);
      const hand = byName.get(slot);
      if (!hand || !container) return;
      const w = container.instantiateModelsToScene((n) => `${n}#${id}`, false);
      const wRoot = w.rootNodes[0] as TransformNode;
      // O __root__ do glTF converte o sistema de coordenadas; dentro do esqueleto isso já foi
      // feito, então prendemos o filho direto e descartamos o __root__ da arma.
      const nodes: TransformNode[] = [];
      for (const child of wRoot.getChildren() as TransformNode[]) {
        child.parent = hand as TransformNode;
        child.position = Vector3.Zero();
        nodes.push(child);
        const glow = glowKey(grade, enchant);
        if (glow) {
          for (const m of child.getChildMeshes(false)) if (m.material) m.material = this.glowMaterial(m.material, glow);
          if (child instanceof AbstractMesh && child.material) child.material = this.glowMaterial(child.material, glow);
        }
      }
      wRoot.dispose();
      held.set(slot, nodes);
    };
    for (const [slot, container] of Object.entries(handItems) as [Slot, AssetContainer][]) attach(slot, container, 0);

    // Visual do corpo: só os heróis têm várias cabeças e o chapéu separado.
    const heads = new Map<string, AbstractMesh>();
    for (const [n, node] of byName) if (n.startsWith("Head_") && node instanceof AbstractMesh) heads.set(n.slice(5), node);
    const hat = byName.get("Hat");
    // Só as malhas com esqueleto (as armas presas nas mãos não entram).
    const bodyMeshes = gltfRoot.getChildMeshes(false).filter((m) => !!m.skeleton && !!m.material);
    const baseMat = bodyMeshes[0]?.material ?? null;
    let painted: BodyLook | null = null; // cores em uso (material repintado)
    let disposed = false;
    const applyLook = (l: BodyLook) => {
      l = normalLook(model, l);
      if (heads.size) {
        const want = heads.has(l.head) ? l.head : model;
        for (const [hid, mesh] of heads) mesh.setEnabled(hid === want);
      }
      if (hat instanceof TransformNode) hat.setEnabled(!l.hideHat);
      if (!baseMat || (painted && colorKey(model, painted) === colorKey(model, l))) return;
      if (painted) this.releasePaint(model, painted);
      painted = null;
      for (const m of bodyMeshes) m.material = baseMat;
      if (!needsRepaint(l)) return;
      painted = l;
      void this.acquirePaint(model, baseMat, l).then((mat) => {
        if (!mat || disposed || painted !== l) return;
        for (const m of bodyMeshes) m.material = mat;
      });
    };
    if (look) applyLook(look);
    else if (heads.size) applyLook(ORIGINAL_LOOK);

    // Animações (só as do conjunto do modelo), reaplicadas pelo nome dos ossos.
    const anims = new Map<AnimRole, AnimationGroup>();
    const cloned = new Map<string, AnimationGroup>();
    for (const [role, animName] of Object.entries(set) as [AnimRole, string][]) {
      let g = cloned.get(animName);
      if (!g) {
        const tpl = this.animTemplates.get(animName);
        if (!tpl) continue;
        g = tpl.clone(`${animName}#${id}`, (target: Node) => byName.get(target.name) ?? null);
        g.stop();
        cloned.set(animName, g);
      }
      anims.set(role, g);
    }

    // Pose de descanso de tudo que alguma animação mexe (antes de qualquer animação tocar).
    const rest: { node: TransformNode; pos: Vector3; rot: Quaternion | null; scale: Vector3 }[] = [];
    const seen = new Set<TransformNode>();
    for (const g of cloned.values()) {
      for (const ta of g.targetedAnimations) {
        const node = ta.target as TransformNode;
        if (!(node instanceof TransformNode) || seen.has(node)) continue;
        seen.add(node);
        rest.push({ node, pos: node.position.clone(), rot: node.rotationQuaternion?.clone() ?? null, scale: node.scaling.clone() });
      }
    }

    return {
      root,
      anims,
      resetPose: () => {
        for (const r of rest) {
          r.node.position.copyFrom(r.pos);
          if (r.rot && r.node.rotationQuaternion) r.node.rotationQuaternion.copyFrom(r.rot);
          r.node.scaling.copyFrom(r.scale);
        }
      },
      setWeapons: (right, left, grade = 0, enchant = 0) => {
        attach("handslot.r", right ? this.weaponFiles.get(right) : null, grade, enchant);
        attach("handslot.l", left ? this.weaponFiles.get(left) : null, grade, enchant);
      },
      setLook: (l) => {
        if (!disposed) applyLook(l);
      },
      meshes: () => root.getChildMeshes(false),
      dispose: () => {
        disposed = true;
        if (painted) this.releasePaint(model, painted);
        painted = null;
        for (const a of cloned.values()) a.dispose();
        for (const s of entries.skeletons) s.dispose();
        // Nunca descartar materiais/texturas aqui: são compartilhados por todas as cópias
        // do modelo (descartá-los deixava os outros personagens brancos).
        root.dispose(false, false);
      },
    };
  }
}

/** Escala que deixa o modelo com TARGET_HEIGHT de altura (com a cabeça da própria classe). */
function measureScale(container: AssetContainer, model: string): number {
  let minY = Infinity;
  let maxY = -Infinity;
  for (const m of container.meshes) {
    if (!m.getTotalVertices()) continue;
    if (m.name.startsWith("Head_") && m.name !== `Head_${model}`) continue;
    m.computeWorldMatrix(true);
    const b = m.getBoundingInfo().boundingBox;
    minY = Math.min(minY, b.minimumWorld.y);
    maxY = Math.max(maxY, b.maximumWorld.y);
  }
  const h = maxY - minY;
  return h > 0.1 && Number.isFinite(h) ? TARGET_HEIGHT / h : 1;
}
