// Torna uma imagem repetível (seamless): desloca meia imagem e mistura com a original por uma máscara
// que vale 1 no centro e 0 nas bordas (onde ficaria a costura). Mede a costura antes e depois (diferença
// média entre a última e a primeira coluna/linha, contra a diferença entre colunas vizinhas).
// Uso (dentro de client/): node scripts/make-seamless.mjs <entrada.png> <saida.jpg> [tamanho=512] [saturacao=1]
import sharp from 'sharp';

const [inp, out, sizeArg = '512', satArg = '1'] = process.argv.slice(2);
const N = Number(sizeArg);
const { data } = await sharp(inp).resize(N, N).modulate({ saturation: Number(satArg) }).removeAlpha().raw().toBuffer({ resolveWithObject: true });
const at = (b, x, y, c) => b[(y * N + x) * 3 + c];

/** Costura: diferença média entre as bordas opostas dividida pela diferença média entre vizinhos. */
function seam(b) {
  let edge = 0, near = 0, n = 0;
  for (let i = 0; i < N; i++) for (let c = 0; c < 3; c++) {
    edge += Math.abs(at(b, 0, i, c) - at(b, N - 1, i, c)) + Math.abs(at(b, i, 0, c) - at(b, i, N - 1, c));
    near += Math.abs(at(b, N / 2, i, c) - at(b, N / 2 - 1, i, c)) + Math.abs(at(b, i, N / 2, c) - at(b, i, N / 2 - 1, c));
    n += 2;
  }
  return { edge: edge / n, near: near / n, ratio: edge / near };
}

const smooth = (t) => t * t * (3 - 2 * t);
const tent = (i) => smooth(1 - Math.abs((2 * (i + 0.5)) / N - 1));
const res = Buffer.alloc(N * N * 3);
const H = N / 2;
for (let y = 0; y < N; y++) for (let x = 0; x < N; x++) {
  const wm = Math.min(tent(x), tent(y));
  const k = wm;
  for (let c = 0; c < 3; c++) res[(y * N + x) * 3 + c] = Math.round(at(data, x, y, c) * k + at(data, (x + H) % N, (y + H) % N, c) * (1 - k));
}
const before = seam(data);
const after = seam(res);
await sharp(res, { raw: { width: N, height: N, channels: 3 } }).jpeg({ quality: 86, mozjpeg: true }).toFile(out);
console.log(`${out}: costura antes ${before.ratio.toFixed(2)} (borda ${before.edge.toFixed(1)}), depois ${after.ratio.toFixed(2)} (borda ${after.edge.toFixed(1)}); 1,0 = sem costura`);
