/** Configurações e informações: gráficos, áudio, controles, jogo (auto-caça), conta e sobre. */

import { ui } from "../art";
import {
  button,
  DEFAULT_KEYS,
  el,
  isTouch,
  keyLabel,
  segmented,
  select,
  settings,
  slider,
  tabs,
  toast,
  toggle,
  type Settings,
} from "../kit";

export interface SettingsOpts {
  close(): void;
  /** Volta para a tela de personagens. */
  exit(): void;
  logout(): void;
  /** Dados para as abas Conta e Sobre. */
  info(): { account: string; character: string; level: number; ping: number; online: number };
}

const pct = (v: number) => `${v}%`;

function row(label: string, control: HTMLElement, hint?: string): HTMLElement {
  return el("div", "set-row", el("div", "set-label", el("span", null, label), hint ? el("small", null, hint) : null), el("div", "set-ctrl", control));
}

function group(title: string, ...rows: HTMLElement[]): HTMLElement {
  return el("section", "set-group", el("h3", "set-group-title", title), ...rows);
}

const set = <K extends keyof Settings>(k: K) => (v: Settings[K]) => settings.set(k, v);

const ACTIONS: { id: string; label: string }[] = [
  { id: "inventory", label: "Inventário" },
  { id: "skills", label: "Árvore de skills" },
  { id: "party", label: "Grupo" },
  { id: "clan", label: "Clã" },
  { id: "character", label: "Personagem / atributos" },
  { id: "map", label: "Mapa do mundo" },
  { id: "settings", label: "Configurações / fechar janela" },
  { id: "auto", label: "Ligar/desligar auto-caça" },
  { id: "target", label: "Próximo alvo" },
  { id: "potion", label: "Poção de vida" },
];

