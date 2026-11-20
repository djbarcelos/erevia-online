# Elementalista (Mago): Concentração

> **Aprovado pelo DIogo em 19/11**, que mandou seguir as propostas. Os números ainda são propostas ancoradas nas skills de hoje e serão calibrados no simulador na fase de conteúdo do Mago. As regras gerais estão no [README](README.md).

## Papel e identidade

É o DPS mágico à distância. Causa dano em área e aplica efeitos elementais, alternando fogo e gelo para bater forte, controlar grupos e aproveitar o alvo que já está queimando ou lento.

A **mana continua sendo o custo das magias**. A Concentração não substitui a mana: ela é o recurso que reforça as magias.

## Armas

| Mão | Permitido |
|---|---|
| Principal | cajado ou varinha |
| Secundária | grimório (só atributos; sem regra própria de combate) |

- **Ao evoluir:** quem não tem um grimório recebe 1 Grimório comum, uma vez.
- **Item novo:** o grimório entra como peça de mão secundária em todos os graus (Int e mana).

## Recurso: Concentração

Máximo de 100. Valem as regras gerais do README: teto de 25 por segundo, até 3 alvos por lançamento, e a barra cai 10 por segundo depois de 5 s fora de combate.

| Gera | Quanto |
|---|---|
| Lançar uma skill **diferente** da última lançada | +10 (a mesma skill duas vezes seguidas gera 0) |
| Ataque básico mágico acertado | +4 |
| Crítico mágico | +3 |
| Reação Elemental acertando um alvo queimado **e** lento | +10 |

| Gasta | Quanto |
|---|---|
| Meteoro | 30 |
| Cataclismo | 40 |
| Ruptura Elemental (chave) | toda (+2% de dano por ponto) |

**Ciclo sem recurso:** Bola de Fogo, Lança Ígnea, Nova de Gelo, Lança de Gelo, Inferno e Zero Absoluto custam só MP.

## Árvore (marcos 20–40)

Os IDs a partir do 150 são **provisórios**. ★ marca as 2 skills iniciais. "(evol.)" indica a versão evoluída de uma skill da base, com ID novo.

### Chamas: dano contínuo e explosão

| Marco | Skill | Tipo | Ranks | Efeito (rank 1 → máx.) | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Bola de Fogo (150, evol.) | ativa | 3 | 250→310% do ataque mágico, até 20 m | 15 MP / 3 s | — |
| 20 | Lança Ígnea (73) | ativa | 3 | 260→330% (até 20 m), de fogo | 28 MP / 6 s | — |
| 20 | Inferno (55) | ativa | 3 | 180→230% no alvo e a até 5 m; queimadura de 25→35%/s por 5 s | 50 MP / 18 s | — |
| 20 | Brasa Viva (151, evol.) | passiva | 5 | As skills de fogo queimam: 6→30% do ataque mágico/s por 3 s | — | — |
| 25 | Chama Persistente (152) | modificador da Bola de Fogo | 3 | A queimadura dura +1→3 s | — | Bola de Fogo 1 |
| 30 | Chuva de Brasas (153, evol.) | ativa | 3 | 160→200% no alvo e a até 5 m (alcance de 16 m) | 35 MP / 12 s | 5 no ramo |
| 35 | Meteoro (154, evol.) | ativa | 3 | 500→600% no alvo e a até 3 m; atordoa por 2 s | 50 MP + **30 Conc.** / 50 s | 10 no ramo |
| 40 | **Coração do Vulcão** (155) | chave | 1 | Skills de fogo em alvo queimado causam +20% e espalham a queimadura a até 3 m | — | 15 no ramo |

### Geada: lentidão e congelamento

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Nova de Gelo (156, evol.) | ativa | 3 | 120→160% a até 6 m e 50% de lentidão por 3→4 s | 30 MP / 10 s | — |
| 20 | Lança de Gelo (157, evol.) | ativa | 3 | 170→210% (até 18 m) e 30% de lentidão por 2 s | 18 MP / 5 s | — |
| 20 | Zero Absoluto (56) | ativa | 3 | 150→200% a até 6 m e congela (atordoa) por 1,5→2 s | 55 MP / 30 s | — |
| 20 | Frio Penetrante (158) | passiva | 5 | Alvos lentos recebem +2→10% de dano seu | — | — |
| 25 | Inverno Eterno (159) | modificador da Nova | 3 | A lentidão dura +1→3 s | — | Nova 1 |
| 30 | Prisão Glacial (160, evol.) | ativa | 3 | 50→80% e prende o alvo no lugar por 3→4 s | 40 MP / 30 s | 5 no ramo |
| 35 | Armadura de Gelo (161, evol.) | ativa | 3 | +30→50% de defesa por 20 s; quem bate de perto fica 30% mais lento | 30 MP / 40 s | 10 no ramo |
| 40 | **Coração do Inverno** (162) | chave | 1 | O próximo golpe seu num alvo congelado ou preso é crítico garantido | — | 15 no ramo |

