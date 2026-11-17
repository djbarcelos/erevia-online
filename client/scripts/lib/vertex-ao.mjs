// AO por vértice, assado em JavaScript (sem Blender): para cada vértice, lança raios numa semiesfera em
// volta da normal e vê quantos batem na própria malha (ou no chão, y = 0) a menos de `maxDist`.
// Serve às peças da cidade do remaster (tunnel#100): o valor vai em COLOR_0 e o jogo multiplica pela cor
// base, então funciona com thin instances (a geometria é a mesma em todas as cópias) e sem segunda textura.
// Determinístico (amostras de Fibonacci, sem sorteio). Laços sempre limitados pelo tamanho da malha.

/** Direções de uma semiesfera com peso de cosseno (+Z é a normal). */
function hemisphere(n) {
  const out = [];
  const golden = Math.PI * (3 - Math.sqrt(5));
  for (let i = 0; i < n; i++) {
    const u = (i + 0.5) / n;
    const r = Math.sqrt(u); // cosseno ponderado
    const a = i * golden;
    out.push([r * Math.cos(a), r * Math.sin(a), Math.sqrt(1 - u)]);
  }
  return out;
}

/**
 * @param {Float32Array|number[]} pos  xyz por vértice
 * @param {Float32Array|number[]} nor  xyz por vértice (normalizadas)
 * @param {Uint32Array|number[]} idx   triângulos
 * @returns {Float32Array} AO por vértice, de 0 (fechado) a 1 (aberto)
 */
export function bakeVertexAO(pos, nor, idx, { samples = 48, maxDist = 1, eps = 1e-4, ground = true } = {}) {
  const nv = pos.length / 3;
  const nt = idx.length / 3;
  // triângulos pré-processados: v0, e1, e2 e caixa
  const tri = new Float32Array(nt * 15);
  for (let t = 0; t < nt; t++) {
    const a = idx[t * 3] * 3, b = idx[t * 3 + 1] * 3, c = idx[t * 3 + 2] * 3;
    const o = t * 15;
    for (let k = 0; k < 3; k++) {
      tri[o + k] = pos[a + k];
      tri[o + 3 + k] = pos[b + k] - pos[a + k];
      tri[o + 6 + k] = pos[c + k] - pos[a + k];
      tri[o + 9 + k] = Math.min(pos[a + k], pos[b + k], pos[c + k]);
      tri[o + 12 + k] = Math.max(pos[a + k], pos[b + k], pos[c + k]);
    }
  }
  const dirs = hemisphere(samples);
  const ao = new Float32Array(nv);
  for (let v = 0; v < nv; v++) {
    const nx = nor[v * 3], ny = nor[v * 3 + 1], nz = nor[v * 3 + 2];
    // base ortonormal (t, b, n)
    const ax = Math.abs(nx) > 0.9 ? 0 : 1, ay = Math.abs(nx) > 0.9 ? 1 : 0;
    let tx = ay * nz, ty = -ax * nz, tz = ax * ny - ay * nx;
    const tl = Math.hypot(tx, ty, tz) || 1;
    tx /= tl; ty /= tl; tz /= tl;
    const bx = ny * tz - nz * ty, by = nz * tx - nx * tz, bz = nx * ty - ny * tx;
    const ox = pos[v * 3] + nx * eps * 10, oy = pos[v * 3 + 1] + ny * eps * 10, oz = pos[v * 3 + 2] + nz * eps * 10;
    let hits = 0;
    for (const d of dirs) {
      const dx = tx * d[0] + bx * d[1] + nx * d[2];
      const dy = ty * d[0] + by * d[1] + ny * d[2];
      const dz = tz * d[0] + bz * d[1] + nz * d[2];
      if (ground && dy < -1e-6 && oy / -dy < maxDist && oy > -1e-3) { hits++; continue; }
      // caixa do raio, para pular triângulos longe
      const ex = ox + dx * maxDist, ey = oy + dy * maxDist, ez = oz + dz * maxDist;
      const mnx = Math.min(ox, ex), mxx = Math.max(ox, ex), mny = Math.min(oy, ey), mxy = Math.max(oy, ey), mnz = Math.min(oz, ez), mxz = Math.max(oz, ez);
      let hit = false;
      for (let t = 0; t < nt && !hit; t++) {
        const o = t * 15;
        if (tri[o + 12] < mnx || tri[o + 9] > mxx || tri[o + 13] < mny || tri[o + 10] > mxy || tri[o + 14] < mnz || tri[o + 11] > mxz) continue;
        // Möller–Trumbore, dos dois lados
        const e1x = tri[o + 3], e1y = tri[o + 4], e1z = tri[o + 5], e2x = tri[o + 6], e2y = tri[o + 7], e2z = tri[o + 8];
        const px = dy * e2z - dz * e2y, py = dz * e2x - dx * e2z, pz = dx * e2y - dy * e2x;
        const det = e1x * px + e1y * py + e1z * pz;
        if (Math.abs(det) < 1e-12) continue;
        const inv = 1 / det;
        const sx = ox - tri[o], sy = oy - tri[o + 1], sz = oz - tri[o + 2];
        const u = (sx * px + sy * py + sz * pz) * inv;
        if (u < 0 || u > 1) continue;
        const qx = sy * e1z - sz * e1y, qy = sz * e1x - sx * e1z, qz = sx * e1y - sy * e1x;
        const w = (dx * qx + dy * qy + dz * qz) * inv;
        if (w < 0 || u + w > 1) continue;
        const tt = (e2x * qx + e2y * qy + e2z * qz) * inv;
        if (tt > eps && tt < maxDist) hit = true;
      }
      if (hit) hits++;
    }
    ao[v] = 1 - hits / samples;
  }
  return ao;
}
