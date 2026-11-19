// Corta a folha 5x5 de ícones de uma classe (prompts.mjs) em client/public/icons/skills/<fx>.webp (128 px).
// A grade ocupa a imagem toda, com linhas pretas finas entre as células: cada célula perde uma margem de
// ~3% de cada lado para a linha não aparecer na borda do ícone.
//   cd client && node scripts/slice-skill-icons.mjs <folha.png> <classe>
import { mkdirSync, readdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import sharp from 'sharp';
import { SHEETS } from '../../tools/art/icones-skill/prompts.mjs';

const [sheet, cls] = process.argv.slice(2);
const list = SHEETS[cls];
if (!list) throw new Error(`classe desconhecida: ${cls}`);
const out = resolve(import.meta.dirname, '../public/icons/skills');
mkdirSync(out, { recursive: true });
const img = sharp(sheet);
const { width, height } = await img.metadata();
const cw = width / 5, ch = height / 5;
const inset = Math.round(cw * 0.03);
for (const [i, [fx]] of list.entries()) {
  const x = Math.round((i % 5) * cw) + inset, y = Math.round(Math.floor(i / 5) * ch) + inset;
  const w = Math.round(cw) - inset * 2, h = Math.round(ch) - inset * 2;
  await sharp(sheet).extract({ left: x, top: y, width: w, height: h }).resize(128, 128).webp({ quality: 82 }).toFile(resolve(out, `${fx}.webp`));
}
console.log(`${cls}: ${list.length} ícones em ${out}`);
// Lista dos ícones em imagem (todas as classes já cortadas), para o skillIcon do cliente.
const all = readdirSync(out).filter((f) => f.endsWith('.webp')).map((f) => f.slice(0, -5)).sort();
writeFileSync(resolve(import.meta.dirname, '../src/ui/skill-images.ts'), `// Gerado por client/scripts/slice-skill-icons.mjs (não editar à mão): skills com ícone em imagem
// (public/icons/skills/<fx>.webp, folhas do Meshy, tunnel#181). As outras usam o SVG de icons.ts.
export const SKILL_IMAGES: ReadonlySet<string> = new Set(${JSON.stringify(all)});
`);
