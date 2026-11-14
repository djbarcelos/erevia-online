// Gera o layout do mundo (determinístico) a partir dos biomas de scripts/biomes/ e grava em:
//   client/public/world.json          → o cliente posiciona o cenário
//   server/internal/world/world.json  → o servidor monta a colisão
//
// Uso: node scripts/build-world.mjs   (rode build-scenery.mjs antes, para os tamanhos)
// O motor está em world-gen.mjs; cada bioma é um arquivo (README, "Como criar um bioma").
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { buildWorld, OUTPUTS } from './world-gen.mjs';

const { world, json } = await buildWorld();
for (const p of OUTPUTS) {
  mkdirSync(dirname(p), { recursive: true });
  writeFileSync(p, json);
}
const byKind = {};
for (const o of world.objects) byKind[o.k] = (byKind[o.k] ?? 0) + 1;
console.log(`world.json: ${world.objects.length} objetos, ${world.colliders.length} colisores, ${(json.length / 1024).toFixed(0)} KB`);
console.log(Object.entries(byKind).map(([k, n]) => `${k}:${n}`).join(' '));
