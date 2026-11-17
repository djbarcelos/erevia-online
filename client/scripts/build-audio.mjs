// Gera o áudio do jogo a partir dos pacotes CC0 baixados (lista e links em public/audio/CREDITS.md):
//   public/audio/sfx.mp3          todos os efeitos num arquivo só (mono, 64 kbps), com 0,25 s de
//                                  silêncio entre eles
//   src/audio/sprite.json         onde cada efeito começa e quanto dura (s)
//   public/audio/music/<nome>.mp3 música de cada região (mono, 64 kbps), carregada sob demanda
//   public/audio/amb/<nome>.mp3   fundo ambiente (mono, 32 kHz, 48 kbps), carregado sob demanda
//
// Os efeitos são cortados (silêncio no começo e no fim), limitados em duração e nivelados
// pelo volume médio, para o catálogo (src/audio/index.ts) só precisar de ajustes finos.
// Usa o ffmpeg do PATH ou, se não houver, a imagem Docker jrottenberg/ffmpeg.
//
// Uso: node scripts/build-audio.mjs C:/caminho/da/pasta-com-os-pacotes
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';

const src = resolve(process.argv[2] ?? '../assets-src/audio');
const outDir = resolve(import.meta.dirname, '../public/audio');
const spriteJson = resolve(import.meta.dirname, '../src/audio/sprite.json');
const tmp = join(src, '.build');
mkdirSync(tmp, { recursive: true });
mkdirSync(join(outDir, 'music'), { recursive: true });
mkdirSync(join(outDir, 'amb'), { recursive: true });
mkdirSync(dirname(spriteJson), { recursive: true });

const RATE = 44100;
const GAP = 0.25;

// Pastas dos pacotes (como saem dos zips).
const K = 'x/RPGsounds_Kenney/OGG'; // Kenney, RPG Audio
const KI = 'x/kenney_interfaceSounds/Audio'; // Kenney, Interface Sounds
const AD = 'x/rpg_sound_pack/RPG Sound Pack'; // artisticdude, RPG Sound Pack
const RD = 'x/80-CC0-RPG-SFX_0'; // rubberduck, 80 CC0 RPG SFX
const C = 'x/150gameeffectsounds'; // untamedpixels, 150 Combat Sound Effects (Kenney Impact + lentikula)
const BR = 'x/bones_rattle'; // congusbongus, Bones rattle
const WB = 'x/SoundPack01'; // wobbleboxx, Level up / power up
const spell = (n) => `${C}/04_magic_spells/${n}.wav`;
const punch = (w, i) => {
  const files = {
    'medium-0': 'impact_punch_medium_000_48kHz_16bit_0.36s_bd322da3_s02',
    'medium-1': 'impact_punch_medium_001_48kHz_16bit_0.35s_bfeb7339_s02',
    'medium-2': 'impact_punch_medium_002_48kHz_16bit_0.42s_212c5895_s02',
    'heavy-0': 'impact_punch_heavy_000_48kHz_16bit_0.65s_8fb58ba7_s02',
    'heavy-1': 'impact_punch_heavy_001_48kHz_16bit_0.54s_fb104b0a_s02',
    'heavy-2': 'impact_punch_heavy_002_48kHz_16bit_0.46s_565485a1_s02',
    'heavy-3': 'impact_punch_heavy_003_48kHz_16bit_0.47s_2546df80_s02',
  };
  return `${C}/01_combat_melee/${files[`${w}-${i}`]}.wav`;
};

