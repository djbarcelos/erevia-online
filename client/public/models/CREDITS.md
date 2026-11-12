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
