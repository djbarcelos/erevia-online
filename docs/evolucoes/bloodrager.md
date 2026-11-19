# Berserker (Bárbaro): Fúria

> **Aprovado pelo DIogo em 19/11.** O ID continua `bloodrager`, e só o nome exibido muda (antes: Sanguinário). Os números são propostas ancoradas nas skills de hoje. A calibragem no simulador vem na fase de conteúdo do Bárbaro. As regras gerais estão no [README](README.md).

## Papel e identidade

**DPS físico corpo a corpo:** pressão sustentada, explosão de dano e execução. O Berserker aceita riscos para manter a pressão. Ele é recompensado por atacar e ficar na luta, não por esperar. **O ciclo principal não depende de MP:** é a evolução que resolve o "Bárbaro sem mana" do design (seção 5).

## Armas

| Mão | Permitido |
|---|---|
| Principal | **machado de duas mãos** (identidade) ou machado de uma mão |
| Secundária | — (sem escudo) |

- **Machado de duas mãos:** os números de hoje são provisórios (2,4 m de alcance, 1,8 s entre golpes) e ficam calibrados nesta fase. Proposta: **+25% de ataque físico da arma** em relação ao machado de uma mão do mesmo grau, para compensar o intervalo maior.
- **Itens:** hoje não existe nenhum machado de duas mãos. Entra uma linha por grau (sem grau, D, C, B), mais o estoque no ferreiro.
- **Ao evoluir:** quem não tem um recebe **1 Machado de Duas Mãos comum**, uma vez.

## Recurso: Fúria

O máximo é 100. Seguem as regras gerais do README: teto de 25 por segundo, até 3 alvos por lançamento, e decaimento de 10 por segundo depois de 5 s fora de combate.

| Gera | Quanto |
|---|---|
| Ataque básico acertado | +6 (+12 com Fúria Incontrolável) |
| Skill acertada (por alvo, até 3) | +4 |
| Crítico | +3 |
| Receber dano de inimigo | +2 por golpe |
| Cólera | +20 ao lançar |
| Golpe Brutal | +10 (+5→15 a mais se for crítico, com Golpe Esmagador) |
| Matar o alvo da Marca da Presa | +30 |

| Gasta | Quanto |
|---|---|
| Golpe Rubro | 20 |
| Machadada Sísmica | 25 |
| Redemoinho Sangrento | 30 |
| Ruptura | 30 |
| Execução | 35 |
| Carnificina | 50 |

Com **Ira Desmedida** (chave), os gastos caem 30%.

**Ciclo sem recurso:**
- Golpe Brutal, Cólera, Frenesi e Marca da Presa custam só MP, e pouco.
- Com Fúria vazia, o Berserker bate com o Golpe Brutal e os ataques básicos e enche a barra em cerca de 6–8 s de luta.

## Árvore (marcos 20–40)

Os IDs de 117 em diante são **provisórios**. ★ marca as 2 iniciais. "(evol.)" indica a versão evoluída de uma skill da base.

### Carnificina: dano e golpes pesados

| Marco | Skill | Tipo | Ranks | Efeito (rank 1 → máx.) | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Golpe Brutal (117, evol.) | ativa | 3 | 220→280% do ataque | 10 MP / 6 s | — |
| 20 | Machadada Sísmica (52) | ativa | 3 | 240→300% no alvo e a até 3 m; +50% em quem estiver abaixo de 30% de vida | **25 Fúria** / 16 s (hoje 35 MP) | — |
| 20 | Machado Pesado (118, evol.) | passiva | 5 | +8→40% de dano crítico | — | — |
| 25 | Golpe Esmagador (119) | modificador do Golpe Brutal | 3 | +10→30% de dano; no crítico, +5→15 Fúria | — | Golpe Brutal 1 |
| 30 | Ruptura (120) | ativa | 3 | 160→200% e sangramento de 30→50% do ataque por segundo, por 6 s | **30 Fúria** / 12 s | 5 no ramo |
| 35 | Abalo (121) | modificador da Machadada | 3 | +1→3 m de raio; os atingidos ficam 30% mais lentos por 2→4 s | — | 10 no ramo |
| 40 | **Ira Desmedida** (122) | chave | 1 | Skills que gastam Fúria custam 30% menos e têm +15% de chance de crítico | — | 15 no ramo |

### Frenesi: velocidade e sequência

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Cólera (123, evol. da Fúria) | ativa | 3 | +30→50% de ataque por 10 s; gera 20 Fúria | 15 MP / 25 s | — |
| 20 | Frenesi (125, evol.) | ativa | 3 | +25→45% de velocidade de ataque por 12 s | 20 MP / 25 s | — |
| 20 | Redemoinho Sangrento (53) | ativa | 3 | 2 golpes de 90→120% em todos a até 4 m; a Sede de Sangue cura a cada golpe | **30 Fúria** / 12 s (hoje 30 MP) | — |
| 20 | Sede de Sangue (124, evol.) | passiva | 5 | Recupera 2→10% do dano causado como vida | — | — |
| 25 | Turbilhão (126) | modificador do Redemoinho | 3 | +10% / +20% de dano; no rank 3, +1 golpe (3 golpes) | — | Redemoinho 1 |
| 30 | Sequência Selvagem (127) | passiva | 5 | Cada ataque básico seguido no mesmo alvo dá +2→4% de velocidade de ataque, até 5 acúmulos (perde tudo após 3 s sem bater) | — | 5 no ramo |
| 35 | Fúria Incontrolável (128) | ativa | 3 | Por 8 s, os ataques básicos geram o dobro de Fúria, e **você recebe +10% de dano** | 20 MP / 40→30 s | 10 no ramo |
| 40 | **Sede Insaciável** (129) | chave | 1 | Abaixo de 50% de vida, a Sede de Sangue cura o dobro e você ganha +20% de velocidade de ataque | — | 15 no ramo |

