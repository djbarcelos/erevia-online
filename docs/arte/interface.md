# Interface "Ouro de Vel'Aran"

Revisão visual das telas de entrada, do HUD e das janelas. Só cliente: nenhuma mudança de protocolo, de migração ou de servidor. O visual é próprio e segue o guia de arte (`docs/guia-de-arte.md`, estilo B): fantasia estilizada heroica, paleta controlada, raridades de `client/src/ui/items.ts`.

Os prints de antes e depois estão em `docs/arte/interface/` (`antes-*.png` e `depois-*.png`, em 1280×720 e em celular 390×844). Para refazê-los, com o servidor de pé (`DEV_LOGIN=true`, cliente compilado em `STATIC_DIR`):

```
cd e2e
E2E_BASE_URL=http://localhost:8087 node interface-shots.mjs depois      # ou "antes"
KINDS=celular ENTRY=1 E2E_BASE_URL=... node interface-shots.mjs depois  # só as telas de entrada
```

## Paleta

Ouro é o único acento. Todos os valores moram em `client/src/styles/tokens.css`; o resto dos estilos usa `var(--...)`.

| Papel | Token | Valor |
|---|---|---|
| Fundo da tela / painel / elevado | `--bg-0` / `--bg-1` / `--bg-2`, `--bg-3` | `#0f0e16` / `#17151f` / `#221f2c`, `#2a2638` |
| Fundo de tela | `--screen-bg` | radial `#2a2638 → #17151f → #0f0e16` |
| Texto / secundário / terciário | `--text` / `--muted` / `--dim` | `#efe6d4` / `#c9bda6` / `#9d917c` (o terciário só em 13 px ou mais sobre `#17151f`) |
| Ouro | `--gold`, `--gold-hi`, `--gold-glow` | `#d6b46e`, `#f0d79a`, `#f6e4b4` |
| Ouro escuro | `--gold-deep`, `--gold-lo`, `--gold-shadow` | `#b8944c`, `#a9843f`, `#6b5428` |
| Degradê do botão e do selo | `--gold-grad` | `#f0d79a → #d6b46e → #b8944c` |
| Filetes | `--line-strong` / `--line` | ouro a 0,45 (borda principal) / 0,22 (discreta) |
| Vida | `--hp-a`, `--hp-b` | `#e0574f → #a8302a` |
| Perigo (alvo inimigo) | `--danger-line`, `--danger-text` | `rgba(201,65,58,0.6)`, `#ff9f94` |
| Mana | `--mp-a`, `--mp-b` | `#5c95ec → #2c5fb4` |
| XP | `--xp-a`, `--xp-b` | `#b8944c → #f0d79a` |
| Raridades | `--r-common` … `--r-legend` | as do jogo (`#b9b4c2`, `#5fd17a`, `#4f9dff`, `#b46cff`, `#ff9a3c`) |

Também em `tokens.css`: espaçamentos (`--sp-1` a `--sp-5`), raios, o tamanho do chanfro (`--chamfer`, `--chamfer-lg`), sombras (`--shadow`, `--shadow-sm`, `--glow-gold`) e o `clip-path` dos cantos (`--chamfer-clip`).

## Tipografia

Duas famílias, ambas com licença OFL, hospedadas em `client/public/fonts/` (woff2 do subconjunto latino, que cobre o pt-BR; as licenças estão ao lado). Nada vem de CDN em tempo de execução.

- **Cinzel** (500 e 700): títulos, nomes, números de nível, teclas dos atalhos, o logo e os botões. `--font-title`.
- **Alegreya Sans** (400, 500 e 700): todo o resto, inclusive o texto miúdo do HUD e do chat. `--font-ui`.

Peso total das fontes: cerca de 125 KB. O check de assets (`npm run check:assets`) só olha os modelos 3D e segue verde.

## Componentes

- **Moldura (`.frame`, em `kit.css`)**: degradê `rgba(34,31,44,.96) → rgba(23,21,31,.97)`, borda de 1 px em ouro 0,45, filete duplo por dentro (`::before`) e cantos chanfrados de 10 px (`clip-path`). A diagonal de cada canto é um degradê fino (`::after`). O `clip-path` também corta sombras de fora, então as molduras não projetam sombra: o fundo é quase opaco. O mesmo desenho vale para janelas, dicas, hotbar, menu, chat e plaqueta do minimapa.
- **Botões**: primário em degradê ouro com texto `#1b1712` e borda `#f6e4b4` (`.btn-gold`); secundário `#2a2638` com borda ouro (`.btn`); fantasma transparente com borda ouro e texto `#f0d79a` (`.btn-ghost`). "Entrar no mundo" é Cinzel espaçado, com as pontas em losango e brilho dourado.
- **Abas**: texto em Cinzel, aba ativa com degradê dourado embaixo, filete e um losango.
- **Janelas**: o título é dourado em Cinzel, com um filete em degradê embaixo. Todas as janelas (inventário, loja, missões…) mudam junto porque usam `.gwin`/`.frame`/`.tab`/`.btn`, sem reescrita de cada uma. No celular a janela ocupa a tela, sem chanfro e com fundo sólido.
- **Barras (`.pbar`)**: borda dourada fina; no HUD, pontas inclinadas e o número dentro.
- **Dica (`.tip`)**: a mesma moldura.