export function buildSettings(opts: SettingsOpts): HTMLElement {
  const s = settings.get();
  const root = el("div", "settings-win");
  const body = el("div", "set-body scroll");
  const nav = tabs(
    [
      { id: "graphics", label: "Gráficos" },
      { id: "audio", label: "Áudio" },
      { id: "controls", label: "Controles" },
      { id: "game", label: "Jogo" },
      { id: "account", label: "Conta" },
      { id: "about", label: "Sobre" },
    ],
    "game",
    (id) => show(id),
    "tabs set-tabs",
  );

  const pages: Record<string, () => HTMLElement> = {
    graphics: () =>
      el(
        "div",
        "set-page",
        group(
          "Qualidade",
          row(
            "Predefinição",
            segmented(
              [
                { id: "baixa", label: "Baixa" },
                { id: "media", label: "Média" },
                { id: "alta", label: "Alta" },
                { id: "ultra", label: "Ultra" },
              ],
              s.quality,
              set("quality"),
            ),
            "Ajusta sombras, distância e efeitos de uma vez.",
          ),
          row("Sombras", toggle(s.shadows, set("shadows"), "Sombras")),
          row("Distância de visão", slider(s.viewDistance, 30, 150, 10, set("viewDistance"), (v) => `${v} m`)),
          row(
            "Limite de FPS",
            select(
              [
                { id: "30", label: "30 FPS (economia de bateria)" },
                { id: "60", label: "60 FPS" },
                { id: "120", label: "120 FPS" },
                { id: "0", label: "Sem limite" },
              ],
              s.fpsLimit,
              set("fpsLimit"),
            ),
          ),
        ),
        group("Interface", row("Escala da interface", slider(s.uiScale, 80, 130, 5, set("uiScale"), pct), "Aumente em telas pequenas ou TVs.")),
      ),
    audio: () =>
      el(
        "div",
        "set-page",
        group(
          "Volume",
          row("Geral", slider(s.vMaster, 0, 100, 1, set("vMaster"), pct)),
          row("Música", slider(s.vMusic, 0, 100, 1, set("vMusic"), pct)),
          row("Efeitos", slider(s.vSfx, 0, 100, 1, set("vSfx"), pct)),
          row("Ambiente", slider(s.vAmbient, 0, 100, 1, set("vAmbient"), pct)),
        ),
        el("p", "hint-box small", "O protótipo ainda não tem som; os controles mostram como a tela vai ficar."),
      ),
    controls: () => {
      const keys = el("div", "keybinds");
      const renderKeys = () => {
        keys.replaceChildren();
        const k = settings.get().keys;
        for (const a of ACTIONS) {
          const b = button(keyLabel(k[a.id]), "keycap");
          b.onclick = () => rebind(a.id, b);
          keys.append(el("div", "kb-row", el("span", null, a.label), b));
        }
        keys.append(el("div", "kb-row fixed", el("span", null, "Barra de atalhos"), el("span", "keycap static", "1 – 0")));
        keys.append(el("div", "kb-row fixed", el("span", null, "Andar"), el("span", "keycap static", "Clique · WASD")));
        keys.append(el("div", "kb-row fixed", el("span", null, "Girar câmera"), el("span", "keycap static", "Botão direito")));
        keys.append(el("div", "kb-row fixed", el("span", null, "Chat"), el("span", "keycap static", "Enter")));
      };
      const rebind = (id: string, b: HTMLButtonElement) => {
        b.textContent = "Pressione…";
        b.classList.add("rebinding");
        const on = (e: KeyboardEvent) => {
          e.preventDefault();
          e.stopPropagation();
          removeEventListener("keydown", on, true);
          b.classList.remove("rebinding");
          if (e.code !== "Escape" || id === "settings") {
            const k = { ...settings.get().keys };
            const clash = Object.entries(k).find(([other, code]) => code === e.code && other !== id);
            if (clash) k[clash[0]] = k[id];
            k[id] = e.code;
            settings.set("keys", k);
            if (clash) toast(`Trocado com "${ACTIONS.find((a) => a.id === clash[0])?.label}"`);
          }
          renderKeys();
        };
        addEventListener("keydown", on, true);
      };
      renderKeys();
      const resetKeys = button("Restaurar teclas", "btn btn-sm", () => {
        settings.set("keys", { ...DEFAULT_KEYS });
        renderKeys();
      });
      return el(
        "div",
        "set-page",
        group(
          "Toque (celular e tablet)",
          row(
            "Controles de toque",
            segmented(
              [
                { id: "auto", label: "Automático" },
                { id: "on", label: "Sempre" },
                { id: "off", label: "Nunca" },
              ],
              s.touchMode,
              set("touchMode"),
            ),
            isTouch() ? "Ligados agora." : "Desligados agora (tela sem toque).",
          ),
          row("Tamanho do joystick", slider(s.joySize, 70, 140, 5, set("joySize"), pct)),
          row("Opacidade dos botões", slider(s.joyOpacity, 30, 100, 5, set("joyOpacity"), pct)),
          row("Modo canhoto", toggle(s.lefty, set("lefty"), "Modo canhoto"), "Joystick à direita e botões à esquerda."),
        ),
        group("Teclado (computador)", keys, el("div", "set-actions", resetKeys)),
      );
    },
    game: () =>
      el(
        "div",
        "set-page",
        group(
          "Auto-caça",
          row("Raio de busca", slider(s.autoRadius, 10, 50, 5, set("autoRadius"), (v) => `${v} m`), "Até onde o personagem procura monstros."),
          row("Usar skills", toggle(s.autoSkills, set("autoSkills"), "Usar skills"), "Usa as skills da barra de atalhos, na ordem."),
          row("Tomar poção abaixo de", slider(s.autoPotion, 10, 90, 5, set("autoPotion"), pct)),
          row("Voltar à cidade sem poção", toggle(s.autoReturn, set("autoReturn"), "Voltar à cidade")),
          row("Pegar itens do chão", toggle(s.autoLoot, set("autoLoot"), "Pegar itens")),
        ),
        group(
          "Tela",
          row("Nomes sobre os monstros", toggle(s.showNames, set("showNames"), "Nomes")),
          row("Números de dano", toggle(s.showDamage, set("showDamage"), "Números de dano")),
          row("Mostrar elmo", toggle(s.showHelm, set("showHelm"), "Mostrar elmo")),
          row(
            "Idioma",
            select(
              [
                { id: "pt-BR", label: "Português (Brasil)" },
                { id: "en", label: "English" },
                { id: "es", label: "Español" },
              ],
              s.language,
              set("language"),
            ),
          ),
        ),
      ),
    account: () => {
      const i = opts.info();
      return el(
        "div",
        "set-page",
        group(
          "Conta",
          el("div", "account-card", el("span", "acc-icon", el("span", null)), el("div", null, el("b", null, i.account || "Conta"), el("div", "muted small", "Sessão neste navegador"))),
          row("Personagem atual", el("span", null, `${i.character} · Nv ${i.level}`)),
        ),
        el(
          "div",
          "set-actions",
          button("Trocar personagem", "btn", opts.exit),
          (() => {
            const b = button("", "btn btn-danger", opts.logout);
            b.innerHTML = `${ui("logout", 16)}<span>Sair da conta</span>`;
            return b;
          })(),
        ),
      );
    },
    about: () =>
      el(
        "div",
        "set-page about",
        el("div", "about-logo", el("span", "logo-word small-logo", "EREVIA"), el("span", "muted small", "Fase 3 · interface nova")),
        group(
          "Conexão",
          row("Ping", el("span", opts.info().ping < 80 ? "ok" : "warn", `${opts.info().ping} ms`)),
          row("Entidades por perto", el("span", null, String(opts.info().online))),
        ),
        group(
          "Atalhos rápidos",
          el(
            "div",
            "shortcut-grid",
            ...[
              ["I", "Inventário"],
              ["K", "Skills"],
              ["C", "Personagem"],
              ["M", "Mapa"],
              ["P", "Grupo"],
              ["N", "Clã"],
              ["F", "Auto-caça"],
              ["Tab", "Próximo alvo"],
              ["Q", "Poção"],
              ["Enter", "Chat"],
              ["Ctrl + clique", "Atacar jogador"],
              ["/g · /c", "Chat do grupo · clã"],
              ["/bug texto", "Relatar um problema"],
            ].map(([k, l]) => el("div", "sc", el("kbd", null, k), el("span", null, l))),
          ),
        ),
        group(
          "Créditos",
          el(
            "p",
            "small muted credits",
            "Personagens, monstros, armas, animações e cenário: KayKit (Kay Lousberg), licença CC0. Interface, ícones e código: equipe Erevia.",
          ),
        ),
      ),
  };

  function show(id: string) {
    body.replaceChildren(pages[id]());
    body.scrollTop = 0;
  }

  const foot = el(
    "div",
    "set-foot",
    button("Restaurar padrões", "btn btn-sm btn-ghost", () => {
      settings.reset();
      toast("Configurações restauradas");
      opts.close();
    }),
    el("span", "muted small", "Salvo automaticamente"),
    button("Fechar", "btn btn-gold btn-sm", opts.close),
  );
  root.append(nav.root, body, foot);
  show("game");
  return root;
}
