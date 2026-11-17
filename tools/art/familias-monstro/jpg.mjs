// PNG do laboratório -> JPG (até 1280 px) em docs/arte/familias-monstro/. uso: node tools/art/familias-monstro/jpg.mjs <pasta-png> <pasta-jpg>
import { createRequire } from 'node:module';
import { readdirSync } from 'node:fs';
import { resolve } from 'node:path';
const sharp = createRequire(resolve(import.meta.dirname, '../../../client/') + '/')('sharp');
const [src, dst] = process.argv.slice(2);
for (const f of readdirSync(src).filter((n) => n.endsWith('.png'))) {
  await sharp(resolve(src, f)).resize({ width: 1280, withoutEnlargement: true }).jpeg({ quality: 82 }).toFile(resolve(dst, f.replace('.png', '.jpg')));
}
