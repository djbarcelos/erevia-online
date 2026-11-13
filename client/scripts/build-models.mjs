// Gera client/public/models/ a partir dos pacotes KayKit (CC0) descompactados.
//
// Uso: node scripts/build-models.mjs <pasta-com-os-zips-descompactados>
//   ex.: node scripts/build-models.mjs ../assets-src/kaykit/x
//
// - Personagens: as peças com esqueleto (braços, pernas, cabeça...) viram uma malha por
//   material — 1 draw call por personagem em vez de ~9.
// - Animações: só as que o jogo usa, sem as malhas do manequim.
// - Armas: .gltf + .bin + .png viram um .glb cada.
// - Monstros: KayKit Skeletons (mesmo esqueleto Rig_Medium dos heróis).
import { NodeIO } from '@gltf-transform/core';
import { dedup, joinPrimitives, prune, resample } from '@gltf-transform/functions';
import { mkdirSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';

const src = resolve(process.argv[2] ?? '../assets-src/kaykit/x');
const out = resolve(import.meta.dirname, '../public/models');
const adventurers = join(src, 'KayKit_Adventurers_2.0_FREE');
const animations = join(src, 'KayKit_Character_Animations_1.1/Animations/gltf/Rig_Medium');
const skeletons = join(src, 'KayKit_Skeletons_1.1_FREE');

const MONSTERS = {
  skeleton_minion: 'Skeleton_Minion',
  skeleton_warrior: 'Skeleton_Warrior',
  skeleton_rogue: 'Skeleton_Rogue',
  skeleton_mage: 'Skeleton_Mage',
};
const MONSTER_WEAPONS = ['Skeleton_Blade', 'Skeleton_Axe', 'Skeleton_Shield_Large_A', 'Skeleton_Staff'];

const CHARACTERS = {
  knight: 'Knight', barbarian: 'Barbarian', mage: 'Mage', rogue: 'Rogue',
  // NPCs da cidade (mesmo esqueleto dos heróis)
  npc_merchant: 'Rogue_Hooded', npc_smith: 'Ranger',
};

// arquivo de origem → animações mantidas
const ANIMS = {
  'Rig_Medium_General.glb': ['Idle_A', 'Hit_A', 'Death_A'],
  'Rig_Medium_MovementBasic.glb': ['Running_A', 'Walking_A'],
  'Rig_Medium_CombatMelee.glb': ['Melee_1H_Attack_Chop', 'Melee_2H_Attack_Chop', 'Melee_Dualwield_Attack_Stab', 'Melee_1H_Attack_Stab'],
  'Rig_Medium_CombatRanged.glb': ['Ranged_Magic_Shoot', 'Ranged_Magic_Spellcasting', 'Ranged_Magic_Raise'],
  'Rig_Medium_Special.glb': ['Skeletons_Idle', 'Skeletons_Walking', 'Skeletons_Death', 'Skeletons_Awaken_Floor'],
};

const WEAPONS = [
  'sword_1handed', 'shield_badge_color', 'axe_1handed', 'shield_round_barbarian', 'staff', 'dagger',
  // Fase 3: armas e escudos das outras raridades
  'sword_2handed', 'sword_2handed_color', 'axe_2handed', 'wand', 'shield_square', 'shield_spikes', 'shield_spikes_color',
];
// Adereços avulsos (Medieval Hexagon): saquinho dos itens no chão.
const PROPS = { loot_sack: 'decoration/props/sack' };
const hexagon = join(src, 'KayKit_Medieval_Hexagon_Pack_1.0_FREE/Assets/gltf');

const io = new NodeIO();
mkdirSync(join(out, 'weapons'), { recursive: true });
const report = [];
const size = (f) => `${(statSync(f).size / 1024).toFixed(0)} KB`;

/**
 * O join() do gltf-transform ignora malhas com skin. Aqui juntamos as peças irmãs que usam a
 * mesma skin e o mesmo material: no glTF a transformação do nó de uma malha com skin é
 * ignorada (os vértices já estão no espaço do bind pose), então basta concatenar.
 */
function joinSkinned(doc, skip = () => false) {
  const groups = new Map();
  for (const node of doc.getRoot().listNodes()) {
    const mesh = node.getMesh();
    const skin = node.getSkin();
    if (!mesh || !skin || skip(node)) continue;
    for (const prim of mesh.listPrimitives()) {
      if (prim.listTargets().length) continue;
      const attrs = prim.listSemantics().sort().map((s) => `${s}:${prim.getAttribute(s).getComponentType()}`).join(',');
      const key = [node.getParentNode()?.getName(), doc.getRoot().listSkins().indexOf(skin), prim.getMaterial()?.getName(), prim.getMode(), attrs].join('|');
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push({ node, mesh, prim });
    }
  }
  let joined = 0;
  for (const list of groups.values()) {
    if (list.length < 2) continue;
    const merged = joinPrimitives(list.map((e) => e.prim));
    const keep = list[0];
    keep.node.setName(`${keep.node.getName().split('_')[0]}_${keep.prim.getMaterial()?.getName() ?? 'mesh'}`);
    const mesh = doc.createMesh(keep.node.getName()).addPrimitive(merged);
    keep.node.setMesh(mesh);
    for (const e of list.slice(1)) e.node.dispose();
    joined += list.length;
  }
  return joined;
}

async function buildCharacter(srcFile, id) {
  const doc = await io.read(srcFile);
  const n = joinSkinned(doc);
  await doc.transform(prune());
  const dst = join(out, `${id}.glb`);
  await io.write(dst, doc);
  const meshes = doc.getRoot().listMeshes().length;
  report.push(`${id}.glb ${size(dst)} (${n} peças → ${meshes} malhas)`);
}

/**
 * Heróis: a cabeça e o chapéu ficam em malhas separadas (a aparência troca a cabeça e
 * esconde o chapéu), e o arquivo leva as 5 cabeças do pacote — todas no mesmo esqueleto e
 * com o mesmo desenho de paleta (pele, cabelo e olhos nas 3 primeiras faixas), então
 * qualquer uma funciona com a textura de qualquer classe.
 */
const HEROES = { knight: 'Knight', barbarian: 'Barbarian', mage: 'Mage', rogue: 'Rogue' };
const HEADS = { knight: 'Knight', barbarian: 'Barbarian', mage: 'Mage', rogue: 'Rogue', ranger: 'Ranger' };
const HAT_PARTS = ['_Helmet', '_HelmetVisor', '_BearHat', '_Hat'];
const isHead = (node) => node.getName().endsWith('_Head');
const isHat = (node) => HAT_PARTS.some((p) => node.getName().endsWith(p));

async function buildHero(id, name) {
  const doc = await io.read(join(adventurers, 'Characters/gltf', `${name}.glb`));
  const root = doc.getRoot();
  // Corpo, braços, pernas e capa numa malha; cabeça e chapéu fora.
  const n = joinSkinned(doc, (node) => isHead(node) || isHat(node));
  const hats = root.listNodes().filter((node) => node.getMesh() && isHat(node));
  if (hats.length) {
    const prims = hats.flatMap((h) => h.getMesh().listPrimitives());
    const merged = prims.length > 1 ? joinPrimitives(prims) : prims[0];
    hats[0].setName('Hat').setMesh(doc.createMesh('Hat').addPrimitive(merged));
    for (const h of hats.slice(1)) h.dispose();
  }
  const own = root.listNodes().find((node) => node.getMesh() && isHead(node));
  if (!own) throw new Error(`${name}: sem cabeça`);
  const skin = own.getSkin();
  const material = own.getMesh().listPrimitives()[0].getMaterial();
  own.setName(`Head_${id}`);
  own.getMesh().setName(`Head_${id}`);
  const parent = own.getParentNode();
  const jointIndex = new Map(skin.listJoints().map((j, i) => [j.getName(), i]));
  const ibm = skin.getInverseBindMatrices();

  for (const [headId, srcName] of Object.entries(HEADS)) {
    if (headId === id) continue;
    const src = await io.read(join(adventurers, 'Characters/gltf', `${srcName}.glb`));
    const head = src.getRoot().listNodes().find((node) => node.getMesh() && isHead(node));
    const srcSkin = head.getSkin();
    const srcJoints = srcSkin.listJoints().map((j) => j.getName());
    // Mesmo esqueleto: confere se a pose de ligação dos ossos usados bate.
    const srcIbm = srcSkin.getInverseBindMatrices();
    const remap = srcJoints.map((jn, i) => {
      const t = jointIndex.get(jn);
      if (t === undefined) throw new Error(`${srcName}: osso ${jn} não existe em ${name}`);
      const a = srcIbm.getElement(i, []);
      const b = ibm.getElement(t, []);
      if (a.some((v, k) => Math.abs(v - b[k]) > 1e-3)) throw new Error(`${srcName}: pose do osso ${jn} difere de ${name}`);
      return t;
    });
    const mesh = doc.createMesh(`Head_${headId}`);
    for (const p of head.getMesh().listPrimitives()) {
      const prim = doc.createPrimitive().setMode(p.getMode()).setMaterial(material);
      for (const sem of p.listSemantics()) {
        const a = p.getAttribute(sem);
        let arr = a.getArray().slice();
        if (sem.startsWith('JOINTS_')) arr = arr.map((j) => remap[j]);
        prim.setAttribute(sem, doc.createAccessor().setType(a.getType()).setArray(arr).setNormalized(a.getNormalized()).setBuffer(root.listBuffers()[0]));
      }
      const idx = p.getIndices();
      if (idx) prim.setIndices(doc.createAccessor().setType(idx.getType()).setArray(idx.getArray().slice()).setBuffer(root.listBuffers()[0]));
      mesh.addPrimitive(prim);
    }
    const node = doc.createNode(`Head_${headId}`).setMesh(mesh).setSkin(skin);
    parent.addChild(node);
  }
  await doc.transform(prune());
  const dst = join(out, `${id}.glb`);
  await io.write(dst, doc);
  const meshes = root.listNodes().filter((x) => x.getMesh()).map((x) => x.getName());
  report.push(`${id}.glb ${size(dst)} (${n} peças → ${meshes.join(', ')})`);
}

for (const [id, name] of Object.entries(CHARACTERS)) {
  if (HEROES[id]) await buildHero(id, name);
  else await buildCharacter(join(adventurers, 'Characters/gltf', `${name}.glb`), id);
}

for (const [id, name] of Object.entries(MONSTERS)) {
  await buildCharacter(join(skeletons, 'characters/gltf', `${name}.glb`), id);
}

for (const w of MONSTER_WEAPONS) {
  const doc = await io.read(join(skeletons, 'assets/gltf', `${w}.gltf`));
  const dst = join(out, 'weapons', `${w.toLowerCase()}.glb`);
  await io.write(dst, doc);
  report.push(`weapons/${w.toLowerCase()}.glb ${size(dst)}`);
}

for (const [file, keep] of Object.entries(ANIMS)) {
  const doc = await io.read(join(animations, file));
  const root = doc.getRoot();
  for (const a of root.listAnimations()) if (!keep.includes(a.getName())) a.dispose();
  for (const n of root.listNodes()) {
    if (n.getMesh()) n.setMesh(null);
    if (n.getSkin()) n.setSkin(null);
  }
  for (const m of root.listMeshes()) m.dispose();
  for (const s of root.listSkins()) s.dispose();
  for (const t of root.listTextures()) t.dispose();
  for (const m of root.listMaterials()) m.dispose();
  await doc.transform(resample(), dedup(), prune({ keepLeaves: true }));
  const found = root.listAnimations().map((a) => a.getName());
  const missing = keep.filter((k) => !found.includes(k));
  if (missing.length) throw new Error(`${file}: animações não encontradas: ${missing}`);
  const dst = join(out, `anims-${file.replace('Rig_Medium_', '').replace('.glb', '').toLowerCase()}.glb`);
  await io.write(dst, doc);
  report.push(`${dst.split('/').pop()} ${size(dst)} [${found.join(', ')}]`);
}

for (const w of WEAPONS) {
  const doc = await io.read(join(adventurers, 'Assets/gltf', `${w}.gltf`));
  const dst = join(out, 'weapons', `${w}.glb`);
  await io.write(dst, doc);
  report.push(`weapons/${w}.glb ${size(dst)}`);
}

mkdirSync(join(out, 'props'), { recursive: true });
for (const [id, file] of Object.entries(PROPS)) {
  const doc = await io.read(join(hexagon, `${file}.gltf`));
  const dst = join(out, 'props', `${id}.glb`);
  await io.write(dst, doc);
  report.push(`props/${id}.glb ${size(dst)}`);
}

console.log(report.join('\n'));
