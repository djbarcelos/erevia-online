# Créditos do áudio

Todos os sons e músicas são **CC0 1.0** (domínio público): uso comercial liberado e crédito
opcional, mas fica registrado aqui. A licença de cada pacote foi conferida na página de origem.

Arquivos do jogo (gerados por `client/scripts/build-audio.mjs`, mono, MP3):

| Arquivo | Conteúdo | Tamanho |
|---|---|---|
| `sfx.mp3` | todos os efeitos, um atrás do outro (posições em `src/audio/sprite.json`) | ~330 KB |
| `music/town.mp3` | Cidade | ~510 KB |
| `music/wild.mp3` | Arredores e Campos dos Ossos | ~495 KB |
| `music/dark.mp3` | Ermos Partidos, Vale das Pedras Vivas e além | ~730 KB |
| `music/crypt.mp3` | Cripta Esquecida (raid boss) | ~470 KB |
| `amb/forest.mp3`, `amb/town.mp3`, `amb/night.mp3`, `amb/wind.mp3` | fundos ambientes | ~140–180 KB cada |

Total: ~3,1 MB. Música e ambiente carregam só quando o jogador entra na região (ou anoitece).

## Efeitos

| Pacote | Autor | Licença | Origem | Usado em |
|---|---|---|---|---|
| RPG Audio ("50 RPG sound effects") | Kenney | CC0 | https://opengameart.org/content/50-rpg-sound-effects (original: https://kenney.nl/assets/rpg-audio) | `chop`, `knifeSlice`, `knifeSlice2` (golpes), `footstep00–03` (passos), `bookOpen`/`bookClose` (janelas), `handleCoins` (comprar), `handleSmallLeather` (pegar item), `dropLeather` (morte de jogador) |
| Interface Sounds | Kenney | CC0 | https://opengameart.org/content/interface-sounds (original: https://kenney.nl/assets/interface-sounds) | `click_001` (clique), `error_005` (encantamento falhou) |
| Impact Sounds | Kenney | CC0 | https://kenney.nl/assets/impact-sounds, baixado na coletânea "150 Combat Sound Effects" de untamedpixels (CC0): https://opengameart.org/content/150-combat-sound-effects | `impact_punch_medium_000–002` (acerto), `impact_punch_heavy_000–001` (crítico), `impact_punch_heavy_002–003` (Golpe de Escudo, golpes pesados) |
| Sci-fi Sounds | Kenney | CC0 | https://kenney.nl/assets/sci-fi-sounds, pela mesma coletânea | `explosion_crunch_000` (Meteoro, golpes do raid boss) |
| Basic Spell Impacts | lentikula | CC0 | https://lentikula.itch.io/freecc0-basic-spell-impacts-sfx, pela mesma coletânea | `Fire Spell Impact 1–2` (fogo), `Ice Spell Impact 1–2` (gelo) |
| Druid Spell Impacts | lentikula | CC0 | https://lentikula.itch.io/druid-spell-impacts, pela mesma coletânea | `Earth Spell Impact 1` (Rachar o Chão, Pisotear), `Wind Spell Impact 1` (Passo Sombrio, Sumir, Piscar, Fumaça), `Plant Spell Impact 1` (veneno, Praga) |
| Healing Spell Impacts | lentikula | CC0 | https://lentikula.itch.io/healing-spell-impacts, pela mesma coletânea | `Healing Spell Impact 1` (Fortaleza, Escudo Arcano) |
| RPG Sound Pack | artisticdude | CC0 | https://opengameart.org/content/rpg-sound-pack | `swing`, `swing2`, `swing3` (erro, Corte Giratório), `magic1` (magia básica), `sword-unsheathe` (Grito de Guerra, Provocar, Estandarte), `bottle` (Frasco Tóxico), `bubble` (poção), `giant2` (Rugido, Fúria, Frenesi) |
| 80 CC0 RPG SFX | rubberduck | CC0 | https://opengameart.org/content/80-cc0-rpg-sfx | `spell_01` (skill genérica), `spell_02` (projéteis, Marca da Morte), `spell_fire_01` (lançar fogo), `stones_02` (morte do golem), `item_coins_01` (moedas), `item_gem_01` (encantamento) |
| Bones rattle | congusbongus | CC0 | https://opengameart.org/content/bones-rattle | `0.ogg`, `6.ogg` (ossos caindo) |
| SkeletonSFXShortPack | Colodical | CC0 | https://opengameart.org/content/skeletonsfxshortpack | `skeleton1sfx`, `skeleton2sfx` (morte de esqueleto e chefes) |
| Level up, power up, Coin get (13 Sounds) | wobbleboxx | CC0 | https://opengameart.org/content/level-up-power-up-coin-get-13-sounds | `Rise02` (subir de nível) |
| Hammer on Anvil | themightyglider | CC0 | https://opengameart.org/content/hammer-on-anvil | `forger.ogg` (ferreiro, de vez em quando na cidade) |

## Música

| Faixa | Autor | Licença | Origem | Uso |
|---|---|---|---|---|
| Medieval: Market Day (versão em laço) | RandomMind | CC0 | https://opengameart.org/content/medieval-market-day | `music/town.mp3` |
| Peaceful Forest | Samza | CC0 | https://opengameart.org/content/peaceful-forest | `music/wild.mp3` |
| Dark Forest Theme | cynicmusic | CC0 | https://opengameart.org/content/dark-forest-theme | `music/dark.mp3` |
| Dungeon Ambience | yd | CC0 | https://opengameart.org/content/dungeon-ambience | `music/crypt.mp3` (primeiro minuto) |

## Ambiente

| Som | Autor | Licença | Origem | Uso |
|---|---|---|---|---|
| Ambient Bird Sounds | isaiah658 | CC0 | https://opengameart.org/content/ambient-bird-sounds | `amb/forest.mp3` |
| Park ambiences | Thimras | CC0 | https://opengameart.org/content/park-ambiences | `amb/town.mp3` (birds, 60–90 s), `amb/wind.mp3` (wind, 30–55 s) |
| Crickets Ambient Noise - loopable | Wolfgang_ | CC0 | https://opengameart.org/content/crickets-ambient-noise-loopable | `amb/night.mp3` |

## Como regerar

Baixe os arquivos acima para uma pasta (fora do repositório ou em `assets-src/audio/`, que o
git ignora), com os zips descompactados em `x/<nome do zip>/`, as músicas em `music/`, os
ambientes e o `forger.ogg` em `amb/` e os dois `skeleton*sfx_0.wav` na raiz. Depois, dentro
de `client/`:

```bash
node scripts/build-audio.mjs ../assets-src/audio   # usa o ffmpeg do PATH ou o Docker (jrottenberg/ffmpeg)
```
