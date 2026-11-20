# Paladino (Cavaleiro): Fé

> **Aprovado pelo DIogo em 19/11.** O ID continua `herald`, e só o nome exibido muda (antes: Arauto). Os números são propostas ancoradas nas skills de hoje. A calibragem no simulador vem na fase de conteúdo do Cavaleiro. As regras gerais estão no [README](README.md).

## Papel e identidade

**Suporte de combate com dano sagrado.** O Paladino luta na linha de frente, abençoa o grupo e pune quem enfraquece. **Ele não é curandeiro.** A cura é pequena, tem recarga e serve para segurar o grupo, não para sustentar um tanque sozinho (decisão pendente 8 do design, com esta como proposta).

Na caça solo ele mata mais rápido que o Bastião e aguenta menos.

## Armas

| Mão | Permitido |
|---|---|
| Principal | espada |
| Secundária | escudo (o bloqueio vale igual, mas não gera Fé) |

O foco sagrado (símbolo ou grimório) fica para depois do lançamento.

## Dano sagrado

- As skills marcadas **sagradas** usam o ataque físico, e a defesa física do alvo reduz o dano como em qualquer golpe.
- O que muda é que elas **geram Fé** e recebem os bônus de Zelo e Fervor.
- **Não é** um tipo de dano novo com resistência própria. Isso evita mexer na fórmula de dano dos monstros agora.

## Recurso: Fé

O máximo é 100. Seguem as regras gerais do README: teto de 25 por segundo, até 3 alvos por lançamento, e decaimento de 10 por segundo depois de 5 s fora de combate.

| Gera | Quanto |
|---|---|
| Acertar uma skill sagrada | +8 (+2→6 com Veredito no Golpe Radiante) |
| Crítico sagrado, com Fervor | +3 |
| Bênção ou cura em um aliado que **não esteja com vida cheia nem com a bênção ativa** | +4 por aliado (máx. 3 por lançamento) |
| Bênção em si mesmo | +4 |
| Abate de inimigo em que você causou dano | +10 |

A regra "o aliado precisa ganhar algo" é o "suporte efetivo" do design. Abençoar o grupo de vida cheia e já abençoado não gera Fé, e assim o recurso não enche fora de combate.

| Gasta | Quanto |
|---|---|
| Renovação | 20 |
| Bênção Suprema (chave) | 60 |
| Sacrifício (chave) | 50 |
| Sentença Sagrada (chave) | toda (+1% de dano por ponto, até +100%) |

Com Escudo da Fé, todo gasto de Fé também dá um escudo.

**Ciclo sem recurso:** Golpe Radiante, Martelo da Aurora, Corte do Horizonte, Bênção da Luz e Sol Nascente custam só MP.

## Árvore (marcos 20–40)

Os IDs de 100 em diante são **provisórios**. ★ marca as 2 iniciais. "(evol.)" indica a versão evoluída de uma skill da base, com ID novo. ✝ marca as skills sagradas.

### Julgamento: dano sagrado

| Marco | Skill | Tipo | Ranks | Efeito (rank 1 → máx.) | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Golpe Radiante ✝ (100, nova) | ativa | 3 | 170→210% do ataque | 12 MP / 6 s | — |
| 20 | Martelo da Aurora ✝ (64) | ativa | 3 | 190→250% no alvo e a até 3 m, sem errar; os atingidos perdem 15→25% de ataque por 6 s | 30 MP / 14 s | — |
| 20 | Corte do Horizonte ✝ (50) | ativa | 3 | 200→260% no alvo e a até 3,5 m; os atingidos ficam 40% mais lentos por 3 s | 32 MP / 15 s | — |
| 20 | Zelo (101) | passiva | 5 | +2→10% de dano das skills sagradas | — | — |
| 25 | Veredito (102) | modificador do Golpe Radiante | 3 | +10→30% contra alvos enfraquecidos ou cegos; +2→6 Fé por acerto | — | Golpe Radiante 1 |
| 30 | Fervor (103) | passiva | 5 | +1→5% de chance de crítico nas sagradas; crítico sagrado gera +3 Fé | — | 5 no ramo |
| 35 | Aurora Ardente (104) | modificador do Martelo | 3 | +1→3 m de raio; o enfraquecimento dura +2→4 s | — | 10 no ramo |
| 40 | **Sentença Sagrada** ✝ (105) | chave | 1 | 450%, sem errar; gasta toda a Fé e ganha +1% de dano por ponto gasto | 40 MP + **toda a Fé** / 45 s | 15 no ramo |

