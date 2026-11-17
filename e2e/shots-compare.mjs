/**
 * Compara os prints do cenário (?shots=1) de dois servidores com a GPU de verdade: tira as cenas fixas de
 * cada um e conta os pixels que diferem (e grava os dois PNG e um de diferença). Serve para provar que uma
 * otimização não muda o visual (docs/desempenho-remaster.md).
 *
 *   node shots-compare.mjs --a http://localhost:8097 --b http://localhost:8098 --out ../.bench/prints "quality=media"
 *
 * O vento mexe nas árvores e na grama com o tempo, então sobra uma diferença pequena nas copas mesmo sem mudança: olhe o
 * PNG de diferença, não só o número.
 */
import { chromium } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";

const argv = process.argv.slice(2);
const opt = { a: "http://localhost:8097", b: "http://localhost:8098", out: "shots-out" };
const queries = [];
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === "--a") opt.a = argv[++i];
  else if (argv[i] === "--b") opt.b = argv[++i];
  else if (argv[i] === "--out") opt.out = argv[++i];
  else queries.push(argv[i]);
}
mkdirSync(opt.out, { recursive: true });
const browser = await chromium.launch({ args: ["--use-angle=d3d11", "--enable-gpu", "--ignore-gpu-blocklist", "--force-high-performance-gpu"] });

async function shoot(base, query, tag) {
  const ctx = await browser.newContext({ baseURL: base, viewport: { width: 1280, height: 720 }, locale: "pt-BR" });
  await ctx.addInitScript(() => {
    try {
      const q = new URLSearchParams(location.search).get("quality") ?? "media";
      localStorage.setItem("erevia.settings", JSON.stringify({ quality: q, qualityPicked: true }));
    } catch {}
  });
  const page = await ctx.newPage();
  const user = `sh${Date.now().toString(36)}${Math.random().toString(36).slice(2, 4)}`;
  await page.request.post("/api/auth/dev", { data: { username: user } });
  const creation = await (await page.request.get("/api/creation")).json();
  const k = creation.classes.knight;
  await page.request.post("/api/characters", { data: { name: `S${user.slice(2, 11)}`, class: "knight", look: { weapon: k.weapons[0].id, offhand: k.offhands[0].id, emblem: k.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
  await page.goto(`/?shots=1&${query}`);
  await page.getByRole("button", { name: "Entrar no mundo" }).click();
  await page.waitForFunction(() => window.__ereviaShots?.ready === true, null, { timeout: 180_000 });
  await page.evaluate(() => {
    for (const id of ["hud", "overlay"]) document.getElementById(id)?.style.setProperty("display", "none", "important");
  });
  const names = await page.evaluate(() => window.__ereviaShots.names);
  const out = {};
  for (const n of names) {
    await page.evaluate((x) => window.__ereviaShots.show(x), n);
    out[n] = (await page.locator("#game").screenshot({ animations: "disabled" })).toString("base64");
    writeFileSync(`${opt.out}/${tag}-${n}.png`, Buffer.from(out[n], "base64"));
  }
  await ctx.close();
  return out;
}

const cmp = await browser.newPage();
for (const q of queries) {
  const A = await shoot(opt.a, q, "a");
  const B = await shoot(opt.b, q, "b");
  for (const n of Object.keys(A)) {
    const r = await cmp.evaluate(async ([a, b]) => {
      const load = (s) => new Promise((ok) => { const i = new Image(); i.onload = () => ok(i); i.src = "data:image/png;base64," + s; });
      const [ia, ib] = await Promise.all([load(a), load(b)]);
      const c = document.createElement("canvas"); c.width = ia.width; c.height = ia.height;
      const g = c.getContext("2d", { willReadFrequently: true });
      g.drawImage(ia, 0, 0); const da = g.getImageData(0, 0, c.width, c.height);
      g.drawImage(ib, 0, 0); const db = g.getImageData(0, 0, c.width, c.height);
      const diff = g.createImageData(c.width, c.height);
      let n = 0, strong = 0;
      for (let i = 0; i < da.data.length; i += 4) {
        const d = Math.max(Math.abs(da.data[i] - db.data[i]), Math.abs(da.data[i + 1] - db.data[i + 1]), Math.abs(da.data[i + 2] - db.data[i + 2]));
        if (d > 8) n++;
        if (d > 40) strong++;
        diff.data[i] = Math.min(255, d * 6); diff.data[i + 1] = Math.min(255, d * 6); diff.data[i + 2] = Math.min(255, d * 6); diff.data[i + 3] = 255;
      }
      g.putImageData(diff, 0, 0);
      return { total: da.data.length / 4, diff8: n, diff40: strong, png: c.toDataURL("image/png").split(",")[1] };
    }, [A[n], B[n]]);
    writeFileSync(`${opt.out}/diff-${n}.png`, Buffer.from(r.png, "base64"));
    console.log(`${q} / ${n}: ${((100 * r.diff8) / r.total).toFixed(3)}% dos pixels diferem (>8), ${((100 * r.diff40) / r.total).toFixed(3)}% (>40)`);
  }
}
await browser.close();
