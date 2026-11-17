// Gerado por client/scripts/make-armor-attach.mjs (não editar à mão): onde cada peça de armadura anexada
// fica no osso de cada classe (posição, rotação e escala locais do osso), calculado no repouso.

export interface ArmorAttach {
  bone: string;
  pos: [number, number, number];
  rot: [number, number, number, number];
  scale: number;
}

export const ARMOR_ATTACH: Record<string, Record<string, ArmorAttach>> = {
  knight: {
    pauldron_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0019,
        -0.0954,
        -0.0088
      ],
      rot: [
        -0.0097,
        0,
        0,
        1
      ],
      scale: 0.72172
    },
    pauldron_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0019,
        -0.1232,
        -0.0092
      ],
      rot: [
        -0.0097,
        0,
        0,
        1
      ],
      scale: 0.72172
    },
    cape_rare: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0019,
        -0.3347,
        -0.1939
      ],
      rot: [
        -0.0097,
        0,
        0,
        1
      ],
      scale: 0.89318
    },
    cape_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0019,
        -0.4847,
        -0.19
      ],
      rot: [
        -0.0097,
        0,
        0,
        1
      ],
      scale: 1.20841
    },
    cape_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0019,
        -0.5596,
        -0.1861
      ],
      rot: [
        -0.0097,
        0,
        0,
        1
      ],
      scale: 1.36884
    },
    helm_knight_rare: {
      bone: "mixamorig:Head",
      pos: [
        -0.0109,
        0.1534,
        -0.013
      ],
      rot: [
        -0.0631,
        0.0361,
        0.0361,
        0.9967
      ],
      scale: 0.5299
    },
    helm_knight_epic: {
      bone: "mixamorig:Head",
      pos: [
        -0.01,
        0.1482,
        -0.0119
      ],
      rot: [
        -0.0631,
        0.0361,
        0.0361,
        0.9967
      ],
      scale: 0.51821
    }
  },
  barbarian: {
    pauldron_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0059,
        -0.1345,
        -0.0356
      ],
      rot: [
        0.02,
        0.019,
        0.019,
        0.9994
      ],
      scale: 0.98011
    },
    pauldron_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0073,
        -0.1721,
        -0.0384
      ],
      rot: [
        0.02,
        0.019,
        0.019,
        0.9994
      ],
      scale: 0.98011
    },
    cape_rare: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0041,
        -0.3042,
        -0.2628
      ],
      rot: [
        0.02,
        0.019,
        0.019,
        0.9994
      ],
      scale: 0.89318
    },
    cape_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0097,
        -0.454,
        -0.2679
      ],
      rot: [
        0.02,
        0.019,
        0.019,
        0.9994
      ],
      scale: 1.20841
    },
    cape_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0126,
        -0.529,
        -0.2686
      ],
      rot: [
        0.02,
        0.019,
        0.019,
        0.9994
      ],
      scale: 1.36884
    },
    helm_barbarian_epic: {
      bone: "mixamorig:Head",
      pos: [
        -0.0151,
        0.1866,
        -0.0268
      ],
      rot: [
        -0.0663,
        0.0372,
        0.0372,
        0.9964
      ],
      scale: 0.65236
    }
  },
  mage: {
    pauldron_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0026,
        -0.076,
        -0.0188
      ],
      rot: [
        0.0147,
        0.0065,
        0.0065,
        0.9998
      ],
      scale: 0.60318
    },
    pauldron_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0029,
        -0.0991,
        -0.0202
      ],
      rot: [
        0.0147,
        0.0065,
        0.0065,
        0.9998
      ],
      scale: 0.60318
    },
    cape_rare: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0035,
        -0.3321,
        -0.2158
      ],
      rot: [
        0.0147,
        0.0065,
        0.0065,
        0.9998
      ],
      scale: 0.89318
    },
    cape_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0054,
        -0.4821,
        -0.2192
      ],
      rot: [
        0.0147,
        0.0065,
        0.0065,
        0.9998
      ],
      scale: 1.20841
    },
    cape_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0.0064,
        -0.5571,
        -0.2191
      ],
      rot: [
        0.0147,
        0.0065,
        0.0065,
        0.9998
      ],
      scale: 1.36884
    },
    hat_mage_epic: {
      bone: "mixamorig:Head",
      pos: [
        -0.0241,
        0.2711,
        -0.0694
      ],
      rot: [
        -0.0622,
        0.0368,
        0.0368,
        0.9967
      ],
      scale: 0.48049
    }
  },
  rogue: {
    pauldron_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0,
        -0.1005,
        0.0016
      ],
      rot: [
        -0.0043,
        0,
        0,
        1
      ],
      scale: 0.6316
    },
    pauldron_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0,
        -0.1248,
        0.001
      ],
      rot: [
        -0.0043,
        0,
        0,
        1
      ],
      scale: 0.6316
    },
    cape_rare: {
      bone: "mixamorig:Spine2",
      pos: [
        0,
        -0.3577,
        -0.1856
      ],
      rot: [
        -0.0043,
        0,
        0,
        1
      ],
      scale: 0.89318
    },
    cape_epic: {
      bone: "mixamorig:Spine2",
      pos: [
        0,
        -0.5077,
        -0.1833
      ],
      rot: [
        -0.0043,
        0,
        0,
        1
      ],
      scale: 1.20841
    },
    cape_legend: {
      bone: "mixamorig:Spine2",
      pos: [
        0,
        -0.5827,
        -0.1803
      ],
      rot: [
        -0.0043,
        0,
        0,
        1
      ],
      scale: 1.36884
    },
    hood_rogue_epic: {
      bone: "mixamorig:Head",
      pos: [
        -0.0041,
        0.0887,
        -0.0093
      ],
      rot: [
        -0.0572,
        0.0218,
        0.0218,
        0.9979
      ],
      scale: 0.36301
    }
  }
};
