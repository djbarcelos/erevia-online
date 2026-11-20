// Ícones das skills novas das 8 evoluções (docs/evolucoes, tunnel#181): 108 ícones em 3 folhas 6x6
// (text-to-image, nano-banana-pro, 9 créditos cada = 27 créditos; em 5x5 seriam 5 folhas = 45).
// As outras 62 skills das árvores reaproveitam os ícones de client/public/icons/skills (mesmo nome).
//   node tools/art/icones-evolucao/prompts.mjs
//   node tools/art/meshy.mjs create text-to-image --est 9 --name icons-evo-<n> --json-file assets-src/meshy/prompts/icons-evo-<n>.json
// A ordem de leitura de cada folha é a ordem de ICONS abaixo, 36 por folha.
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

// [evolução:skill, o que o ícone mostra]
export const ICONS = [
  // Bastião
  ['bastion:Guarda Inabalável', 'a knight crouched behind a raised tower shield, blue steel glow, unbreakable stance'],
  ['bastion:Pele de Aço', 'a forearm covered in polished steel plates like skin, cold blue sheen'],
  ['bastion:Postura de Ferro', 'two iron boots planted firmly on cracked stone, heavy grey aura'],
  ['bastion:Último Reduto', 'a kneeling knight holding a broken shield, a last golden spark of life in his chest'],
  ['bastion:Vínculo de Guarda', 'two shields linked by a glowing teal chain of light'],
  ['bastion:Égide Compartilhada', 'a golden hexagon barrier splitting into three smaller barriers'],
  ['bastion:Interceptar', 'a knight leaping in front of an ally to catch an arrow on his shield, motion trail'],
  ['bastion:Guarda do Escudo', 'a round shield deflecting a sword with bright sparks'],
  ['bastion:Juramento de Proteção', 'a raised gauntlet swearing an oath, a teal ring of light over allies silhouettes'],
  ['bastion:Desafio Reforçado', 'a knight helmet with a widening red ring of challenge around it'],
  ['bastion:Zona de Provocação', 'a glowing red circle sigil on the ground with arrows pointing inward'],
  ['bastion:Eco do Escudo', 'a shield bash impact echoing into three blue shockwave rings'],
  ['bastion:Prisão do Guardião', 'iron chains bursting from the ground around a shield emblem, gold light'],
  // Paladino
  ['herald:Golpe Radiante', 'a sword slash leaving a bright golden holy arc'],
  ['herald:Zelo', 'a burning golden flame inside a holy sunburst emblem'],
  ['herald:Veredito', 'a golden scale of justice above a downward sword'],
  ['herald:Fervor', 'a glowing heart with golden fire and sparks'],
  ['herald:Aurora Ardente', 'a warhammer head exploding in a wide ring of dawn light'],
  ['herald:Sentença Sagrada', 'a colossal golden sword of light piercing down through clouds'],
  ['herald:Bênção da Luz', 'two open hands releasing golden light over a shield and a sword'],
  ['herald:Bênção da Prontidão', 'a winged golden hourglass radiating light'],
  ['herald:Bênção Ampliada', 'a golden blessing halo expanding into a bigger halo'],
  ['herald:Renovação', 'a soft green and gold healing light rising from a chalice'],
  ['herald:Graça', 'a white dove with golden light wings'],
  ['herald:Bênção Suprema', 'a radiant golden crown above a circle of praying silhouettes'],
  ['herald:Escudo da Fé', 'a shield made of golden prayer light with a sun emblem'],
  ['herald:Luz Cegante', 'a blinding white flash over a covered eye'],
  ['herald:Purificar', 'a hand washing away dark green poison with clear golden water'],
  ['herald:Penitência', 'a golden brand sigil burning on a kneeling shadow figure'],
  ['herald:Sacrifício', 'a paladin with open arms absorbing red light from allies behind him'],
  // Berserker
  ['bloodrager:Golpe Esmagador', 'an axe crushing a helmet flat, red cracks'],
  ['bloodrager:Ruptura', 'a deep axe wound gushing crimson blood drops'],
  ['bloodrager:Abalo', 'a ground shockwave ring of rocks and dust around an axe'],
  ['bloodrager:Ira Desmedida', 'a screaming red face with blazing eyes and red lightning'],
  ['bloodrager:Cólera', 'a clenched fist wrapped in red flames of rage'],
  ['bloodrager:Turbilhão', 'a red tornado full of spinning axes'],
  ['bloodrager:Sequência Selvagem', 'three fast claw scratches in a row with speed lines'],
  ['bloodrager:Fúria Incontrolável', 'a broken chain shattering around a roaring barbarian silhouette'],
  ['bloodrager:Sede Insaciável', 'a fanged mouth drinking a stream of red blood'],
  ['bloodrager:Marca da Presa', 'a bloody claw mark target over a fleeing silhouette'],
  ['bloodrager:Carnificina Final', 'a giant executioner axe over a pile of skulls, red moon'],
  // Senhor da Guerra
  ['warlord:Brado de Guerra', 'a bearded warlord shouting with golden sound waves'],
  ['warlord:Liderança', 'a raised axe leading a line of soldier silhouettes, golden glow'],
  ['warlord:Ordem de Avanço', 'a pointing gauntlet with a golden arrow moving forward'],
  ['warlord:Grito de Reagrupar', 'a war horn breaking iron chains with orange sound waves'],
  ['warlord:Estandarte Tremulante', 'a golden war banner waving strongly in the wind'],
  ['warlord:Comando Absoluto', 'a golden crown with crossed axes and a commanding aura'],
  ['warlord:Golpe de Comando', 'an axe strike with a golden command sigil burst'],
  ['warlord:Avanço de Guerra', 'a warrior leaping through the air with an axe raised, orange trail'],
  ['warlord:Braço de Guerra', 'a muscular armored arm gripping an axe, golden veins'],
  ['warlord:Quebra-Linha', 'an axe smashing through a wall of shields'],
  ['warlord:Impacto', 'a shoulder charge impact with a stun star burst'],
  ['warlord:Ímpeto de Carga', 'a charging bull silhouette made of orange fire'],
  ['warlord:Investida Implacável', 'three chained leaping warrior afterimages with an axe'],
  ['warlord:Desafio', 'a barbarian beating his chest with a red challenge aura'],
  ['warlord:Domínio do Campo', 'a heavy boot stomping a widening green shockwave'],
  ['warlord:Presença Dominante', 'a giant shadowed warlord looming over tiny enemies, green aura'],
  // Elementalista
  ['elementalist:Chama Persistente', 'a small stubborn flame that keeps burning on a coal'],
  ['elementalist:Coração do Vulcão', 'a burning heart of magma inside a volcano'],
  ['elementalist:Frio Penetrante', 'an icicle piercing through frost, cold blue mist'],
  ['elementalist:Inverno Eterno', 'a frozen hourglass covered in snow'],
  ['elementalist:Coração do Inverno', 'a crystal ice heart glowing pale blue'],
  ['elementalist:Reação Elemental', 'a fire orb and an ice orb colliding in a violet explosion'],
  ['elementalist:Convergência', 'fire and frost spirals converging into one bright point'],
  ['elementalist:Ruptura Elemental', 'the ground splitting with fire on one side and ice on the other, violet core'],
  // Arcanista
  ['occultist:Agonia', 'a twisted purple face screaming in pain, cursed runes'],
  ['occultist:Ruína', 'crumbling purple cursed stone ruins'],
  ['occultist:Maldição Persistente', 'a purple cursed rune chain wrapped around a heart'],
  ['occultist:Fraqueza', 'a cracking purple shield with drained energy'],
  ['occultist:Abismo Interior', 'a dark vortex eye inside a hooded head, purple glow'],
  ['occultist:Praga Arcana', 'violet arcane plague spores swirling from a cracked vial'],
  ['occultist:Contágio', 'green plague spreading between three linked skulls'],
  ['occultist:Febre', 'a burning sick green thermometer-like flask with fumes'],
  ['occultist:Nuvem Pestilenta', 'a big green toxic cloud with skull shapes inside'],
  ['occultist:Putrefação', 'a rotting green hand with decaying flesh, spores'],
  ['occultist:Pandemia', 'green plague bursting outward from a body to many silhouettes'],
  ['occultist:Mente Afiada', 'a sharp violet crystal shard inside a glowing mind'],
  ['occultist:Prisão Etérea', 'a ghostly figure trapped inside a violet ethereal crystal'],
  ['occultist:Rajada Concentrada', 'a narrow concentrated violet arcane beam with rings'],
  ['occultist:Fluxo Arcano', 'a flowing violet river of mana circling an orb'],
  ['occultist:Colapso Arcano', 'a violet arcane star collapsing and exploding'],
  // Assassino
  ['cutthroat:Corte Profundo', 'a dagger leaving a deep red gash'],
  ['cutthroat:Fio Perfeito', 'a gleaming dagger edge with a perfect white shine'],
  ['cutthroat:Dança das Lâminas', 'a ring of spinning daggers around a dancing silhouette'],
  ['cutthroat:Véu Mortal', 'a translucent dark veil over a falling enemy'],
  ['cutthroat:Sombra Persistente', 'a dark afterimage shadow trailing a running assassin'],
  ['cutthroat:Assassino das Sombras', 'red eyes glowing from a hood in pure darkness, dagger'],
  ['cutthroat:Marca Profunda', 'a deep red target mark carved into skin'],
  ['cutthroat:Golpe de Misericórdia', 'a thin dagger striking a kneeling enemy, red flash'],
  ['cutthroat:Sangrar', 'a dagger dripping fresh red blood drops'],
  ['cutthroat:Execução Perfeita', 'two crossed daggers over a red skull, clean crit flash'],
  // Patrulheiro
  ['hunter:Tiro Preciso', 'an arrow flying straight to a bullseye target'],
  ['hunter:Disparo Perfurante', 'an arrow piercing through three apples in a line'],
  ['hunter:Olho de Falcão', 'a sharp golden falcon eye'],
  ['hunter:Flecha Carregada', 'a drawn bow with a glowing charged green arrow'],
  ['hunter:Chuva de Flechas', 'dozens of arrows raining down from the sky'],
  ['hunter:Flecha Mortal', 'a black arrow with a skull-shaped tip glowing red'],
  ['hunter:Mira Perfeita', 'a crosshair reticle over a deer silhouette, golden focus'],
  ['hunter:Armadilha de Caça', 'an open steel bear trap with sharp teeth'],
  ['hunter:Campo de Espinhos', 'a field of sharp green thorny vines'],
  ['hunter:Mestre Armadilheiro', 'a pair of gloves holding a trap and a coiled rope'],
  ['hunter:Emboscada', 'a hunter crouched in tall grass ready to shoot'],
  ['hunter:Marca da Presa', 'a green hunting mark painted on a wolf'],
  ['hunter:Passo de Caça', 'a leather boot running over leaves with green speed lines'],
  ['hunter:Recuo', 'an archer jumping backwards while shooting, motion trail'],
  ['hunter:Perseguição', 'paw prints leading to a fleeing silhouette, green arrow'],
  ['hunter:Instinto do Caçador', 'a wolf head with glowing green eyes'],
  ['hunter:Caçada Implacável', 'a pack of spectral green wolves running forward'],
];

