# Heróis v4, Fase 1: bíblia visual

Conceitos 2D gerados no Meshy (`nano-banana-pro`), com o corpo escolhido como referência de todas as folhas seguintes (`image-to-image`). Prompts em [tools/art/herois-v4/prompts.mjs](../../../../tools/art/herois-v4/prompts.mjs); kits descritos em [kits.md](../kits.md).

## Créditos

| Folhas | Chamadas | Créditos |
|---|---|---|
| Corpo (2 variações) | 2 text-to-image | 18 |
| Rostos e cabelos, recruta, 4 classes com as evoluções | 6 image-to-image | 54 |
| 28 duplas nas duas ordens | 10 image-to-image | 90 |
| **Total** | 18 | **162** (orçamento da fase: ~220) |

## Folhas

| Folha | O que tem |
|---|---|
| [corpo-a.jpg](corpo-a.jpg) | **escolhido.** Homem e mulher, frente, lado e costas, A-pose a 45°, corpo de guerreiro |
| [corpo-b.jpg](corpo-b.jpg) | descartado: corpo mais magro, braços colados ao corpo (ruim para o rig) |
| [rostos.jpg](rostos.jpg) | 3 rostos por gênero e 4 cabelos por gênero |
| [recruta.jpg](recruta.jpg) | recruta masculino e feminino, frente, lado e costas |
| [classe-cavaleiro.jpg](classe-cavaleiro.jpg) | Cavaleiro, Bastião e Paladino |
| [classe-barbaro.jpg](classe-barbaro.jpg) | Bárbaro, Berserker e Senhor da Guerra |
| [classe-mago.jpg](classe-mago.jpg) | Mago, Elementalista e Arcanista |
| [classe-ladino.jpg](classe-ladino.jpg) | Ladino, Assassino e Patrulheiro |
| [duplas-01.jpg](duplas-01.jpg) … [duplas-10.jpg](duplas-10.jpg) | as 28 duplas, cada uma nas duas ordens, na ordem da tabela de [kits.md](../kits.md#5-as-28-duplas) |
| [silhuetas-85px.png](silhuetas-85px.png) | todas as folhas em preto, no tamanho do jogo (85 px), ampliadas 3x |

## Teste de silhueta (85 px, em cinza)

| Nível | Resultado |
|---|---|
| Recruta | ✅ o "antes": silhueta lisa, sem nenhuma peça |
| Classes | ✅ as 4 se distinguem: escudo e elmo; chifres e machado; chapéu e cajado; capuz |
| Evoluções | ✅ Bastião (ameias, elmo de balde), Paladino (asas), Berserker (crânio de chifre), Senhor da Guerra (elmo de chifre), Assassino (capa em tiras), Patrulheiro (penas e arco). ⚠️ **Elementalista e Arcanista** viram o mesmo capuz pontudo de manto: cristais e correntes somem a 85 px |
| Duplas | ⚠️ metade se lê pela peça grande (estandarte, chifres, chapéu de aba, capuz de lobo, capa de fumaça). Na outra metade, a peça própria é pequena e a figura vira um bloco de manto ou placa |

## Problemas e correções propostas

| # | Onde | Problema | Correção |
|---|---|---|---|
| 1 | Elementalista × Arcanista | mesma silhueta | Elementalista: **cristais flutuando acima dos ombros**, maiores que a cabeça. Arcanista: **capuz caído para a frente e correntes pendendo até o chão** |
| 2 | Duplas que viram bloco | peça própria pequena | a peça de silhueta da dupla **30–50% maior** e saindo do contorno (para cima ou para os lados), como nos que passaram |
| 3 | Penitente (Assassino → Paladino) | saiu um macacão branco liso | refazer: gibão de couro tingido de branco, capuz e uma asa de placa num ombro |
| 4 | Espreitador | veio uma figura a mais (a do meio) | as válidas são a 1ª (capuz de lobo e arco) e a 3ª (capa de fumaça); a do meio é descartada |
| 5 | Chefe de Clã (Senhor da Guerra → Berserker) | couro em vez de bronze | refazer com a couraça de bronze |

Refazer 1, 2, 3 e 5 custa ~5 folhas (45 créditos), dentro do orçamento da fase.

## Portão da Fase 1

- [ ] Corpo A aprovado como base.
- [ ] Rostos e cabelos aprovados.
- [ ] Recruta, as 4 classes e as 8 evoluções aprovadas (com a correção 1).
- [ ] As 28 duplas aprovadas (com as correções 2 a 5).
- [ ] Teste de silhueta refeito depois das correções.
