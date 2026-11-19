# Maestria

Referência da Maestria do Erevia: a progressão que começa no nível máximo.

Fonte da verdade (este documento é um retrato do código; se divergir, vale o código):

- [server/internal/game/mastery.go](../server/internal/game/mastery.go): regras, nós e valores
- [server/internal/zone/mastery.go](../server/internal/zone/mastery.go): onde cada efeito é aplicado no jogo

Veja também: [classes.md](classes.md) e [armas.md](armas.md).

## Resumo

- **É igual para todas as classes.** Cavaleiro, Bárbaro, Mago e Ladino têm a mesma árvore, com os mesmos nós e valores. Não existe Maestria por classe nem por especialização.
- **Abre no nível 40**, o nível máximo (`MasteryLevel = MaxLevel`). Quem ganhou pontos quando o máximo era 30 continua com eles.
- **É horizontal:** os nós dão conveniência (poções, ouro, viagem, controle, regeneração) e quase nenhuma força. Todo o ganho de combate da árvore inteira fica limitado a **5%**.

## Como se ganham pontos

- No nível 40, toda a XP (de monstros, missões e eventos, com as regras de sempre) passa a encher a **barra de Maestria**.
- Uma barra cheia vale **45.200.000 de XP**, o mesmo custo de subir do 39 para o 40 (`MasteryPointXP`). Cada barra cheia dá **1 ponto de Maestria**.
- A XP que sobrar ao chegar no 40 já entra na barra.
- **Teto:** 60 pontos (12 nós × 5 pontos). Com a árvore cheia, a barra para de encher.
- Os pontos são gastos na aba **Maestria** da janela de skills (tecla **K**).

### Bônus diário

- A primeira barra de cada dia enche **4× mais rápido**. O dia vira à meia-noite de Brasília.
- Quem fica sem jogar acumula o bônus por até **7 dias** (7 barras com bônus).
- Ao chegar no nível 40, o personagem já ganha o bônus do dia.

### Morte

- Morrer no 40 tira **2% de uma barra** (a mesma fração que a morte tira de um nível do 30 em diante) de XP de Maestria. Personagens com karma perdem o dobro.
- A morte nunca tira pontos já ganhos; só esvazia parte da barra atual.
- O nó **Segunda Chance** reduz essa perda.

### Redistribuir

- Devolve **todos** os pontos gastos.
- É **grátis**, uma vez por dia (meia-noite de Brasília).

## Regras da árvore

- **3 ramos com 4 nós cada.** Cada nó aceita até **5 pontos**.
- **Pré-requisito:** o 3º nó de cada ramo pede **5 pontos** gastos nos outros nós do ramo; o 4º pede **10**.
- **Retorno decrescente:** o 1º ponto de um nó vale 5× o 5º. Os pontos dão, em ordem, 40%, 25%, 15%, 12% e 8% do valor total do nó.
- **Teto de combate:** somando a árvore inteira, os efeitos de combate (dano, vida, defesa, crítico e velocidade de ataque) não passam de 5%. Hoje só dois nós entram nesse teto: Instinto do Caçador e Couro Curtido.

## Os nós

Os valores aparecem por ponto investido, de 1 a 5. A última coluna é o total com os 5 pontos.

### 🟡 Caça: poções, moedas e materiais

| Nó | Requisito | Efeito | 1 | 2 | 3 | 4 | 5 |
|---|---|---|---|---|---|---|---|
| **Alquimia de Campo** | – | Cura das poções | +8% | +13% | +16% | +18,4% | **+20%** |
| **Bolsa Farta** | – | Moedas que os monstros deixam | +6% | +9,8% | +12% | +13,8% | **+15%** |
| **Olho do Coletor** | 5 pontos em Caça | Chance de 1 material a mais no saque de monstros comuns (não vale para chefes e raids) | 6% | 9,8% | 12% | 13,8% | **15%** |
| **Instinto do Caçador** ⚔️ | 10 pontos em Caça | Dano (ataque físico e mágico) | +0,8% | +1,3% | +1,6% | +1,8% | **+2%** |

### 🟢 Sobrevivência: controle, morte e fôlego

| Nó | Requisito | Efeito | 1 | 2 | 3 | 4 | 5 |
|---|---|---|---|---|---|---|---|
| **Firmeza** | – | Duração de atordoamento e lentidão em você | −10% | −16,3% | −20% | −23% | **−25%** |
| **Segunda Chance** | – | XP da barra perdida ao morrer | −20% | −32,5% | −40% | −46% | **−50%** |
| **Fôlego** | 5 pontos em Sobrevivência | Regeneração de vida e mana fora de combate | +16% | +26% | +32% | +36,8% | **+40%** |
| **Couro Curtido** ⚔️ | 10 pontos em Sobrevivência | Vida máxima | +0,6% | +1% | +1,2% | +1,4% | **+1,5%** |

### 🔵 Utilidade: viagem e deslocamento

| Nó | Requisito | Efeito | 1 | 2 | 3 | 4 | 5 |
|---|---|---|---|---|---|---|---|
| **Passo Leve** | – | Velocidade fora de combate (o teto é 4%) | +1,6% | +2,6% | +3,2% | +3,7% | **+4%** |
| **Viajante** | – | Preço das Pedras de Teleporte | −12% | −19,5% | −24% | −27,6% | **−30%** |
| **Leitura Rápida** | 5 pontos em Utilidade | Tempo para viajar (Pedra de Teleporte e Pergaminho de Retorno) | −16% | −26% | −32% | −36,8% | **−40%** |
| **Caminho de Casa** | 10 pontos em Utilidade | Recarga do Pergaminho de Retorno | −20% | −32,5% | −40% | −46% | **−50%** |

⚔️ = nó de combate, que entra no teto de 5%.

## Exemplos práticos

- **Morte com Segunda Chance completa:** a perda cai de 4% para 2% da barra.
- **Leitura Rápida completa:** o Pergaminho de Retorno, que leva 5 s para ser lido, passa a levar 3 s.
- **Árvore cheia (60 pontos):** em combate, o personagem ganha só +2% de dano e +1,5% de vida máxima. O resto é conforto e economia.

## Comandos de GM

| Comando | O que faz |
|---|---|
| `/maestria xp N [Nome]` | Dá N de XP como se fosse caça, com o bônus diário |
| `/maestria dia [Nome]` | Simula a virada do dia: +1 dia de bônus e o Redistribuir liberado |
