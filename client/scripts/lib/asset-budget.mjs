// Orçamento por categoria do guia de arte (docs/guia-de-arte.md, seção 5) e as medições que
// o conferem. Compartilhado por import-asset.mjs (na entrada de um asset novo) e por
// check-assets.mjs (no CI, sobre tudo que já está em public/models). Ajustar junto com o guia.
//
// tris/lod1/lod2: triângulos por LOD (null = a categoria não tem esse LOD).
// texture: maior lado (px). materials: por arquivo. bones: ossos do maior esqueleto.
// ratios: fração de triângulos do LOD0 que o import gera para o LOD1 e o LOD2 (null = não gera),
//   as mesmas proporções da tabela do guia (3000/8000, 1000/8000, ...).
export const BUDGET = {
  personagem: { tris: 8000, lod1: 3000, lod2: 1000, texture: 1024, materials: 2, bones: 60, ratios: [0.375, 0.125] },
  arma: { tris: 800, lod1: 300, lod2: null, texture: 1024, materials: 1, bones: 0, ratios: [0.375, null] },
  monstro: { tris: 4000, lod1: 1500, lod2: 500, texture: 1024, materials: 1, bones: 60, ratios: [0.375, 0.125] },
  chefe: { tris: 15000, lod1: 5000, lod2: 2000, texture: 2048, materials: 3, bones: 60, ratios: [0.333, 0.133] },
  // Peça de cenário (casa, árvore): o limite vale por peça. Um kit com várias peças num
  // arquivo (env-town.glb) usa "porPeca" no manifesto e é medido pela maior peça.
  cenario: { tris: 3000, lod1: 1000, lod2: null, texture: 2048, materials: 1, bones: 0, ratios: [0.333, null] },
};

export const CATEGORIES = Object.keys(BUDGET);
// Categorias sem malha própria (só animações): conferidas só por existirem no manifesto.
export const SEM_MALHA = ['animacao'];

const lodOf = (name) => Number(/_LOD(\d)$/i.exec(name ?? '')?.[1] ?? 0);

/**
 * Mede um documento do gltf-transform.
 * variants: regex (texto) de nomes de nós que são variantes que o jogo esconde (cabeças e
 * chapéus alternativos). Cada regex é um grupo e conta só a maior variante, pois só uma aparece.
 * perPiece: o arquivo é um kit; os triângulos são os da maior peça, não a soma.
 */
export function measure(doc, { variants = [], perPiece = false } = {}) {
  const root = doc.getRoot();
  const groups = variants.map((v) => ({ re: new RegExp(v), max: 0 }));
  const sum = { 0: 0, 1: 0, 2: 0 };
  const piece = { 0: 0, 1: 0, 2: 0 };
  for (const node of root.listNodes()) {
    const mesh = node.getMesh();
    if (!mesh) continue;
    let n = 0;
    for (const prim of mesh.listPrimitives()) {
      const idx = prim.getIndices();
      n += Math.floor((idx ? idx.getCount() : (prim.getAttribute('POSITION')?.getCount() ?? 0)) / 3);
    }
    const lod = lodOf(node.getName());
    const group = groups.find((g) => g.re.test(node.getName().replace(/_LOD\d$/i, '')));
    if (group && lod === 0) group.max = Math.max(group.max, n);
    else {
      sum[lod] += n;
      piece[lod] = Math.max(piece[lod], n);
    }
  }
  const variantMax = groups.reduce((s, g) => s + g.max, 0);
  const tris = perPiece ? { ...piece, 0: Math.max(piece[0], ...groups.map((g) => g.max)) } : { ...sum, 0: sum[0] + variantMax };
  const sizes = root.listTextures().map((t) => Math.max(...(t.getSize() ?? [0, 0])));
  const joints = root.listSkins().map((s) => s.listJoints().length);
  return {
    tris,
    materials: root.listMaterials().length,
    texture: Math.max(0, ...sizes),
    bones: Math.max(0, ...joints),
  };
}

/**
 * Confere as medições contra o orçamento da categoria. `exceptions` é o mapa
 * { metrica: { max, motivo } } do manifesto: a métrica passa até esse teto (que trava o
 * número de hoje: crescer de novo falha). Métricas: tris, lod1, lod2, texture, materials, bones.
 * Retorna { problems, waived, stale } (listas de texto).
 */
export function evaluate(stats, category, exceptions = {}) {
  const b = BUDGET[category];
  const checks = [
    ['tris', stats.tris[0], b.tris, 'triângulos (LOD0)'],
    ['lod1', stats.tris[1], b.lod1, 'triângulos do LOD1'],
    ['lod2', stats.tris[2], b.lod2, 'triângulos do LOD2'],
    ['texture', stats.texture, b.texture, 'px de textura'],
    ['materials', stats.materials, b.materials, 'materiais'],
    ['bones', stats.bones, b.bones, 'ossos'],
  ];
  const problems = [];
  const waived = [];
  const used = new Set();
  for (const [key, value, limit, label] of checks) {
    if (limit === null || limit === undefined) {
      if (key.startsWith('lod') && value > 0) problems.push(`${label}: ${value}, mas a categoria ${category} não tem esse LOD`);
      continue;
    }
    if (value <= limit) continue;
    const ex = exceptions[key];
    if (ex && value <= ex.max) {
      used.add(key);
      waived.push(`${label}: ${value} (máximo ${limit}; exceção até ${ex.max}: ${ex.motivo})`);
    } else {
      problems.push(`${label}: ${value} (máximo ${limit}${ex ? `; a exceção cobre até ${ex.max}` : ''})`);
    }
  }
  // Exceção que já não é necessária deve sair do manifesto (a lista só encolhe).
  const stale = Object.keys(exceptions)
    .filter((k) => !used.has(k) && !problems.some((p) => p.startsWith(checks.find((c) => c[0] === k)?.[3] ?? '\0')))
    .map((k) => `exceção de ${k} sem uso: remova do manifesto`);
  return { problems, waived, stale };
}
