# Remaster: armadura visível que muda com o equipamento (tunnel#96)

> **Atualização (2026-11-17):** o remaster agora é o visual padrão do jogo. Onde este documento diz "atrás de `?remaster=1`" ou "com `?remaster=1`", leia "no padrão"; `?remaster=0` desliga, para comparar ("sem remaster"). `?render=antigo` é independente: troca só o render (tons, vento, água, nuvens, relevo), não a arte.

Sequência de `remaster-herois.md`: o peito, o elmo e a capa que o jogador veste mudam o que todo mundo vê, nas **5 raridades** (comum, bom, raro, épico, lendário). Tudo atrás de `?remaster=1`; sem o parâmetro nada muda.

Como olhar: `/?remaster=1`, vestir peças de raridades diferentes (a troca aparece na hora para todos) ou o laboratório `client/lab.html?remaster=1` no `vite dev` (`window.lab.show({ items: [{ model, armor: { chest, helm, cape } }] })`).

## O que o jogador vê

O corpo segue o **peito** equipado; o elmo e a capa seguem o item do próprio encaixe.

| Raridade do peito | Corpo | Ombreiras | Capa | Brilho |
|---|---|---|---|---|
| 1 comum | o modelo de sempre | | | |
| 2 bom | a mesma textura, mais clara e lisa, com um fio de aço azulado | | | |
| 3 raro | retexture A: aço prateado com esmalte azul-cobalto e filetes de prata | | | |
| 4 épico | retexture B: aço negro com ouro e vermelho | ombreiras épicas (asa de ouro e gema vermelha) | | |
| 5 lendário | retexture C: ouro radiante com branco e runas ciano | ombreiras lendárias (asas de ouro, gema ciano) | capa lendária (branca, com sol de ouro e runas) | dourado que pulsa de leve |

- **Capa** (encaixe Capa, ou o peito lendário): rara = capa azul até o joelho com águia de prata; épica = capa carmesim longa com emblema alado e gola alta; lendária = capa real branca com sol de ouro e runas. Comum e boa não mostram capa.
- **Elmo** (encaixe Cabeça): troca o `Hat` do corpo por uma variante da classe, quando a raridade do elmo é:

| Classe | Raro (3) | Épico ou lendário (4 e 5) |
|---|---|---|
| Cavaleiro | elmo de aço prateado com listras azuis, crista e asinhas | elmo negro e ouro com plumas carmesim |
| Bárbaro | o de sempre (cabeça nua) | elmo de chifres de marfim, ferro negro e pele |
| Mago | o chapéu de sempre | chapéu roxo alto com runas de ouro e gema violeta |
| Ladino | o capuz de sempre | capuz negro com bordas de ouro, mais alto e pontudo |

- **Pernas, luvas e botas:** o corpo é uma textura só, então não têm região própria. Contam só quando o peito está vazio (a melhor delas, no máximo rara). Ver "O que ficou em aberto".
- O "esconder chapéu" do Barbeiro esconde também as variantes de elmo.
- Raridade 5 (lendário): hoje só armas, anel e colar são lendários no servidor (peças do Rei Ossário); quando peito, elmo ou capa lendários entrarem, o cliente já os mostra.

## Protocolo

Nenhum campo no `oneof` e nenhuma migração. Dois campos livres e uma mensagem nova:

```proto
message ArmorLook { uint32 chest = 1; uint32 helm = 2; uint32 cape = 3; uint32 legs = 4; uint32 hands = 5; uint32 feet = 6; }
message Entity     { ...  ArmorLook armor = 29; }   // quem entra em vista
message Appearance { ...  ArmorLook armor = 7;  }   // a troca de peça, para todos
```

Cada número é a raridade (`Grade`, 1 a 5; 0 = encaixe vazio) do item equipado; ausente = nada equipado. O servidor monta no `inventory.armorLook()` e manda no `entityPB` e no `appearancePB` (que já era enviado a todos a cada troca de equipamento). Teste: `server/internal/zone/armor_look_test.go`.

## Como funciona no cliente

- **Corpo:** o retexture do Meshy mantém a malha e a UV do image-to-3d (conferido: 5.936 vértices, posições e UV idênticas), então a textura nova entra no mesmo GLB com rig e animações. `client/scripts/make-armor-tiers.mjs` junta o retexture com a textura original: pele e cabelo (canais G e B da máscara de repintura) voltam ao pixel original (o retexture deixava o rosto de mármore) e só a roupa e a armadura vêm do retexture. Saída: `public/models/armor/<classe>_<faixa>.ktx2` (1024 px, UASTC), 1,3 MB cada, baixada **só quando alguém na tela veste aquela faixa**.
- **Um material por classe e faixa**, compartilhado por todos que vestem aquilo: 50 jogadores de equipamentos diferentes não somam draw calls (cada personagem continua com as mesmas malhas) nem materiais além dos 4 x 4 possíveis. A repintura do Barbeiro (cores de pano, pele e cabelo) vale por cima: o material repintado leva a faixa na chave.
- **Peças anexadas** (ombreiras, capas, elmos): GLB estáticos de uma malha e um material, presos nos ossos `Spine2` e `Head` com `instantiateModelsToScene` (materiais compartilhados). A posição vem de `client/scripts/make-armor-attach.mjs`, que mede o corpo no repouso (largura dos ombros, altura da cabeça, caixa do `Hat`), põe a peça no lugar e converte para o espaço local do osso (`src/world/armor-attach.ts`, gerado): a peça acompanha o osso em qualquer animação, sem eixo calibrado à mão. As peças seguem o LOD do corpo (LOD1 a partir de 20 m).
- **Lógica pura** em `src/world/armor.ts` (raridade para faixa, ombreiras, capa e elmo), com teste em `tests/armor.test.ts`.
- `lab.html` ganhou `armor` por item e os prints saem de `tools/art/remaster-armaduras/lab.mjs`.

