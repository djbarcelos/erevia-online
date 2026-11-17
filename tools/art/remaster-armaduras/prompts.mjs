// Prompts de retexture (Meshy) por classe e faixa de raridade (tunnel#96). O retexture mantém a malha e a UV
// do image-to-3d (conferido: 5.936 vértices, posição e UV idênticas), então a textura nova serve no GLB com rig.
export const STYLE = 'hand-painted stylized fantasy MMORPG look, vibrant saturated colors, clean painterly shading';

// id do image-to-3d de cada classe (o mesmo do GLB do jogo)
export const SOURCES = {
  knight: '01a1e6ce-e61d-730f-97b4-95dd05a99164',
  barbarian: '01a1e6ce-e558-766b-b6ec-98ca893d581c',
  mage: '01a1e712-5a03-74ae-b20d-c54f10d4d407',
  rogue: '01a1e6ce-e5e6-7672-9de1-ee754807e6b6',
};

// raro = A (azul e prata), épico = B (negro e ouro com vermelho), lendário = C (ouro radiante e branco com runas ciano)
export const TIERS = {
  rare: {
    knight: 'polished silver steel plate armor with bright cobalt blue enamel panels and silver engraved trim, deep blue cloth tabard and cape',
    barbarian: 'reinforced dark leather and riveted silver steel plates with cobalt blue painted war markings, blue wolf fur trim',
    mage: 'deep cobalt blue embroidered robe with silver thread patterns and silver trim, pale blue gemstones',
    rogue: 'cobalt blue and silver studded leather armor with silver buckles, deep blue hooded cloak',
  },
  epic: {
    knight: 'blackened steel plate armor with ornate gold filigree and gold edges, deep crimson cloth tabard and cape, red gems',
    barbarian: 'blackened iron plates and black leather with gold rune engravings, crimson war paint, dark fur trim, red gems',
    mage: 'black and deep purple embroidered robe with gold thread runes and gold trim, glowing violet gemstones',
    rogue: 'blackened leather armor with gold filigree and gold buckles, dark crimson hooded cloak',
  },
  legend: {
    knight: 'radiant polished golden plate armor with white and gold tabard, glowing cyan runes engraved on the armor, bright white cape with gold trim',
    barbarian: 'radiant golden plate and white fur armor with glowing cyan runes engraved, gold horns and gold trim',
    mage: 'luminous white and gold robe with glowing cyan runes and gold trim, radiant cyan gemstones',
    rogue: 'radiant white and gold leather armor with glowing cyan runes and gold trim, white and gold hooded cloak',
  },
};
