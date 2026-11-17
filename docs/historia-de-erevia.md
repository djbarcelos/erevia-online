# A história de Erevia

Referência curta para quem escreve missões, falas e descrições. A direção criativa é a do
`docs/roadmap-areas.md` (seção 2.1, "A história que liga as regiões"); este documento junta
o que já está no jogo (tunnel#71) e o que cada região conta. As falas e as missões estão nos
dados do servidor: `server/internal/game/talk.go`, `story.go` e `dailies.go`.

## O tom

Fantasia sombria, mas acolhedora. O mundo está rachado e os mortos andam, mas quem vive nele
é gente comum que continua fazendo o seu trabalho: o moleiro defende o trigal, a coveira
fecha as covas, o batedor conta as fendas. Ninguém faz discurso. As falas são curtas, ditas
por quem está cansado e ainda assim agradece. O medo aparece no detalhe ("faz três noites que
não durmo"), não no adjetivo.

- Português do Brasil, frases curtas, sem gíria e sem arcaísmo forçado.
- Cada monstro tem um motivo para estar onde está (o pilar 1 do roadmap). Se não dá para
  explicar em uma frase, a fala está errada.
- A Partição é história antiga para todo mundo. Só os Ilvarin falam dela como coisa de
  família.

## O que aconteceu

1. **Os Ilvarin.** Há três séculos, um povo de tecelões de runas fazia a pedra obedecer. Eles
   teceram uma rede de runas sob a terra, ergueram os monólitos do Vale como nós dessa rede e
   deram vida aos golens para guardá-la.
2. **Ozrhal.** O rei de Erevia quis uma coroa que não morresse com ele e pagou com os ossos do
   próprio reino: os mortos de Erevia viraram o exército dele.
3. **A Partição.** Para detê-lo, os Ilvarin romperam os veios de runa sob os pés do rei. A
   terra rachou de um horizonte ao outro. O rio Lamagris virou brejo, as minas de Brasaferro
   desabaram, a cidade dos Ilvarin caiu ao norte (cantando, dizem) e navios se partiram no
   Recife dos Dentes.
4. **O selo.** Ozrhal ficou preso na **Cripta Esquecida**, no nordeste, e o exército dele ficou
   plantado nos campos, em fileira, esperando ordem. Cinco **Pedras-Selo**, espalhadas pelos
   Ilvarin pelas Terras Partidas, seguram o rei lá dentro.
5. **Vel'Aran.** A cidade foi erguida em cima do campo de batalha, para vigiar o selo. A
   muralha não é para segurar quem vem de fora: é para lembrar o que está embaixo.

## Agora: o selo racha

- os mortos levantam toda noite, cada noite mais cedo;
- as pedras do Vale andam: a rede de runas afrouxou e o rei a puxa de dentro da Cripta,
  fio por fio;
- o **Senhor dos Ossos**, arauto do rei, ronda a estrada sul;
- os **Devotos do Ossário**, gente viva que se vendeu ao rei, cantam na porta da Cripta para
  que o selo ceda;
- nos cantos do mundo, onde a Partição rachou mais fundo, saem os **Partidos**: mortos que a
  própria Partição deformou.

## Quem segura o mundo

| Quem | O que é | No jogo |
|---|---|---|
| **Guarda de Vel'Aran** | segura as muralhas e as estradas desde a fundação | Cássio (o Arauto, as diárias), Dárcio, a Capitã Rovena, Otto; facção "Guarda de Vel'Aran" |
| **Ordem do Bastião Cinzento** | monges e soldados que vigiam a Cripta; guardam a 1ª Pedra-Selo | o Irmão Gaspar e Sor Bertrand; facção "Ordem do Bastião Cinzento" |
| **Os Ilvarin** | poucos, espalhados; ninguém em Vel'Aran confia em quem rachou o mundo | Maelis, no Vale, e a Mestra Ysolde, na praça |

## As linhas de história (uma por região)

Cada região tem um contador de história. Um convite chega sozinho com o nível ("vá falar com
fulano"); depois o NPC oferece de 3 a 4 missões, uma de cada vez, e recebe cada uma na
conversa. Cada linha conta um pedaço do todo e termina apontando para a próxima região.

| Região (níveis) | Contador de história | Onde fica | O que a linha revela |
|---|---|---|---|
| Arredores (1–5) | **Anselmo**, o Moleiro | perto do moinho, a leste da cidade (50, 11) | os mortos dos Arredores ficaram rasos; debaixo do moinho aflora uma runa Ilvarin, um veio da rede que prende o rei (Cássio, o Arauto, lê as runas) |
| Campos dos Ossos (6–12) | **Marga**, a Coveira | perto da pedra dos Campos (−6, 90) | o exército do rei foi enterrado em fileira e está em formação; os crânios têm nomes da Guarda do Rei; o estandarte de Ozrhal ainda está fincado; o relato vai a Iselda |
| Ermos Partidos (13–20) | **Dárcio**, Batedor da Guarda | perto da pedra dos Ermos (−122, −10) | os Magos Esqueletos eram a corte do rei; a Essência Sombria é a Partição escorrendo das fendas; o Passo de Brasaferro; calar o arauto (Senhor dos Ossos) |
| Brejo de Lamagris (15–20) | **Tomé**, o Barqueiro | Posto do Barqueiro, no fim da estrada leste (169, −3) | o Lamagris represado afogou Vau Velho e o veneno do selo levanta os Afogados à noite; os Brejeiros defendem a água e só atacam à noite; o Antídoto vem da Glândula de Sapo e do Junco-de-Prata; a Mãe-Lodo guarda a lagoa, e uma das Pedras-Selo está debaixo dela (a Pedra-Selo ainda não existe como item: a linha termina no aviso) |
| Bosque Gris (9–16) | **Hélio**, o Carroceiro | junto à Carroça Tombada, no noroeste (−37, 94) | os esqueletos dos Campos expulsaram os lobos para o Bosque; o Bando do Corvo, desertores da guarda de Vel'Aran, assalta as caravanas de Brasaferro e paga os lobos com carne; Caio, o irmão da Lívia, está refém na Toca do Corvo; o Corvo Rubro é o chefe |
| Vale das Pedras Vivas (20–30) | **Maelis**, Tecelã de Runas | acampamento ao norte do Vale (−8, −201) | os monólitos são nós da rede e os golens, os guardiões dela; a rede está desafinada; Ysolde confirma que o rei puxa a rede de dentro da Cripta |
| Terras Mortas (24–28) | **Irmão Caedmon**, do Bastião Cinzento | pátio do Bastião Cinzento, no nordeste (199, 93) | os Devotos do Ossário, gente viva que perdeu alguém, cavam as covas da Partição, levantam os mortos e alimentam o selo com velas de sebo negro; o Sino Rachado da Torre toca 3 vezes quando faltam 10 min para o Ozrhal acordar; a Ordem guarda a primeira Pedra-Selo (o Fragmento de Selo da última missão); a Lápide do Último Regicida, ao lado do cemitério, grava quem mais feriu o Rei Ossário na última queda dele |
| Fundo do Vale (30–35) | **Irmão Gaspar**, do Bastião | acampamento ao norte do Vale (14, −206) | os Golens Ancestrais foram os primeiros guardiões; as pedras do círculo do Fundo se inclinam para a Cripta |
| Vigília Ossária (33–38) | **Sor Bertrand**, Cavaleiro do Bastião | posto a sudoeste da Cripta (79, 79) | os Devotos cantam para o selo ceder; a guarda pessoal do rei já está na porta; a queda de Ozrhal (o raid) dá um respiro ao selo |
| Confins Partidos (35–40) | **Capitã Rovena**, da Guarda | posto a noroeste da cidade (−79, 79) | as três rachaduras crescem; os relatos de Maelis e de Bertrand fecham a conta: o rei puxa a rede, a rede puxa a terra e a terra racha; das cinco Pedras-Selo, só se sabe onde está uma |

A praça também conversa: **Cássio, o Arauto da Guarda** (−9,5; 0,5) conta a história de
Erevia e dá as diárias; Lívia, Bruno, Teodoro, Otto, Iselda e Severino ganharam falas (a
conversa abre quando há missão para entregar a eles; senão o clique abre a loja, o armazém,
o quadro ou o leilão, como antes).

## O que vem depois (a espinha das missões principais)

As cinco Pedras-Selo são o arco grande. A Ordem do Bastião guarda a primeira. As outras quatro
estão nas regiões do roadmap que ainda não existem: o Brejo de Lamagris (Posto do Barqueiro),
as Minas de Brasaferro (Acampamento Lanterna), a Costa e as Ruínas de Ilvaren (Vigília de
Ilvaren). Juntar as cinco abre a Necrópole do Rei Ossário. As linhas de hoje terminam
apontando para lá: Rovena diz que "as outras estão além das montanhas" e que Vel'Aran vai
precisar do jogador "quando as estradas abrirem". Quem escrever as regiões novas continua
daí: cada região nova tem um contador de história, e a última missão da linha dela entrega
uma Pedra-Selo.

## Diárias: o rol do Arauto

Do nível 5 ao 35 (no topo, quem paga é o Quadro de Contratos), Cássio oferece três serviços
por dia, um de cada vaga, sorteados por personagem, dia e nível. Renovam à meia-noite de
Brasília. Têm outro sabor que os contratos de caça:

- **coleta:** pegar materiais do saque (ossos para a fogueira, crânios para a tinta dos
  escribas, essência para enterrar em sal, núcleos para a muralha);
- **entrega:** levar itens a alguém (poções para Anselmo, Dárcio ou Bertrand; ossos e núcleos
  para Bruno; crânios para Lívia; essência para Maelis);
- **ronda ou chefe:** passar por lugares do mapa (o moinho, o Estandarte Caído, o Passo de
  Brasaferro, os círculos do Vale, a borda da Cripta) ou calar o Senhor dos Ossos.

## Lugares com nome (para missões de "ir até")

| Lugar | Onde | Região |
|---|---|---|
| Moinho dos Arredores | (56, 16) | Arredores |
| Marco da estrada norte | (0, 60) | Arredores |
| Estandarte Caído | (−66, 66) | Campos dos Ossos |
| Marco da estrada leste | (95, 0) | Campos dos Ossos |
| Passo de Brasaferro | (−146, 0), o fim da estrada oeste | Ermos Partidos |
| Fenda do Sudoeste | (−96, −96) | Ermos Partidos |
| Círculo Oeste e Círculo Leste | (−36, −244) e (40, −258) | Vale das Pedras Vivas |
| Pedras Caídas do Oeste e do Leste | (−104, −284) e (108, −272) | bordas do Vale |
| Círculo do Fundo | (−10, −302) | Fundo do Vale |
| Borda da Cripta | (86, 86) | Vigília Ossária |
| Rachaduras do Noroeste, Sudoeste e Sudeste | (−122, 122), (−122, −122) e (122, −122) | Confins Partidos |
| Árvore-Forca | (−52, 84), um carvalho morto de 19 m cheio de corvos | Bosque Gris |
| Carroça Tombada | (−33, 90) | Bosque Gris |
| Toca do Corvo | (−128, 118), o acampamento com paliçada | Bosque Gris |
| Bastião Cinzento | (196, 94) | Terras Mortas |
| Torre do Sino Rachado | (238, 122) | Terras Mortas |
| Cemitério dos Reis Sem Nome | (172, 122) | Terras Mortas |