## Orçamento

Novo no guia (seção 5): **peça de armadura anexada ≤ 1.600 tri, LOD1 ≤ 650, 1 material, textura 512** (categoria `armadura` no `import-asset`/`check:assets`). O personagem com tudo vestido fica abaixo de 8.000 + o orçamento de cada anexo (o corpo de 7.3k a 7.4k fica como estava):

| Peça | LOD0 | LOD1 | Arquivo |
|---|---|---|---|
| Ombreiras épicas | 1.576 | 630 | `armor/pauldron_epic.glb` 393 KB |
| Ombreiras lendárias | 1.548 | 618 | `armor/pauldron_legend.glb` 405 KB |
| Capa rara | 1.056 | 390 | `armor/cape_rare.glb` 388 KB |
| Capa épica | 1.028 | 396 | `armor/cape_epic.glb` 392 KB |
| Capa lendária | 890 | 342 | `armor/cape_legend.glb` 358 KB |
| Elmo do cavaleiro raro | 877 | 274 | `armor/helm_knight_rare.glb` 377 KB |
| Elmo do cavaleiro épico | 1.066 | 490 | `armor/helm_knight_epic.glb` 376 KB |
| Chapéu do mago épico | 805 | 317 | `armor/hat_mage_epic.glb` 356 KB |
| Capuz do ladino épico | 916 | 344 | `armor/hood_rogue_epic.glb` 370 KB |
| Elmo de chifres do bárbaro | 1.044 | 348 | `armor/helm_barbarian_epic.glb` 370 KB |

Pior caso visível de uma vez (lendário): corpo 7,4k + ombreiras 1,5k + capa 0,9k + elmo 1,0k = ~10,8k triângulos no LOD0, contra os 8k do corpo antigo; no LOD1 (o normal a 20 m ou mais) 2,8k + 0,6k + 0,3k + 0,5k = 4,2k. `npm run check:assets` fica verde.

## Fluxo e créditos do Meshy

| Etapa | Chamadas | Créditos |
|---|---|---|
| Retexture das 4 classes em 3 faixas (raro, épico, lendário), `meshy-6`, `enable_original_uv` | 12 | 120 |
| Conceito 2D das peças (nano-banana, 3 créditos): 2 ombreiras, 3 capas, 5 elmos | 10 | 30 |
| Imagem para 3D das peças (smart topology, 800 a 1.500 tri) | 10 | 150 |
| **Total** | 32 | **300** de 400 |

Prompts em `tools/art/remaster-armaduras/prompts.mjs` (retexture) e `pieces.mjs` (peças); `import.sh` importa as peças. Sem repetição de task paga. O saldo da conta caiu mais do que isso porque outras frentes gastam na mesma conta.

## Prints (`docs/arte/remaster-armaduras/`)

- `<classe>-5-raridades-front.jpg` e `-back.jpg`: as 5 raridades lado a lado, da esquerda para a direita **lendário, épico, raro, bom, comum** (o laboratório põe o primeiro à direita; de costas a ordem se inverte).
- `<classe>-de-perto-three.jpg` e `-back.jpg`: raro, épico e lendário de perto (3/4 e costas).
- `4-classes-grau1|3|4-front|back.jpg`: as 4 classes em comum, rara e épica lado a lado.
- `jogo-solo-knight-<comum|rare|epic>.jpg` (e `-lado`, `-costas`): no jogo (`?remaster=1`, praça, servidor de verdade): um cavaleiro com peito, elmo e capa comuns, raros e épicos postos no banco e vistos pelo próprio cliente (`MODE=solo CLS=knight G=0|2|3 node tools/art/remaster-armaduras/jogo.mjs <pasta>`; o modo com vários jogadores também existe, mas numa máquina cheia o SwiftShader não deu conta de 3 páginas). A raridade lendária não aparece no jogo porque o servidor ainda não tem peito, elmo nem capa lendários (só armas, anel e colar); o cliente já os trata e o laboratório mostra como ficam.

## O que ficou em aberto

1. **Pernas, luvas e botas por raridade** sem região própria: pede uma máscara por região (quadril/pernas, mãos, pés) e um shader que misture as texturas de faixa por região, o que multiplica as texturas de corpo por combinação (50 jogadores diferentes passariam de 16 materiais para centenas). Ficou fora de propósito pelo teto de FPS; o corpo segue o peito.
2. **Textura de faixa de 1,3 MB cada** (UASTC 1024): vale testar ETC1S (cerca de 300 KB) e medir se o atlas aguenta.
3. **Ombreiras anexadas** atravessam o braço em poses de braço levantado (a peça é rígida no osso do tronco).
4. **A capa é rígida** (sem simulação de pano); em corrida a capa não balança.
5. **Tela de personagens e Barbeiro** (`preview3d`) ainda não mostram a armadura equipada.
6. **FPS de GPU real** continua sem medida (SwiftShader).