/** Efeitos: nome no sprite, arquivo, duração máxima (s) e ganho extra (dB). */
const SFX = [
  // Golpes
  ['hit1', punch('medium', 0)],
  ['hit2', punch('medium', 1)],
  ['hit3', punch('medium', 2)],
  ['crit1', punch('heavy', 0)],
  ['crit2', punch('heavy', 1)],
  ['bash', punch('heavy', 2)],
  ['heavy', punch('heavy', 3)],
  ['chop', `${K}/chop.ogg`],
  ['slice1', `${K}/knifeSlice.ogg`],
  ['slice2', `${K}/knifeSlice2.ogg`],
  ['miss1', `${AD}/battle/swing.wav`],
  ['miss2', `${AD}/battle/swing2.wav`],
  ['miss3', `${AD}/battle/swing3.wav`],
  ['arcane', `${AD}/battle/magic1.wav`, 0.8],
  // Skills
  ['spell', `${RD}/spell_01.ogg`],
  ['bolt', `${RD}/spell_02.ogg`],
  ['fireCast', `${RD}/spell_fire_01.ogg`],
  ['fireHit', spell('Fire Spell Impact 1_s06'), 1.3],
  ['fireHit2', spell('Fire Spell Impact 2_s06'), 1.3],
  ['boom', `${C}/03_explosions_destruction/explosion_crunch_000_48kHz_16bit_0.78s_e875be4e_s04.wav`],
  ['ice', spell('Ice Spell Impact 1_s06'), 1.4],
  ['ice2', spell('Ice Spell Impact 2_s06'), 1.4],
  ['earth', spell('Earth Spell Impact 1_s07'), 1.5],
  ['buff', spell('Healing Spell Impact 1_s08'), 1.6],
  ['shadow', spell('Wind Spell Impact 1_s07'), 1.2],
  ['poison', spell('Plant Spell Impact 1_s07'), 1.2],
  ['throw', `${AD}/inventory/bottle.wav`],
  ['warcry', `${AD}/battle/sword-unsheathe.wav`],
  ['roar', `${AD}/NPC/giant/giant2.wav`],
  // Mortes e nível
  ['skel1', 'skeleton2sfx_0.wav', 1.3],
  ['skel2', 'skeleton1sfx_0.wav', 1.3],
  ['bones1', `${BR}/0.ogg`],
  ['bones2', `${BR}/6.ogg`],
  ['rubble', `${RD}/stones_02.ogg`],
  ['fall', `${K}/dropLeather.ogg`],
  ['levelup', `${WB}/Rise02.aif`, 1.6, -2],
  // Passos (jogador local)
  ['step1', `${K}/footstep00.ogg`],
  ['step2', `${K}/footstep01.ogg`],
  ['step3', `${K}/footstep02.ogg`],
  ['step4', `${K}/footstep03.ogg`],
  // Interface
  ['click', `${KI}/click_001.ogg`],
  ['open', `${K}/bookOpen.ogg`],
  ['close', `${K}/bookClose.ogg`, 0.3, -4],
  ['coins', `${RD}/item_coins_01.ogg`],
  ['buy', `${K}/handleCoins.ogg`],
  ['pickup', `${K}/handleSmallLeather.ogg`],
  ['drink', `${AD}/inventory/bubble.wav`],
  ['enchant', `${RD}/item_gem_01.ogg`],
  ['fail', `${KI}/error_005.ogg`],
  // Ambiente (avulsos)
  ['anvil', 'amb/forger.ogg', 2.5],
];

/** Músicas (uma por região) e fundos ambientes: arquivo, início e duração (s). */
const MUSIC = [
  ['town', 'music/Loop_Market_Day.wav', 0, 0],
  ['wild', 'music/Peaceful_forest.mp3', 0, 0],
  ['dark', 'music/GameMusic_ForestTheme_24.mp3', 0, 0],
  ['crypt', 'music/dungeon002.ogg', 0, 60],
];
const AMB = [
  ['forest', 'amb/birds-isaiah658.ogg', 0, 0],
  ['town', 'amb/park_ambience_birds.wav', 60, 30],
  ['night', 'amb/crickets-oneloop.mp3', 0, 0],
  ['wind', 'amb/park_ambience_wind.wav', 30, 25],
];

// ---------------------------------------------------------------------------------------

const hasLocal = spawnSync('ffmpeg', ['-version'], { stdio: 'ignore' }).status === 0;

/** Roda o ffmpeg com caminhos relativos à pasta dos pacotes (montada em /src no Docker). */
function ffmpeg(args) {
  const run = hasLocal
    ? spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', ...args], { cwd: src, maxBuffer: 1 << 30 })
    : spawnSync('docker', ['run', '--rm', '-v', `${src}:/src`, '-w', '/src', 'jrottenberg/ffmpeg:6.1-alpine', '-hide_banner', '-loglevel', 'error', ...args], {
        maxBuffer: 1 << 30,
      });
  if (run.status !== 0) throw new Error(`ffmpeg falhou (${args.join(' ')}): ${run.stderr}`);
  return run.stdout;
}

const rel = (p) => relative(src, join(src, p)).replaceAll('\\', '/');

/** Decodifica para PCM mono float 44,1 kHz. */
function decode(file) {
  const out = ffmpeg(['-i', rel(file), '-ac', '1', '-ar', String(RATE), '-f', 'f32le', 'pipe:1']);
  return new Float32Array(out.buffer, out.byteOffset, Math.floor(out.byteLength / 4));
}

/** Corta o silêncio das pontas, limita a duração (com fade) e nivela pelo volume médio. */
function prepare(pcm, maxDur, gainDb) {
  const thr = 10 ** (-48 / 20);
  let a = 0;
  let b = pcm.length - 1;
  while (a < b && Math.abs(pcm[a]) < thr) a++;
  while (b > a && Math.abs(pcm[b]) < thr) b--;
  a = Math.max(0, a - Math.round(0.002 * RATE));
  let clip = pcm.slice(a, b + 1);
  if (maxDur && clip.length > maxDur * RATE) {
    clip = clip.slice(0, Math.round(maxDur * RATE));
    const fade = Math.min(clip.length, Math.round(0.25 * RATE));
    for (let i = 0; i < fade; i++) clip[clip.length - fade + i] *= 1 - i / fade;
  }
  // Fade curtíssimo nas pontas (sem estalo).
  const edge = Math.min(64, clip.length >> 1);
  for (let i = 0; i < edge; i++) {
    clip[i] *= i / edge;
    clip[clip.length - 1 - i] *= i / edge;
  }
  let peak = 0;
  let sum = 0;
  for (const v of clip) {
    peak = Math.max(peak, Math.abs(v));
    sum += v * v;
  }
  const rms = Math.sqrt(sum / Math.max(1, clip.length));
  const g = Math.min(10 ** (-16 / 20) / Math.max(rms, 1e-6), 0.89 / Math.max(peak, 1e-6)) * 10 ** ((gainDb ?? 0) / 20);
  for (let i = 0; i < clip.length; i++) clip[i] *= g;
  return clip;
}

