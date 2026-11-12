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
 * Personagens KayKit (CC0). Todos usam o mesmo esqueleto ("Rig_Medium"), então as
 * animações vêm de arquivos separados e são reaplicadas em cada personagem pelo nome dos ossos.
 */

const BASE = "/models/";
/** Altura desejada do personagem em metros (o modelo é escalado para isso). */
const TARGET_HEIGHT = 1.8;

type Slot = "handslot.r" | "handslot.l";
const WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  knight: { "handslot.r": "sword_1handed", "handslot.l": "shield_badge_color" },
  barbarian: { "handslot.r": "axe_1handed", "handslot.l": "shield_round_barbarian" },
  mage: { "handslot.r": "staff" },
  rogue: { "handslot.r": "dagger", "handslot.l": "dagger" },
};

export type AnimName = "Idle_A" | "Running_A" | "Walking_A" | "Hit_A" | "Death_A";

export interface CharacterInstance {
  root: TransformNode;
  anims: Map<AnimName, AnimationGroup>;
  dispose(): void;
}

interface ClassAssets {
  body: AssetContainer;
  weapons: Partial<Record<Slot, AssetContainer>>;
  scale: number;
}

export class CharacterModels {
  private constructor(
    private classes: Map<string, ClassAssets>,
    private animTemplates: AnimationGroup[],
  ) {}

  /** Baixa personagens, armas e animações em paralelo. */
  static async load(scene: Scene): Promise<CharacterModels> {
    const load = (file: string) => LoadAssetContainerAsync(BASE + file, scene);

    const weaponFiles = [...new Set(Object.values(WEAPONS).flatMap((w) => Object.values(w)))];
    const [general, movement, weaponList, bodies] = await Promise.all([
      load("anims-general.glb"),
      load("anims-movementbasic.glb"),
      Promise.all(weaponFiles.map((w) => load(`weapons/${w}.glb`).then((c) => [w, c] as const))),
      Promise.all(Object.keys(WEAPONS).map((cls) => load(`${cls}.glb`).then((c) => [cls, c] as const))),
    ]);
    const weaponByFile = new Map(weaponList);

    const classes = new Map<string, ClassAssets>();
    for (const [cls, body] of bodies) {
      const weapons: ClassAssets["weapons"] = {};
      for (const [slot, file] of Object.entries(WEAPONS[cls]) as [Slot, string][]) {
        weapons[slot] = weaponByFile.get(file);
      }
      classes.set(cls, { body, weapons, scale: measureScale(body) });
    }

    // Os modelos de animação não entram na cena; os grupos servem de molde.
    const templates = [...general.animationGroups, ...movement.animationGroups];
    for (const g of templates) g.stop();
    return new CharacterModels(classes, templates);
  }

  /** Cria uma cópia animável do personagem da classe (ou null se a classe não existir). */
  instantiate(cls: string, id: string): CharacterInstance | null {
    const assets = this.classes.get(cls);
    if (!assets) return null;

    // Malhas com esqueleto precisam ser clonadas (não instanciadas).
    const entries = assets.body.instantiateModelsToScene((n) => `${n}#${id}`, false, { doNotInstantiate: true });
    const gltfRoot = entries.rootNodes[0] as TransformNode;

    const root = new TransformNode(`model#${id}`, gltfRoot.getScene());
    gltfRoot.parent = root;
    root.scaling.setAll(assets.scale);

    const byName = new Map<string, Node>();
    for (const n of gltfRoot.getDescendants(false)) byName.set(n.name.split("#")[0], n);
    for (const m of gltfRoot.getChildMeshes()) m.isPickable = false;

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
        for (const m of child.getChildMeshes(false)) m.isPickable = false;
        if ("isPickable" in child) (child as unknown as { isPickable: boolean }).isPickable = false;
      }
      wRoot.dispose();
    }

    // Animações reaplicadas pelo nome dos ossos.
    const anims = new Map<AnimName, AnimationGroup>();
    for (const tpl of this.animTemplates) {
      const clone = tpl.clone(`${tpl.name}#${id}`, (target: Node) => byName.get(target.name) ?? null);
      clone.stop();
      anims.set(tpl.name as AnimName, clone);
    }

    return {
      root,
      anims,
      dispose: () => {
        for (const a of anims.values()) a.dispose();
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