### Bênçãos: suporte e fortalecimento

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | ★ Bênção da Luz (106, nova) | ativa | 3 | Você e os aliados a até 12 m ganham +10→20% de ataque e +10→20% de defesa por 15 s | 20 MP / 30 s | — |
| 20 | Bênção da Prontidão (65, antes "Bênção do Arauto") | ativa | 3 | Você e os aliados a até 12 m ganham +15→25% de velocidade de ataque por 15 s | 40 MP / 40 s | — |
| 20 | Aura de Coragem (107, evol.) | passiva | 5 (na base, 3) | Você e os aliados a até 10 m regeneram 2→10 de vida por segundo | — | — |
| 25 | Bênção Ampliada (108) | modificador da Bênção da Luz | 3 | +5→15 s de duração | — | Bênção da Luz 1 |
| 30 | Renovação (109) | ativa | 3 | **Cura limitada:** você e os aliados a até 10 m recuperam 8→12% da vida máxima ao longo de 6 s | 35 MP + **20 Fé** / 40 s | 5 no ramo |
| 35 | Graça (110) | passiva | 3 | Curas e escudos que você dá ficam +5→15% mais fortes | — | 10 no ramo |
| 40 | **Bênção Suprema** (111) | chave | 1 | Renova as suas bênçãos ativas nos aliados a até 15 m e dobra o efeito delas por 8 s | **60 Fé** / 90 s | 15 no ramo |

### Expiação: debilitação e proteção

| Marco | Skill | Tipo | Ranks | Efeito | Custo / Recarga | Requisito |
|---|---|---|---|---|---|---|
| 20 | Sol Nascente ✝ (66) | ativa | 3 (hoje 1) | 250→310% a até 7 m e cega os inimigos (35% mais erros) por 5 s | 60 MP / 50 s | — |
| 20 | Escudo da Fé (112) | passiva | 5 | A cada 10 de Fé gasta, ganha um escudo de 1→3% da vida máxima por 10 s | — | — |
| 25 | Luz Cegante (113) | modificador do Sol Nascente | 3 | +5→15% na cegueira e +1→3 s de duração | — | Sol Nascente 1 |
| 30 | Purificar (114) | ativa | 3 | Tira 1 efeito negativo (veneno, queimadura, praga, lentidão ou prender) de você e de 1→3 aliados a até 10 m e cura 3% da vida máxima de cada um | 25 MP / 20→14 s | 5 no ramo |
| 35 | Penitência ✝ (115) | ativa | 3 | 120% e marca o alvo por 8 s: ele recebe +8→16% de dano sagrado e causa −10% de dano | 25 MP / 20 s | 10 no ramo |
| 40 | **Sacrifício** (116) | chave | 1 | Por 6 s, os aliados a até 10 m não caem abaixo de 25% de vida: o dano que passaria disso vai para você | **50 Fé** / 120 s | 15 no ramo |

### Conta da árvore

| Ramo | Ranks | Ranks no marco 20 |
|---|---|---|
| Julgamento | 26 | 14 |
| Bênçãos | 21 | 11 |
| Expiação | 18 | 8 |
| **Total** | **65** (1,59× os 41 pontos do 40) | **33** (21 pontos no 20) |