### Execução: finalização e risco

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | Golpe Rubro (67) | ativa | 3 | 260→330%; +30% em alvos abaixo de 50% de vida | **20 Fúria** / 10 s (hoje 25 MP) | — |
| 20 | Fúria Cega (68) | passiva | 5 (hoje 3) | +2→10% de chance de crítico (hoje 3→9%) | — | — |
| 25 | Marca da Presa (130) | ativa | 3 | Marca o alvo por 10 s: +8→16% de dano seu nele; matá-lo devolve 30 Fúria | 10 MP / 15 s | — |
| 30 | Execução (131, evol.) | ativa | 3 | 300→350%; o dobro em alvos abaixo de 30% de vida (em chefes, no máximo +50%) | **35 Fúria** / 25 s | 5 no ramo |
| 35 | Carnificina (69) | ativa | 3 (hoje 1) | 3 golpes de 110→140% em todos a até 5 m (a Sede de Sangue cura a cada golpe) | **50 Fúria** / 45 s | 10 no ramo |
| 40 | **Carnificina Final** (132) | chave | 1 | Matar com Execução ou Golpe Rubro zera a recarga dele (uma vez a cada 10 s) | — | 15 no ramo |

### Conta da árvore

| Ramo | Ranks | Ranks no marco 20 |
|---|---|---|
| Carnificina | 21 | 11 |
| Frenesi | 26 | 14 |
| Execução | 18 | 8 |
| **Total** | **65** (1,59× os 41 pontos do 40) | **33** (21 pontos no 20) |

- **Ativas:** 10. **Builds esperadas no 40:**
  - Carnificina + Execução: golpes pesados com finalização;
  - Frenesi + Carnificina: rápido, em área, com roubo de vida;
  - Frenesi + Execução: duelo contra um alvo.
- **Quem já tem a spec:** a Carnificina (69) passa do nível 28 para o marco 35. Entre o 28 e o 34, o personagem perde a skill até o 35, e os pontos voltam para distribuir (regra de migração do README).

## Ao evoluir

- **Iniciais:** Golpe Brutal (117) e Cólera (123), ambas com rank 1.
- **Hotbar:** Golpe Brutal (3) da base vira 117, e Fúria (4) vira Cólera (123). Os IDs 52, 53 e 67–69 ficam onde estavam.
- **Equipamento:** o machado de uma mão continua valendo, e o escudo vai para a bolsa. Quem não tiver um machado de duas mãos ganha 1 Machado de Duas Mãos comum, uma vez.
- **Bônus de atributo:** continua +12 Força.
- **Nome:** a skill-base "Fúria" vira **Cólera** na evolução, porque Fúria passa a ser o nome do recurso.

## Simulador: linha de base e metas

| | 20–24 | 25–29 | 30–34 | 35–40 |
|---|---|---|---|---|
| Tempo para matar hoje (Sanguinário) | 3,1 s (−7%) | 2,4 s (−18%) | 2,2 s (−29%) | 2,0 s (−19%) |

**Metas:**
1. **Tempo para matar:** entre **−20% e −5%** da média em todas as faixas, porque é a evolução de DPS corpo a corpo. Hoje passa do limite no 30–34 (−29%).
2. **Tempo para morrer:** abaixo da média. É o preço do risco.
3. **Mana:** numa luta de 60 s, o Berserker **nunca** fica parado por falta de MP. A métrica de mana sustentada vem da Fase 8.
4. **Fúria:** fica em média entre 40 e 80 numa luta contínua. A Carnificina sai cerca de 1 vez por luta de chefe de 60 s.

## O que o código precisa

- **Machado de duas mãos de verdade:** itens por grau e números finais em `weapons.go`. O teste `twoHandedAxe` passa a usar o item real.
- **Sangramento:** um efeito de dano por segundo como o veneno, com outro nome no cliente. Pode ser o `EffectPoison` com outro `Label`, para não criar regra nova.
- **Acúmulos de velocidade de ataque** (Sequência Selvagem): estado por jogador, como o `ambushCrit`.
- **Zerar a recarga ao matar** (Carnificina Final): gancho no abate.
- **Teto de execução em chefe:** regra global de +50% em chefes. Vale também para o Golpe Fatal e a Sentença.
- **Novos `PassiveKind`:** `PassiveAttackSpeedStack` e `PassiveResCostCut`.

## Decisões do DIogo (19/11)

1. **Machado de duas mãos:** +25% de ataque em relação ao de uma mão do mesmo grau, como ponto de partida para a calibragem.
2. A skill-base "Fúria" passa a se chamar **Cólera** na evolução.
3. **Fúria Incontrolável:** o risco é receber +10% de dano.
4. **Carnificina** no marco 35. Quem a tem entre o 28 e o 34 recebe os pontos de volta até chegar ao 35.
