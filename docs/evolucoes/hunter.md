# Patrulheiro (Ladino): Precisão

> **Aprovado pelo DIogo em 19/11**, que mandou seguir as propostas. O ID continua `hunter`, e só muda o nome exibido (antes: Caçador). Os números são propostas ancoradas nas skills de hoje e serão calibrados no simulador na fase de conteúdo do Ladino. As regras gerais estão no [README](README.md).

## Papel e identidade

É o DPS físico à distância: perseguição, armadilhas e controle de espaço. Mantém a distância, prepara o terreno e pune quem tenta chegar perto ou fugir. É a única evolução de dano físico que bate de longe.

## Armas

| Mão | Permitido |
|---|---|
| Principal | **arco** (2 mãos), **besta** (1 mão) ou **besta pesada** (2 mãos) |
| Secundária | — (adagas como secundária ficam para o futuro) |

- **Números das armas** (calibrados nesta fase; hoje são provisórios em `weapons.go`):

| Arma | Alcance | Intervalo | Ataque da arma |
|---|---|---|---|
| Arco | 14 m | 1,5 s | igual ao das adagas do mesmo grau |
| Besta | 13 m | 1,7 s | +15% |
| Besta pesada | 15 m | 2,2 s | +45% |

- **Itens:** arco, besta e besta pesada por grau (sem grau, D, C, B), com estoque no ferreiro.
- **Ao evoluir:** as adagas vão para a bolsa, e ele ganha **1 Arco comum**, uma vez.
- **Pré-requisito:** a Fase 3 (armas no cliente: modelo de arco e besta, animação de tiro, flecha e virote).

## Recurso: Precisão

Máximo de 100. Valem as regras gerais do README: teto de 25 por segundo, até 3 alvos por lançamento, e a barra cai 10 por segundo depois de 5 s fora de combate.

| Gera | Quanto |
|---|---|
| Acerto à distância com o alvo a 6 m ou mais | +6 |
| Acerto à distância com o alvo a 12 m ou mais | +10 |
| Acerto seguido no mesmo alvo | +2 extra por acerto seguido (até +8) |
| **Errar** | **zera a Precisão** |
| Golpe a menos de 6 m | 0 |

| Gasta | Quanto |
|---|---|
| Perseguição | 20 |
| Campo de Espinhos | 25 |
| Flecha Mortal | 40 |

Com Mira Perfeita (chave), a Precisão cheia faz o próximo tiro sair com crítico garantido e atravessar inimigos.

**Ciclo sem recurso:** Tiro Preciso, Disparo Perfurante, Faca de Arremesso, Rede, Armadilha e Miasma custam só MP.

## Árvore (marcos 20–40)

Os IDs a partir do 203 são **provisórios**. ★ marca as 2 skills iniciais.

### Arqueria: dano à distância

| Marco | Skill | Tipo | Ranks | Efeito (rank 1 → máx.) | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Tiro Preciso (203) | ativa | 3 | 180→230% à distância (alcance da arma) | 12 MP / 5 s | — |
| 20 | Disparo Perfurante (204) | ativa | 3 | 150→190% em até 3 inimigos em linha | 25 MP / 10 s | — |
| 20 | Olho de Falcão (205) | passiva | 5 | +1→5% de crítico e +0,5→2,5 m de alcance | — | — |
| 25 | Flecha Carregada (206) | modificador do Tiro Preciso | 3 | Com 50+ de Precisão, o Tiro Preciso causa +10→30% | — | Tiro Preciso 1 |
| 30 | Chuva de Flechas (84, antes "Chuva de Lâminas") | ativa | 3 (hoje 1) | 2 golpes de 160→200% no alvo e a até 5 m | 60 MP / 40 s | 5 no ramo |
| 35 | Flecha Mortal (207) | ativa | 3 | 350→420%, sem errar | 30 MP + **40 Precisão** / 25 s | 10 no ramo |
| 40 | **Mira Perfeita** (208) | chave | 1 | Com Precisão cheia, o próximo tiro sai com crítico garantido e atravessa até 3 inimigos | — | 15 no ramo |

### Armadilhas: controle de área

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | Rede de Caça (83) | ativa | 3 | 60% no alvo e a até 3 m, que ficam presos no lugar por 2,5→3,5 s | 30 MP / 20 s | — |
| 20 | Armadilha de Caça (209) | ativa | 3 | Armadilha no chão (dura 30 s): quem pisa leva 100→140% e fica preso por 2→3 s | 20 MP / 15 s | — |
| 20 | Miasma (60) | ativa | 3 | Envenena todos a até 6 m: 30→40% do ataque/s por 6 s | 40 MP / 20 s | — |
| 25 | Bomba de Fumaça (210, evol.) | ativa | 3 | Inimigos a até 4 m erram 20→40% mais por 6 s | 22 MP / 25 s | — |
| 30 | Campo de Espinhos (211) | ativa | 3 | Área de 6 m por 8 s: 20→30% do ataque/s e 30% de lentidão | 30 MP + **25 Precisão** / 30 s | 5 no ramo |
| 35 | Mestre Armadilheiro (212) | passiva | 5 | Armadilhas e redes causam +6→30% de dano e prendem por +0,1→0,5 s | — | 10 no ramo |
| 40 | **Emboscada** (213) | chave | 1 | Inimigos presos por armadilha ou rede recebem +25% de dano dos seus tiros | — | 15 no ramo |

