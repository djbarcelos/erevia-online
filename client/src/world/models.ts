import "@babylonjs/loaders/glTF";
import {
  AnimationGroup,
  AssetContainer,
  LoadAssetContainerAsync,
  Node,
  Scene,
  TransformNode,
  Vector3,
} from "@babylonjs/core";

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
};

type Slot = "handslot.r" | "handslot.l";
const WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  knight: { "handslot.r": "sword_1handed", "handslot.l": "shield_badge_color" },
  barbarian: { "handslot.r": "axe_1handed", "handslot.l": "shield_round_barbarian" },
  mage: { "handslot.r": "staff" },
  rogue: { "handslot.r": "dagger", "handslot.l": "dagger" },
};

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
  ) {}

  /** Baixa modelos, armas e animações em paralelo. Arquivos que faltarem são ignorados. */
  static async load(scene: Scene): Promise<CharacterModels> {
    const load = (file: string) => LoadAssetContainerAsync(BASE + file, scene);
    const tryLoad = (file: string) => load(file).catch(() => null);

    const allWeapons = { ...WEAPONS, ...MONSTER_WEAPONS };
    const weaponFiles = [...new Set(Object.values(allWeapons).flatMap((w) => Object.values(w)))];
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
      models.set(id, { body, weapons, scale: measureScale(body) });
    }

    // Os grupos de animação servem de molde; não tocam sozinhos.
    const templates = new Map<string, AnimationGroup>();
    for (const c of animContainers) {
      for (const g of c.animationGroups) {
        g.stop();
        templates.set(g.name, g);
      }
    }
    return new CharacterModels(models, templates);
  }

  has(model: string): boolean {
    return this.models.has(model);
  }

  /** Cria uma cópia animável do modelo (ou null se ele não existir). */
  instantiate(model: string, id: string): CharacterInstance | null {
    const assets = this.models.get(model);
    const set = ANIM_SETS[model];
    if (!assets || !set) return null;

    // Malhas com esqueleto precisam ser clonadas (não instanciadas).
    const entries = assets.body.instantiateModelsToScene((n) => `${n}#${id}`, false, { doNotInstantiate: true });
    const gltfRoot = entries.rootNodes[0] as TransformNode;

    const root = new TransformNode(`model#${id}`, gltfRoot.getScene());
    gltfRoot.parent = root;
    root.scaling.setAll(assets.scale);

    const byName = new Map<string, Node>();
    for (const n of gltfRoot.getDescendants(false)) byName.set(n.name.split("#")[0], n);

    // Armas presas nos encaixes das mãos.
    for (const [slot, container] of Object.entries(assets.weapons) as [Slot, AssetContainer][]) {
      const hand = byName.get(slot);
      if (!hand) continue;
      const w = container.instantiateModelsToScene((n) => `${n}#${id}`, false);
      const wRoot = w.rootNodes[0] as TransformNode;
      // O __root__ do glTF converte o sistema de coordenadas; dentro do esqueleto isso já foi
      // feito, então prendemos o filho direto e descartamos o __root__ da arma.
      for (const child of wRoot.getChildren() as TransformNode[]) {
        child.parent = hand as TransformNode;
        child.position = Vector3.Zero();
      }
      wRoot.dispose();
    }

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

    return {
      root,
      anims,
      dispose: () => {
        for (const a of cloned.values()) a.dispose();
        for (const s of entries.skeletons) s.dispose();
        root.dispose(false, true);
      },
    };
  }
}

/** Escala que deixa o modelo com TARGET_HEIGHT de altura. */
function measureScale(container: AssetContainer): number {
  let minY = Infinity;
  let maxY = -Infinity;
  for (const m of container.meshes) {
    if (!m.getTotalVertices()) continue;
    m.computeWorldMatrix(true);
    const b = m.getBoundingInfo().boundingBox;
    minY = Math.min(minY, b.minimumWorld.y);
    maxY = Math.max(maxY, b.maximumWorld.y);
  }
  const h = maxY - minY;
  return h > 0.1 && Number.isFinite(h) ? TARGET_HEIGHT / h : 1;
}