- **Ativas:** 10, contando as chaves Sentença Sagrada, Bênção Suprema e Sacrifício. Uma build completa usa 6–7 teclas.
- **Builds esperadas no 40:**
  - Julgamento + Expiação: dano solo, com Penitência e Sentença;
  - Bênçãos + Expiação: suporte de grupo;
  - Julgamento + Bênçãos: híbrido.

## Ao evoluir

- **Iniciais:** Golpe Radiante e Bênção da Luz, ambas com rank 1.
- **Hotbar:** Golpe de Escudo (1) vira Golpe Radiante (100). Fortaleza (2) vira Bênção da Luz (106). Os IDs 50 e 64–66 ficam onde estavam.
- **Equipamento:** nada muda (espada e escudo).
- **Bônus de atributo:** **+6 Força e +6 Sabedoria**, no lugar dos +12 Sabedoria de hoje, porque o dano sagrado usa o ataque físico.

## Simulador: linha de base e metas

| | 20–24 | 25–29 | 30–34 | 35–40 |
|---|---|---|---|---|
| Tempo para matar hoje (Arauto) | 4,8 s (+44%) | 4,5 s (+53%) | 5,2 s (+64%) | 3,9 s (+59%) |

**Metas:**
1. **Tempo para matar:** no 20, igual ou menor que hoje (≤ 4,8 s). No 40, dentro de **+15% a +35%** da média. Hoje está em +59%.
2. **Tempo para morrer:** entre o do Bastião e o da média.
3. **Fé:** a Sentença Sagrada sai com 60+ de Fé a cada ~45 s numa caça solo contínua. A Renovação não pode sair mais de 1 vez por luta de 30 s.
4. **Cura:** em grupo de 3, a cura do Paladino (Renovação, Aura e Purificar) fica **abaixo de 30%** do dano recebido pelo grupo numa luta de chefe simulada. Não é curandeiro.

## O que o código precisa

- **Marca "sagrada"** na skill (`SkillDef.Holy`) e geração de Fé no `applyHit`.
- **Geração "efetiva" de Fé por aliado:** só conta o aliado sem a bênção ativa ou sem a vida cheia, com teto de 3. O gancho fica no laço de buffs de aliados do `castSkill`.
- **Cura em área ao longo do tempo** (Renovação): efeito novo `EffectRegen`, com valor e duração, aplicado a você e aos aliados.
- **Tirar efeito negativo** (Purificar): o consumível com `Cleanse` já faz isso, e o código passa a valer também para uma skill com alvo em aliados.
- **Piso de vida em aliados** (Sacrifício): uma variante do Imortal (`EffectUndying`) com piso de 25% e o excedente redirecionado ao Paladino. Usa o mesmo redirecionamento do Bastião.
- **Novos `PassiveKind`:** `PassiveHolyDamagePct`, `PassiveHolyCrit` e `PassiveHealPower`.

## Decisões do DIogo (19/11)

1. **Cura limitada:** fica como está acima (Renovação, Aura de Coragem e Purificar). O Paladino não é curandeiro.
2. **Bênção do Arauto** passa a se chamar **Bênção da Prontidão**. O ID continua o 65.
3. **Bônus de atributo:** +6 Força e +6 Sabedoria, no lugar de +12 Sabedoria.
4. **Sol Nascente:** fica no marco 20, com 3 ranks. Quem já tem a skill não perde o rank.

## Calibragem no simulador (Fase 9, 19/11)

| Skill | Doc | Calibrado |
|---|---|---|
| Golpe Radiante (100) | 170→210% | **250→310%** |

Resultado (tempo para matar, distância da média): 20–24 **5,0 s (+24%)**, 25–29 4,7 s (+18%), 30–34 5,8 s (+26%), 35–40 **4,1 s (+24%)**, dentro da meta do 40 (+15% a +35%). O dano por segundo no 20 (138) já passa o do Cavaleiro base no 19 (130).
