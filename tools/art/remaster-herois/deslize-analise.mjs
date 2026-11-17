// Resume um registro de deslize (deslize.mjs): node tools/art/remaster-herois/deslize-analise.mjs <registro.json> [...]
import { readFileSync } from 'node:fs';
for (const f of process.argv.slice(2)) {
  const d = JSON.parse(readFileSync(f, 'utf8'));
  for (const src of ['self', 'remote']) {
    const rows = d[src];
    if (!rows.length) continue;
    // velocidade desenhada recalculada pela posição (não depende do campo vis, que só existe na regra nova)
    const sp = rows.map((r, i) => (i ? Math.hypot(r.x - rows[i - 1].x, r.z - rows[i - 1].z) / Math.max(1e-3, (r.t - rows[i - 1].t) / 1000) : 0));
    // suaviza em 5 quadros para tirar o tremido do quadro a quadro
    const sm = sp.map((_, i) => { const a = sp.slice(Math.max(0, i - 4), i + 1); return a.reduce((x, y) => x + y, 0) / a.length; });
    const dt = rows.map((r, i) => (i ? (r.t - rows[i - 1].t) / 1000 : 0));
    let slideIdle = 0, runInPlace = 0, footRatioSum = 0, footN = 0, maxFootRatio = 0, minFootRatio = 9;
    for (let i = 1; i < rows.length; i++) {
      const r = rows[i];
      const loco = (r.run ?? 0) + (r.walk ?? 0);
      if (sm[i] > 0.5 && (r.idle ?? 0) > 0.5) slideIdle += dt[i];
      if (sm[i] < 0.1 && loco > 0.3) runInPlace += dt[i];
      if (sm[i] > 1.5 && (r.run ?? 0) > 0.9 && r.runNat) {
        // pé de apoio contra o chão: velocidade do pé = natural x velocidade do clipe
        const ratio = (r.runNat * r.runRatio) / sm[i];
        footRatioSum += ratio; footN++;
        maxFootRatio = Math.max(maxFootRatio, ratio); minFootRatio = Math.min(minFootRatio, ratio);
      }
    }
    const peak = Math.max(...sm);
    const first = (pred) => rows.find((r, i) => pred(r, i));
    const t0 = first((_, i) => sm[i] > 0.5)?.t;
    const tRun = first((r) => (r.run ?? 0) + (r.walk ?? 0) > 0.7 && (!t0 || r.t >= t0))?.t;
    console.log(`${d.query || 'kaykit'} ${d.cls} ${src}: ${rows.length} quadros, pico ${peak.toFixed(2)} m/s;`
      + ` desliza tocando o parado ${slideIdle.toFixed(2)} s; corre no lugar ${runInPlace.toFixed(2)} s;`
      + ` pé/chão ${footN ? (footRatioSum / footN).toFixed(2) : '-'} (min ${footN ? minFootRatio.toFixed(2) : '-'}, max ${footN ? maxFootRatio.toFixed(2) : '-'}; 1,00 = sem deslizar);`
      + ` atraso do clipe ao começar ${t0 !== undefined && tRun !== undefined ? tRun - t0 : '-'} ms`);
  }
}
