// Parte 2 (tunnel#97): formas raras (grau 3) das 6 famílias e as formas altas da varinha e da adaga.
// Conceito 2D (nano-banana, 3 créditos) -> assets-src/meshy/prompts/<id>.json; depois
//   MESHY_BUDGET=330 node tools/art/meshy.mjs create text-to-image --est 3 --name <id> --json-file assets-src/meshy/prompts/<id>.json
// Mesma moldura de estilo da parte 1 (prompts.mjs); a forma rara fica entre a comum e a alta.
import { mkdirSync, writeFileSync } from 'node:fs';
const style = 'hand-painted stylized fantasy MMORPG weapon concept, vibrant saturated colors, bright clean painterly look, soft painted shading with dark outline feel, chunky readable silhouette with slightly exaggerated proportions, a single object shown whole, upright vertical orientation, front view, centered, no hand, no person, no stand, plain neutral light gray background, no text';
const items = {
  sword_rare: 'Rare one-handed curved saber pointing straight up, slightly curved bright steel blade with a blue-tinted edge, swept silver crossguard with a small blue gem, blue-wrapped grip and a round silver pommel, rare quality, distinct silhouette. ',
  axe_rare: 'Rare one-handed bearded battle axe pointing straight up, large single curved steel blade with a hooked beard and a back spike, brass bands and blue-wrapped dark wooden haft, rare quality, distinct silhouette. ',
  staff_rare: 'Rare wizard staff pointing straight up, dark wood shaft wrapped with silver bands, topped by a silver crescent moon cradling a glowing teal orb, rare quality, distinct silhouette. ',
  shield_rare: 'Rare heater knight shield with a pointed bottom seen straight from the front, deep blue field with a wide polished steel border, a white star emblem and a small steel boss, rare quality, distinct silhouette. ',
  wand_rare: 'Rare magic wand pointing straight up, twisted spiral dark wood with silver inlay, a faceted glowing teal crystal held by three silver claws at the tip, rare quality. ',
  wand_high: 'Epic archmage scepter wand pointing straight up, slim golden wand with a large glowing purple orb held in a small golden crown of wings at the tip, gold rings on the shaft, legendary quality, striking silhouette. ',
  dagger_rare: 'Rare dagger pointing straight up, wavy flame-shaped steel blade, curved silver guard with a blue gem, blue-wrapped grip, rare quality, distinct silhouette. ',
  dagger_high: 'Epic assassin dagger pointing straight up, long dark steel blade with golden filigree, an ornate winged golden crossguard with a large red gem, black leather grip and gold pommel, legendary quality, striking silhouette. ',
};
mkdirSync('assets-src/meshy/prompts', { recursive: true });
for (const [k, v] of Object.entries(items)) writeFileSync(`assets-src/meshy/prompts/${k}.json`, JSON.stringify({ ai_model: 'nano-banana', prompt: v + style, aspect_ratio: k.startsWith('shield') ? '1:1' : '9:16' }));