### Convergência: combinação elemental

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | Reação Elemental (164) | ativa | 3 | 150→200%; o dobro se o alvo estiver queimado **e** lento (consome os dois efeitos) | 30 MP / 10 s | — |
| 20 | Foco Elemental (74) | passiva | 5 (hoje 3) | +9→45% de dano crítico | — | — |
| 25 | Convergência (165) | modificador da Reação | 3 | A Reação atinge também a até 1→3 m do alvo | — | Reação 1 |
| 30 | Cataclismo (75) | ativa | 3 (hoje 1) | 400→480% no alvo e a até 6 m, com 50% de lentidão por 4 s | 60 MP + **40 Conc.** / 60 s | 5 no ramo |
| 35 | Sobrecarga (163, evol.) | passiva | 3 | +5→15% de ataque mágico; as skills custam 10% a mais de mana | — | 10 no ramo |
| 40 | **Ruptura Elemental** (166) | chave | 1 | 300% a até 6 m, +2% por ponto de Concentração gasto (toda); aplica queimadura e lentidão | 40 MP + **toda a Conc.** / 45 s | 15 no ramo |

### Conta da árvore

| Ramo | Ranks | Ranks no marco 20 |
|---|---|---|
| Chamas | 24 | 14 |
| Geada | 24 | 14 |
| Convergência | 18 | 8 |
| **Total** | **66** (1,61× os 41 pontos do 40) | **36** (21 pontos no 20) |

**Quem já tem a spec:** o Cataclismo (75) passa do nível 28 para o marco 30. Entre o 28 e o 29, fica sem ele até o 30, com os pontos de volta.

## Ao evoluir

- **Iniciais:** Bola de Fogo (150) e Nova de Gelo (156), com rank 1.
- **Hotbar:** Bola de Fogo (5) vira 150, e Nova de Gelo (6) vira 156. Os IDs 55, 56 e 73–75 ficam onde estavam.
- **Equipamento:** cajado e varinha continuam. Ganha 1 Grimório comum se não tiver.
- **Bônus de atributo:** continua +12 Inteligência.

## Simulador: linha de base e metas

| | 20–24 | 25–29 | 30–34 | 35–40 |
|---|---|---|---|---|
| Tempo para matar hoje (Elementalista) | 1,6 s (−52%) | 1,2 s (−60%) | 1,6 s (−48%) | 1,2 s (−51%) |

**Metas:**
1. **Tempo para matar:** entre **−25% e −10%** da média. Hoje mata rápido demais, e quem corrige isso é a mana da Fase 8, não a árvore.
2. **Mana:** numa luta de 60 s, o Elementalista precisa escolher. A mana não pode durar a luta inteira lançando tudo o que sai da recarga.
3. **Concentração:** quem alterna as skills fica com a barra entre 50 e 80, e quem repete a mesma skill fica abaixo de 30.

## O que o código precisa

- **"Última skill lançada"** por jogador, para gerar Concentração só quando a skill muda.
- **Reação Elemental:** checar queimadura **e** lentidão no alvo e consumir os dois efeitos.
- **Crítico garantido no próximo golpe** em alvo congelado ou preso, no padrão do `ambushCrit` do Sumir.
- **Item de grimório** (`OffhandGrimoire` já existe em `weapons.go`) em todos os graus, mais o estoque no NPC.
- **Novos `PassiveKind`:** `PassiveSlowedDamagePct` e `PassiveFireSpread` (chave).

## Decisões (19/11, propostas aceitas)

1. **Grimório** só com atributos (Int e mana), sem regra de combate própria.
2. O **Cataclismo** vai para o marco 30 e passa a ter 3 ranks.
3. A **mana** segue como o custo principal das magias, e a Concentração só reforça.

## Calibragem no simulador (Fase 11, 19/11)

O Mago base já matava rápido demais (−50% da média no 19), e a mana não pesava. Na árvore do Elementalista, todas as skills de dano ficaram com **50% do dano do doc** e **1,5× o custo de mana**:
- **Dano:** a Bola de Fogo, por exemplo, ficou em 125→155% e 23 MP.
- **Tooltips:** o texto de cada skill mostra o valor calibrado.

Resultado:
- **Tempo para matar** (distância da média): 20–24 **−21%**, 25–29 −22%, 30–34 −27%, 35–40 **−26%**, na borda da meta (−25% a −10%).
- **Mana:** numa luta de 60 s sem poções, ela acaba em cerca de 40 s. Lançando tudo, não dá: é preciso escolher (meta 2).
