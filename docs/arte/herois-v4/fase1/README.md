# Heróis v4, Fase 1: bíblia visual

Conceitos 2D gerados no Meshy (`nano-banana-pro`), com o corpo escolhido como referência de todas as folhas seguintes (`image-to-image`). Prompts em [tools/art/herois-v4/prompts.mjs](../../../../tools/art/herois-v4/prompts.mjs); kits descritos em [kits.md](../kits.md).

## Créditos

| Folhas | Chamadas | Créditos |
|---|---|---|
| Corpo (2 variações) | 2 text-to-image | 18 |
| Rostos e cabelos, recruta, 4 classes com as evoluções | 6 image-to-image | 54 |
| 28 duplas nas duas ordens | 10 image-to-image | 90 |
| Correções: folha do Mago e 7 folhas de duplas | 8 image-to-image | 72 |
| **Total** | 26 | **234** (orçamento da fase: ~220) |

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

Depois das correções (segunda rodada):

| Nível | Resultado |
|---|---|
| Recruta | ✅ o "antes": silhueta lisa, sem nenhuma peça |
| Classes | ✅ as 4 se distinguem: escudo e elmo; chifres e machado; chapéu e cajado; capuz |
| Evoluções | ✅ as 8 se distinguem. O Elementalista ganhou uma coroa de cristais e chamas acima da cabeça; o Arcanista ficou com capuz e correntes, que agora não se confunde com o chapéu do Mago nem com a coroa do Elementalista |
| Duplas | ✅ cada figura tem uma peça grande saindo do contorno: asas de penas, penacho, coroa de cristais, disco de sol, lanterna, capuz branco em ponta, máscara de bico, chapéu de aba com penas, círculo de runas, cântaro nas costas, estandartes e chifres |

## O que foi corrigido

| # | Onde | Antes | Depois |
|---|---|---|---|
| 1 | Elementalista × Arcanista | mesma silhueta | Elementalista com coroa de cristais e chamas acima da cabeça ([classe-mago.jpg](classe-mago.jpg)) |
| 2 | Duplas que viravam bloco (folhas 2, 3, 4, 7, 8 e 9) | peça própria pequena | peça própria grande, saindo do contorno |
| 3 | Penitente (Assassino → Paladino) | macacão branco liso | couro tingido de branco, capuz e uma asa de placa num ombro |
| 5 | Chefe de Clã (Senhor da Guerra → Berserker) | couro | couraça de bronze, elmo de chifres grandes e crânio no ombro |

**Ficou como está:**
- **Espreitador** ([duplas-10.jpg](duplas-10.jpg)): veio uma figura a mais. Valem a 1ª (capuz de lobo) e a 3ª (capa de fumaça).
- **Incendiário** (Berserker → Elementalista): a pele em brasa se lê pela cor, não pela silhueta. A 85 px em cinza ele fica parecido com o Berserker; no jogo, o brilho do emissivo resolve.
- **Campeão:** o estandarte de uma figura encosta na outra na folha; é só a composição da imagem.

## Portão da Fase 1

- [ ] Corpo A aprovado como base.
- [ ] Rostos e cabelos aprovados.
- [ ] Recruta, as 4 classes e as 8 evoluções aprovadas.
- [ ] As 28 duplas aprovadas.
- [x] Teste de silhueta refeito depois das correções.
