// Prompts do conceito 2D (text-to-image, nano-banana, 3 créditos) das armas e escudos do remaster (tunnel#97).
// Gera assets-src/meshy/prompts/<id>.json para:
//   MESHY_BUDGET=180 node tools/art/meshy.mjs create text-to-image --est 3 --json-file assets-src/meshy/prompts/<id>.json
import { mkdirSync, writeFileSync } from 'node:fs';
const style = 'hand-painted stylized fantasy MMORPG weapon concept, vibrant saturated colors, bright clean painterly look, soft painted shading with dark outline feel, chunky readable silhouette with slightly exaggerated proportions, a single object shown whole, upright vertical orientation, front view, centered, no hand, no person, no stand, plain neutral light gray background, no text';
const items = {
  sword: 'One-handed straight longsword pointing straight up, simple steel blade with a central fuller, plain gold crossguard, brown leather wrapped grip and round pommel, common basic quality. ',
  sword_high: 'Epic heroic greatsword pointing straight up, long broad bright steel blade with an engraved fuller, ornate winged golden crossguard with a large red gem, leather grip and gold pommel, legendary quality, striking silhouette. ',
  axe: 'One-handed battle axe pointing straight up, plain steel single-bit axe head on a short wooden handle with leather grip, common basic quality. ',
  axe_high: 'Epic two-handed double-bladed battle axe pointing straight up, large crescent steel blades with golden engraved trim, a blue gem at the center, long dark wooden haft with gold rings, legendary quality, striking silhouette. ',
  staff: 'Wizard staff pointing straight up, simple twisted wooden staff with a plain round blue crystal held in a small claw at the top, common basic quality. ',
  staff_high: 'Epic archmage staff pointing straight up, ornate golden staff with a large glowing purple crystal floating inside a crown of golden wings at the top, gold rings on the shaft, legendary quality, striking silhouette. ',
  wand: 'Short magic wand pointing straight up, slim wooden wand with a small glowing blue crystal tip and a gold ring. ',
  dagger: 'Single dagger pointing straight up, short straight steel blade, small plain crossguard, leather wrapped grip, common basic quality. ',
  shield: 'Round heater knight shield seen straight from the front, wooden shield with steel rim and a central steel boss, painted red and white, common basic quality. ',
  shield_high: 'Epic kite shield seen straight from the front, bright steel with golden ornate border and a large golden lion emblem and a red gem boss, legendary quality, striking silhouette. ',
};
mkdirSync('assets-src/meshy/prompts', { recursive: true });
for (const [k, v] of Object.entries(items)) writeFileSync(`assets-src/meshy/prompts/${k}.json`, JSON.stringify({ ai_model: 'nano-banana', prompt: v + style, aspect_ratio: k.startsWith('shield') ? '1:1' : '9:16' }));
