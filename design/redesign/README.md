# Redesign das telas (direção "Vigília")

Exportado do canvas de design do Claude (artefato "Erevia — Redesign das telas").
Telas: 1 Entrar, 2 Seus personagens, 3 Carregamento, 4 HUD no PC, 5 HUD no celular.

- `project/*.dc.html`: uma tela por arquivo (formato Design Component). `project/canvas.json` é o índice/layout; `project/ds/erevia/tokens.json` são os tokens do design system.
- `assets/`: imagens usadas pelas telas. Nos `.dc.html` elas aparecem como `/_blob/<id>`; o `<id>` é o nome do arquivo em `assets/`.
- Referência visual para implementar no cliente (`client/`); ainda não está ligado ao código do jogo.
- Cores: tinta-noite translúcida, ouro `#d6b46e` só em ação principal/estado ativo. Fontes: Cormorant Garamond (títulos), Barlow Semi Condensed (interface), Barlow (texto).
