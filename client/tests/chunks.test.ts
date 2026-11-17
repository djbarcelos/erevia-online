// Recortes do cenário do remaster (src/world/chunks.ts): a divisão por célula e o corte do mapa de sombras,
// sem navegador (a luz, o gerador e as malhas são falsos, só com o que o código lê).
import assert from "node:assert/strict";
import { test } from "node:test";
import { CHUNK, chunkKey, ShadowCuller, splitByChunk } from "../src/world/chunks.ts";

test("splitByChunk: cada item vai para a célula da sua posição e a ordem se mantém", () => {
  const items = [
    { x: 1, z: 1, id: "a" },
    { x: CHUNK + 1, z: 1, id: "b" },
    { x: 2, z: 2, id: "c" },
    { x: -1, z: -1, id: "d" },
  ];
  const g = splitByChunk(items);
  assert.equal(g.size, 3);
  assert.deepEqual(g.get(chunkKey(1, 1))!.map((i) => i.id), ["a", "c"]);
  assert.deepEqual(g.get(chunkKey(CHUNK + 1, 1))!.map((i) => i.id), ["b"]);
  assert.deepEqual(g.get(chunkKey(-1, -1))!.map((i) => i.id), ["d"]);
  assert.equal([...g.values()].reduce((n, l) => n + l.length, 0), items.length);
});

type FakeMesh = { name: string; getBoundingInfo(): { boundingSphere: { centerWorld: { x: number; y: number; z: number }; radiusWorld: number } } };
const mesh = (name: string, x: number, y: number, z: number, r = 5): FakeMesh => ({ name, getBoundingInfo: () => ({ boundingSphere: { centerWorld: { x, y, z }, radiusWorld: r } }) });

function rig(direction = { x: 0, y: -1, z: 0 }) {
  const renderList: FakeMesh[] = [];
  const pos = { x: 0, y: 100, z: 0 };
  const light = { getAbsolutePosition: () => pos, getShadowDirection: () => direction, shadowFrustumSize: 70, shadowMaxZ: 200 };
  const shadows = { addShadowCaster: (m: FakeMesh) => renderList.push(m), getShadowMap: () => ({ renderList }) };
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const culler = new ShadowCuller(shadows as any, light as any);
  return { culler, renderList, pos };
}

test("ShadowCuller: fica na lista só o que cabe no volume da luz, e volta quando a luz se aproxima", () => {
  const { culler, renderList, pos } = rig();
  const near = mesh("perto", 10, 0, 10);
  const farSide = mesh("lado", 200, 0, 0);
  const beyond = mesh("fundo", 0, -200, 0); // além do plano de trás (profundidade 300 > 200)
  const front = mesh("frente", 0, 150, 0); // antes do plano de perto: o ShadowGenerator prende a profundidade, ainda projeta
  for (const m of [near, farSide, beyond, front]) culler.add(m as never);
  assert.equal(renderList.length, 4);

  culler.update();
  assert.deepEqual(renderList.map((m) => m.name).sort(), ["frente", "perto"]);

  // A luz segue o jogador: o recorte do lado entra; o de perto e o da frente (agora a 200 m para o lado) saem.
  pos.x = 200;
  culler.update();
  assert.deepEqual(renderList.map((m) => m.name), ["lado"]);

  // Sem mudança de estado, nada é empurrado em dobro.
  culler.update();
  assert.equal(renderList.length, 1);
});

test("ShadowCuller: sol baixo e inclinado enxerga recortes que estão na direção da luz", () => {
  // A luz olha para +x e para baixo (-y): os recortes no chão a 100 m dela, ao longo do raio, projetam no volume.
  const { culler, renderList } = rig({ x: 0.8, y: -0.6, z: 0 });
  const along = mesh("raio", 80, 0, 0); // 80 m para +x e 100 m abaixo: na linha da luz
  const aside = mesh("fora", 0, 0, 120); // 120 m para o lado
  culler.add(along as never);
  culler.add(aside as never);
  culler.update();
  assert.deepEqual(renderList.map((m) => m.name), ["raio"]);
});
