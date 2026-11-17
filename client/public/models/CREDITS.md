# Créditos dos modelos 3D

Personagens, monstros, armas, animações e cenário: **KayKit Adventurers 2.0**, **KayKit Skeletons 1.1**,
**KayKit Character Animations 1.1**, **KayKit Forest Nature Pack 1.0** e **KayKit Medieval Hexagon Pack 1.0**,
por Kay Lousberg ([www.kaylousberg.com](https://www.kaylousberg.com)).

Licença: **CC0 1.0** (domínio público). Uso comercial liberado e crédito opcional, mas fica registrado aqui.

- https://kaylousberg.itch.io/kaykit-adventurers
- https://kaylousberg.itch.io/kaykit-skeletons
- https://kaylousberg.itch.io/kaykit-character-animations
- https://kaylousberg.itch.io/kaykit-forest
- https://kaylousberg.itch.io/kaykit-medieval-hexagon

Para regerar a partir dos zips: descompacte-os em `assets-src/kaykit/x/` e, dentro de `client/`, rode:

```bash
node scripts/build-models.mjs ../assets-src/kaykit/x    # personagens, monstros, armas, animações
node scripts/build-scenery.mjs ../assets-src/kaykit/x   # kits de cenário (env-forest.glb, env-town.glb)
node scripts/build-world.mjs                            # layout + colisão (cliente e servidor)
```

## Remaster (estilo B)

`knight_b.glb`, `skeleton_warrior_b.glb` e `plaza_*_b.glb` (só carregam com `?remaster=1`) são **modelos próprios do Erevia**,
feitos em código bpy no Blender (`tools/art/fatia-vertical/`), sem biblioteca, gerador nem arquivo externo. Mesma licença do projeto.

## Famílias de monstro não humanoides (tunnel#135)

`wolf_b.glb`, `spider_b.glb`, `slime_b.glb` e `frog_b.glb` partem de modelos **CC0** de **Quaternius** ([quaternius.com](https://quaternius.com)), com malha, esqueleto e animações
de origem. A cor é nova: o retexture do Meshy repintou a malha (a UV foi refeita no Blender, pois os originais só têm cor por material).

| Arquivo | Modelo de origem (Poly Pizza, licença conferida na página: CC0 1.0) |
|---|---|
| `wolf_b.glb` | Wolf, https://poly.pizza/m/P1gU3Qkr9r (série "Ultimate Animated Animals", https://quaternius.com/packs/ultimateanimatedanimals.html, CC0) |
| `spider_b.glb` | Spider, https://poly.pizza/m/yRYJiAJyiM |
| `slime_b.glb` | Slime, https://poly.pizza/m/LyjSUKHKnh |
| `frog_b.glb` | Frog, https://poly.pizza/m/37wofOCOzG |

Quaternius publica tudo em CC0 1.0 (https://creativecommons.org/publicdomain/zero/1.0/); crédito opcional, registrado aqui. Fora de uso por não serem CC0: nenhum
(o QAL do Quaternius não foi usado). Montagem, retexture e medidas: `docs/arte/familias-monstro.md` e `tools/art/familias-monstro/`.