### Telas de entrada

- **Login**: "AS TERRAS DE VEL'ARAN" sobre o logo (EREVIA em Cinzel, até 112 px), a silhueta da cidade com janelas acesas no rodapé (desenhada uma vez, em `backdrop.ts`), o painel ENTRAR à direita (Google primário, Discord secundário, divisor "ou, no servidor de testes" e o login DEV) e, no rodapé, o estado do servidor (`GET /healthz`) e os links. As novidades ficam sob o logo (no celular, depois do painel).
- **Personagens**: cartões à esquerda (selo hexagonal na cor da classe, com filete na cor do emblema, nome em Cinzel, classe e nível grande), pedestal 3D no centro, painel "Detalhes" à direita e "ENTRAR NO MUNDO" embaixo, no centro. A criação usa os mesmos cartões na escolha de classe e o editor de aparência com os tokens novos.
- **Pedestal (`preview3d.ts`)**: luz de três pontos (principal quente, preenchimento frio e recorte na cor do emblema). A luz de recorte não ilumina mais o pedestal, que antes estourava em branco. Há um degrau a mais na base e o fundo da cena fica desfocado (`filter: blur(2px)` só na tela de personagens), o que faz o papel da profundidade de campo sem custo no 3D.
- **Carregamento**: "VOCÊ ESTÁ INDO PARA" e o nome do mapa em destaque, a barra em quatro segmentos (Mapa, Cenário, Personagens, Preparando…) e uma dica com ícone de lâmpada. `setLoadingProgress` e `leaveLoadingScreen` seguem iguais; `setLoadingDestination(nome)` é novo.

### HUD

| Peça | Onde | Mudança |
|---|---|---|
| Quadro do jogador | alto à esquerda | retrato hexagonal, selo de nível dourado, nome em Cinzel, vida com o número dentro e mana. Há espaço embaixo da mana para a barra de recurso da frente `evolucoes` |
| Buffs e efeitos | logo abaixo do quadro | saíram de dentro da moldura; ouro para buff, vermelho para debuff |
| Grupo | abaixo dos buffs | selo hexagonal por membro |
| Alvo | alto ao centro | borda e brilho vermelhos; NPC e aliado ficam com a borda dourada |
| Minimapa | alto à direita | aro dourado e plaqueta embaixo com a região e a hora (a hora saiu de dentro do mapa); zoom em coluna ao lado |
| Missões | à direita, abaixo do minimapa | alinhadas à direita |
| Chat | embaixo à esquerda | moldura nova e abas em Cinzel |
| Hotbar | embaixo, no centro | 10 atalhos; a barra de XP fina fica logo abaixo; o AUTO fica ao lado |
| Menu | embaixo à direita | grade de ícones com a tecla no canto (11 ícones: 6 por linha, 4 abaixo de 1400 px e 3 abaixo de 1000 px) |

No toque o arranjo continua o de antes (joystick, ataque e skills em arco, chat numa pílula, botão de menu), só com o visual novo. O DOM e as classes que os testes usam (`.hud.hud-screen`, `.hud-me .pbar`, `.hotbar .hb-slot`, `.mm-coords .mm-cv`, `.mm-zone`, `.hud-target`, `.roster-slot`) não mudaram.

## Desempenho

- Sem `backdrop-filter` nas molduras (o `.frame.glass` perdeu o blur) e sem animação contínua nova. As que já existiam seguem (vida baixa, AUTO ligado, selo de pontos livres).
- Molduras em CSS puro (degradês e `clip-path`), sem imagens. O logo, os selos e a cidade do login não são arquivos: a cidade é desenhada num canvas uma vez por redimensionamento.
- O desfoque de fundo da tela de personagens custa só nas telas de entrada, fora do jogo.
- A medição `?bench=1` não foi refeita aqui: o SwiftShader do Playwright não dá FPS que valha comparar (ver `docs/desempenho-remaster.md`). Como o `.frame` perdeu o blur e nada novo anima, a expectativa é custo igual ou menor.

## Decisões a revisar

- **Fontes**: Cinzel e Alegreya Sans, como no pedido.
- **Evolução no quadro do jogador e na lista**: o cliente só recebe nome, classe e nível em `GET /api/characters`. A lista mostra a classe, "subclasse" quando há e o papel da classe. Região, XP, clã e vitalidade por personagem pedem campos novos no servidor e ficaram de fora; o painel "Detalhes" usa só o que existe (classe, subclasse, evolução e emblema). Os botões "Aparência" e "Apagar" também não entraram: não há chamada para apagar só um personagem, e a aparência muda no Barbeiro.
- **Menu com 11 ícones**: não cabe numa grade 4×2; ficou em 6×2 (4×3 e 3×4 em telas menores).
- **Chanfros e sombras**: o `clip-path` corta a sombra externa, então as janelas ficam sem sombra projetada.
