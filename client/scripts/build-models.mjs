// Gera client/public/models/ a partir dos pacotes KayKit (CC0) descompactados.
//
// Uso: node scripts/build-models.mjs <pasta-com-os-zips-descompactados>
//   ex.: node scripts/build-models.mjs ../assets-src/kaykit/x
//
// - Personagens: copiados como estão (.glb, textura embutida).
// - Animações: só as que o jogo usa, sem as malhas do manequim.
// - Armas: .gltf + .bin + .png viram um .glb cada.
import { NodeIO } from '@gltf-transform/core';
import { dedup, prune } from '@gltf-transform/functions';
import { copyFileSync, mkdirSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';

const src = resolve(process.argv[2] ?? '../assets-src/kaykit/x');
const out = resolve(import.meta.dirname, '../public/models');
const adventurers = join(src, 'KayKit_Adventurers_2.0_FREE');
const animations = join(src, 'KayKit_Character_Animations_1.1/Animations/gltf/Rig_Medium');

const CHARACTERS = { knight: 'Knight', barbarian: 'Barbarian', mage: 'Mage', rogue: 'Rogue' };

// arquivo de origem → animações mantidas
const ANIMS = {
  'Rig_Medium_General.glb': ['Idle_A', 'Hit_A', 'Death_A'],
  'Rig_Medium_MovementBasic.glb': ['Running_A', 'Walking_A'],
};

const WEAPONS = ['sword_1handed', 'shield_badge_color', 'axe_1handed', 'shield_round_barbarian', 'staff', 'dagger'];

const io = new NodeIO();
mkdirSync(join(out, 'weapons'), { recursive: true });
const report = [];
const size = (f) => `${(statSync(f).size / 1024).toFixed(0)} KB`;

for (const [id, name] of Object.entries(CHARACTERS)) {
  const dst = join(out, `${id}.glb`);
  copyFileSync(join(adventurers, 'Characters/gltf', `${name}.glb`), dst);
  report.push(`${id}.glb ${size(dst)}`);
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
  await doc.transform(dedup(), prune({ keepLeaves: true }));
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

console.log(report.join('\n'));
