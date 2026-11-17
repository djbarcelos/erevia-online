// Kit de natureza do remaster (src/world/nature-map.json, feito por scripts/build-nature-kit.mjs):
// cada item do mundo que o kit substitui tem peça, escala sã e arquivo no jogo, sem navegador.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";
import map from "../src/world/nature-map.json" with { type: "json" };
import sizes from "../src/world/kit-sizes.json" with { type: "json" };

const world = JSON.parse(readFileSync(new URL("../public/world.json", import.meta.url), "utf8")) as { objects: { k: string }[] };
const manifest = JSON.parse(readFileSync(new URL("../scripts/assets-manifest.json", import.meta.url), "utf8")) as { modelos: Record<string, unknown> };

test("toda peça do kit existe em public/models e no manifesto", () => {
  for (const m of map.models) {
    assert.ok(existsSync(new URL(`../public/models/nature_${m}_b.glb`, import.meta.url)), `falta nature_${m}_b.glb`);
    assert.ok(manifest.modelos[`nature_${m}_b.glb`], `nature_${m}_b.glb fora do manifesto`);
  }
});

test("os itens KayKit do mundo que o kit cobre apontam para uma peça existente e têm altura parecida", () => {
  const kinds = map.kinds as Record<string, { model: string; s: number[] }>;
  for (const [kind, e] of Object.entries(kinds)) {
    assert.ok(map.models.includes(e.model), `${kind}: peça ${e.model} desconhecida`);
    assert.ok(e.s.every((v) => v > 0 && Number.isFinite(v)), `${kind}: escala inválida`);
    // As peças do Brejo (tronco, juncos, cabana, bote: build-brejo-kit.mjs) não substituem item do KayKit: o arquivo já tem o
    // tamanho real, em escala 1.
    if (!(sizes as Record<string, number[]>)[kind]) {
      assert.deepEqual(e.s, [1, 1, 1], `${kind}: peça nova do Brejo em escala 1`);
      assert.equal(e.model, kind);
      continue;
    }
    // A peça nova tem altura 1: a escala em y é a altura do item antigo (o vento conta com ela).
    assert.ok(Math.abs(e.s[1] - (sizes as Record<string, number[]>)[kind][1]) < 0.01, `${kind}: altura mudou`);
  }
  // Natureza do mapa que sobra de fora é só o que é desenhado em código (monólitos) ou é cidade.
  const nature = /^(tree_|bush_|grass_|rock_|mountain_|hills_)/;
  const left = new Set(world.objects.map((o) => o.k).filter((k) => nature.test(k) && !kinds[k]));
  assert.deepEqual([...left], [], "natureza sem peça no kit novo");
});
