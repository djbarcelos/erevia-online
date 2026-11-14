# Erevia: roadmap de novas áreas, cidades e bestiário

> Proposta de direção criativa, 13/11/2026. Base: a main (cópia `lgpd-telemetria`, commit 027e470), o board (#64, #68, #71, #77, #79, #80, #81, #82, #86, #98, #99, #100), o relatório de FPS anexado à #64 e o guia de arte do estilo B (`docs/guia-de-arte.md`). As fontes externas estão na seção 13.
>
> Nada aqui foi aplicado ao repositório ou ao board. As tasks da seção 11 estão prontas para serem criadas.

---

## 0. Resumo das decisões

1. **Oito áreas novas**, níveis 7 a 40: Bosque Gris, Terras Mortas, Brejo de Lamagris, Minas de Brasaferro, Costa dos Naufrágios, Ruínas de Ilvaren, Dentes do Inverno e Coração de Brasa. Elas incluem o pântano, as minas e as ruínas da #68.
2. **Duas cidades novas** (Maresia, no nível 20, e Valdurn, no 30, com o castelo da #82) e **quatro postos** (Posto do Barqueiro, Bastião Cinzento, Acampamento Lanterna e Vigília de Ilvaren).
3. **Cinco masmorras instanciadas** para a #80. A primeira, as Catacumbas de Vel'Aran, só usa arte que já existe.
4. **Faixas que mais precisam de conteúdo:** 13–20 e 20–30. Hoje o 20–30 inteiro tem 12 golens (12 monstros para o servidor todo) e o 13–20 tem 20 esqueletos.
5. **O mapa atual cresce uma vez só,** para leste (Brejo e Terras Mortas), do mesmo jeito que cresceu ao sul com o Vale. Daí em diante, cada região grande é um **mapa separado**, numa zona própria do servidor, ligada por portões. É a mesma infraestrutura que as masmorras (#80) e os canais (#86) pedem.
6. **Todo monstro novo pertence a uma de 5 famílias de rig** (humanoide, quadrúpede, artrópode, amorfo e construto). Variantes saem de troca de paleta e de acessório. Os comuns usam VAT (#98). Assim o custo cresce por família, não por monstro.
7. **Antes de qualquer conteúdo vem a Fase A0 (desempenho e sistemas).** O celular simulado já roda a 8–10 FPS com o servidor vazio (#64). Pôr mais monstros sem o A0 piora o que já está ruim.

---

## 1. O jogo hoje: o que existe e onde estão os buracos

### 1.1 O mapa e as faixas (dados de `server/internal/game/monsters.go` e `client/scripts/build-world.mjs`)

| Região | Onde | Níveis | Monstros (quantidade) | Densidade |
|---|---|---|---|---|
| Vel'Aran | centro, raio 38 m (segura até 40) | — | — | — |
| Arredores | anel r 46–74 | 1–5 | 30 Lacaios | 1 por 350 m² |
| Campos dos Ossos | anel r 80–108 | 6–12 | 12 Guerreiros + 12 Ladinos | 1 por 690 m² |
| Ermos Partidos | anel r 116–140 | 13–20 | 10 Magos + 10 Guerreiros | 1 por 965 m² |
| Senhor dos Ossos | ponto (0, −132) | 22 | 1 chefe (renasce em 5 min) | — |
| Vale das Pedras Vivas | disco (0, −270), r 52 | 20–30 | 12 Golens | 1 por 708 m² |
| Cripta Esquecida | disco (108, 108), r 24 | 25 | Ozrhal (raid, 3–4 h) | — |

- **Total:** 88 monstros e 6 modelos (4 esqueletos KayKit, o golem em código e o Ozrhal, que reaproveita o esqueleto mago). Os limites do mapa vão de x −150 a 150 e de z −330 a 150 (+z é o norte e +x o leste).
- **Pedras de Teleporte (5):** Cidade (7, 7), Arredores (60, 5), Campos (5, 94), Ermos (−128, 5) e Vale (6, −198), custando de 20 a 220 moedas.
- **Os cantos do quadrado original não têm monstros.** Os anéis param em r 140, mas os cantos vão até r 212. São cerca de 28.400 m² vazios, e um deles abriga a Cripta. É espaço de graça para o A1.
- O nome do 4º anel, "Terras Mortas", já existe em `client/src/ui/regions.ts` (`RING_NAMES`) e não é usado. A área 2 o adota.

### 1.2 Buracos que o roadmap fecha

| Buraco | Número | Consequência |
|---|---|---|
| Faixa 20–30 rasa | 12 golens para o servidor inteiro, com renascimento de 45–90 s | Com 10 jogadores no Vale, sobra menos de 1 golem por jogador a cada 10 s. É a faixa mais longa (+10% de XP por nível acima do 19) |
| Um único tema | tudo é morto-vivo ou pedra | Nada muda de silhueta, de som ou de paleta do nível 1 ao 30 |
| Equipamento só até o nível 20 | itens nos níveis 1, 6, 13, 18 e 20 (`items.go`) | Do 20 ao 30 não há o que conquistar fora o raid. As áreas novas trazem receitas nos níveis 24 e 28 |
| 5 materiais | ossos, crânios, essência, núcleo, coroa e fragmento | O Ferreiro tem 24 receitas e nenhuma é de arma |
| Uma cidade | Vel'Aran | Não há hub de comércio nem cidade como recompensa |
| Monstros sem comportamento | a IA de `zone/monster.go` é vagar → perseguir → atacar → voltar | Todo monstro é o mesmo combate com números diferentes |
| Avisos no chão só no raid | 3 tipos (onda, chuva, marca), todos círculos | Os comuns não ensinam a desviar antes do raid |

### 1.3 O orçamento técnico de verdade (relatório da #64)

- **PC com o servidor vazio:** 60 FPS. Com 100 bots, cai para 18–34 FPS em todas as regiões (agora a AOI de 70 m da #88 limita o que chega ao cliente).
- **Celular simulado (CPU 4×):** 8–10 FPS com o servidor vazio. **O gargalo é a CPU**:
  - escolher as malhas ativas leva 43–54 ms por quadro;
  - o desenho leva 20–55 ms, com 130–160 draw calls.
- **O que pesa é o número de malhas e de entidades, não o tamanho do mapa.** Cada monstro animado KayKit tem cerca de 20 ossos e várias malhas. O golem tem cerca de 20 malhas por instância. Cada entidade tem 1 cilindro de clique.
- **Conclusão para o design:** o mapa pode crescer se o cenário estiver em thin instances divididas em blocos. Já os **monstros visíveis ao mesmo tempo** têm teto. Cada área nasce com contagem de entidades e de modelos fechada (seção 4).

---

## 2. Visão e pilares

### 2.1 A história que liga as regiões

Há três séculos, os **Ilvarin**, um povo de tecelões de runas, faziam a pedra obedecer: ergueram os monólitos do Vale e deram vida aos golens. **Ozrhal**, rei de Erevia, quis uma coroa que não morresse com ele e pagou com os ossos do próprio reino. Para detê-lo, os Ilvarin romperam os veios rúnicos sob os pés do rei.

**A Partição** rachou a terra de um horizonte ao outro: o rio Lamagris virou brejo, as minas de Brasaferro desabaram, a cidade dos Ilvarin caiu, navios se partiram no Recife dos Dentes. Ozrhal ficou preso na **Cripta Esquecida**, e o seu exército ficou plantado nos campos. **Vel'Aran** foi erguida sobre o campo de batalha para vigiar o selo.

Agora o selo racha:
- os mortos levantam à noite;
- as pedras do Vale acordaram;
- o Senhor dos Ossos, arauto do rei, ronda o sul;
- os **Devotos do Ossário** cantam na porta da Cripta.

O que sobrou do reino são as **Terras Partidas**. Cada região delas guarda uma das **cinco Pedras-Selo** que os Ilvarin espalharam para manter o rei preso. Juntar as cinco é a espinha das missões principais (#71) e a chave da Necrópole (seção 8).

### 2.2 Pilares (regras para decidir conteúdo)

1. **Toda região é uma cicatriz da Partição.** Cada monstro tem um motivo para estar ali: lodo porque o rio represou, kobolds porque o veio rúnico os atrai, ecos porque a cidade caiu cantando. *Teste:* se não dá para explicar em uma frase por que o monstro está naquela região, ele não entra.
2. **Legível a 85 px.** É a regra do guia de arte aplicada a monstros e ataques. O papel do monstro (bruto, atirador, curandeiro…) se lê pela silhueta, e o perigo se lê pelo aviso no chão. *Teste:* a 18 m de câmera e em tons de cinza, o jogador diz qual monstro bater primeiro.
3. **A noite cobra e paga.** O ciclo já existe: 40 min, sendo 10 de noite, com +15% de dano dos monstros, +25% de XP e percepção de 11 m em vez de 8. Cada área ganha uma camada noturna: monstros que só saem à noite, os raros, os eventos.
4. **Sozinho até o meio, junto no topo.** Os comuns se caçam sozinho. Raros e elites pedem atenção. Chefes de campo, eventos, masmorras e o raid pedem grupo ou clã.
5. **Leve por construção.** O orçamento faz parte do design:
   - no máximo 5 modelos de monstro por área;
   - no máximo 18 monstros no raio de 70 m;
   - variantes por paleta antes de modelo novo.

   *Teste:* `?bench=1` na região cheia, antes do merge.

---

## 3. Mapa de progressão

### 3.1 Faixas de nível (1 → 30, com o esboço 30 → 40)

Como no Lineage 2, cada faixa tem 2 ou 3 opções sobrepostas. Assim o jogador escolhe pelo tema e pelo risco, e os jogadores se espalham.

| Nível | Mundo aberto | Grupo/instância | Cidade/posto | Chefes |
|---|---|---|---|---|
| 1–5 | Arredores | — | Vel'Aran | — |
| 6–12 | Campos dos Ossos · **Bosque Gris** (7–15) | — | — | Capitão Ossudo (raro, 14) · Corvo Rubro (campo, 16) |
| 12–16 | — | **Catacumbas de Vel'Aran** (D1) | Vel'Aran (igreja) | Abade Ulric (D1) |
| 13–20 | Ermos Partidos · **Brejo de Lamagris** (14–20) | — | **Posto do Barqueiro** | Senhor dos Ossos (campo, 22) · Mãe-Lodo (campo, 21) |
| 16–22 | **Minas de Brasaferro** (Mapa 3) | **Galerias Profundas** (D2, 18–22) | **Acampamento Lanterna** | Tecelã das Galerias (campo, 23) |
| 20 | 2ª classe (#79, já feita) | — | **Maresia** abre | — |
| 20–26 | Vale das Pedras Vivas · **Costa dos Naufrágios** (Mapa 2) | — | **Maresia** | Rei-Pinça (campo, 27) |
| 24–30 | **Terras Mortas** (24–28) · **Ruínas de Ilvaren** (25–30, Mapa 4) · Vale (25–30) | **Santuário Submerso** (D3, 26–30) · raid Ozrhal | **Bastião Cinzento** · **Vigília de Ilvaren** | Malthus (campo, 29) · Colosso Sem Rosto (campo, 32) |
| 30 (máx.) | — | **Necrópole do Rei Ossário** (D4, com as 5 Pedras-Selo) | — | Ozrhal Desperto |
| 30–35 | **Dentes do Inverno** (Mapa 5) | — | **Valdurn** (castelo, #82) | Vharok, Wyrm de Geada (mundo, 36) |
| 35–40 | **Coração de Brasa** (Mapa 6) | **Forja Primeva** (D5, 38–40) | Valdurn | Brasa-Viva (campo, 39) |

### 3.2 Como uma área leva à outra

- **1 → 12:** o tutorial (#66) já leva aos Arredores e aos Campos. No nível 8, um NPC novo na praça (o Batedor Caio, irmão da Mercadora Lívia) abre a missão do Bosque Gris: a caravana do oeste sumiu.
- **12 → 16:** no nível 12, o Sacristão da igreja chama grupos para as Catacumbas. É o primeiro conteúdo de grupo e ensina os avisos no chão antes do raid.
- **14 → 20:** a estrada leste já vai da cidade até x = 150. Ela passa a seguir até o **Posto do Barqueiro** (165, 0), na entrada do Brejo. Pela estrada oeste, o **Passo de Brasaferro** (−148, 0), ao lado da pedra dos Ermos, leva às Minas, que abrem no nível 16.
- **20:** a 2ª classe (Ysolde, "A Encruzilhada") e a **chegada a Maresia** pela foz do Lamagris, no extremo leste do Brejo. **A cidade é a recompensa de travessia**: só quem foi a pé uma vez ganha o teleporte de Vel'Aran para Maresia (mecânica de descoberta, task T16).
- **24 → 30:**
  - a Ordem do Bastião Cinzento recruta para as Terras Mortas, em volta da Cripta;
  - Ysolde manda o jogador ao norte, para as Ruínas de Ilvaren, onde está o povo dela (gancho de lore que liga ao #79);
  - as 5 Pedras-Selo fecham no 30 e abrem a Necrópole.
- **30 → 40 (esboço):** das Ruínas, o portão norte leva às Terras Altas, a Valdurn e ao castelo. Sob a montanha de Valdurn fica o Coração de Brasa, a forja onde os Ilvarin fizeram as runas e o selo.

### 3.3 O mundo (entre mapas)

```
                      [Mapa 5: Terras Altas do Norte]  30–35
                      Dentes do Inverno · VALDURN (castelo #82)
                           |                 \
                           |                  [Mapa 6: Coração de Brasa]  35–40
                           |                   (instância D5: Forja Primeva)
                      [Mapa 4: Planalto de Ilvaren]  25–30
                      Ruínas de Ilvaren · Vigília de Ilvaren
                      (instância D3: Santuário Submerso)
                           |  portão norte (0, 150)
[Mapa 3: Serra de     ---- [Mapa 1: Vale de Vel'Aran] ---- foz (330, -60) ---- [Mapa 2: Costa de Maresia]
 Brasaferro] 16–22    (-150,0)  cidade, anéis, Vale, Cripta,                     MARESIA · Costa 20–26
 Minas · Acamp.                 Bosque Gris, Terras Mortas, Brejo
 Lanterna (D2: Galerias)        (D1: Catacumbas, D4: Necrópole)
```

### 3.4 Mapa 1 (o atual, crescido para leste no A2)

Os limites passam de x −150..150 para **x −150..330** (z continua de −330 a 150). O mapa vai a 480 × 480 m, o tamanho máximo do Mapa 1.

```
  x=-150                  x=0                    x=+150                  x=+330
z=+150 +--------------- portão N → Mapa 4 ----------+-----------------------+
       | TOCA DO CORVO   Campos(N)   pedra(5,94)    | TERRAS MORTAS (ext.)  |
       | (Corvo Rubro)                 Cripta(108,108)  Bastião (170,110)  |
       |  BOSQUE GRIS                  TERRAS MORTAS (núcleo, canto NE)    |
       |  setor NO 100°–170°                        +----------- z=+60 ----+
       |  r 80–150, nív. 7–15                       |                      |
portão O  anéis: 1–5 · 6–12 · 13–20                 | Posto do Barqueiro   |
(Mapa 3)=====VEL'ARAN(0,0)=====estrada leste========|==(165,0)             |
(-148,0)|  pedra Ermos(-128,5)                      | BREJO DE LAMAGRIS    |== foz (330,-60)
       |                                            | 14–20, Vau Velho     |   → Mapa 2
       |           Senhor dos Ossos (0,-132)        | (250,-40)            |
z=-150 +----------- estrada sul ---------------------+----------------------+
       |        VALE DAS PEDRAS VIVAS 20–30          | montanhas (sem acesso)|
z=-330 +---------------------------------------------+----------------------+
```

### 3.5 Tabela de adjacência

| De | Para | Ligação | Nível de quem atravessa |
|---|---|---|---|
| Vel'Aran | Arredores, Campos, Ermos, Vale | estradas N/S/L/O e pedras (já existe) | 1+ |
| Campos dos Ossos (setor NO) | Bosque Gris | contíguo; trilha a partir da estrada oeste (x −80) | 7+ |
| Ermos (leste) | Brejo de Lamagris | estrada leste até o Posto do Barqueiro (165, 0) | 14+ |
| Ermos (nordeste) | Terras Mortas | contíguo ao canto NE; o Bastião fica em (170, 110) | 24+ |
| Brejo (norte) | Terras Mortas (extensão) | Ponte de Troncos (atalho, abre por missão) em (200, 58) | 24+ |
| Brejo (foz) | Mapa 2: Costa de Maresia | portão em (330, −60) | 20+ |
| Ermos (oeste) | Mapa 3: Serra de Brasaferro | Passo de Brasaferro (−148, 0) | 16+ |
| Campos (norte) | Mapa 4: Planalto de Ilvaren | portão norte (0, 150), no fim da estrada | 25+ |
| Mapa 4 | Mapa 5: Terras Altas | portão norte do Planalto | 30+ |
| Mapa 5 | Mapa 6: Coração de Brasa | Elevador de Valdurn | 35+ |

**Regra de portão:** abaixo do nível indicado, o portão avisa ("Perigoso para o seu nível") mas deixa passar. É risco e recompensa, não muro.

---

## 4. Regras técnicas que valem para todas as áreas

### 4.1 Orçamento por área (vale para todas)

| Item | Teto | Por quê |
|---|---|---|
| Monstros na área | 30–45 | Capacidade de caça ≈ monstros × 10 s ÷ renascimento. Com 36 monstros a 40 s, cabem uns 9 caçando sem fila. Mais gente vai para um canal (#86) |
| Monstros no raio de 70 m (AOI) | **≤ 18** | Calibrado com o #64: até ~20 entidades animadas o PC fica em 60 FPS |
| Modelos de monstro distintos | **≤ 5 por área** (o raro e o chefe de campo são variantes) | Cada modelo é um conjunto de malhas e materiais, e o VAT custa um draw por modelo e LOD |
| Monstros noturnos | **trocam** os do dia, não somam | À noite, 30% dos spawns do dia somem (só os que estão fora de combate) e os noturnos ocupam o lugar. A contagem fica igual. Na T2 (pronta): a troca é **por região** (`Spawn.Region`), então em cada região a soma de `WhenDay` tem que ser igual à de `WhenNight` (teste `TestDefaultSpawnsBalanceDayAndNight`); quem está lutando na virada fica até a luta acabar e segura a vaga de um noturno |
| Setores e áreas com dono (T2) | ângulos em graus, 0° = leste (+X), crescendo para o norte (+Z): 90° = norte, 180° = oeste, 270° = sul; `AngleMin > AngleMax` cruza o 0° | Disco ou setor com `Region` tem dono: os anéis das outras regiões sorteiam fora dele (6 m de folga), **mudando de lugar sem mudar a contagem nem os ids**. Tirar monstros de um anel baixando o `Count` muda o id de todos os spawns seguintes; a conta "os anéis perdem 9 esqueletos" da 5.1 vira "9 esqueletos mudam de lugar", a não ser que a #137 decida outra coisa |
| Cenário | ≤ 1.500 objetos por bloco de 240 × 240 m, em thin instances divididas em células de 60 m | Hoje são cerca de 1.800 objetos no mapa todo, num bloco só |
| Malhas-fonte novas de cenário | ≤ 25 por bioma | Cada uma é pelo menos 1 draw call quando aparece |
| Partículas | só as do orçamento global da #101 | Vaga-lumes, cinzas e neve contam no mesmo limite |
| Draw calls | ≤ 150 no celular e ≤ 300 no PC (guia, seção 5) | — |

### 4.2 Famílias de monstro (custo por família, não por monstro)

Todo monstro novo herda o rig e as animações de uma família. O contrato de animação continua o de hoje (parado, andar, golpe, esmagar, rugido, dano, morte), para não mexer no servidor (decisão da #98). A **forma** do modelo segue o guia (silhueta, paleta, teste de 85 px). Os pacotes CC0 servem de **base de malha e de animação**, que o pipeline Blender (#90) refaz no estilo B e confere com `import-asset.mjs`.

| Família | Rig e animação de base | Membros neste roadmap | Variar por |
|---|---|---|---|
| **Humanoide** | o rig estilo B dos personagens (#94, ≤ 45 ossos), com as animações das 4 classes; no A1, os KayKit atuais | bandoleiros, devotos, brejeiros, kobolds (em escala 0,7), mineiros, piratas, ecos Ilvarin, saqueadores, fundidores | paleta, acessórios (capuz, elmo, cauda), arma da `public/models/weapons` |
| **Esqueleto** | a família atual (#98) | esqueletos, carniçais, afogados, cavaleiro ossário | paleta (algas, cinza, cobre), acessórios |
| **Quadrúpede** | Wolf/Husky da Quaternius Ultimate Animated Animals (CC0) como rig padrão | lobo cinzento, lobo da geada, javali, lince rúnico, salamandra | malha nova sobre o mesmo rig, paleta |
| **Artrópode** | Spider da Quaternius (CC0, animada) | aranha-de-musgo, aranha das galerias, caranguejo-couraça, Rei-Pinça, Tecelã | proporção das patas, paleta, carapaça |
| **Amorfo** | Blob da Quaternius Ultimate Monsters (CC0); shape keys → VAT | lodo vivo, Mãe-Lodo, diabrete de escória, wisp | cor, emissivo, escala |
| **Construto** | o golem (remodelado no Blender pela #98) | golem de ferro, sentinela de mármore, golem de gelo, golem de magma, vigia flutuante, Colosso, Brasa-Viva | paleta do atlas (rocha, mármore, gelo, magma), runas |

**Fora das famílias,** cada um desses é modelo único: Sapo-Boi, Yeti, gaivota, o Draco/Wyrm (base: Dragon da Quaternius, CC0) e o mecanismo do Colosso. Por isso entram só onde a área precisa.

**Licenças, para conferir no download:**
- A Quaternius passou a usar uma licença própria (QAL) em alguns pacotes, como o Bestiary e o Fantasy Races. Esses ficam de fora: guarde a licença e a data de cada download.
- Na poly.pizza, os modelos "Poly by Google" são CC-BY, então só entra o que estiver marcado "Public Domain (CC0)".
- Os extras pagos da KayKit também são CC0. O Skeletons Extra (Necromancer e Skeleton Golem) custa US$ 7,95.

### 4.3 Como pagar por mapas maiores e mais monstros

1. **Zonas separadas por mapa (servidor).** Cada mapa (de 1 a 6) é uma `zone.Zone` com goroutine, `world.json`, spawns e A* próprios, no mesmo processo. A transferência acontece por portão, com tela de carregamento de 2–4 s.
   - O cliente descarta o cenário anterior e carrega o kit do bioma sob demanda, como já faz com a música (#67).
   - É a mesma peça que as **instâncias** (#80) e os **canais** (#86) usam: uma zona criada a partir de um modelo de mapa.
2. **Cenário em blocos.** O `build-world` grava as thin instances por célula de 60 m (uma thin instance por tipo e por célula), e o Babylon descarta pelo frustum e pela névoa.
   - Montanhas da borda a mais de 120 m viram impostor (um quad).
   - Sem os blocos, o mapa de 480 m desenharia o cenário inteiro em cada quadro.
3. **Monstros por VAT com instâncias.** Na #98, os comuns viram uma malha por modelo e LOD, animada por `BakedVertexAnimationManager` com o buffer `bakedVertexAnimationSettingsInstanced`. Isso derruba o custo de esqueleto e de malhas ativas, os dois maiores gargalos da #64. Os chefes continuam com esqueleto.
4. **Clique sem malha.** Trocar o cilindro de clique de cada entidade (98 malhas na #64) por um teste de raio contra cilindros em código, sobre a lista de entidades: zero malhas a mais por monstro.
5. **LOD por distância:** LOD1 a partir de 20 m e LOD2 a partir de 35 m (guia). A AOI corta em 70 m, e o servidor já não faz vagar monstro sem jogador a menos de 45 m (`wakeRange`).
6. **Canais (#86) antes de áreas maiores.** Quando um mapa passa de cerca de 60 jogadores, abre-se o Canal 2 do mesmo mapa. Com isso, o "pico de 18 monstros mais N jogadores na tela" se mantém.

### 4.4 Arquétipos de inimigo (a IA nova, task T3)

Hoje a IA é uma só. A proposta é um campo `Archetype` em `MonsterDef`, mais parâmetros, com o comportamento em `zone/monster.go`. As áreas usam esta tabela:

| Arquétipo | Comportamento | Como o jogador lê | Contra-jogo |
|---|---|---|---|
| **Bruto** | persegue e bate forte e devagar (intervalo ≥ 2 s) | grande e lento; anima o "esmagar" | desviar do golpe avisado, lentidão |
| **Atirador** | mantém 7–10 m; se o jogador encosta, recua 5 m (no máximo 1 vez a cada 6 s) | arma longa, fica atrás | Investida e Piscar, atordoar, chegar por trás de pedras |
| **Curandeiro** | cura um aliado abaixo de 50% de vida em 15% (recarga de 8 s); foge do corpo a corpo | brilho verde na mão; nome em verde-água no quadro do alvo | matar primeiro, cegueira, silêncio |
| **Invocador** | chama 2 lacaios a cada 20 s (no máximo 4 vivos); os lacaios morrem com ele | círculo no chão antes de chamar | matar o invocador, área |
| **Emboscador** | escondido (invisível ou como objeto) até o jogador chegar a 4–5 m; o 1º golpe dá +50% | sem nome na tela até revelar; som de aviso | andar devagar, provocar de longe |
| **Matilha** | 3–5 ligados: bater em um puxa todos, e com o último vivo, frenesi de +20% de velocidade | mesmo modelo, um líder maior | puxar com cuidado, área, prender |
| **Escudo frontal** | toma 60% menos dano num cone de 120° à frente | escudo grande, vira devagar (90°/s) | flanquear, provocar e bater pelas costas, magia |
| **Dividir** | ao morrer, vira 2 cópias com 30% da vida e 2 níveis abaixo | massa amorfa | área, não deixar acumular |
| **Explosivo** | ao morrer, explode em círculo de 2,5 m depois de 1,0 s | brilho pulsando | sair do círculo |
| **Elite** | 3× a vida e 1,5× o dano, com 1 habilidade avisada; nome com moldura prateada | 1,3× maior | grupo de 2–3 |

**Avisos no chão para monstros comuns (T4).** Hoje só o raid tem aviso (`zone/raid.go`: onda, chuva e marca, com 2,0–5,0 s). A proposta generaliza para **círculo, cone, linha e anel**, com habilidades por dados (`Abilities []AbilityDef`). Os tempos:
- **Mínimo de 1,0 s em comuns.** A conta: ping de 150 ms + reação de 250 ms + sair de 3 m a 5 m/s (0,6 s).
- **1,5–2,5 s em elites e chefes de campo.**
- **2–5 s no raid**, como hoje.
- **Dano do aviso: 25–40% do dano recebido na luta.** Lição do WildStar: se o auto-ataque faz quase todo o dano, desviar parece inútil.

**Chefe de campo genérico (T5).** O modelo é o Lineage 2 e o Ragnarok, aproveitando o `raidState`:
- renasce num mínimo fixo mais uma janela aleatória (de 60 a 180 min, conforme a área), e o mapa mostra a janela, não o minuto (como `setWindow` já faz);
- fases por vida, em dados;
- a XP e o saque são divididos por dano (como o raid);
- anúncio para o mapa 5 min antes;
- um **túmulo** no ponto mostra a hora da morte e quem mais causou dano, como os MVPs do Ragnarok.

**Raros, os "named" (T6):**
- quando um monstro hospedeiro renasce, há 2–4% de chance de nascer o raro no lugar, com mínimo de 45 min entre um raro e outro;
- nome dourado e 1,2× maior;
- saque garantido de 1 material raro e uma chance de equipamento;
- não aparece no mapa, e só quem está a até 60 m é avisado.

---

## 5. As áreas novas

Os números de atributo são relativos às definições atuais em `monsters.go` (ex.: "base Guerreiro ×1,2 de vida" significa `HP` e `HPLvl` do `skeleton_warrior` vezes 1,2). Assets: **[A1]** marca o que funciona já, com modelos existentes recoloridos; **[B]** marca o modelo final no estilo B.

---

### 5.1 Bosque Gris (níveis 7–15)

- **Onde:** setor noroeste dos anéis 2 e 3 (ângulo de 100° a 170°, r 80–150, cerca de 9.400 m²), mais o canto NO (r 150–212), a **Toca do Corvo**.
- **Bioma:** floresta fechada de carvalhos velhos com musgo cinza-prateado nos troncos.
- **Fantasia:** "a estrada do oeste ficou perigosa, e quem a assalta conhece cada árvore."
- **Identidade visual:**
  - paleta base musgo-cinza `#7d8a6a` e tronco `#4e4234`;
  - destaque: pano vermelho dos bandidos `#a2302a`;
  - sombra `#2b3530`;
  - árvores 30% mais juntas que nos Campos (espaçamento de 5 m em vez de 7,5 m) e pouca luz no chão.
- **Identidade sonora:** corvos, galhos estalando, um apito de bandido que avisa quando o jogador entra no acampamento.

**Ecologia e lore.** Os esqueletos dos Campos expulsaram os lobos da planície para o bosque. O **Bando do Corvo**, desertores da guarda de Vel'Aran, viu nisso uma muralha natural: assalta as caravanas que vão a Brasaferro e paga os lobos com carne. À noite, as aranhas-de-musgo descem das copas.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Bandoleiro do Corvo | Emboscador | invisível até 5 m; 1º golpe com +50%; base Ladino (vida ×1,1) | dia e noite | [A1] `rogue` KayKit com capuz vermelho (tint) · [B] humanoide |
| Atirador de Facas | Atirador | facas a 10 m; recua 5 m; base Mago (dano físico, alcance 10) | dia e noite | [A1] `rogue` com `dagger` · [B] humanoide |
| Brutamontes do Corvo | Escudo frontal | 60% de redução na frente; escudo `shield_round_barbarian` | dia | [A1] `barbarian` KayKit · [B] humanoide |
| Lobo Cinzento | Matilha (3–4) | o líder uiva aos 50% e chama +1 lobo; velocidade 4,8 m/s (menos que os 5 do jogador) | dia: 3 · noite: 4 por matilha | [B] quadrúpede (Wolf CC0) |
| Javali Espinhoso | Bruto | investida em **linha** de 6 m, 1,2 s de aviso; passivo até apanhar | só de dia | [B] quadrúpede |
| Aranha-de-Musgo | Emboscador | desce da árvore a 4 m; teia prende 1,5 s | só à noite (troca os javalis) | [B] artrópode (Spider CC0) |

- **Raro:** **Presa-Branca**, lobo alfa albino de nível 15.
  - Hospedeiro: o líder da matilha, com 3% de chance.
  - Aos 50%, chama 2 lobos e ganha 30% de velocidade de ataque por 8 s.
  - Saque: Pele Alva, garantida, para uma capa boa de receita (+esquiva).
- **Chefe de campo: Corvo Rubro, o Rei dos Bandoleiros** (nível 16)
  - Na Toca do Corvo (−128, 118). Janela de 60–90 min.
  - **F1 (100–60%):** duelista. A cada 12 s, "Bomba de Fumaça": círculo de 5 m com 1,5 s de aviso, que cega por 3 s quem ficar.
  - **F2 (60–30%):** apita e chama 4 Bandoleiros (no máximo 4 vivos). "Chuva de Facas": cone de 90° e 8 m na direção do alvo, com 1,5 s.
  - **F3 (30–0%):** sobe na paliçada (fica sem alvo por 4 s) e salta num jogador. É uma marca que segue por 3 s e cai num círculo de 4 m (+40% de dano).
- **Materiais:**
  - Pele de Lobo (comum): couraças e botas de couro;
  - Presa de Javali (comum);
  - Seda de Musgo (incomum, só à noite);
  - Insígnia do Corvo (incomum): troca na missão e entra em capas e luvas raras de nível 13.
- **Pontos de interesse:**
  - **Árvore-Forca:** carvalho morto de 19 m, cheio de corvos, visível dos Campos. É o landmark.
  - **Carroça Tombada:** o começo da história da caravana.
  - **Toca da Matilha:** clareira de ossos.
  - **Acampamento do Corvo:** paliçada com 6 tendas e o chefe.
- **Segredo:** o Atirador de Facas deixa cair, com 0,5% de chance, um "Mapa Rasgado". Ele aponta um baú enterrado sob a Árvore-Forca, que dá 1 pergaminho de encantar 1×/dia por personagem.
- **Ganchos de missão (#71):**
  - "A caravana de Caio": o irmão da Lívia sumiu. São 4 passos: Carroça Tombada → 8 insígnias → resgatar Caio no acampamento → Corvo Rubro.
  - Diária: "Couro para o inverno", 10 Peles de Lobo.
- **Custo técnico:**
  - **Monstros:** 22, mas os anéis 2 e 3 perdem 9 esqueletos no setor, então o saldo é de +13 monstros.
  - **Pico no raio de 70 m:** cerca de 14.
  - **Modelos:**
    - no A1: 0 modelos novos (rogue e barbarian recoloridos);
    - no A2: +3 (lobo, javali e aranha), dois deles das famílias quadrúpede e artrópode, que outras áreas reutilizam.
  - **Cenário:** +300 objetos do kit de floresta atual, sem malha nova. Paliçada e tendas usam `fence_wood_straight` e `tent`, que já existem.

---

### 5.2 Terras Mortas (níveis 24–28)

- **Onde:**
  - **núcleo (A1):** o canto NE do quadrado (r > 142, de 15° a 75°), em volta da Cripta. São cerca de 4.300 m² úteis, descontado o covil;
  - **extensão (A2):** o bloco de x 150–270 e z 60–150, que nasce junto com o Brejo. São cerca de 10.800 m², somando uns 15.000 m².
- **Bioma:** terra cinza, grama morta, cemitérios sem nome e velas negras.
- **Fantasia:** "a porta da Cripta tem fiéis."
- **Identidade visual:**
  - base cinza-osso `#8f8a80`;
  - destaque roxo da Cripta `#8a5cff` e chama de vela verde-pálida `#b8f0a0`;
  - sombra `#1b1a24`;
  - névoa baixa e densa (a névoa por região já existe, #92).
- **Identidade sonora:**
  - coro grave e distante;
  - o **Sino Rachado** toca 3 vezes quando faltam 10 min para Ozrhal renascer (o `raidWarnBefore` de 10 min já existe; o sino dá o som).

**Ecologia e lore.** Os **Devotos do Ossário** acham que o Rei Ossário vai devolver os mortos queridos. Eles cavam as covas da Partição, levantam os mortos e alimentam o selo rachado com velas de sebo negro. A **Ordem do Bastião Cinzento**, monges-guerreiros de Vel'Aran, vigia a Cripta desde a fundação e perde terreno a cada noite.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Acólito do Ossário | Curandeiro | cura um aliado abaixo de 50% em 15% (8 s); foge do corpo a corpo; base Mago (vida ×0,9) | dia e noite | [A1] `mage` KayKit em preto e roxo · [B] humanoide |
| Necromante Devoto | Invocador | 2 Lacaios de nível −5 a cada 20 s (no máximo 4) | dia e noite | [A1] `skeleton_mage` com manto · [B] KayKit Skeletons Extra (Necromancer, CC0, pago) |
| Cavaleiro Ossário | Escudo frontal + elite | 60% de redução na frente; "Investida" em linha de 8 m com 1,5 s; 3× a vida | dia e noite | [A1] `skeleton_warrior` + `skeleton_shield_large_a` |
| Carniçal | Matilha (3) | sangramento de 4 s por golpe; velocidade 5,2 m/s (corre mais que o jogador) | só à noite | [A1] `skeleton_minion` em cinza-carne · [B] esqueleto curvado |
| Espectro | Emboscador | translúcido até 5 m; cada golpe queima 8% de mana | só à noite | [A1] `skeleton_mage` com fresnel/transparência · [B] esqueleto |

- **Raro:** **Irmã Vésper, a Primeira Devota** (nível 29)
  - Nasce só à noite, no Altar dos Devotos, com 1 chance em 3 a cada noite, se ninguém a matou nas últimas 2 h.
  - Cura os Devotos a até 10 m e solta uma "Ladainha" (anel de 4 a 10 m, com 2 s) que silencia por 3 s.
  - Saque: Tomo Profano, garantido.
- **Chefe de campo: Malthus, o Coveiro** (nível 29)
  - No Cemitério dos Reis Sem Nome (150, 130), depois da extensão do A2. Janela de 2–3 h.
  - **F1:** golpes de pá. "Cova Aberta": círculo de 3 m sob um jogador aleatório, 2,0 s de aviso, prende por 2 s.
  - **F2 (60%):** 3 covas brilham 3 s antes e de cada uma levanta um Carniçal. Se o grupo ficar em cima da cova, ela não abre.
  - **F3 (30%):** "Mortalha", um **anel** de 4 a 12 m com 2,5 s de aviso. A salvação é ficar colado nele. Reforça o que o raid ensina com a Onda.
- **Materiais:**
  - Cinza Sepulcral (comum);
  - Vela Negra (incomum);
  - Tomo Profano (raro): cetro e varinha épicos de nível 24 e pergaminho abençoado;
  - Fragmento de Selo (missão): a 1ª Pedra-Selo.
- **Pontos de interesse:**
  - **Torre do Sino Rachado:** 22 m, visível da cidade. É o landmark da região.
  - **Altar dos Devotos.**
  - **Cemitério dos Reis Sem Nome:** lápides, mausoléu e cruzes de KayKit Halloween Bits e Kenney Graveyard Kit, ambos CC0 e grátis.
  - **Bastião Cinzento:** posto, seção 7.
- **Segredo: a "Lápide do Último Regicida".** Ela mostra o nome de quem mais causou dano na última morte de Ozrhal. O dado já existe: `BossData.TopName`.
- **Ganchos de missão:**
  - Cadeia "As velas do Ossário": 5 passos, que terminam na 1ª Pedra-Selo.
  - Diária: "Apague 6 Velas Negras", interagindo com velas no chão que renascem a cada 10 min.
  - Evento noturno: "O Bastião resiste" (seção 7, T17).
- **Custo técnico:**
  - **Monstros:** no A1, 12 (núcleo) mais o raro; no A2, +14, somando 26.
  - **Pico no raio de 70 m:** 14. Atenção: o covil fica fora da contagem.
  - **Modelos:** 0 novos no A1, todos esqueletos e humanoides recoloridos. No [B], viram parte da #98.
  - **Cenário:** cerca de 150 objetos, com 8 malhas novas de cemitério (CC0 grátis) e a torre (1 malha nova).

---

### 5.3 Brejo de Lamagris (níveis 14–20), o "pântano" da #68

- **Onde:** expansão leste do Mapa 1, x de 150 a 330 e z de −150 a 60. São 37.800 m², dos quais uns 24.000 de caça. A borda oeste fica no nível 14 e a leste no 20.
- **Bioma:** pântano com água rasa, árvores mortas com barba-de-velho, palafitas, cogumelos que brilham.
- **Fantasia:** "o rio que a Partição represou ainda tenta chegar ao mar e afoga quem estiver no caminho."
- **Identidade visual:**
  - base lodo `#5b5a3a` e musgo `#6b7d3e`;
  - destaque fungo bioluminescente `#9fe870`;
  - sombra e névoa verde-escura `#24302a`;
  - vaga-lumes à noite, dentro do orçamento de partículas da #101.
- **Identidade sonora:**
  - sapos e bolhas;
  - de vez em quando, o sino da capela afundada de Vau Velho, embaixo d'água e abafado.

**Ecologia e lore.** O Lamagris corria da serra até o mar. A Partição derrubou a encosta e o rio se espalhou por cima da aldeia de **Vau Velho**. A água parada guardou os afogados, e o veneno do selo de Ozrhal, que escorre pelos veios, os levanta à noite.

Os **Brejeiros**, homens-lagarto que sempre viveram ali, não servem a ninguém: defendem a água deles. São **neutros de dia** e agressivos à noite, quando os afogados os deixam em pânico.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Brejeiro Lanceiro | Bruto social | alcance de 3,2 m; ao ser atacado, chama 2 Brejeiros a até 12 m | **dia: neutro** · noite: agressivo | [B] humanoide + cauda |
| Brejeiro Xamã | Curandeiro | cura (8 s) e "Lama nos Olhos": cegueira de 2 s a 10 m | dia: neutro · noite: agressivo | [B] humanoide (a mesma malha do lanceiro, com outro acessório) |
| Sapo-Boi | Bruto + controle | "Língua": **linha** de 8 m com 1,0 s de aviso, puxa o jogador para 2 m; intervalo de 2,4 s | dia e noite | [B] modelo único (base: Frog da Quaternius Ultimate Monsters, CC0) |
| Lodo Vivo | Dividir + enxame | ao morrer, vira 2 lodos menores (30% da vida, nível −2); golpe dá lentidão de 20% por 3 s | dia e noite | [B] amorfo (Blob CC0, VAT por shape keys) |
| Afogado | Emboscador | sai da água quando o jogador chega a 4 m; agarra e prende 1,5 s | só à noite (troca metade dos sapos) | [B] esqueleto com algas e verde-água |

- **Raro:** **Velha Escamosa**, brejeira anciã de nível 20.
  - Hospedeiro: o Xamã, com 2% de chance.
  - Cura em área (3 aliados) e invoca 2 Lodos.
  - Saque: "Cajado de Junco", raro de nível 18 do mago, com 15% de chance.
- **Chefe de campo: Mãe-Lodo** (nível 21)
  - No meio da lagoa de Vau Velho (250, −40). Janela de 60–90 min.
  - **F1:** "Golfada", **cone** de 60° e 7 m, 1,5 s de aviso, deixa lentidão de 30%.
  - **F2 (66%):** divide-se em 3 Lodos médios. Se não morrerem em 30 s, voltam para ela e a curam em 20%. É o DPS checado com relógio.
  - **F3 (33%):** "Maré de Lama", 4 círculos de 3 m com 1,8 s de aviso. Cada um deixa uma poça de lentidão por 10 s, e o chão vai encolhendo.
- **Materiais:**
  - Escama de Brejeiro (comum);
  - Lodo Negro (comum);
  - Glândula de Sapo (incomum): a 1ª poção feita no Ferreiro, o Antídoto, que limpa veneno e sangramento;
  - Junco-de-Prata (incomum, só nas margens, de dia);
  - Lágrima de Vau (raro): conjunto "do Brejo" de nível 18, botas e luvas com +velocidade de movimento.
- **Pontos de interesse:**
  - **Vau Velho:** só os telhados e a torre da capela ficam fora d'água. É o landmark, visível do Posto.
  - **Palafitas dos Brejeiros:** aldeia neutra de dia.
  - **Ilha dos Vaga-lumes.**
  - **Árvore-Mãe:** raiz gigante que atravessa o brejo.
  - **Ponte de Troncos:** atalho para as Terras Mortas, que abre por missão.
  - **Foz do Lamagris:** portão para Maresia, em (330, −60).
- **Segredo: o Sino Afogado.** À noite, interagir com o sino da torre (3 s de canalização) invoca uma onda de 6 Afogados e um baú. O sino só pode ser tocado de novo 30 min depois.
- **Ganchos de missão:**
  - Cadeia "O filho do Barqueiro": Tomé perdeu o filho, que agora é um Afogado. São 5 passos, que terminam na 2ª Pedra-Selo, presa no sino.
  - "Paz com os Brejeiros": matar mais de 10 Brejeiros de dia deixa a aldeia hostil por 1 h para o personagem.
  - Diária: "8 Juncos-de-Prata".
- **Custo técnico:** é a área mais cara do A2.
  - **Mapa:** +37.800 m² de mapa e +1.300 objetos, ~12 malhas-fonte novas (árvore com barba-de-velho, juncos em cartões, palafita, telhado afundado, torre, cogumelo emissivo).
  - **Água:** um plano de água com shader, sem reflexo (#99).
  - **Colisão:** a água rasa é andável, e a funda vira colisor no `world.json`.
  - **Monstros:** 36 e o chefe, com pico de 16 no raio de 70 m.
  - **Modelos novos:** 4 (brejeiro com 2 variantes, sapo, lodo, afogado = variante de esqueleto).

---

### 5.4 Minas de Brasaferro (níveis 16–22), as "minas" da #68

- **Onde:** **Mapa 3, Serra de Brasaferro**, de 240 × 240 m. Do Passo de Brasaferro (−148, 0) chega-se a uma mina a céu aberto em 3 patamares (verticalidade), com galerias sem teto.
  - **Regra de câmera:** a câmera fica a 18 m e de cima. Por isso não há teto: as paredes de rocha que ficam entre a câmera e o personagem somem (fade).
  - A entrada das **Galerias Profundas** (masmorra D2) fica no fundo.
- **Bioma:** pedreira de ferro avermelhado, andaimes, trilhos e vagonetes, veios de cristal rúnico âmbar.
- **Fantasia:** "a mina cavou fundo demais e o veio respondeu."
- **Identidade visual:**
  - base rocha ferrugem `#8a4f35` e madeira de andaime `#6b4a2b`;
  - destaque cristal rúnico `#f0a640`, o mesmo âmbar dos golens do Vale (é o mesmo veio);
  - sombra `#2a2024`;
  - lanternas penduradas como pontos de luz pintados, sem luz dinâmica.
- **Identidade sonora:** picaretas ao longe (de dia, os kobolds), goteiras, rangido de vagonete, eco.

**Ecologia e lore.** **Brasaferro** extraía o minério que os Ilvarin usavam nas runas. Os golens do Vale são feitos desse mesmo veio, e o **Núcleo Rúnico** é um pedaço dele. Há 20 anos, os mineiros abriram uma câmara Ilvarin e o veio "acordou": o Golem de Ferro-Bruto se levantou, os mineiros fugiram e quem morreu lá dentro continua no turno.

Os **kobolds** (os "Tocos") adoram o brilho do cristal e se mudaram para lá. O **Acampamento Lanterna** é a volta dos mineiros vivos.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Kobold Cavador | Enxame (4) | pouca vida (base Lacaio ×1,3); joga pedras a 8 m | dia: 4 por grupo · noite: 3 | [B] humanoide em escala 0,7 |
| Kobold Dinamiteiro | Atirador + explosivo | carga: **círculo** de 3 m a até 10 m, com 1,5 s; ao morrer, explode (2,5 m, 1,0 s) | dia e noite | [B] o mesmo kobold, com mochila |
| Aranha das Galerias | Emboscador | pendurada no andaime, desce a 5 m; teia: **linha** de 8 m, 1,2 s, prende 2 s | dia e noite | [B] artrópode (o mesmo rig da Aranha-de-Musgo) |
| Mineiro do Turno Eterno | Bruto | golpe de picareta pesado (intervalo 2,4 s; "esmagar" com aviso em círculo de 2,5 m e 1,0 s) | **só à noite**, o "turno da noite" (troca metade dos cavadores) | [B] esqueleto de mineiro (capacete e picareta) |
| Golem de Ferro-Bruto | Elite + tanque | 50% de resistência a dano físico e fraco a magia (−20% de defesa mágica); 3× a vida | dia e noite | [B] construto com paleta de ferrugem (o golem da #98) |

- **Raro:** **Capataz Hurlo** (nível 23)
  - Hospedeiro: o Mineiro, com 3% de chance (só à noite).
  - "Chamada de Turno": 3 Mineiros. Ao morrer, deixa cair a "Lanterna do Capataz" (mão secundária rara de nível 18, +regeneração de mana).
- **Chefe de campo: Tecelã das Galerias** (nível 23)
  - Na Câmara do Eco. Janela de 90–150 min.
  - **F1:** teia em **linha** de 10 m, 1,2 s de aviso, prende por 2 s.
  - **F2 (60%):** sobe no teto. Fica sem alvo por 8 s e larga 4 ovos (círculos de 2,5 m, 2 s de aviso). Ovo que ninguém pisa vira 1 Aranha das Galerias.
  - **F3 (30%):** "Casulo", uma marca que segue um jogador por 4 s e o atordoa por 4 s dentro de um casulo. O casulo é uma entidade com 1/8 da vida do jogador e se quebra com dano dos aliados. Ensina o grupo a salvar o colega.
- **Materiais:**
  - Minério de Brasaferro (comum);
  - Pólvora Kobold (comum);
  - Seda de Aranha (incomum);
  - Cristal Rúnico Bruto (raro, dos golens de ferro).
  - **Receitas de armas**, que hoje o Ferreiro não tem: 1 arma rara de nível 18 por classe e 1 épica de nível 22 por classe, usando Minério e Cristal.
- **Pontos de interesse:**
  - **Poço Aberto:** 3 patamares ligados por rampas.
  - **Elevador de Carga:** atalho entre o patamar 1 e o 3, que abre por missão (o atalho do Dark Souls).
  - **Câmara do Eco:** onde fica a chefe.
  - **Veio Pulsante:** cristal de 8 m, landmark e luz da região.
  - **Trilhos:** o "Vagonete Fujão" é um teleporte curto de 60 m pelos trilhos, a 5 moedas.
  - **Portão das Galerias:** a masmorra D2.
- **Segredo:** 5 páginas do "Diário do Mineiro Ulfo", espalhadas pela mina (objetos que se clicam). Juntar as 5 dá o título "Arqueólogo" e mostra no mapa a sala secreta do veio Ilvarin.
- **Ganchos de missão:**
  - Cadeia "Reabrir Brasaferro": Greta, a mestre de obras, pede 6 passos. Libera o elevador e termina com a 3ª Pedra-Selo, presa no Veio Pulsante.
  - Os kobolds roubaram as lanternas: diária de coleta.
- **Custo técnico:**
  - **Primeiro mapa separado:** depende da T13.
  - **Monstros:** 40 e o chefe, com pico de 18 no raio de 70 m.
  - **Modelos novos:** 3 (kobold, aranha, já pronta no Bosque Gris, e o mineiro, variante de esqueleto) e o golem de ferro, que é troca de paleta.
  - **Cenário:** ~1.500 objetos e ~20 malhas-fonte. A base é o Kenney Modular Cave Kit (CC0) e o KayKit Dungeon Pack (CC0). Trilho, vagonete e cristal precisam ser modelados, e são formas simples.

---

### 5.5 Costa dos Naufrágios (níveis 20–26)

- **Onde:** **Mapa 2, Costa de Maresia**, de 300 × 300 m. O lado oeste é a foz do Lamagris, o centro é a cidade de **Maresia** (seção 6) e o leste é a costa de caça, que vai do 20 (perto da cidade) ao 26 (no recife).
- **Bioma:** praia de areia escura, falésias, navios encalhados, coral vermelho, farol quebrado.
- **Fantasia:** "o mar devolve o que a Partição afundou, e alguém sempre chega antes para saquear."
- **Identidade visual:**
  - base areia `#b9a27a` e rocha `#5f6670`;
  - destaque coral `#d9573f` e lanterna de navio `#ffc65a`;
  - sombra `#22303d`;
  - espuma pintada no shader da água (#99).
- **Identidade sonora:** ondas, gaivotas (de dia), madeira de casco rangendo, sino de boia.

**Ecologia e lore.** A Partição abriu o **Recife dos Dentes** na entrada da baía, e todo navio que tenta a rota velha encalha. Os **Sem-Bandeira**, piratas sem porto, vivem de saquear os cascos e de cobrar "pedágio" de Maresia. As **Sacerdotisas da Maré** acham que o mar quer os Ilvarin de volta e oferecem náufragos ao recife.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Pirata Cutelo | Bruto rápido | golpes duplos (intervalo de 1,0 s, base Ladino ×1,3) | dia e noite | [B] humanoide (tricórnio, cutelo) |
| Pirata Arpoador | Atirador + controle | arpão: **linha** de 10 m, 1,2 s de aviso, puxa o jogador; fica a 9 m | dia e noite | [B] o mesmo humanoide, com arpão |
| Caranguejo-Couraça | Escudo frontal | carapaça: 70% de redução até levar 3 golpes fortes ou um atordoamento; aí fica sem carapaça por 6 s | dia e noite | [B] artrópode (base: Crab Enemy da Quaternius, CC0) |
| Gaivota Carniceira | Enxame (5) | pouca vida; dá meia-volta e mergulha a cada 3 s | só de dia | [B] modelo único pequeno, com VAT |
| Sacerdotisa da Maré | Invocadora + curandeira | invoca 2 "Águas-Vivas" (amorfo, explodem) e cura | só à noite (troca as gaivotas) | [B] humanoide com manto de algas; a água-viva é amorfo |

- **Raro:** **Barba-de-Sal** (nível 27)
  - Hospedeiro: o Pirata Cutelo, com 2% de chance.
  - "Bala de Canhão": **círculo** de 4 m com 2,0 s de aviso, a até 15 m.
  - Saque: o "Tricórnio de Barba-de-Sal" (elmo raro de nível 24, com silhueta própria, a regra do guia para o raro).
- **Chefe de campo: Rei-Pinça** (nível 27)
  - Caranguejo de 6 m na Praia das Âncoras. Janela de 2–3 h.
  - **F1:** pinça em **cone** de 90° e 5 m, 1,2 s de aviso.
  - **F2 (60%):** cava na areia. Fica sem alvo por 6 s e sai sob um jogador aleatório (círculo de 4 m, 2 s de aviso, joga para cima e atordoa por 1,5 s).
  - **F3 (30%):** "Maré Alta". A praia inteira vira um **anel** de dano e só 3 pedras altas são seguras (círculos de 2,5 m), com 3 s de aviso, a cada 25 s.
- **Materiais:**
  - Coral Rubro (comum);
  - Sal Rúnico (incomum): com o Núcleo Rúnico, faz pergaminhos de encantar;
  - Pérola Negra (raro): joias raras e épicas de nível 24;
  - Corda de Cânhamo (comum).
- **Pontos de interesse:**
  - **Navio-Fantasma encalhado:** mastro de 25 m, o landmark.
  - **Farol Quebrado:** verticalidade, com uma rampa em espiral. Lá de cima, o minimapa revela a costa inteira.
  - **Gruta dos Contrabandistas:** atalho entre a praia e a doca de Maresia.
  - **Cemitério de Âncoras.**
  - **Barra de Areia:** uma faixa de 12 × 60 m que só dá para atravessar de dia, porque de noite a maré cobre. Na prática, um colisor que liga e desliga com o ciclo. Leva à Ilha do Recife, onde nascem os raros.
- **Segredo:** a garrafa com mensagem (0,3% de chance em qualquer monstro da costa) aponta um ponto da Barra de Areia. Cavar ali (canalizar 3 s) dá 1 Pérola Negra, 1×/dia.
- **Ganchos de missão:**
  - "Acender o Farol": o faroleiro foi sequestrado. São 5 passos e um **evento de defesa** à noite (5 min de ondas de piratas), que termina na 4ª Pedra-Selo, a lente do farol.
  - Diária de pesca de coral.
- **Custo técnico:**
  - **Monstros:** 40 e o chefe, com pico de 16 no raio de 70 m.
  - **Modelos novos:** 4 (pirata, caranguejo, gaivota, sacerdotisa; a água-viva é do amorfo).
  - **Navios:** cerca de 6 malhas, do Kenney Pirate Kit e do Quaternius Pirate Kit, os dois CC0.
  - **Cenário:** ~1.400 objetos.
  - **Dependências fortes:** água com espuma (#99) e a cidade (seção 6).

---

### 5.6 Ruínas de Ilvaren (níveis 25–30), as "ruínas" da #68

- **Onde:** **Mapa 4, Planalto de Ilvaren**, de 300 × 300 m, pelo portão norte (0, 150). A cidade caída fica em terraços (3 níveis de altura), e no centro está o Monólito-Mestre.
- **Bioma:** mármore branco e dourado, colunatas, aquedutos partidos, flores lilás, pedras rúnicas flutuando.
- **Fantasia:** "a cidade dos que fizeram o selo ainda o defende, mesmo sem ninguém para mandar."
- **Identidade visual:**
  - base mármore `#d9d2c3` e folha lilás `#9c7fc4`;
  - destaque runa ciano `#6fe0e8`. É o contraste com o âmbar do Vale: runa viva é ciano, runa corrompida é âmbar;
  - sombra `#2d3550`.
- **Identidade sonora:** sinos de vento, coro sem palavras de dia (os Ecos), silêncio e vento à noite.

**Ecologia e lore.** Os Ilvarin morreram cantando a runa que partiu a terra, e as **Sentinelas** de mármore ainda obedecem a última ordem: ninguém sem o **Sigilo** passa. De dia, os **Ecos** dos Ilvarin repetem o último dia da cidade, como lembranças presas no lugar. À noite, a lembrança vira a batalha.

**Ysolde, a Mestra dos Caminhos** (a NPC da 2ª classe), é descendente dos Ilvarin. É o elo de missão entre Vel'Aran e as ruínas.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Sentinela de Mármore | Escudo frontal + emboscador | parada como estátua (sem nome) até 6 m; 60% de redução na frente; **passiva para quem tem o Sigilo** | dia e noite | [B] construto com paleta de mármore |
| Eco Ilvarin | Atirador | raio mágico a 12 m; Piscar de 6 m quando o jogador encosta (1× a cada 8 s) | **dia: passivo** (anda numa rotina, "lembrança") · noite: agressivo | [B] humanoide translúcido (fresnel) |
| Guardiã Lírica | Curandeira e buff | "Égide" num aliado, um escudo de 20% da vida (12 s) | dia e noite | [B] o mesmo humanoide translúcido, com manto |
| Lince Rúnico | Matilha (2) | salto em **linha** de 8 m com 1,0 s de aviso | dia e noite | [B] quadrúpede (malha nova, o rig do lobo) |
| Vigia Flutuante | Invocador | abre 2 círculos rúnicos que soltam 3 "Faíscas" (enxame, explodem) | só à noite | [B] construto simples (esfera e anéis, VAT) |

- **Raro:** **Lirael, a Última Cantora** (nível 31)
  - Hospedeira: o Eco, com 2% de chance (só à noite).
  - "Canção Partida": círculo de 6 m em volta dela, 2 s de aviso, silencia por 3 s (sem skills).
  - Saque: "Diadema de Lirael", colar épico de nível 28.
- **Chefe de campo: O Colosso Sem Rosto** (nível 32)
  - Estátua de 9 m (escala 4) na Grande Escadaria. Janela de 2–3 h.
  - **F1:** pisão em **círculo** de 6 m em volta dele, 2,0 s de aviso; mãos lentas.
  - **F2 (65%):** "Olhar do Selo", 3 **linhas** de 20 m que giram 30° a cada aviso de 1,5 s.
  - **F3 (35%):** perde um braço e chama 4 Sentinelas. Ganha um escudo de 80% de redução que só cai com a destruição de 2 **Pilares-Runa** (entidades com 15% da vida dele). É o grupo que se divide, como as baterias do Tequatl no GW2.
- **Materiais:**
  - Pó de Mármore (comum);
  - Fio de Runa (incomum);
  - Lágrima de Eco (raro): armaduras épicas de nível 28, com forma nova na silhueta (regra do guia, 6.4);
  - Pedra-Selo (missão): a 5ª.
- **Pontos de interesse:**
  - **Grande Escadaria:** terraços, onde fica o chefe.
  - **Anfiteatro dos Ecos:** onde os Ecos "ensaiam" de dia. É o melhor lugar para ver a lore em cena.
  - **Aqueduto Partido:** ponte e atalho entre o terraço 1 e o 3.
  - **Jardim Suspenso.**
  - **Biblioteca Afundada:** a entrada do Santuário Submerso (D3).
  - **Monólito-Mestre:** 30 m, com um feixe de luz ciano. É o landmark, visível de todo o planalto.
- **Segredo: a "Melodia".** São 4 pedestais de sino. O mural do anfiteatro mostra a ordem (que troca a cada dia). Tocar na ordem certa abre o Cofre Ilvarin, 1 baú por personagem por dia.
- **Ganchos de missão:**
  - "O Sigilo": a cadeia de Ysolde, em 6 passos. Dá o Sigilo, que deixa as Sentinelas passivas para o personagem. Quem quiser o saque delas ataca de propósito.
  - "Por que os Ilvarin partiram a terra": revela como refazer o selo e prepara a Necrópole.
  - Diária: "Ouvir 3 Ecos", com o diálogo de dia.
- **Custo técnico:**
  - **Monstros:** 38 e o chefe, com pico de 16 no raio de 70 m.
  - **Modelos novos:** 3 (Eco, lince e vigia; a sentinela é paleta do construto).
  - **Shader de translúcido:** fresnel com transparência, aplicado a 2 monstros. Atenção ao custo de ordenação de transparência: no máximo 10 translúcidos no raio de 70 m.
  - **Cenário:** é o mais caro. São ~1.400 objetos e ~25 malhas novas (colunas, arcos, estátuas, aqueduto, escadaria), porque não existe ruína élfica CC0 no estilo. A base é o Ultimate Modular Ruins da Quaternius (CC0), retexturizado para o atlas. Os terraços pedem relevo no terreno (#99).

---

### 5.7 Dentes do Inverno (níveis 30–35, esboço do 30 → 40)

- **Onde:** **Mapa 5, Terras Altas do Norte**, de 360 × 300 m. A metade sul é a cidade de **Valdurn** com o seu castelo; a metade norte são os picos, que vão do 30 (perto de Valdurn) ao 35 (na geleira).
- **Bioma:** pinheiros nevados, gelo azul, aldeias de pedra abandonadas, nevasca à noite.
- **Fantasia:** "os sobreviventes Ilvarin fugiram para o frio, e o frio não os deixou ir embora."
- **Identidade visual:**
  - base neve `#e6edf2` e pinho `#3d5a4a`;
  - destaque gelo `#7fd3ff`;
  - sombra `#283a52`.
  - **Variante de paleta:** as árvores e rochas do kit atual ficam com o topo branco no atlas, o mesmo kit com outra cor, sem custo de GPU.
- **Identidade sonora:** vento forte, lobos ao longe, gelo estalando.

**Ecologia e lore.** Os Ilvarin que escaparam da queda subiram a serra levando o segredo da forja. O frio da altitude preservou as runas de gelo, e os **Saqueadores das Neves**, tribos humanas que vieram depois, saqueiam os túmulos Ilvarin. O **Wyrm de Geada** dorme na geleira em cima de uma porta Ilvarin.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Lobo da Geada | Matilha (4) | golpe dá lentidão de 15% (acumula até 3×) | dia: 3 · noite: 5 | [B] quadrúpede (paleta do lobo) |
| Yeti Rochoso | Bruto | "Rugido": círculo de 5 m, 1,5 s de aviso, medo de 2 s (o jogador foge sem controle) | dia e noite | [B] modelo único (base: Yeti da Quaternius, CC0) |
| Golem de Gelo | Elite | "Toque Glacial": cone de 90° e 5 m, 1,5 s, congela por 1,5 s | dia e noite | [B] construto com paleta de gelo |
| Saqueador das Neves | Atirador | dardos a 12 m; em grupo de 3 com 1 Saqueador corpo a corpo | só de dia | [B] humanoide com peles (o rig do bárbaro) |
| Fogo-Fátuo da Geada | Enxame | queima de mana; atravessa obstáculos | só à noite (troca os saqueadores) | [B] amorfo emissivo |

- **Raro:** **Grimma, a Mãe-do-Gelo**, xamã dos Saqueadores, de nível 35.
- **Chefe de mundo: Vharok, Wyrm de Geada** (nível 36)
  - Horário fixo, 2× por dia (20:00 e 13:00, horário de Brasília), anunciado no servidor inteiro, como os chefes de horário fixo do L2 Essence e do GW2.
  - **F1:** sopro em **cone** de 60° e 12 m, 2,0 s de aviso.
  - **F2 (60%):** voa. Fica sem alvo por 10 s e larga 6 estalactites (círculos de 3 m, 2 s de aviso).
  - **F3 (30%):** aura de geada, um anel de 6 a 20 m, mais o chão congelando, que encolhe a área segura.
- **Materiais:**
  - Pele de Yeti (comum);
  - Gelo Eterno (incomum);
  - Escama de Wyrm (raro): equipamento de nível 32.
- **Pontos de interesse:**
  - **Valdurn e o castelo.**
  - **Geleira de Vharok.**
  - **Túmulos Ilvarin:** 6 entradas pequenas com baús.
  - **Ponte de Gelo:** atalho que só existe à noite.
  - **Elevador de Valdurn:** a descida para o Coração de Brasa.
- **Ganchos de missão:** a linha "Os que fugiram" (os Ilvarin do norte); a disputa do castelo (#82).
- **Custo técnico:**
  - **Monstros:** 40, com pico de 18 no raio de 70 m.
  - **Modelos novos:** 2 (yeti e saqueador, este quase só acessórios) e o Wyrm (modelo único de chefe, base Dragon da Quaternius, CC0).
  - **Cenário:** sai quase todo das variantes de paleta, mas a neve caindo conta no orçamento de partículas.
  - **Cidade:** Valdurn tem ~350 objetos.

---

### 5.8 Coração de Brasa (níveis 35–40, esboço)

- **Onde:** **Mapa 6**, de 240 × 240 m, sob a montanha de Valdurn. É uma caldeira vulcânica com plataformas sobre lava. A Forja Primeva (D5) fica no centro.
- **Bioma:** rocha negra, rios de lava, forjas Ilvarin gigantes, correntes e bigornas.
- **Fantasia:** "a forja onde o selo foi feito ainda está acesa, e tem gente querendo forjar outra coisa nela."
- **Identidade visual:**
  - base basalto `#2e2a2b`;
  - destaque lava `#ff7a2a` e runa âmbar;
  - sombra `#140c0c`.
  - A lava é só shader (UV correndo e emissivo), sem asset.
- **Identidade sonora:** borbulhar grave, martelos ritmados, rugido de fole.

**Ecologia e lore.** Foi aqui que os Ilvarin forjaram os Núcleos Rúnicos e as cinco Pedras-Selo. Os **Fundidores**, uma ordem de ferreiros renegados, querem refundir a coroa de Ozrhal: o "Fragmento do Rei Ossário", que hoje só se vende, ganha aqui o seu uso. Se conseguirem, o selo não precisa mais ser quebrado: o rei volta inteiro. Enquanto isso, os guardiões da forja (construtos de magma) atacam qualquer um.

| Monstro | Papel | Mecânica | Dia / noite | Asset |
|---|---|---|---|---|
| Golem de Magma | Bruto | ao morrer, deixa uma poça de fogo de 2 m por 5 s | dia e noite | [B] construto com paleta de magma |
| Salamandra Ígnea | Atirador | cuspe de fogo a 10 m (queimadura por 4 s); corre 6 m em linha quando o jogador encosta | dia e noite | [B] quadrúpede (malha nova) |
| Fundidor Renegado | Elite e buff | "Têmpera": +20% de dano nos aliados a até 10 m por 10 s; martelo em linha de 6 m, 1,5 s | dia e noite | [B] humanoide (avental, martelo) |
| Diabrete de Escória | Enxame + explosivo | em grupo de 5; explode ao morrer | dia e noite | [B] amorfo (base: Demon da Quaternius, CC0, em escala 0,5) |
| Draco de Magma | Elite | sopro em cone de 45° e 8 m, 1,5 s | só "à noite" (a forja aquece no ciclo) | [B] o mesmo modelo do Wyrm, com paleta de magma |

- **Raro:** **Mestre-Fundidor Ignaz** (nível 40). Saque: o molde de uma arma lendária de nível 38.
- **Chefe de campo: Brasa-Viva, a Bigorna que Respira** (nível 39)
  - **F1:** martelo em linha de 10 m, 1,5 s.
  - **F2 (60%):** 8 respiros no chão entram em erupção um depois do outro (círculos de 2,5 m, 1,0 s cada, com 0,5 s entre eles). É o "exaflare".
  - **F3 (30%):** "Fusão". Puxa todos para o centro e explode num círculo de 8 m, com 2,5 s para sair.
- **Materiais:**
  - Escória Viva (comum);
  - Coração de Magma (raro);
  - Lingote Ilvarin (lendário): com o Fragmento do Rei Ossário, faz a arma lendária de nível 38.
- **Pontos de interesse:** Bigorna Primeva (o landmark), Rio de Lava (com pontes que caem e voltam em 20 s), Sala dos Moldes, Fole Gigante.
- **Custo técnico:**
  - **Monstros:** 40.
  - **Modelos novos:** 3 (salamandra, fundidor e diabrete). O resto é paleta.
  - **Cenário:** ~20 malhas novas (forja, correntes, basalto).
  - **Shader de lava:** 1 material.

---

### 5.9 Raros e chefes nas regiões que já existem (entram no A1, sem arte nova)

| Região | Raro (hospedeiro, chance) | Mecânica | Saque garantido |
|---|---|---|---|
| Arredores | **Ossudo, o Teimoso**, de nível 6 (Lacaio, 3%) | levanta 1× quando morre, com 30% da vida | 3 Crânios Rachados |
| Campos dos Ossos | **Capitão Ossudo**, de nível 14 (Guerreiro, 2%; o nome vem do protótipo em `proto/data/mock.ts`) | "Formação!": 2 Guerreiros com escudo | Elmo raro de nível 13 (20%) e Insígnia do Capitão |
| Ermos Partidos | **Vossk, o Cinzento**, de nível 21 (Mago, 2%) | Chuva de Ossos mirim: 2 círculos de 3 m, 1,5 s | 2 Essências Sombrias |
| Vale das Pedras Vivas | **Ômrun, Golem Ancião**, de nível 31 (Golem, 2%, só na metade sul) | pisão: anel de 3 a 7 m, 2,0 s | 2 Núcleos Rúnicos |
| Ermos (chefe) | **Senhor dos Ossos** vira chefe de campo (T11) | F1: "Golpe Giratório", círculo de 4 m, 2,0 s · F2 (60%): chama 3 Guerreiros · F3 (25%): "Marcha dos Mortos", linha de 12 m, 1,5 s, e fúria. Janela de 30–45 min, em vez dos 5 min fixos de hoje | o saque atual (`lootBoss`) |

---

## 6. Cidades novas

### 6.1 Maresia, o Porto da Foz (nível 20+, Mapa 2)

- **Identidade:** porto de refugiados da Partição que enriqueceu com o comércio, feito de madeira pintada, cordame e telhados azuis. Os destaques são as velas brancas e as lanternas amarelas. A planta é aberta, sem muralha: a cidade se defende com a doca e uma paliçada de 3 portões.
  - A paleta segue o padrão da tabela 3.3 do guia: base madeira clara `#c7b08a`, destaque azul `#3f6fa8`, sombra `#2a3448`.
- **Função no jogo:** o **hub de comércio** e o segundo ponto de respawn. É o papel de Giran no L2.
- **Zona segura:** raio de 45 m.
- **NPCs e serviços:**

| NPC | Serviço |
|---|---|
| Capitã Doria, a Mestra do Porto | missões da Costa (#71) e a linha do Farol |
| Fio, o Leiloeiro | **Praça do Mercado**: as lojas pessoais ou o leilão da #78 só funcionam aqui e em Vel'Aran. Ter o comércio concentrado num lugar movimenta a cidade |
| Nara, a Armazém | o mesmo armazém da conta de Otto (tabela compartilhada) |
| Bento, o Ferreiro Naval | receitas da Costa (Pérola, Coral, Sal Rúnico) e encantar com Sal Rúnico |
| Mercadora Lia | poções, pergaminhos de retorno (agora "para a última cidade visitada") |
| Mestre de Armas Taro | **Arena das Marés** (#81): a fila e as temporadas ficam aqui, num anfiteatro de 30 m de diâmetro |
| Pedra de Teleporte | Vel'Aran, Posto do Barqueiro e Bastião Cinzento |

- **O que destrava:**
  - o 2º ponto de renascer, escolhido no Mestre do Porto;
  - o mercado entre jogadores (#78) e a arena (#81);
  - as receitas de nível 24.
- **Custo técnico:**
  - ~300 objetos, com 10 a 12 malhas: casas de madeira modulares e docas. A base é o KayKit Medieval Hexagon e o Kenney Fantasy Town, os dois CC0, retexturizados no kit coeso da #100.
  - **NPCs:** 7.

### 6.2 Valdurn, a Cidadela do Norte (nível 30+, Mapa 5)

- **Identidade:** cidade de pedra escura e telhados de ardósia nevada, encostada num **castelo** que domina a cidade de cima. O destaque é o estandarte do clã dono do castelo, que muda de cor conforme quem o tem.
- **Função no jogo:** endgame e política.
  - O **Castelo de Valdurn** é o alvo dos cercos da #82.
  - O clã dono define o imposto das lojas de NPC de Valdurn, de 0% a 10% sobre compras.
  - Seguindo o L2, cerca de 60% do imposto vai para o tesouro do castelo. Imposto alto demais leva o comércio a Maresia, e esse é o freio natural.
- **NPCs e serviços:**

| NPC | Serviço |
|---|---|
| Senescal Orrin | registro de cerco, impostos e tesouro (#82) |
| Ysolde (2ª presença) | a 3ª classe, no futuro nível 40 |
| Mestre Rúnico Hald | receitas de nível 32–38 e o Lingote Ilvarin |
| Armazém, Mercador, Pedra | os serviços padrão |
| Sede de Clã | salões alugáveis (o "clan hall" da #82: teleporte mais barato, buffs para o clã) |

- **O que destrava:** cercos, impostos, sedes de clã, a 3ª classe e o Coração de Brasa.

---

## 7. Postos avançados

**Padrão de posto (todos):**
- zona segura de 10 m de raio;
- Pedra de Teleporte;
- 1 mercador de poções e pergaminho de retorno;
- 1 NPC de missões da região (#71);
- 1 serviço próprio;
- ~40 objetos, todos de kits que já existem.

| Posto | Onde | Serviço próprio | Destrava | Identidade |
|---|---|---|---|---|
| **Posto do Barqueiro** | Mapa 1 (165, 0), na entrada do Brejo | Tomé, o Barqueiro: **barco** até a Ilha dos Vaga-lumes (teleporte curto, 10 moedas) e Antídotos | a cadeia "O filho do Barqueiro" e a 2ª Pedra-Selo | palafita com lanterna verde e um sino de barco |
| **Bastião Cinzento** | Mapa 1 (170, 110), nas Terras Mortas | Irmão Caedmon: lista de **quem matou Ozrhal**, registro de grupos para o raid e bênção da Ordem (+5% de defesa por 30 min, 200 moedas) | a cadeia das velas e a 1ª Pedra-Selo | torre de pedra cinza, estandarte branco com um olho |
| **Acampamento Lanterna** | Mapa 3, na boca da mina | Greta, a Mestre de Obras: **ferreiro de campo** (só as receitas de Brasaferro) | o Elevador de Carga (atalho) e a 3ª Pedra-Selo | tendas, lanternas penduradas, carroça de minério |
| **Vigília de Ilvaren** | Mapa 4, na borda sul das ruínas | Aelin, a Erudita: o **Sigilo** e as leituras de mural (lore) | as Sentinelas passivas e a 5ª Pedra-Selo | acampamento de estudiosos dentro de uma colunata |

**O posto que pode cair (o Bastião Cinzento, ligado à #77 e à T17).**
- Em cada noite do ciclo (a cada 40 min), há 50% de chance de os Devotos atacarem o Bastião em 3 ondas de 2 min, com 8 monstros por onda.
- Se nenhum jogador defender, ou se o defensor morrer, o Bastião "cai": o teleporte e a loja fecham por 15 min, até que alguém mate o Estandarte dos Devotos plantado na porta.
- Recompensa por participação, como as medalhas do GW2: 100%, 85% ou 75% da recompensa, conforme a contribuição (dano, cura ou estar no círculo).

---

## 8. Masmorras instanciadas (fecham a #80)

**Regras comuns:**
- grupo de 1 a 6 pessoas, projetado para 3;
- a cópia é criada quando o líder entra e destruída 5 min depois de ficar vazia (ou ao fim de 45 min);
- **baú do chefe 1×/dia por personagem.** Dá para refazer só pela XP e pelos materiais;
- monstros ≤ 40 por cópia;
- nenhum jogador de fora aparece, o que faz da masmorra **o conteúdo mais leve do jogo para o FPS** (lembrado na #80).

| Masmorra | Níveis | Entrada | Andares e tamanho | Chefe e mecânica principal | Arte |
|---|---|---|---|---|---|
| **D1 Catacumbas de Vel'Aran** | 12–16 | porta na igreja de Vel'Aran | 2 andares de 60 × 60 m, ~20 min | **Abade Ulric**: o monge que enterrou os primeiros mortos da Partição. Fases: "Responso" (anel de 3 a 9 m), "Ossuário" (invoca 4) e "Vela Final" (marca que segue). Ensina os 3 avisos do raid antes do raid | **só arte que já existe:** esqueletos KayKit e KayKit Dungeon Pack (CC0, grátis) |
| **D2 Galerias Profundas** | 18–22 | Minas de Brasaferro | 3 andares, um mais fundo e mais forte que o outro (como os andares do Ragnarok), ~25 min | **O Veio Desperto**: um construto de cristal. Os cristais pelo chão carregam o chefe, e destruí-los tira 10% do escudo dele | Minas, mais 4 malhas de cristal |
| **D3 Santuário Submerso** | 26–30 | Biblioteca Afundada (Ruínas) | 2 andares e uma sala alagada, ~25 min | **Guardião do Selo**: lógica de "leitura de runas". O mural mostra 3 símbolos, e o grupo fica nos 3 pedestais certos antes de 8 s | Ruínas, mais água |
| **D4 Necrópole do Rei Ossário** | 30 | sob a Cripta Esquecida; pede as **5 Pedras-Selo** (uma vez, por missão) | 3 salas e o trono, ~35 min, para 6 jogadores | **Ozrhal Desperto**: a versão difícil, de grupo, com as fases do raid comprimidas e uma 4ª fase (o selo se rompe, e o chão some em setores) | reaproveita o raid (`zone/raid.go`) |
| **D5 Forja Primeva** | 38–40 | Coração de Brasa | 3 salas, ~30 min | **Os Fundidores**: 3 mini-chefes e a coroa refundida | Coração de Brasa |

---

## 9. Roadmap em fases

As fases vão da mais barata e de maior impacto à mais cara. Cada uma termina com `?bench=1` na região nova cheia (com `cmd/loadbot -bots 100`) e com a telemetria (`v_fps`, `v_level_times`, `v_deaths_by_region`) olhada 1 semana depois.

### A0: Alicerce (desempenho e sistemas, sem conteúdo visível)
- **Entra:**
  - **T1:** as correções 1 a 3 da #64;
  - **T2:** spawns por setor e por dia/noite;
  - **T3:** arquétipos;
  - **T4:** avisos genéricos;
  - **T7:** gerador de bioma.
- **Depende de:** nada novo (a #88 já está na main).
- **Riscos:** os arquétipos podem quebrar a auto-caça, que espera monstros que vêm até o jogador. Os testes da zona e a auto-caça precisam cobrir atirador e emboscador.
- **Pronto quando:**
  - com 100 bots, a malha ativa cai pelo menos 30% e o PC volta a 60 FPS no Vale e na Cripta;
  - no celular simulado, o tempo de "malhas ativas" cai pelo menos 30%;
  - `world.json` sai byte a byte igual depois da refatoração do gerador;
  - testes novos para cada arquétipo e cada forma de aviso.

### A1: Mais mundo sem mapa novo (o mais barato e de maior impacto)
- **Entra:**
  - **T5:** chefe de campo genérico;
  - **T6:** raros;
  - **T10:** Bosque Gris (só os bandidos) e o núcleo das Terras Mortas;
  - **T11:** Senhor dos Ossos com fases;
  - os 4 raros das regiões atuais.
- **Depende de:** A0. As missões das duas áreas entram pela #71.
- **Custo de arte:** **zero modelos novos.** Tudo são variantes de cor e acessório de modelos que já existem. Quando a #98 trocar os esqueletos e a #94 os personagens, as variantes herdam o novo visual.
- **Riscos:**
  - as variantes por paleta podem parecer "a mesma coisa": exigir 1 acessório de silhueta por variante;
  - o raid no nível 25, com as Terras Mortas em 24–28, deixa o Ozrhal fácil. Decidir subir o Ozrhal para 30 (pendência da #69).
- **Pronto quando:**
  - a faixa 20–30 passa de 12 para pelo menos 24 monstros comuns (os 12 golens e os 12 das Terras Mortas), além dos raros;
  - as duas áreas aparecem no mapa do mundo, com nome, faixa e pedra (o Bastião);
  - o Senhor dos Ossos tem 3 fases com avisos;
  - os 4 raros nascem e avisam;
  - FPS dentro da meta no Bosque Gris cheio.

### A2: Primeira expansão (o Brejo e as famílias de monstro)
- **Entra:**
  - **T8:** cenário em blocos;
  - **T9:** famílias de rig e VAT;
  - **T12:** Brejo de Lamagris, o Posto do Barqueiro e a extensão das Terras Mortas;
  - a fauna do Bosque Gris (lobos, javalis, aranhas).
- **Depende de:**
  - **#93** (decidir seguir com o estilo B);
  - **#98** (o VAT e o bestiário);
  - **#99** (a água);
  - **#90 e #95** (o pipeline e o orçamento no CI).
- **Riscos:**
  - a produção de arte é o gargalo: 7 modelos novos;
  - a água no celular: o preset "baixa" usa água sem animação de normal.
- **Pronto quando:**
  - o Mapa 1 tem 480 m de largura e o celular simulado não perde FPS em relação ao A0 nas regiões antigas;
  - o Brejo tem 36 monstros e o chefe, com pico ≤ 16 no raio de 70 m;
  - os Brejeiros são neutros de dia;
  - o lote de missões do Brejo está pronto (#71).

### A3: Mapas e instâncias
- **Entra:**
  - **T13:** zonas múltiplas e transferência por portão;
  - **T14:** Catacumbas de Vel'Aran (D1, fecha a #80);
  - **T16:** descoberta de teleportes;
  - **T17:** eventos de área, com a defesa do Bastião.
- **Depende de:** A1. A #86 (canais) usa a mesma peça e pode vir logo depois.
- **Riscos:**
  - o estado que atravessa zonas (grupo, clã, chat, amigos, armazém) precisa continuar funcionando. Hoje tudo está numa zona só;
  - salvamento no meio da transferência: só uma zona pode ser dona do personagem em cada momento.
- **Pronto quando:**
  - um grupo entra nas Catacumbas, mata o Abade e sai com o saque, enquanto outras 5 cópias rodam;
  - o mundo aberto não perde tick (o "tick stats" do log, < 5 ms com 100 bots);
  - o portão funciona nos dois sentidos sem duplicar item (testes de caos da #117).

### A4: Minas de Brasaferro e as Galerias Profundas
- **Entra:** **T15**, o Mapa 3, o Acampamento Lanterna, as receitas de arma e a D2.
- **Depende de:** A2 (as famílias artrópode e construto) e A3 (os mapas).
- **Pronto quando:**
  - o Mapa 3 tem 40 monstros, o chefe, o elevador e as 3 salas da D2;
  - existem 8 receitas de arma;
  - FPS dentro da meta no patamar com mais monstros.

### A5: Ruínas de Ilvaren e o Santuário Submerso
- **Entra:** **T18**, o Mapa 4, a Vigília, o Sigilo e a D3.
- **Depende de:** A4, a #99 (relevo dos terraços) e o shader translúcido.
- **Riscos:** são 25 malhas de ruína novas, a maior carga de cenário do roadmap.
- **Pronto quando:** as 5 Pedras-Selo dá para obter, e a D4 (Necrópole) fica liberada para quem tem as 5.

### A6: Costa e Maresia (a segunda cidade)
- **Entra:** **T19**, o Mapa 2, Maresia, a Costa, o Farol e a Barra de Areia.
- **Depende de:**
  - A3;
  - **#78** (o mercado mora em Maresia);
  - **#81** (a arena mora em Maresia);
  - **#100** (o kit de cidade coeso).
- **Riscos:**
  - a cidade é o lugar com mais jogadores parados: o orçamento de draw calls (≤ 150 no celular) e os canais (#86) precisam estar prontos;
  - dividir os jogadores entre duas cidades: o respawn escolhido e o mercado nas duas evitam uma cidade fantasma.
- **Pronto quando:**
  - Maresia tem 7 NPCs, renascimento próprio, mercado e arena;
  - a costa tem 40 monstros;
  - a cidade cheia (50 bots) fica ≥ 30 FPS no celular médio de verdade.

### A7: Do 30 ao 40 (Terras Altas, Valdurn, castelo e Coração de Brasa)
- **Entra:** **T20**, o nível máximo 40, os Mapas 5 e 6, Valdurn, o Castelo de Valdurn (#82), as D4 e D5 e o Wyrm.
- **Depende de:** todas as anteriores e a #82.
- **Pronto quando:** este documento ganha a sua versão 2, com a curva de XP do 30 ao 40, a 3ª classe e o calendário de cerco.

---

## 10. Riscos gerais e decisões em aberto

1. **O ritmo da arte define o roadmap, não o código.** Do A2 ao A6 são cerca de 20 modelos de monstro e ~90 malhas de cenário. Sem a #93 aprovada e o pipeline da #90/#95 rodando, o A2 para. O A1 existe justamente para dar conteúdo enquanto isso.
2. **O nível do Ozrhal:** 25 hoje, com o nível máximo em 30 (pendência da #69). A proposta é 30, na abertura das Terras Mortas (A1).
3. **Mapas separados ou mundo sem costura.** A escolha deste documento é: crescer o Mapa 1 uma vez (A2) e, depois, mapas separados por portão. Um mundo contínuo a 30 FPS no celular exigiria streaming de blocos no servidor e no cliente, o que fica muito além do orçamento de um desenvolvedor.
4. **Monstros neutros de dia** (Brejeiros e Ecos) mudam a auto-caça. A auto-caça deve ignorar monstros neutros, salvo se o jogador marcar a opção.
5. **Instância com renascimento:** os monstros de uma cópia não renascem. O XP por hora da masmorra precisa ficar ≤ 1,2× o do mundo aberto da mesma faixa (medir em `v_level_times`), para não esvaziar os mapas abertos.

---

## 11. Backlog de tasks propostas (em ordem)

Convenções:
- "Depende" cita estas tasks (T…) e as do board (#…);
- os números de migração e de campo do protocolo seguem as faixas reservadas da #89 (ler antes de pegar);
- toda task de cliente termina com `?bench=1` antes e depois, colando a tabela na task.

---

**T1. Aplicar as correções de FPS 1–3 do levantamento antes de pôr conteúdo**
- **Contexto:**
  - O relatório da #64 (anexo `relatorio-fps-2026-11-13.md`) mostrou que o gargalo é a CPU nas malhas ativas e nas animações.
  - A #88 já limita as entidades recebidas a 70 m. Faltam os pontos 2 e 3 do relatório:
    - manter só a cabeça escolhida de cada personagem (hoje as 5 são carregadas, 515 malhas "Head" com 100 jogadores);
    - juntar as pedras e os brilhos do golem por osso, de ~20 malhas para ~3 (`client/src/world/golem.ts`);
    - `doNotSyncBoundingInfo` nas malhas com esqueleto;
    - não reiniciar animação de quem está fora da tela e pausar os seus grupos (`client/src/world/entities.ts`, perto do LOD de animação);
    - trocar o cilindro de clique por entidade por um teste de raio contra cilindros em código.
  - Arquivos: `client/src/world/entities.ts`, `models.ts`, `golem.ts`, `input.ts`.
- **Pronto quando:**
  - com `?bench=1` e 100 bots, as malhas ativas por quadro caem pelo menos 30% e o PC fica em 60 FPS no Vale e na Cripta;
  - no celular simulado, o tempo de "Malhas ativas" cai pelo menos 30%;
  - o clique em monstro, jogador, NPC e item no chão continua igual, no PC e no toque.
- **Depende de:** nada (a #88 está na main). Destrava a #65.

**T2. Spawns por setor, por período (dia/noite) e com exclusão de áreas**
- **Contexto:**
  - Hoje `game.Spawn` (`server/internal/game/monsters.go`) só conhece anel, ponto fixo e disco.
  - Acrescentar:
    - `AngleMin/AngleMax` (setor de anel, em graus, 0° = leste);
    - `When` (`Any`, `Day`, `Night`): na virada para a noite (`game.IsNight`), os spawns do dia fora de combate somem e os da noite entram, e no amanhecer o contrário. A contagem total fica igual;
    - a exclusão de áreas declaradas: o anel não sorteia dentro de um setor ou disco que tenha dono, como já faz com o covil (`RaidLairRadius`).
  - O `mapZones` (`server/internal/zone/mapzones.go`) e o `regions.ts` (`client/src/ui/regions.ts`) desenham o setor no mapa do mundo. Campo novo de protocolo dentro da faixa da #89.
  - **Decisão:** mudar os spawns não pode mudar os ids dos monstros que já existem. Os novos vão para o fim da lista, como o comentário de `DefaultSpawns` já exige.
- **Pronto quando:**
  - teste: um setor 100°–170° só tem monstros no setor;
  - um spawn `Night` só existe à noite;
  - a contagem de monstros vivos fora de combate é a mesma de dia e de noite;
  - o mapa do mundo mostra o setor com nome e faixa.
- **Depende de:** nada.

**T3. Arquétipos de IA de monstro**
- **Contexto:**
  - A IA atual (`server/internal/zone/monster.go`, `think`) é uma só.
  - Acrescentar `Archetype` e parâmetros em `MonsterDef`, com o comportamento da tabela 4.4:
    - bruto (o padrão de hoje);
    - atirador (mantém 7–10 m e recua 5 m, 1× a cada 6 s);
    - curandeiro (cura um aliado abaixo de 50% em 15%, 8 s);
    - invocador (2 a cada 20 s, no máximo 4, usando o `summon` de `raid.go`);
    - emboscador (invisível até 4–5 m, usando o efeito de invisibilidade que já existe, e o 1º golpe com +50%);
    - matilha (ligados por grupo de spawn: um puxa todos);
    - escudo frontal (redução num cone de 120°, girando a 90°/s);
    - dividir;
    - explosivo.
  - Mais a flag `Neutral` (só ataca quem bateu nele), com variante `NeutralByDay`.
  - A auto-caça do cliente (`client/src/world/autohunt.ts`) ignora monstros neutros, salvo se o jogador marcar a opção.
  - Tudo determinístico com `z.rng`.
- **Pronto quando:**
  - um teste por arquétipo em `server/internal/zone`;
  - `-race` verde;
  - o quadro do alvo mostra o papel (ícone pequeno: escudo, cruz, olho);
  - a auto-caça não trava com atirador nem com emboscador.
- **Depende de:** nada. Pode correr junto com a T2.

**T4. Avisos no chão para qualquer monstro (círculo, cone, linha e anel) e habilidades por dados**
- **Contexto:**
  - O raid já tem avisos (`server/internal/zone/raid.go`: `addTelegraph`, `updateTelegraphs`, `resolveTelegraph`), mas só em círculo e com tipos fixos (`TelegraphKind` WAVE/RAIN/MARK em `proto/erevia/v1/game.proto`).
  - Generalizar para:
    - `Abilities []AbilityDef` em `MonsterDef`, com forma, raio, ângulo, comprimento, atraso, recarga, multiplicador, efeito, alvo (si mesmo, alvo, aleatório, marca que segue) e fase mínima;
    - formas novas no protocolo (`CONE`, `LINE`, `DONUT`), com direção.
  - O Ozrhal passa a usar o mesmo caminho, sem mudar o comportamento dele (os testes do raid seguem verdes).
  - No cliente, o desenho dos avisos usa decal ou malha plana instanciada (uma por forma), coordenado com as marcas no chão da #101.
  - **Regras:** atraso mínimo de 1,0 s em comuns e de 1,5 s em elites e chefes.
- **Pronto quando:**
  - testes de acerto para as 4 formas, com borda, ângulo e jogador saindo no último tick;
  - o raid igual ao de antes;
  - no máximo 6 avisos simultâneos por jogador desenhados (os demais só no servidor);
  - FPS igual com 10 avisos na tela.
- **Depende de:** nada. Coordenar com a #101.

**T5. Sistema genérico de chefe de campo**
- **Contexto:**
  - Hoje só o raid tem fases, janela e saque por dano.
  - Criar o "chefe de campo" por dados:
    - `FieldBoss` em `MonsterDef`, com renascimento em mínimo mais janela (minutos, em relógio de parede, salvo no banco como o `BossData` do raid);
    - fases por vida, com as habilidades da T4;
    - anúncio 5 min antes, para quem está no mesmo mapa;
    - XP e saque por dano do grupo, reaproveitando `raidCredit`/`pools`;
    - um **túmulo** no chão por 10 min, com a hora da morte e o nome de quem mais causou dano.
  - O mapa mostra a janela, não o minuto (`setWindow`).
  - Arquivos: `server/internal/zone/raid.go` (extrair o comum), `monsters.go`, `client/src/ui/raid.ts` (barra do chefe).
- **Pronto quando:**
  - um chefe de teste com 3 fases morre, divide o saque e renasce dentro da janela depois de um reinício do servidor;
  - o túmulo aparece;
  - o raid continua igual.
- **Depende de:** T4.

**T6. Monstros raros (named)**
- **Contexto:**
  - `RareDef`: hospedeiro, chance por renascimento (2–4%), mínimo de 45 min entre aparições, condição (só à noite, só na metade sul de uma área), atributos (nível +2, vida ×4, 1,2× maior, nome dourado) e saque garantido.
  - Aviso só para quem está a até 60 m ("Algo grande se move no bosque…").
  - Conquista nova "Caçador de Raros" (matar os 4 primeiros) em `server/internal/game/achievements.go`.
  - Telemetria: evento `rare_spawn` e `rare_kill` em `game_events`.
  - Os 4 primeiros são os da tabela 5.9.
- **Pronto quando:**
  - teste: com chance 1, o raro nasce no lugar do hospedeiro e respeita o mínimo de 45 min;
  - o nome dourado aparece no cliente;
  - o saque garantido cai.
- **Depende de:** T2 (condição de período). Usa a T4 se o raro tiver aviso.

**T7. Gerador de bioma no build-world (refatorar sem mudar o mapa)**
- **Contexto:**
  - `client/scripts/build-world.mjs` tem 544 linhas, com a floresta, o Vale e a Cripta no mesmo arquivo, cada um com o seu PRNG (`20261113`, `20261114`, `20261115`).
  - Separar em `client/scripts/biomes/<nome>.mjs`, cada um com:
    - semente própria;
    - limites;
    - regra de transição (como o `rocky()` do Vale);
    - listas de malhas;
    - cristas (`RIDGES`);
    - landmarks;
    - colisores;
    - a função que o cliente usa para pintar o chão (`map-painter.ts`).
  - O `world.json` ganha `biomes: []` em vez do `biome` único, com o cliente aceitando os dois formatos durante a transição.
- **Pronto quando:**
  - o `world.json` gerado é **byte a byte igual** ao de antes (teste no CI comparando o hash);
  - criar um bioma de teste é escrever 1 arquivo;
  - o README explica como.
- **Depende de:** nada.

**T8. Cenário em blocos (thin instances por célula) e impostores de montanha**
- **Contexto:**
  - Hoje cada tipo de objeto é uma thin instance do mapa inteiro (`client/src/world/environment.ts`). Para o Mapa 1 crescer a 480 m (A2), dividir:
    - as thin instances por célula de 60 m, para o frustum e a névoa descartarem;
    - as montanhas da borda a mais de 120 m viram impostor;
    - `freezeWorldMatrix` em tudo que é estático.
  - Lembrar que a névoa por região já existe (#92).
- **Pronto quando:**
  - no mapa atual, draw calls ≤ os de hoje e malhas ativas iguais ou menores em todas as regiões (tabela do `?bench=1` na task);
  - com um bioma de teste de +180 m, as regiões antigas não perdem FPS.
- **Depende de:** T7.

**T9. Famílias de monstro: rigs quadrúpede, artrópode e amorfo, com VAT**
- **Contexto:**
  - Seção 4.2 deste documento. A #98 já decidiu VAT nos comuns e esqueleto nos chefes, e cuida do esqueleto e do construto (golem).
  - Esta task cria as 3 famílias que faltam:
    - **quadrúpede:** base Wolf/Husky da Quaternius Ultimate Animated Animals, CC0;
    - **artrópode:** base Spider da Quaternius, CC0;
    - **amorfo:** base Blob da Quaternius Ultimate Monsters, CC0, com shape keys.
  - Cada família recebe:
    - o contrato de animação de hoje;
    - VAT assado no build com `VertexAnimationBaker` do Babylon, salvo em JSON;
    - LOD0/1/2 dentro do orçamento do guia (monstro comum ≤ 4.000 triângulos);
    - 1 atlas com troca de paleta.
  - Os primeiros membros: Lobo Cinzento, Aranha-de-Musgo e Lodo Vivo.
  - Tudo passa por `client/scripts/import-asset.mjs`.
  - Conferir e guardar a licença CC0 de cada download no `client/public/models/CREDITS.md`, porque a Quaternius tem pacotes sob a QAL.
- **Pronto quando:**
  - 20 lobos animados na tela custam ≤ 3 draw calls por LOD e menos de 1 ms de CPU de animação no PC;
  - passam no teste de 85 px do guia.
- **Depende de:** #90, #93, #95 e #98 (o padrão do VAT).

**T10. Bosque Gris (bandidos) e o núcleo das Terras Mortas, com modelos atuais**
- **Contexto:** seções 5.1 e 5.2. Entram:
  - o setor NO dos anéis 2 e 3 (100°–170°, r 80–150), com 3 monstros humanoides: Bandoleiro, Atirador de Facas e Brutamontes, que reaproveitam `rogue` e `barbarian` com cor e acessório;
  - a Toca do Corvo, com o Corvo Rubro (chefe de campo, T5);
  - o canto NE (r > 142, 15°–75°), com Acólito, Necromante, Cavaleiro Ossário, Carniçal (noite) e Espectro (noite), reaproveitando os esqueletos e o mago;
  - a Torre do Sino Rachado, que toca nos 10 min de aviso do Ozrhal (`raidWarnBefore`);
  - a Lápide do Último Regicida (`BossData.TopName`);
  - o Bastião Cinzento: o posto, a pedra a 260 moedas e o NPC Irmão Caedmon;
  - 8 materiais novos em `items.go` e 6 receitas em `recipes.go` (capas e luvas de nível 13; cetro e varinha épicos e pergaminho abençoado de nível 24);
  - uma música e um ambiente por região (sistema da #67).
  - Os anéis 2 e 3 perdem 9 esqueletos no setor.
- **Pronto quando:**
  - as duas regiões aparecem no mapa e no minimapa;
  - o pico de monstros a 70 m é ≤ 14;
  - FPS dentro da meta com as regiões cheias;
  - as cadeias de missões das duas regiões estão escritas como dados para a #71.
- **Depende de:** T2, T3, T5 e T6. As missões dependem da #71.

**T11. Senhor dos Ossos como chefe de campo com 3 fases**
- **Contexto:**
  - Hoje o Senhor dos Ossos (`bone_lord`, nível 22, em (0, −132)) renasce a cada 5 min e não tem habilidade.
  - Fases da tabela 5.9, janela de 30–45 min e anúncio.
  - O modelo próprio continua na #98 (que já o lista) e na #68.
- **Pronto quando:** as 3 fases têm avisos, o saque é por dano e há teste de fases.
- **Depende de:** T4 e T5.

**T12. Brejo de Lamagris: expansão leste, Posto do Barqueiro e extensão das Terras Mortas**
- **Contexto:**
  - Seção 5.3. Os limites do mapa passam a `maxX = 330` (em `build-world.mjs` e no servidor, `internal/world`):
    - bioma Brejo em x 150–330 e z −150..60;
    - extensão das Terras Mortas em x 150–270 e z 60–150;
    - estrada leste até (165, 0).
  - 5 monstros (Brejeiros neutros de dia via T3), a Mãe-Lodo e a Velha Escamosa.
  - Água rasa andável e funda com colisor.
  - Posto do Barqueiro (pedra a 180 moedas), com o barco até a Ilha dos Vaga-lumes.
  - O Antídoto como a 1ª poção de receita.
  - Paleta da região no guia (`docs/guia-de-arte.md`, tabela 3.3) e na LUT (#92).
  - A fauna do Bosque Gris (lobo, javali, aranha) entra junto, porque usa as mesmas famílias.
- **Pronto quando:**
  - 36 monstros e o chefe, com pico ≤ 16 a 70 m;
  - regiões antigas sem perda de FPS;
  - celular simulado ≥ o do A0 no Brejo;
  - o lote de missões para a #71 está pronto.
- **Depende de:** T7, T8, T9, #99 (água) e #98.

**T13. Zonas múltiplas no mesmo processo, com transferência por portão**
- **Contexto:**
  - Hoje `server/cmd/zone/main.go` cria uma zona só (`zone.New(..., "starter")`). Criar:
    - um `zone.Manager` que segura N zonas, cada uma com `world.json`, spawns, pedras, NPCs e goroutine próprios;
    - uma entidade "portão" que, ao ser usada, salva e transfere o personagem para outra zona, com uma só dona por vez e sem duplicar itens.
  - Grupo, clã, chat geral, sussurro, amigos, correio e armazém continuam funcionando entre zonas, via um barramento no processo (hoje a zona é a dona de tudo; separar o que é global).
  - No cliente:
    - tela de carregamento;
    - descarte da cena;
    - carregamento do `world.json` e dos kits do mapa novo;
    - minimapa e mapa do mundo por mapa.
  - É a base da #80 (instância = zona criada de um modelo) e da #86 (canal = cópia do mesmo mapa).
- **Pronto quando:**
  - teste com 2 zonas e 50 bots passando de uma para a outra por 10 min, sem item duplicado nem personagem em duas zonas (usar as invariantes da #112);
  - grupo e chat funcionam entre zonas;
  - o tick de cada zona fica < 5 ms.
- **Depende de:** #112 (invariantes). Destrava a #80, a #86 e as T14, T15, T18 e T19.

**T14. Masmorra 1: Catacumbas de Vel'Aran (fecha a #80)**
- **Contexto:**
  - Seção 8. Porta na igreja da cidade. O líder do grupo entra e cria a cópia; os demais entram até 2 min depois. A cópia some 5 min depois de vazia ou com 45 min.
  - 2 andares de 60 × 60 m, gerados por um bioma da T7 com o KayKit Dungeon Pack (CC0, grátis; incluir no `CREDITS.md`).
  - ~30 esqueletos e o Abade Ulric (chefe com as 3 formas de aviso da T4).
  - Baú 1×/dia por personagem (salvo no personagem).
  - Telemetria por cópia: duração e mortes.
- **Pronto quando:**
  - o critério da #80: grupo entra, mata e sai com saque próprio, e várias cópias rodam ao mesmo tempo sem afetar o mundo aberto;
  - FPS ≥ ao do Arredores vazio.
- **Depende de:** T13, T4 e T5.

**T15. Minas de Brasaferro (Mapa 3), Acampamento Lanterna e receitas de arma**
- **Contexto:**
  - Seção 5.4. Mapa de 240 × 240 m com 3 patamares, sem teto (as paredes entre a câmera e o personagem somem).
  - Kobold (humanoide em escala 0,7), Aranha das Galerias, Mineiro (esqueleto) e Golem de Ferro-Bruto (paleta do construto).
  - Tecelã das Galerias (chefe com a mecânica do casulo como entidade) e o Capataz Hurlo.
  - Elevador de Carga (atalho por missão), vagonete (teleporte curto) e as 8 receitas de arma.
  - A D2 (Galerias Profundas, 3 andares) pode ser uma subtask.
- **Pronto quando:**
  - 40 monstros e o chefe, com pico ≤ 18;
  - o portão de Brasaferro funciona;
  - o elevador abre só depois da missão;
  - FPS dentro da meta.
- **Depende de:** T13, T9 e T12 (as famílias prontas).

**T16. Descoberta de teleportes e pedras por mapa**
- **Contexto:**
  - Hoje toda pedra vê toda pedra (`server/internal/zone/teleport.go`, `teleportStones`). Passar a:
    - um destino só aparece depois que o personagem tocou na pedra a pé uma vez (`characters.discovered`, migração na faixa da #89);
    - pedras por mapa, com viagem entre mapas pela T13;
    - preços novos: Barqueiro 180, Bastião 260, Lanterna 240, Maresia 300, Vigília 320, Valdurn 400.
  - O Pergaminho de Retorno passa a levar à última cidade visitada.
  - Quem já tem personagem ganha as 5 pedras atuais como descobertas.
- **Pronto quando:** testes de descoberta; a janela de teleporte mostra as pedras "não descobertas" em cinza, com a região.
- **Depende de:** T13 (para viagem entre mapas). A parte do Mapa 1 pode vir antes.

**T17. Eventos de área com etapas e medalha por participação**
- **Contexto:**
  - Generaliza a #77 (invasão noturna). `EventDef` com:
    - área;
    - gatilho (noite, chance, horário);
    - etapas (matar N, defender um NPC ou objeto por X s, chefe), com limite de tempo;
    - recompensa por participação (100%, 85% ou 75%, conforme dano, cura ou presença no círculo).
  - O resultado muda o mundo: o posto cai, a loja fecha por 15 min.
  - Primeiro evento: "O Bastião resiste" (seção 7). Segundo: "Acender o Farol" (Costa, A6).
- **Pronto quando:**
  - o evento de teste roda as 3 etapas, dá as medalhas certas e fecha e reabre o posto;
  - o anúncio sai só para o mapa;
  - a #77 pode ser reescrita em cima dele.
- **Depende de:** T5. Coordenar com a #77.

**T18. Ruínas de Ilvaren (Mapa 4), Vigília, o Sigilo e o Santuário Submerso**
- **Contexto:**
  - Seção 5.6.
  - O Sigilo é uma flag no personagem que deixa as Sentinelas neutras para ele.
  - Ecos passivos de dia, andando numa rota de "lembrança" (pontos fixos).
  - Shader translúcido (fresnel), com no máximo 10 translúcidos a 70 m.
  - O Colosso com os Pilares-Runa (entidades) e o segredo dos 4 sinos (ordem diária).
  - A D3 é uma subtask.
  - As 5 Pedras-Selo destravam a D4 (Necrópole).
- **Pronto quando:** 38 monstros e o chefe; o Sigilo e os Ecos funcionando; FPS dentro da meta nos terraços.
- **Depende de:** T13, T9, #99 (relevo) e #71 (a linha de Ysolde).

**T19. Costa dos Naufrágios e Maresia (Mapa 2), a segunda cidade**
- **Contexto:** seções 5.5 e 6.1.
  - Portão na foz do Brejo (330, −60).
  - Maresia com 7 NPCs, renascimento escolhível, a Praça do Mercado (#78) e a Arena das Marés (#81).
  - A costa com 5 monstros, o Rei-Pinça e o Barba-de-Sal.
  - A Barra de Areia: colisor que liga e desliga com o ciclo dia/noite (o A* precisa recalcular só essa faixa).
- **Pronto quando:** a cidade cheia (50 bots) fica ≥ 30 FPS num celular médio de verdade; a costa tem 40 monstros; o mercado funciona em Maresia.
- **Depende de:** T13, T16, T17, #78, #81, #86, #99 e #100.

**T20. Do 30 ao 40: plano detalhado (Terras Altas, Valdurn, castelo e Coração de Brasa)**
- **Contexto:**
  - Seções 5.7, 5.8 e 6.2.
  - Antes de código, escrever a versão 2 deste documento com:
    - a curva de XP do 31 ao 40;
    - os itens de nível 32 a 38;
    - a 3ª classe (nível 40);
    - o calendário de cerco do Castelo de Valdurn (L2: cerco de 2 h, em domingos alternados, às 20h);
    - o teto de imposto (10%);
    - o orçamento dos Mapas 5 e 6.
- **Pronto quando:** o documento é aprovado e quebrado em tasks.
- **Depende de:** A6 e #82.

---

## 12. Quadro-resumo de custo por área

| Área | Fase | Mapa | Monstros (pico a 70 m) | Modelos novos (famílias) | Objetos de cenário (malhas novas) | Dependências fortes |
|---|---|---|---|---|---|---|
| Bosque Gris | A1/A2 | 1 | 22 (14) | A1: 0 · A2: 3 (quadrúpede, artrópode) | +300 (0) | T2, T3, T9 |
| Terras Mortas | A1/A2 | 1 | 26 (14) | 0 (esqueleto, humanoide) | +150 (9) | T5, T6 |
| Brejo de Lamagris | A2 | 1 | 37 (16) | 4 (humanoide, amorfo, único, esqueleto) | +1.300 (12) | T7, T8, T9, #99 |
| Minas de Brasaferro | A4 | 3 | 41 (18) | 3 + paleta | 1.500 (20) | T13, T15 |
| Ruínas de Ilvaren | A5 | 4 | 39 (16) | 3 + paleta | 1.400 (25) | T13, #99 |
| Costa dos Naufrágios | A6 | 2 | 41 (16) | 4 | 1.400 + cidade 300 (6 + 12) | T13, #78, #81, #100 |
| Dentes do Inverno | A7 | 5 | 40 (18) | 2 + Wyrm | paleta + cidade 350 | T20, #82 |
| Coração de Brasa | A7 | 6 | 40 (18) | 3 | 1.200 (20) | T20 |

---

## 13. Fontes

**Lineage 2 (zonas, Seven Signs, castelos, chefes)**
- https://tab1-web.github.io/faq/eternal/levelling (zonas e catacumbas por faixa)
- https://forums.lineage2.com/topic/22600-hunting-zones-lvl-40-45/
- https://www.ludo.guide/guide/lineage-ii/boss-guides/core
- https://wiki.l2db.club/classic/Forge%20of%20the%20Gods.html
- https://wiki.l2db.club/classic/Castle%20Sieges.html
- https://l2db.net/guides/en/l2-leveling-guide-1-75
- https://predator.ge/en/news/lineage-2-locations-guide-starting-villages-territories-cities-and-hunting-zones
- https://predator.ge/en/news/understanding-lineage-2s-seven-signs-system-a-complete-guide-to-catacombs-and-ancient-adena
- https://predator.ge/en/news/the-castle-system-in-lineage-2-a-complete-guide-to-taxes-and-territory-control
- https://www.gamemeca.com/view.php?gid=52839
- https://www.l2scroll.com/2015/10/clan-halls.html
- https://www.lineage2.com/news/gludio-castle-siege
- https://guildorder.com/games/lineage_2/wiki/epic-boss-windows-and-scouting
- https://devtrackers.gg/lineageii/p/84e7dbb5-new-field-raid-bosses-may-18-2022
- https://eu.4game.com/patchnotes/lineage2essence/188
- https://www.invenglobal.com/articles/21801/the-ivory-tower-awaits-oren-update-confirmed-for-june-4 (Lineage Classic)

**World of Warcraft**
- https://warcraft.wiki.gg/wiki/Spawn
- https://warcraft.wiki.gg/wiki/Un%27Goro_Crater
- https://www.engadget.com/2011-03-02-gdc-2011-tom-chilton-discusses-cataclysm-design-talent-and-zon.html
- https://gameinformer.com/games/world_of_warcraft_cataclysm/b/pc/archive/2009/12/02/blizzard-s-jeff-kaplan-on-warcraft-s-past-and-future
- https://blizzardwatch.com/2016/11/06/wows-level-designers-telling-stories-landscapes
- https://shacknews.com/article/115595/after-the-fact-john-staats-on-storytelling-in-world-of-warcraft

**Guild Wars 2**
- https://wiki.guildwars2.com/wiki/Queensdale
- https://wiki.guildwars2.com/wiki/Renown_heart
- https://wiki.guildwars2.com/wiki/Dynamic_event
- https://www.guildwars2.com/en/news/the-megaserver-system-world-bosses-and-events/
- https://wiki.guildwars2.com/wiki/World_boss
- https://wiki.guildwars2.com/wiki/Defeat_Tequatl_the_Sunless
- https://wiki.guildwars2.com/wiki/Dragon's_Stand_(meta_event)
- https://wiki.guildwars2.com/wiki/The_Silverwastes

**Old School RuneScape, Albion, Black Desert, FFXIV e Ragnarok**
- https://oldschool.runescape.wiki/w/Wilderness_boss · https://oldschool.runescape.wiki/w/Wilderness_level · https://oldschool.runescape.wiki/w/Slayer_Master · https://oldschoolrunescape.fandom.com/wiki/Revenant_Cave · https://theoatrix.net/post/slayer-masters-in-osrs
- https://albionfreemarket.com/articles/view/albion-online-guide-to-zones-and-flagging
- https://www.blackdesertfoundry.com/nodes-guide · https://www.blackdesertfoundry.com/daynight-vendors-guide
- https://ffxiv.consolegameswiki.com/wiki/FATE · https://ffxiv.consolegameswiki.com/wiki/The_Hunt · https://ffxiv.consolegameswiki.com/wiki/Area_of_Effect · https://ffxiv.consolegameswiki.com/wiki/Grand_Company
- https://irowiki.org/wiki/MVP · https://irowiki.org/wiki/Clock_Tower · https://www.irowiki.org/wiki/OGH

**Design de áreas e de inimigos**
- https://www.gamedeveloper.com/design/a-taxonomy-of-weenies-the-landmarks-that-define-i-ghost-of-tsushima-i- (a cada 30 s, algo chama o jogador)
- https://www.tweaktown.com/news/59420/witcher-3s-40-second-rule-kept-players-engaged/index.html
- https://www.gameinformer.com/2021/05/20/more-than-just-dots-on-a-map-the-challenges-behind-open-world-game-creation
- https://www.thegamer.com/dark-souls-1-fromsoftwares-magnum-opus-of-interconnected-level-design/
- https://book.leveldesignbook.com/process/combat/enemy
- https://www.gamedeveloper.com/design/cyber-demons-the-ai-of-doom-2016-
- https://www.gamedeveloper.com/design/the-dark-souls-design-of-doom-eternal
- https://www.gamedeveloper.com/design/enemy-attacks-and-telegraphing
- https://craftingworlds.com/forums/threads/wildstar-interview-with-lead-combat-designer-chris-lynch-part-1.3625
- https://www.digitaltrends.com/gaming/god-of-war-combat-guide/

**Assets CC0 e VAT**
- KayKit: https://kaylousberg.itch.io/ (Adventurers, Skeletons e Extra, Character Animations, Forest, Dungeon Pack, Medieval Hexagon, Halloween Bits, Resource Bits)
- Quaternius (licença e pacotes): https://quaternius.com/license.html · https://quaternius.com/packs/ultimatemonsters.html · https://quaternius.com/packs/ultimateanimatedanimals.html · https://quaternius.com/packs/ultimatemodularruins.html · https://quaternius.com/packs/piratekit.html · https://quaternius.com/packs/animatedmonster.html
- Kenney: https://kenney.nl/assets/modular-cave-kit · https://kenney.nl/assets/graveyard-kit · https://kenney.nl/assets/pirate-kit · https://kenney.nl/assets/nature-kit · https://kenney.nl/assets/fantasy-town-kit · https://kenney.nl/assets/castle-kit · https://kenney.nl/assets/survival-kit
- poly.pizza (modelos CC0 da Quaternius): Wolf https://poly.pizza/m/P1gU3Qkr9r · Spider https://poly.pizza/m/yRYJiAJyiM · Crab https://poly.pizza/m/Gs3yfsV5lB · Yeti https://poly.pizza/m/ceRHrn8HHE · Goleling https://poly.pizza/m/71gomWolax
- https://flo-bit.itch.io/low-poly-nature-pack (árvores mortas e cogumelos, CC0)
- VAT: documentação do Babylon.js, "Baked Texture Animations" (doc.babylonjs.com) · https://github.com/sharpen3d/openvat · https://github.com/flanb/VAT-blender-addon (conferir a licença)

**Do próprio projeto**
- `README.md`, `RELATORIO.md`, `docs/guia-de-arte.md`
- `server/internal/game/monsters.go`, `items.go`, `recipes.go`, `quests.go`, `daytime.go`
- `server/internal/zone/monster.go`, `raid.go`, `teleport.go`, `aoi.go`, `mapzones.go`
- `client/src/ui/regions.ts`, `client/scripts/build-world.mjs`, `client/src/world/golem.ts`
- Relatório de FPS: anexo 11 da tunnel#64 (`relatorio-fps-2026-11-13.md`)