### Caçada: marcação e perseguição

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Faca de Arremesso (82) | ativa | 3 | 180→230% à distância (até 16 m) e 30% de lentidão por 3 s | 20 MP / 6 s | — |
| 20 | Marca da Presa (214) | ativa | 3 | Marca o alvo por 12 s: +8→16% de dano seu nele | 10 MP / 15 s | — |
| 20 | Passo de Caça (215) | passiva | 5 | +2→10% de velocidade de movimento | — | — |
| 25 | Recuo (216) | ativa | 3 | Salta 6 m para trás e dispara 120→160% | 20 MP / 18→12 s | — |
| 30 | Perseguição (217) | ativa | 3 | Por 6 s, +30% de velocidade e +10→20% de dano contra o alvo marcado | 10 MP + **20 Precisão** / 25 s | 5 no ramo |
| 35 | Instinto do Caçador (218) | passiva | 3 | +5→15% de dano contra alvos lentos ou presos | — | 10 no ramo |
| 40 | **Caçada Implacável** (219) | chave | 1 | Matar o alvo marcado devolve 50 de Precisão e passa a marca para o inimigo mais próximo | — | 15 no ramo |

### Conta da árvore

| Ramo | Ranks | Ranks no marco 20 |
|---|---|---|
| Arqueria | 21 | 11 |
| Armadilhas | 21 | 9 |
| Caçada | 21 | 11 |
| **Total** | **63** (1,54× os 41 pontos do 40) | **31** (21 pontos no 20) |

**Quem já tem a spec:** a Chuva de Lâminas (84), agora Chuva de Flechas, passa do nível 28 para o marco 30. Entre o 28 e o 29, fica sem ela até o 30, com os pontos de volta. A Faca de Arremesso (82) e a Rede de Caça (83) ficam no 20.

## Ao evoluir

- **Iniciais:** Tiro Preciso (203) e Faca de Arremesso (82).
- **Hotbar:** Golpe Furtivo (7) vira Tiro Preciso (203), e Passo Sombrio (8) vira Faca de Arremesso (82). Os IDs 60 e 82–84 ficam onde estavam.
- **Equipamento:** as adagas vão para a bolsa, e ele recebe o Arco comum. Com a bolsa cheia, a evolução é recusada (como na troca de caminho).
- **Bônus de atributo:** continua +6 Força e +6 Destreza.

## Simulador: linha de base e metas

| | 20–24 | 25–29 | 30–34 | 35–40 |
|---|---|---|---|---|
| Tempo para matar hoje (Caçador) | 2,6 s (−21%) | 2,6 s (−14%) | 2,7 s (−13%) | 2,2 s (−10%) |

**Metas:**
1. **Tempo para matar:** entre **−10% e +5%** da média. Bater de longe já é uma vantagem, então ele não precisa ser o mais rápido.
2. **Tempo para morrer:** contando o tempo que o monstro leva para chegar perto, fica acima da média.
3. **Precisão:** quem fica longe e não erra mantém a barra entre 50 e 80, e quem luta de perto fica perto de 0.

## O que o código precisa

- **Fase 3 inteira:** arco e besta no cliente (modelo, animação de tiro, flecha e virote) e itens por grau.
- **Distância do acerto** no `applyHit`, para gerar Precisão. O erro zera (`ResetOnMiss`).
- **Armadilha no chão:** objeto com raio e duração, como o Estandarte e as zonas, que dispara quando um inimigo entra.
- **Disparo em linha** (Disparo Perfurante): alvos num retângulo à frente.
- **Recuo:** um "Piscar" para trás, seguido de um tiro.
- **Passar a marca** ao matar: gancho no abate.
- **Novos `PassiveKind`:** `PassiveRangeBonus`, `PassiveTrapPower`, `PassiveRootedDamagePct` e `PassiveMoveSpeedPct`.

## Decisões (19/11, propostas aceitas)

1. **Miasma** (ID 60) fica no Patrulheiro, no ramo Armadilhas.
2. **Chuva de Lâminas** passa a se chamar **Chuva de Flechas** (o ID 84 continua).
3. **Arco de presente** ao evoluir. As adagas vão para a bolsa.
4. Adagas como secundária ficam **para depois do lançamento**.

## Calibragem no simulador (Fase 12, 19/11)

**Armas (Fase 3):** o Patrulheiro usa arco, besta ou besta pesada, e não usa mais as adagas.
- **Itens:** cada arma do Ladino do nível 16 em diante ganha as três versões, com o mesmo grau e o mesmo bônus. O ataque da arma é igual ao das adagas no arco, +15% na besta e +45% na besta pesada.
- **Na loja:** o Ferreiro vende as três do grau C.
- **Ao evoluir:** ele ganha o Arco do Patrulheiro. Quem já era Patrulheiro recebe o arco no login, que já entra equipado.

Com o arco (tiro a cada 1,5 s, contra 0,8 s das adagas), as skills de dano da árvore ficaram com **115% do dano do doc**. Com as 8 árvores completas, ele ficou entre **−7% e +7%** da média (20–40), dentro da meta (−10% a +5%).