const STYLE =
  'Hand-painted stylized fantasy MMORPG skill icons, vibrant saturated colors, bold readable silhouette centered in each tile, ' +
  'soft painterly shading, each tile has its own dark rich gradient background matching its element, ' +
  'no text, no letters, no numbers, no frames, no borders inside the tiles.';
export const GRID = 6;
export const sheets = () => {
  const n = GRID * GRID, out = [];
  for (let i = 0; i < ICONS.length; i += n) out.push(ICONS.slice(i, i + n));
  return out;
};

if (process.argv[1] && import.meta.filename === resolve(process.argv[1])) {
  const dir = resolve(import.meta.dirname, '../../../assets-src/meshy/prompts');
  mkdirSync(dir, { recursive: true });
  sheets().forEach((list, s) => {
    const n = GRID * GRID;
    const cells = list.map(([, d], i) => `${i + 1}. ${d}`).join('; ');
    const prompt =
      `A sprite sheet: a perfect ${GRID} by ${GRID} grid of ${n} equal square tiles separated by thin pure black lines, filling the whole square image. ` +
      `Tiles in reading order (left to right, top to bottom): ${cells}` +
      (list.length < n ? `; tiles ${list.length + 1} to ${n} are empty plain black. ` : '. ') + STYLE;
    writeFileSync(resolve(dir, `icons-evo-${s + 1}.json`), JSON.stringify({ ai_model: 'nano-banana-pro', prompt, aspect_ratio: '1:1' }));
    console.log(`folha ${s + 1}:`, list.length, 'ícones,', prompt.length, 'caracteres');
  });
}
