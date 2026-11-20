# Plano: heróis v4 (visual novo das classes, pronto para aplicar depois)

Plano de produção do visual novo dos heróis: **um corpo** (masculino e feminino na mesma malha), as armaduras de cada fase da progressão (recruta, classe, evolução e as 28 classes de dupla das [maestrias](../maestrias/README.md)), armas e animações próprias de cada classe. Tudo é feito num conjunto separado, atrás da chave `?herois=v4`, e só vira o padrão quando estiver aprovado.

A base é o processo do [remaster dos heróis](remaster-herois.md) (tunnel#94) e o polimento que veio depois (tunnel#181). O plano corrige na origem o que lá foi consertado depois.

Referência de clima: o Grim Dawn (ARPG sombrio, proporção adulta, metal gasto, couro e pano pesados). Os designs, as silhuetas e as armaduras são nossos; nada é copiado.

---

## 1. Decisões (DIogo, 19/11)

| # | Decisão |
|---|---|
| 1 | **Só os heróis agora.** Monstros, NPCs e cenário ficam para depois. O guia de arte ganha uma seção de heróis com o clima novo. |
| 2 | **Um visual por dupla (28 classes).** Ex.: Bastião + Paladino = Cruzado; Berserker + Arcanista = Possesso. |
| 3 | **Nada genérico.** Cada classe tem a sua forma de andar, correr, ficar parada e lutar, feita no Blender (5.3, instalado). |
| 4 | **Claude faz tudo**: conceito, modelagem, rig, animação, integração e testes. O DIogo aprova em cada portão. |
| 5 | **Fatia vertical com o Cavaleiro**: a primeira classe sai inteira, do conceito ao jogo, antes das outras 3. |
| 6 | **Um corpo só**, com ajuste masculino/feminino na mesma malha. A classe aparece pelo kit, pela arma e pelo movimento. |
| 7 | **A classe é escolhida dentro do jogo.** O personagem nasce recruta e escolhe a classe com um NPC (seção 9). |
| 8 | **Base de movimento compartilhada + camada da classe**, para cortar tempo sem perder a personalidade. |
| 9 | **Peças simples feitas no Blender** (cintos, correias, placas lisas, capas, bolsas); o Meshy fica para as peças de destaque. |

---

## 2. O que o processo anterior ensinou

Cada linha é um problema que apareceu tarde e custou retrabalho. A coluna da direita diz como este plano evita.

| O que aconteceu | Por que | Como evitar agora |
|---|---|---|
| Arma flutuando na palma, adaga mal presa, espada invisível, escudo de fio | O rig do Meshy tem 28 ossos e **nenhum dedo**. Foi preciso dobrar os dedos na malha (`make-remaster-fists.mjs`) e calibrar a pega por tipo de arma | Rig próprio no Blender, **com dedos**, e encaixes fixos (seção 4) |
| Faltou animação: parado e correr de duas mãos, corte em arco, tiro de arco | A lista de clipes saiu da biblioteca disponível, não do jogo. O bárbaro de duas mãos virou remendo (`compose-2h-clips.mjs`) | **Lista de clipes primeiro**, por papel e por arma (seção 5), e animação feita para ela |
| Clipes comprados e descartados (Axe Stance, Spear Walk, Rifle Charge, Reaping Swing) | Escolha às cegas, vendo o clipe só depois de pago | As animações são feitas no Blender; nada é comprado sem pré-visualizar |
| Rig refeito noutra conta, com retargeting (`add-remaster-clips.mjs`) | O esqueleto mudou no meio do caminho | **Um esqueleto e um corpo só**, congelados no contrato |
| Elmo e chapéu tirados da cabeça depois, com elmo flutuando e tufos de cabelo | O corpo veio com o acessório colado | Corpo, cabeça, cabelo e cada peça de armadura **como malhas separadas** desde o início |
| Pés deslizando, corrida no lugar, golpe que continuava com o corpo andando | Sem velocidade do pé nem regras de mistura na entrega | Medição automática no portão de qualidade (seção 7) |
| Rostos todos iguais | Um rosto por modelo, preso na malha | Kit de rostos e cabelos no contrato |
| Retornos chegando por partes, depois de entrar na `main` | Sem critério de aceite escrito antes | **Portões com checklist**, prints e GIFs antes de seguir |
| Créditos subindo de 245 para 385 | Rodadas de correção não planejadas | Orçamento por fase, com reserva |

---

## 3. A progressão visual do personagem

| Nível | O personagem é | O que veste |
|---|---|---|
| 1 até a escolha | recruta, sem classe | **kit de recruta** (1) |
| da escolha até 19 | classe-base (Cavaleiro, Bárbaro, Mago, Ladino) | **kit da classe** (4) |
| 20–29 | 1ª maestria (Bastião, Paladino…) | **kit da evolução** (8) |
| 30+ | dupla (Cruzado, Possesso…) | **kit da dupla** (28) |

São **41 kits**, todos no mesmo corpo. A raridade do equipamento não cria kit novo: ela troca material, acabamento e brilho do mesmo kit (aço gasto → aço polido → ouro e runas acesas), por textura e shader, como a repintura por máscara de hoje.

### O kit da dupla

O design mistura as duas maestrias: a peça dominante vem da 1ª, o detalhe de destaque vem da 2ª. A ordem muda o visual mesmo com o mesmo nome:
- **Berserker + Arcanista** (Possesso): couro e machado do Berserker, com correntes e runas roxas do Arcanista.
- **Arcanista + Berserker** (Possesso): manto do Arcanista, rasgado por garras e manchado de sangue do Berserker.

As duas versões usam as mesmas peças em proporções diferentes, então custam pouco mais que uma.

### Biblioteca de peças

Cada kit é montado de **peças de uma biblioteca** (peitoral, ombreiras, braçais, cinto, pernas, botas, capa, elmo ou capuz, e acessórios como corrente, livro, aljava ou estandarte), mais uma **paleta** e um **emblema** próprios.
- Uma peça pode servir a mais de um kit, com cor e emblema diferentes.
- Cada kit tem pelo menos uma peça só dele na silhueta (cabeça, ombros ou capa), para ser reconhecível de longe.
- **Peças de destaque** (elmos, ombreiras, capas com forma) saem do Meshy. **Peças simples** (cintos, correias, placas lisas, bolsas, capas retas) são modeladas no Blender, sem créditos.

---

## 4. Contrato do personagem

Escrito e aprovado **antes** de gerar qualquer coisa. O cliente lê o contrato; o modelo se adapta a ele, e não o contrário.

- **Corpo:** uma malha só, com shape key **masculino ↔ feminino** (mesma topologia e mesmo UV). As peças de armadura seguem o shape key por deformação de superfície, então são feitas uma vez para os dois.
- **Escala e pose:** altura 1,80 m no masculino e 1,70 m no feminino, quadril em ~1,0 m, A-pose, frente para +Z, pés em y = 0.
- **Esqueleto:** feito no Blender, ≤ 45 ossos que deformam (guia, seção 5). Polegar e indicador com 3 ossos, os outros dedos juntos com 2: a mão fecha no cabo de verdade. Nomes fixos. Ossos extras **sem deformar** para capa e tecido.
- **Encaixes:** mão direita, mão esquerda, antebraço esquerdo (escudo), costas (arma guardada), cintura (adaga e aljava), cabeça (elmo e chapéu), peito (efeito de buff).
- **Malhas separadas:** corpo, cabeça (3 rostos por gênero), cabelo (4 cortes por gênero), e cada peça de armadura por slot.
- **Orçamento:** corpo e armadura ≤ 8.000 triângulos no LOD0, LOD1 e LOD2, textura 1024 em KTX2, 1 material e 1 emissivo, máscara de repintura (pano, pele, cabelo e metal da raridade).
- **Clipes:** os da seção 5, com nome fixo por papel, quadro de impacto marcado no próprio clipe, laço sem salto, sem root motion e velocidade do pé registrada.

---

## 5. Animações com personalidade

### Base compartilhada + camada da classe

Todos usam o mesmo corpo e o mesmo esqueleto. As animações se dividem em duas camadas:

| Camada | O que tem | Quem usa |
|---|---|---|
| **Base** | andar, correr, girar no lugar, reações (dano, atordoado, morte, levantar), sociais (acenar, sentar, dançar, comemorar) e o recruta inteiro | todos |
| **Classe** | postura (camada aditiva por cima da base), parado e pausas, andar em combate, postura de combate, ataques por arma, bloqueio ou esquiva, as 10 categorias de skill | uma por classe |

A postura da classe se aplica por cima da base. O Bárbaro corre com o mesmo ciclo de pernas, mas com o tronco inclinado e os braços abertos; o Ladino, agachado e na ponta dos pés. A personalidade aparece em tudo, e o ciclo de pernas é feito uma vez só.

### A personalidade de cada classe

| Classe | Peso e postura | Ritmo | Assinatura |
|---|---|---|---|
| Recruta | neutro, um pouco inseguro | comum | segura a arma sem jeito; nenhum estilo ainda |
| Cavaleiro | ereto, centro de gravidade baixo, escudo sempre à frente do peito | firme, golpes curtos e sem desperdício | o escudo participa de tudo: bloqueia, empurra e bate |
| Bárbaro | inclinado para a frente, ombros largos, braços abertos | pesado: lento para começar, brutal no impacto, com recuperação longa | a arma é arrastada e jogada; o corpo inteiro gira no golpe |
| Mago | leve, mãos sempre ativas, manto arrastando | fluido e contínuo, com pausas de concentração | gestos de mão desenham o feitiço antes de ele sair |
| Ladino | baixo, joelhos dobrados, peso na ponta dos pés | rápido e quebrado, com fintas e trocas de base | as adagas giram nas mãos; deslocamentos curtos e diagonais |

As **evoluções e duplas** mudam o detalhe, não a base: o Paladino ergue a arma ao céu no grito; o Possesso tem um tique de cabeça e os ombros puxados por correntes no parado.

### Lista de clipes

| Grupo | Clipes | Quantidade |
|---|---|---|
| Base (uma vez) | andar, correr, girar, dano (2), atordoado, morte (2), levantar, acenar, sentar com transição, dançar, comemorar, e o recruta (parado e ataque) | ~16 |
| Por classe | postura aditiva, parado, 2 pausas, andar em combate, postura de combate | 6 |
| Por classe e arma | ataque em combo de 3, bloqueio ou esquiva | 4 por arma |
| Skills por classe | golpe pesado, giro, salto ou investida, grito ou buff, projétil, área no chão, canalizar (laço), tiro carregado, arremesso, esquiva ou teleporte (as que a classe usa) | 6 a 10 |

Armas (os tipos do servidor, `weapons.go`): Cavaleiro (espada e escudo); Bárbaro (machado de uma mão, machado de duas mãos); Mago (cajado, varinha e grimório); Ladino (adagas duplas, arco, besta, besta pesada). Cada uma das 170 skills aponta para uma categoria e para o seu roteiro visual (protótipo de maestrias).

**No total:** ~16 da base + ~25 por classe ≈ **115 clipes**, contra ~160 sem a base compartilhada.

### Como faço no Blender

1. **Bloqueio:** poses-chave do clipe (contato, passagem, impacto, recuperação), conforme a ficha da classe.
2. **Ritmo:** espaçamento e tempo de cada pose, com antecipação antes do golpe e pausa no impacto.
3. **Arcos e sobreposição:** braços e arma seguem arcos limpos; cabeça, capa e cabelo chegam um pouco depois.
4. **Contato:** pés presos no chão por IK durante o apoio, mãos presas na arma de duas mãos por IK.
5. **Conferência:** tiras de quadros de frente, lado e costas, e GIF de cada clipe para você aprovar.

Bases de movimento de bibliotecas abertas (como o Mixamo, com a licença conferida) só entram como **referência de tempo**, nunca como clipe final.

---

## 6. Fases

Cada fase termina num **portão**: um checklist com prints e GIFs que você aprova antes da fase seguinte. Nada entra na `main` sem passar pelo portão, e mesmo depois de entrar fica atrás de `?herois=v4`.

### Fase 0: guia e contrato (sem créditos)
- Seção de heróis no guia de arte com o clima novo: paleta por classe, evolução e dupla, materiais, proporção e o teste de silhueta a 85 px.
- O contrato (seção 4), as fichas de movimento (seção 5) e a lista de peças da biblioteca.
- **Portão:** guia, contrato e fichas aprovados.

### Fase 1: bíblia visual (conceito 2D)
- O corpo: masculino e feminino, de frente, lado e costas, com 2 variações para escolher.
- Os 41 kits em folhas por classe (classe, 2 evoluções e as duplas em que ela é a 1ª maestria), mais o recruta, na técnica de folha usada nos ícones.
- Folha de rostos e cabelos.
- **Portão:** silhueta a 85 px em cinza. Dá para saber a classe, a evolução e a dupla de longe?

### Fase 2: corpo e fatia vertical do Cavaleiro
Do conceito ao jogo, para validar o caminho antes de repetir.
1. O corpo único: malha, shape key masculino/feminino, limpeza, LODs, máscara, rig com dedos e encaixes, rostos e cabelos.
2. A base de animação (~16 clipes) e o recruta.
3. A camada do Cavaleiro (~25 clipes), com a ficha de movimento dele.
4. Kits: recruta, Cavaleiro, Bastião, Paladino, **Cruzado** (dupla da mesma classe) e **Guardião do Véu** (Bastião + Arcanista, dupla entre classes).
5. Armas: espada e escudo, com a pega pelos dedos.
6. No jogo, atrás de `?herois=v4`: laboratório, praça e caça.
- **Portão (o mais rígido):** relatório automático sem falhas, GIFs de cada clipe nos dois gêneros, combate real e FPS no celular. O que for aprendido aqui volta para o contrato.

### Fase 3: Bárbaro, Mago e Ladino
- A camada de animação de cada um (~25 clipes), as armas e os kits em que a classe é a 1ª maestria.
- **Portão:** o mesmo da Fase 2, por classe.

### Fase 4: os kits que faltam
- Fechar os 41 kits, incluindo as duas versões (pela ordem) das duplas entre classes.
- **Portão:** as 28 duplas lado a lado, nos dois gêneros e em 3 raridades.

### Fase 5: skills e efeitos
- Ligar cada skill à animação e implementar os efeitos visuais dos roteiros, começando pelas peças compartilhadas (sangramento, ataque e defesa reduzidos, cegueira, marcas sobre o alvo, fios de dreno e contágio, selo de crítico, lobo espectral).
- Ícones próprios das skills que hoje usam ícone emprestado.
- **Portão:** cada skill lançada no laboratório, com a gravação do efeito.

### Fase 6: aplicação
- Bench de FPS no PC e no celular com a praça cheia.
- Migração dos personagens que já existem: ficam com a classe que têm, ganham o corpo novo (masculino por padrão) e uma troca grátis de gênero, rosto e cabelo no Barbeiro.
- Trocar o padrão para `v4`, mantendo `?herois=v3` por um tempo para voltar atrás.

---

## 7. Portão de qualidade automático

Um comando (`node client/scripts/qa-herois.mjs <classe>`, a criar) roda no laboratório e gera um relatório com prints. Junta o que hoje são scripts soltos (`clip-peaks.mjs`, `foot-speeds`, `impact.py`, `list-clips.mjs`, `inspect-glb.mjs`, `montage.mjs`).

| Checagem | Passa se |
|---|---|
| Clipes | todos os da lista existem, com o nome do contrato |
| Laço | parado, andar e correr fecham sem salto |
| Pé | velocidade natural registrada; deslize no jogo = 1,00 ± 0,05 |
| Impacto | quadro marcado no clipe e conferido com o pico da mão |
| Pega | o cabo passa pelo centro do punho em todos os clipes; as duas mãos no de duas mãos |
| Kit | cada peça no lugar nos dois gêneros, sem atravessar a malha nos clipes de maior torção |
| Malha | triângulos, textura e materiais dentro do orçamento; LODs existem |
| Silhueta | render a 85 px em cinza, para conferir a olho |
| Tiras | frente, lado e costas de cada clipe |

---

## 8. Orçamento e prazo

Os créditos do Meshy vão só para conceito e peças de destaque; corpo, rig, animações e peças simples são feitos no Blender, sem créditos.

| Fase | O que gasta | Créditos |
|---|---|---|
| 1 Bíblia visual | corpo (x2), 41 kits em folhas, rostos e cabelos | ~220 |
| 2 Corpo e fatia (Cavaleiro) | malha do corpo, cabeças, cabelos e as peças de destaque dos 6 kits | ~200 |
| 3 Outras 3 classes | peças de destaque dos kits de 1ª maestria | ~350 |
| 4 Kits restantes | peças de destaque que faltam | ~150 |
| 5 Ícones | refazer os que não passarem | ~30 |
| Reserva (15%) | correções | ~140 |
| **Total** | | **~1.100** |

Com ~600 créditos por mês, são **2 meses**. A parte mais longa é a animação (~115 clipes com revisão), que não gasta créditos.

---

## 9. Escolha de classe dentro do jogo (mudança de jogo, não de arte)

O personagem nasce **recruta** e escolhe a classe com um NPC, como o Grim Dawn faz com a primeira maestria.

- **Quando:** proposta no **nível 5**, depois de o jogador provar as 4 armas num treino rápido. Até lá ele usa o kit de recruta, uma arma simples e 2 skills de recruta.
- **O que muda no código:**
  - criação de personagem sem classe (`POST /api/characters` em `server/internal/api/api.go`, `store.CreateCharacter`, tela em `client/src/ui/screens.ts`);
  - uma "classe" recruta com status, kit inicial e 2 skills próprias;
  - o NPC e a missão da escolha, no molde da Ysolde e da `encruzilhada`;
  - os pontos de skill dos níveis 1–5 passam para a árvore da classe escolhida;
  - os personagens que já existem mantêm a classe.
- **Em paralelo à arte.** Nada da Fase 0 à 5 depende disso; só a Fase 6 (aplicação) precisa da escolha pronta.

---

## 10. Em aberto

1. O nível da escolha de classe (proposta: 5) e o que o recruta faz até lá.
2. Licença das bibliotecas usadas como referência de tempo (conferir antes da Fase 2).
3. Capa e tecido: mola simples no cliente (barato) ou ossos animados à mão em cada clipe.
