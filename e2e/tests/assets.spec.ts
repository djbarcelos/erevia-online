import { execFileSync } from "node:child_process";
import { createServer, type Server } from "node:http";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect } from "@playwright/test";

/**
 * O Babylon do jogo carrega o que o pipeline de arte gera? Pega um GLB feito pelo mesmo código
 * do import-asset (LOD1/LOD2 por simplify e textura KTX2/Basis), serve de um servidor local
 * junto com o decodificador de KTX2 (client/public/ktx2) e carrega com src/world/assets.ts num
 * navegador de verdade (WebGL por software). Não precisa do servidor do jogo.
 *
 * Confere: o KTX2 decodifica (textura pronta, com o tamanho certo), os nós _LOD1/_LOD2 viram LOD
 * do Babylon (a malha troca pela distância) e a cena desenha o modelo sem erro de JavaScript.
 */

const CLIENT = fileURLToPath(new URL("../../client/", import.meta.url));
const MIME: Record<string, string> = { ".js": "text/javascript", ".wasm": "application/wasm", ".html": "text/html", ".glb": "model/gltf-binary" };

let dir = "";
let server: Server;
let origin = "";

base.beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), "erevia-asset-page-"));
  // Pagina de teste (Babylon empacotado) + decodificador; o GLB sai do mesmo codigo do import.
  execFileSync(process.execPath, [join(CLIENT, "scripts/build-ktx2-decoder.mjs")], { cwd: CLIENT, stdio: "pipe" });
  execFileSync(process.execPath, [join(CLIENT, "tests/assets/build-page.mjs"), dir], { cwd: CLIENT, stdio: "pipe" });
  const glb = execFileSync(process.execPath, ["--input-type=module", "-e", GENERATE_GLB], { cwd: CLIENT, maxBuffer: 64 << 20, env: { ...process.env, OUT: join(dir, "corpo.glb") } });
  expect(glb.toString()).toContain("ok");
  server = createServer((req, res) => {
    const path = normalize(decodeURIComponent((req.url ?? "/").split("?")[0]!)).replace(/^([/\\])+/, "") || "index.html";
    try {
      const body = readFileSync(join(dir, path));
      res.writeHead(200, { "content-type": MIME[extname(path)] ?? "application/octet-stream" }).end(body);
    } catch {
      res.writeHead(404).end();
    }
  });
  await new Promise<void>((ok) => server.listen(0, "127.0.0.1", ok));
  const addr = server.address();
  origin = `http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}`;
});

base.afterAll(async () => {
  await new Promise((ok) => server?.close(ok));
  if (dir) rmSync(dir, { recursive: true, force: true });
});

// Um GLB de 3.600 triangulos com textura 64x64, passado por generateLods + compressTextures.
const GENERATE_GLB = `
import { NodeIO } from '@gltf-transform/core';
import { KHRTextureBasisu } from '@gltf-transform/extensions';
import { writeFileSync } from 'node:fs';
import { denseDoc } from './tests/assets/helpers.mjs';
import { compressTextures, generateLods } from './scripts/lib/asset-optimize.mjs';
const io = new NodeIO().registerExtensions([KHRTextureBasisu]);
const doc = denseDoc(42);
const lod = await generateLods(doc, 'monstro');
const ktx = await compressTextures(doc);
writeFileSync(process.env.OUT, await io.writeBinary(doc));
console.log('ok', JSON.stringify({ lod, ktx }));
`;

base("o Babylon carrega LOD e KTX2 do pipeline de arte", async ({ page }) => {
  base.setTimeout(120_000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(`console.error: ${m.text()}`);
  });
  await page.goto(origin + "/");
  const r = (await page.evaluate((glb) => window.runAssetLoad(glb), `${origin}/corpo.glb`)) as {
    meshes: string[];
    lods: number;
    textureReady: boolean;
    textureSize: [number, number] | null;
    lodAt: { perto: string | null; medio: string | null; longe: string | null };
    centerPixel: number[];
  };
  console.log(JSON.stringify(r));
  expect(errors).toEqual([]);
  expect(r.meshes.sort()).toEqual(["Corpo", "Corpo_LOD1", "Corpo_LOD2"]);
  expect(r.lods).toBe(2);
  expect(r.textureReady).toBe(true);
  expect(r.textureSize).toEqual([64, 64]);
  // LOD do guia: LOD1 a partir de 20 m e LOD2 a partir de 35 m.
  expect(r.lodAt).toEqual({ perto: "Corpo", medio: "Corpo_LOD1", longe: "Corpo_LOD2" });
  // Desenhou o modelo com a textura no centro da tela (nao e o fundo preto).
  expect(r.centerPixel.slice(0, 3).some((c) => c > 0)).toBe(true);
});

declare global {
  interface Window {
    runAssetLoad: (glb: string) => Promise<unknown>;
  }
}