function writeWav(path, pcm) {
  const buf = Buffer.alloc(44 + pcm.length * 2);
  buf.write('RIFF', 0);
  buf.writeUInt32LE(36 + pcm.length * 2, 4);
  buf.write('WAVEfmt ', 8);
  buf.writeUInt32LE(16, 16);
  buf.writeUInt16LE(1, 20);
  buf.writeUInt16LE(1, 22);
  buf.writeUInt32LE(RATE, 24);
  buf.writeUInt32LE(RATE * 2, 28);
  buf.writeUInt16LE(2, 32);
  buf.writeUInt16LE(16, 34);
  buf.write('data', 36);
  buf.writeUInt32LE(pcm.length * 2, 40);
  for (let i = 0; i < pcm.length; i++) buf.writeInt16LE(Math.round(Math.max(-1, Math.min(1, pcm[i])) * 32767), 44 + i * 2);
  writeFileSync(path, buf);
}

// Efeitos → um arquivo só.
const parts = [];
const sprite = {};
let cursor = 0.05;
for (const [name, file, maxDur, gainDb] of SFX) {
  const clip = prepare(decode(file), maxDur, gainDb);
  sprite[name] = [Math.round(cursor * 1000) / 1000, Math.round((clip.length / RATE) * 1000) / 1000];
  parts.push({ at: Math.round(cursor * RATE), clip });
  cursor += clip.length / RATE + GAP;
  console.log(`${name.padEnd(9)} ${(clip.length / RATE).toFixed(2)} s  ${file}`);
}
const all = new Float32Array(Math.round((cursor + 0.05) * RATE));
for (const p of parts) all.set(p.clip, p.at);
writeWav(join(tmp, 'sfx.wav'), all);
ffmpeg(['-y', '-i', '.build/sfx.wav', '-ac', '1', '-ar', String(RATE), '-c:a', 'libmp3lame', '-b:a', '64k', '.build/sfx.mp3']);
copyFileSync(join(tmp, 'sfx.mp3'), join(outDir, 'sfx.mp3'));
writeFileSync(spriteJson, `${JSON.stringify(sprite, null, 0).replaceAll('],', '],\n ')}\n`);

/** Trecho de música/ambiente: nivelado (loudnorm), mono, com fade curto nas pontas. */
function track(kind, name, file, start, dur, lufs, rate, kbps) {
  const len = dur || probe(file) - start;
  const af = `loudnorm=I=${lufs}:TP=-1.5:LRA=11,afade=t=in:d=0.03,afade=t=out:st=${(len - 0.08).toFixed(2)}:d=0.08`;
  const out = `.build/${kind}-${name}.mp3`;
  ffmpeg(['-y', '-ss', String(start), '-t', String(len), '-i', rel(file), '-af', af, '-ac', '1', '-ar', String(rate), '-c:a', 'libmp3lame', '-b:a', `${kbps}k`, out]);
  copyFileSync(join(src, out), join(outDir, kind, `${name}.mp3`));
  console.log(`${kind}/${name}.mp3  ${len.toFixed(1)} s  ${(statSync(join(outDir, kind, `${name}.mp3`)).size / 1024).toFixed(0)} KB`);
}

/** Duração de um arquivo (s), lida do cabeçalho pelo ffmpeg. */
function probe(file) {
  const run = hasLocal
    ? spawnSync('ffmpeg', ['-hide_banner', '-i', rel(file)], { cwd: src })
    : spawnSync('docker', ['run', '--rm', '-v', `${src}:/src`, '-w', '/src', 'jrottenberg/ffmpeg:6.1-alpine', '-hide_banner', '-i', rel(file)]);
  const m = /Duration: (\d+):(\d+):([\d.]+)/.exec(String(run.stderr));
  if (!m) throw new Error(`sem duração: ${file}`);
  return Number(m[1]) * 3600 + Number(m[2]) * 60 + Number(m[3]);
}

for (const [name, file, start, dur] of MUSIC) track('music', name, file, start, dur, -19, 44100, 64);
for (const [name, file, start, dur] of AMB) track('amb', name, file, start, dur, -23, 32000, 48);

const size = (p) => statSync(p).size;
let total = size(join(outDir, 'sfx.mp3'));
for (const [n] of MUSIC) total += size(join(outDir, 'music', `${n}.mp3`));
for (const [n] of AMB) total += size(join(outDir, 'amb', `${n}.mp3`));
console.log(`sfx.mp3 ${(size(join(outDir, 'sfx.mp3')) / 1024).toFixed(0)} KB · total ${(total / 1024 / 1024).toFixed(2)} MB`);
