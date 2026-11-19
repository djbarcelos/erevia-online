// Gerado por client/scripts/make-remaster-fists.mjs (não editar à mão).
// Punho fechado de cada herói do remaster, no espaço do encaixe handslot (sem a rotação SLOT_INV):
// pos = centro do punho, axis = direção do cabo (para a frente do corpo), fingers = direção dos dedos.
export interface FistFrame { pos: [number, number, number]; axis: [number, number, number]; fingers: [number, number, number] }
export const HERO_FISTS: Record<string, { r?: FistFrame; l?: FistFrame }> = {
  knight: {"r":{"pos":[-0.032,0.049,-0.038],"axis":[-0.637,0,0.771],"fingers":[0,1,0]},"l":{"pos":[0.028,0.061,-0.046],"axis":[0.645,0,0.765],"fingers":[0,1,0]}},
  barbarian: {"r":{"pos":[0.026,0.13,-0.086],"axis":[-0.906,0,0.423],"fingers":[0,1,0]},"l":{"pos":[0.073,0.142,-0.04],"axis":[0.889,0,0.457],"fingers":[0,1,0]}},
  mage: {"r":{"pos":[-0.025,0.086,-0.061],"axis":[-0.612,0,0.791],"fingers":[0,1,0]},"l":{"pos":[0.044,0.091,-0.037],"axis":[0.636,0,0.772],"fingers":[0,1,0]}},
  rogue: {"r":{"pos":[-0.011,0.031,-0.061],"axis":[-0.588,0,0.809],"fingers":[0,1,0]},"l":{"pos":[0.031,0.047,-0.042],"axis":[0.536,0,0.844],"fingers":[0,1,0]}},
};
