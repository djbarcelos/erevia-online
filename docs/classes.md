# Classes, skills e especializações

Referência das 4 classes jogáveis do Erevia: atributos, árvore de skills (3 ramos × 5 degraus) e as 2 especializações de cada classe.

> Retrato da main de 19/11. As especializações vão virar **evoluções** com árvore própria (tunnel#180); o desenho novo está em [evolucoes/](evolucoes/README.md).

Fonte da verdade (este documento é um retrato do código; se divergir, vale o código):

- [server/internal/game/classes.go](../server/internal/game/classes.go): atributos base e por nível
- [server/internal/game/skills.go](../server/internal/game/skills.go): efeitos, passivas e árvores das classes
- [server/internal/game/specs.go](../server/internal/game/specs.go): especializações
- [server/internal/game/spec_skills.go](../server/internal/game/spec_skills.go): skills das especializações

## Regras gerais

- **Nível máximo:** 40.
- **Pontos de skill:** nível + 1 até o nível 30 (`SkillPointMaxLevel`); do 31 ao 40, o nível dá só pontos de atributo. Cada classe começa com 2 skills no rank 1, e esses dois pontos já contam. (Com as evoluções, passa a ser 1 ponto por nível até o teto: decisão 33 da tunnel#180.)
- **Degraus da árvore:** liberados nos níveis **1, 5, 10, 15 e 20**. Para aprender uma skill, a skill de cima no mesmo ramo precisa estar aprendida.
- **Ranks máximos:** 3 nas ativas, 5 nas passivas do começo do ramo e 1 nas skills do 4º degrau.
- **Os pontos não dão para tudo:** a árvore de uma classe soma cerca de 43 ranks, e no nível 30 o personagem tem 31 pontos. É preciso escolher.
- **Especialização:** liberada no nível 20 pela missão **"A Encruzilhada"**, com a Mestra dos Caminhos na praça. Ela dá:
  - um bônus passivo de atributos;
  - um ramo extra de 3 skills, liberadas nos níveis **20, 24 e 28**.

  Trocar de especialização custa **50.000** e devolve os pontos das skills do caminho antigo.

## Atributos base

Valores no nível 1; entre parênteses, o ganho por nível.

| Classe | HP | MP | Atq. físico | Atq. mágico | Def. física | Def. mágica | Alcance | Intervalo entre ataques | Crítico | Velocidade |
|---|---|---|---|---|---|---|---|---|---|---|
| **Cavaleiro** | 160 (+22) | 40 (+4) | 8 (+2,2) | 4 (+0,5) | 24 (+3,5) | 15 (+2) | 2,2 m | 1,2 s | 5% | 5 m/s |
| **Bárbaro** | 150 (+20) | 35 (+3) | 10 (+3,0) | 3 (+0,4) | 16 (+2,5) | 12 (+1,5) | 2,2 m | 1,4 s | 10% | 5 m/s |
| **Mago** | 95 (+11) | 120 (+14) | 6 (+0,6) | 12 (+3,2) | 12 (+1,5) | 25 (+3) | 12 m (mágico) | 1,6 s | 5% | 5 m/s |
| **Ladino** | 120 (+15) | 60 (+6) | 7 (+2,0) | 4 (+0,5) | 16 (+2) | 12 (+1,5) | 2,2 m | 0,8 s | 25% | 5,5 m/s |

Como ler as tabelas de skills:

- Os valores aparecem como **rank 1 → rank máximo**.
- ★ marca as skills iniciais.
- "Ataque" é o ataque físico; nas skills do Mago, é o ataque mágico.

---

## ⚔️ Cavaleiro: tanque e suporte

| Ramo | Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|---|
| **Guardião**<br>defesa e controle da linha de frente | 1 | ★ Golpe de Escudo | 150→190% do ataque e atordoa por 1,5→2 s | 10 / 8 s |
| | 5 | Pele de Ferro (passiva) | +4→20% de defesa física | – |
| | 10 | Provocar | Obriga os inimigos a até 6 m a atacar você por 4→8 s | 18 / 15 s |
| | 15 | Muralha Viva (passiva) | Abaixo de 30% de vida, ganha um escudo de 15% da vida máxima por 10 s (uma vez a cada 60 s) | – |
| | 20 | Baluarte | 140→180% a até 5 m, atordoa por 1,5→2 s e provoca os atingidos | 35 / 24 s |
| **Lâmina**<br>esgrima e dano corpo a corpo | 1 | Esgrima (passiva) | +3→15% de ataque físico com espada | – |
| | 5 | Corte Giratório | 105→135% em todos a até 3 m | 16 / 9 s |
| | 10 | Investida | Salta até o alvo (até 12 m), 120→160% e atordoa por 1 s | 14 / 12 s |
| | 15 | Julgamento | 400% do ataque, não erra | 40 / 45 s |
| | 20 | Corte do Horizonte | 200→260% no alvo e a até 3,5 m dele; os atingidos ficam 40% mais lentos por 3 s | 32 / 15 s |
| **Comando**<br>gritos e auras para os aliados | 1 | ★ Fortaleza | +50→70% de defesa por 10 s | 15 / 25 s |
| | 5 | Grito de Guerra | Você e os aliados a até 10 m ganham +10→20% de ataque por 20 s | 20 / 30 s |
| | 10 | Aura de Coragem (passiva) | Você e os aliados a até 10 m regeneram 2→6 de vida por segundo | – |
| | 15 | Estandarte | Estandarte no chão por 20 s: +20% de defesa para quem estiver a até 8 m | 45 / 90 s |
| | 20 | Égide da Ordem | Escudo de 250→500 de dano por 8 s em você e nos aliados a até 10 m | 45 / 45 s |

### Especializações do Cavaleiro

**Bastião** (+12 Constituição): a muralha que não cede. Provoca, protege e aguenta o que vier.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Desafio do Bastião | Provoca os inimigos a até 7 m por 4 s e dá a você um escudo de 300→600 por 10 s | 35 / 25 s |
| 24 | Vigor do Bastião (passiva) | +5→15% de vida máxima | – |
| 28 | Último Bastião | 160% a até 6 m, atordoa por 2 s e dá +60% de defesa a você por 8 s | 60 / 60 s |

**Arauto** (+12 Sabedoria): leva a luz da ordem ao campo, com golpes que enfraquecem e bênçãos para o grupo.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Martelo da Aurora | 190→250% no alvo e a até 3 m dele, sem errar; os atingidos perdem 15→25% de ataque por 6 s | 30 / 14 s |
| 24 | Bênção do Arauto | Você e os aliados a até 12 m ganham +15→25% de velocidade de ataque por 15 s | 40 / 40 s |
| 28 | Sol Nascente | 250% a até 7 m e cega os inimigos (35% mais erros) por 5 s | 60 / 50 s |

---

## 🪓 Bárbaro: dano físico bruto e resistência

| Ramo | Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|---|
| **Massacre**<br>golpes pesados que encerram lutas | 1 | ★ Golpe Brutal | 220→280% do ataque | 10 / 6 s |
| | 5 | Machado Pesado (passiva) | +8→40% de dano crítico | – |
| | 10 | Rachar o Chão | 125→155% a até 5 m; os atingidos ficam 40% mais lentos por 3 s | 22 / 14 s |
| | 15 | Execução | 350%; o dobro em alvos abaixo de 30% de vida | 30 / 30 s |
| | 20 | Machadada Sísmica | 240→300% no alvo e a até 3 m dele; +50% em quem estiver abaixo de 30% de vida | 35 / 16 s |
| **Sangue**<br>fúria, roubo de vida e teimosia | 1 | ★ Fúria | +30→50% de ataque por 10 s | 15 / 25 s |
| | 5 | Sede de Sangue (passiva) | Recupera 2→10% do dano causado como vida | – |
| | 10 | Frenesi | +25→45% de velocidade de ataque por 12 s | 20 / 25 s |
| | 15 | Imortal | Por 5 s, a vida não cai abaixo de 1 | 50 / 120 s |
| | 20 | Redemoinho Sangrento | 2 golpes de 90→120% em todos a até 4 m; a Sede de Sangue cura a cada golpe | 30 / 12 s |
| **Selvagem**<br>resistência e instinto animal | 1 | Couro Grosso (passiva) | +5→25% de vida máxima | – |
| | 5 | Rugido | Inimigos a até 6 m perdem 15→25% de ataque por 8 s | 18 / 20 s |
| | 10 | Pisotear | 100% a até 4 m e atordoa por 1,5→2,5 s | 24 / 18 s |
| | 15 | Espírito do Urso (passiva) | Abaixo de 50% de vida, regenera 3% da vida máxima por segundo | – |
| | 20 | Pele de Pedra | +35→55% de defesa por 10 s; os inimigos a até 5 m levam 100→130% e ficam 30% mais lentos por 3 s | 35 / 30 s |

### Especializações do Bárbaro

**Sanguinário** (+12 Força): vive para a próxima ferida, com golpes enormes e críticos sem fim.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Golpe Rubro | 260→330%; +30% em alvos abaixo de 50% de vida | 25 / 10 s |
| 24 | Fúria Cega (passiva) | +3→9% de chance de crítico | – |
| 28 | Carnificina | 3 golpes de 110% em todos a até 5 m (a Sede de Sangue cura a cada golpe) | 60 / 45 s |

**Caudilho** (+6 Força e +6 Constituição): comanda a carga, com brados para os aliados, avanços e o estandarte de guerra.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Brado do Caudilho | Você e os aliados a até 12 m ganham +12→20% de ataque por 20 s | 30 / 35 s |
| 24 | Avanço do Caudilho | Salta até o alvo (até 12 m), 180→230% nele e a até 4 m dele, e atordoa por 1→1,5 s | 40 / 22 s |
| 28 | Estandarte de Guerra | 150% a até 6 m (os inimigos perdem 20% de ataque por 8 s) e finca um estandarte por 20 s: +20% de defesa a até 8 m | 55 / 75 s |

---

## 🔥 Mago: dano mágico à distância e controle

| Ramo | Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|---|
| **Fogo**<br>dano alto à distância | 1 | ★ Bola de Fogo | 250→310% do ataque mágico, até 20 m | 15 / 3 s |
| | 5 | Brasa Viva (passiva) | As skills de fogo queimam o alvo: 6→30% do ataque mágico por segundo, por 3 s | – |
| | 10 | Chuva de Brasas | 160→200% no alvo e a até 5 m dele (alcance de 16 m) | 35 / 12 s |
| | 15 | Meteoro | 500% no alvo e a até 3 m dele, e atordoa por 2 s | 80 / 60 s |
| | 20 | Inferno | 180→230% no alvo e a até 5 m dele, mais queimadura de 25→35% por segundo por 5 s | 50 / 18 s |
| **Gelo**<br>controle e proteção | 1 | ★ Nova de Gelo | 120→160% a até 6 m e 50% de lentidão por 3→4 s | 30 / 10 s |
| | 5 | Lança de Gelo | 170→210% (até 18 m) e 30% de lentidão por 2 s | 18 / 5 s |
| | 10 | Armadura de Gelo | +30→50% de defesa por 20 s; quem bate de perto fica 30% mais lento por 2 s | 30 / 40 s |
| | 15 | Prisão Glacial | 50% e prende o alvo no lugar por 4 s | 40 / 30 s |
| | 20 | Zero Absoluto | 150→200% a até 6 m e congela (atordoa) por 1,5→2 s | 55 / 30 s |
| **Arcano**<br>mana, mobilidade e poder bruto | 1 | Mente Clara (passiva) | +10→50% de regeneração de mana | – |
| | 5 | Escudo Arcano | Por 8→12 s, o dano recebido sai da mana em vez da vida | 25 / 30 s |
| | 10 | Piscar | Teleporta 8→12 m para a frente | 20 / 18 s |
| | 15 | Sobrecarga (passiva) | +15% de ataque mágico; as skills custam 10% a mais de mana | – |
| | 20 | Rajada Arcana | 300→380% (até 20 m), não erra | 45 / 10 s |

### Especializações do Mago

**Elementalista** (+12 Inteligência): fogo e gelo na forma mais pura, com dano alto e cataclismos.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Lança Ígnea | 260→330% (até 20 m); é uma skill de fogo, então a Brasa Viva queima | 28 / 6 s |
| 24 | Foco Elemental (passiva) | +15→45% de dano crítico | – |
| 28 | Cataclismo | 400% no alvo e a até 6 m dele, com 50% de lentidão por 4 s | 90 / 60 s |

**Ocultista** (+6 Inteligência e +6 Espírito): pestes, maldições e marcas que fazem o inimigo apodrecer por dentro.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Peste Sombria | 120→160% e praga de 20→30% por segundo por 8 s; se o alvo morrer, a praga pula para os inimigos a até 6 m | 30 / 12 s |
| 24 | Drenar Vontade | 160→210% e o alvo perde 20→30% de ataque por 8 s | 30 / 14 s |
| 28 | Marca do Abismo | 150% no alvo e a até 5 m dele; os atingidos recebem +20% de dano de todas as fontes por 10 s | 70 / 45 s |

---

## 🗡️ Ladino: críticos, mobilidade e venenos

| Ramo | Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|---|
| **Assassinato**<br>críticos e golpes letais | 1 | ★ Golpe Furtivo | 200→260% com +50% de chance de crítico | 10 / 6 s |
| | 5 | Ponto Fraco (passiva) | +2→10% de chance de crítico | – |
| | 10 | Lâminas Gêmeas | 2 golpes de 115→145% | 16 / 8 s |
| | 15 | Marca da Morte | 20% do ataque; o alvo recebe +25% de dano de todas as fontes por 10 s | 30 / 40 s |
| | 20 | Golpe Fatal | 250→330% com +50% de crítico; +50% em alvos abaixo de 35% de vida | 35 / 18 s |
| **Sombras**<br>mobilidade, esquiva e invisibilidade | 1 | ★ Passo Sombrio | +40→60% de velocidade por 6 s | 12 / 20 s |
| | 5 | Esquiva (passiva) | +6→30 de esquiva | – |
| | 10 | Bomba de Fumaça | Inimigos a até 4 m erram 20→40% mais por 6 s | 22 / 25 s |
| | 15 | Sumir | Invisível por 5 s; atacar revela, e o primeiro golpe é crítico garantido | 35 / 60 s |
| | 20 | Golpe das Sombras | Salta até o alvo (até 12 m), 180→240% e o cega (30→50% mais erros) por 4 s | 30 / 20 s |
| **Venenos**<br>dano contínuo e controle | 1 | Lâmina Envenenada (passiva) | 5→25% de chance de envenenar ao acertar (4% do ataque por segundo, por 5 s) | – |
| | 5 | Frasco Tóxico | Envenena o alvo e todos a até 4 m dele: 25→35% do ataque por segundo, por 4 s | 18 / 10 s |
| | 10 | Paralisia | 80% do ataque; se o alvo estiver envenenado, atordoa por 2→3 s | 20 / 20 s |
| | 15 | Praga | 20% do ataque por segundo por 8 s; se o alvo morrer, a praga pula para os inimigos a até 6 m | 40 / 45 s |
| | 20 | Miasma | Envenena todos a até 6 m: 30→40% do ataque por segundo, por 6 s | 40 / 20 s |

### Especializações do Ladino

**Sicário** (+12 Destreza): mata de perto e rápido, com rajadas de lâminas e a sentença final.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Lâmina Oculta | 3 golpes de 100→130% com +20% de chance de crítico | 28 / 10 s |
| 24 | Instinto Assassino (passiva) | +10→30% de dano crítico | – |
| 28 | Sentença | Salta até o alvo (até 12 m) e causa 450%, sem errar; +50% em alvos abaixo de 30% de vida | 60 / 50 s |

**Caçador** (+6 Força e +6 Destreza): caça de longe, com facas de arremesso, redes e chuva de lâminas.

| Nível | Skill | Efeito | MP / Recarga |
|---|---|---|---|
| 20 | Faca de Arremesso | 180→230% à distância (até 16 m) e 30% de lentidão por 3 s | 20 / 6 s |
| 24 | Rede de Caça | 60% no alvo e a até 3 m dele, que ficam presos no lugar por 2,5→3,5 s | 30 / 20 s |
| 28 | Chuva de Lâminas | 2 golpes de 160% no alvo e a até 5 m dele | 60 / 40 s |

---

## Mecânicas compartilhadas

### Efeitos

| Efeito | O que faz |
|---|---|
| Atordoar | Não age nem anda |
| Prender | Não anda, mas ainda ataca e usa skills |
| Lentidão | Reduz a velocidade em X% |
| Provocação | O monstro é obrigado a atacar quem provocou |
| Cegueira | Chance extra de errar |
| Enfraquecer | −X% de ataque |
| Marca | +X% de dano recebido de todas as fontes |
| Veneno / Queimadura | Dano por segundo |
| Praga | Dano por segundo; pula para os vizinhos (6 m) se o alvo morrer infectado |
| Escudo | Absorve uma quantidade fixa de dano |
| Escudo de mana | O dano sai da mana em vez da vida |
| Imortal | A vida não cai abaixo de 1 |
| Invisível | Monstros não veem; atacar revela, e o próximo golpe é crítico |
| Armadura de Gelo | +defesa; quem bate de perto (até 4 m) fica 30% mais lento por 2 s |
| Buffs | Ataque, defesa, velocidade, velocidade de ataque, Grito de Guerra (soma com outros buffs), Estandarte e Aura de Coragem |

### Modificadores de golpe

| Modificador | O que faz |
|---|---|
| Golpes múltiplos | A skill acerta N vezes seguidas; a Sede de Sangue cura a cada golpe |
| Não erra | Ignora o teste de precisão contra esquiva |
| Crítico extra | Soma chance de crítico só naquela skill |
| Execução | Multiplica o dano em alvos abaixo de X% de vida |
| Investida | Salta até o alvo antes de bater |
| Piscar | Teleporta para a frente |
| Área no alvo | Acerta todos em volta do alvo |
| Área em volta de si | Acerta todos em volta de quem lança |
| Aliados | Buffs com raio aplicam o efeito também nos aliados próximos |
| Estandarte | Objeto no chão que dá buff a quem estiver perto |
