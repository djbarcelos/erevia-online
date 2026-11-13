/**
 * Telas de entrada (fora do mundo 3D): carregando, login, personagens (seleção e criação
 * com o modelo 3D no pedestal) e mensagens. Tudo dentro de #overlay, sobre um fundo animado.
 */

import "../styles/tokens.css";
import "../styles/kit.css";
import "../styles/screens.css";
import { api, ApiError, type CharacterInfo, type ClassId, type Providers } from "../api";
import { CLASSES, classInfo } from "../classes";
import { ui, uniqueIds } from "./art";
import { animatedBackdrop } from "./backdrop";
import { skillIcon } from "./icons";
import { art, button, el, isCompact, tabs, toast } from "./kit";
import { CLASS_BAR_NAMES, CLASS_EXTRA, DEFAULT_LOOK } from "./looks";
import { createPreview, type Preview } from "./preview3d";

const overlay = document.getElementById("overlay")!;
const content = el("div", "entry-content");
overlay.append(content);

let stopBg: (() => void) | null = null;
let preview: Preview | null = null;

/** Troca o conteúdo do overlay (descarta o 3D da tela anterior). */
function mount(kind: string, ...nodes: Node[]): void {
  preview?.dispose();
  preview = null;
  overlay.hidden = false;
  if (!stopBg) stopBg = animatedBackdrop(overlay, { castle: kind !== "chars", tint: kind === "chars" ? "#3a2a48" : undefined });
  content.className = `entry-content ${kind}`;
  content.replaceChildren(...nodes);
}

type Screen = "screen-loading" | null;

/** Mostra a tela de carregando, ou esconde tudo (null = entrou no jogo). */
export function showScreen(screen: Screen): void {
  if (screen === null) {
    preview?.dispose();
    preview = null;
    stopBg?.();
    stopBg = null;
    overlay.hidden = true;
    content.replaceChildren();
    return;
  }
  mount("loading", el("div", "entry-loading", el("h1", "logo-word", "EREVIA"), el("span", "spinner"), el("p", "muted", "Carregando…")));
}

export function showMessage(title: string, text: string, buttonText: string, onClick: () => void): void {
  const box = el("section", "entry-msg frame", el("h2", "title", title), el("p", null, text), button(buttonText, "btn btn-gold", onClick));
  box.setAttribute("role", "alertdialog");
  mount("message", box);
}

// ---------------------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------------------

/** Mensagens amigáveis para os códigos de erro do callback OAuth. */
const LOGIN_ERRORS: Record<string, string> = {
  cancelado: "Login cancelado.",
  sessao_expirada: "A tentativa de login expirou. Tente de novo.",
  state_invalido: "Não foi possível confirmar o login. Tente de novo.",
  troca_falhou: "O provedor recusou o login. Tente de novo.",
  perfil_falhou: "Não foi possível ler seu perfil no provedor.",
  erro_interno: "Erro no servidor. Tente de novo.",
};

const NEWS = [
  { tag: "Atualização", title: "Itens e loja", text: "Monstros deixam itens no chão. A Mercadora e o Ferreiro abriram as portas na praça." },
  { tag: "Atualização", title: "Dia e noite", text: "Tochas acendem na cidade e, à noite, os monstros batem mais forte e dão mais XP." },
  { tag: "Novo", title: "Interface nova", text: "Nova tela de personagens, HUD, mapa do mundo e controles de toque no celular." },
];

export function showLogin(providers: Providers, loginError: string | null, onLoggedIn: () => void): void {
  const card = el("section", "login-card frame");
  card.setAttribute("aria-label", "Entrar");
  card.append(el("h2", "title card-title", "Entrar"));

  const social = el("div", "stack");
  if (providers.google) {
    const a = el("a", "btn btn-block btn-social google");
    a.href = "/api/auth/google/login";
    a.innerHTML = `${ui("google", 20)}<span>Entrar com Google</span>`;
    social.append(a);
  }
  if (providers.discord) {
    const a = el("a", "btn btn-block btn-social discord");
    a.href = "/api/auth/discord/login";
    a.innerHTML = `${ui("discord", 20)}<span>Entrar com Discord</span>`;
    social.append(a);
  }
  if (social.childElementCount) card.append(social);

  const error = el("p", "login-error bad small");
  error.setAttribute("role", "alert");
  if (loginError) error.textContent = LOGIN_ERRORS[loginError] ?? "Falha no login.";

  if (providers.dev) {
    if (social.childElementCount) card.append(el("div", "divider", "ou"));
    const form = el("form", "dev-login");
    const user = el("input", "input");
    user.placeholder = "Nome de usuário";
    user.minLength = 3;
    user.maxLength = 16;
    user.pattern = "[A-Za-z0-9]{3,16}";
    user.required = true;
    user.autocomplete = "off";
    user.setAttribute("aria-label", "Usuário de desenvolvimento");
    const submit = button("Entrar", "btn btn-gold");
    submit.type = "submit";
    form.append(user, submit);
    form.onsubmit = async (ev) => {
      ev.preventDefault();
      if (!user.checkValidity()) {
        user.classList.add("invalid");
        error.textContent = "Use de 3 a 16 letras ou números.";
        return;
      }
      submit.disabled = true;
      submit.classList.add("loading");
      try {
        await api.devLogin(user.value.trim());
        onLoggedIn();
      } catch (err) {
        error.textContent = (err as Error).message;
      } finally {
        submit.disabled = false;
        submit.classList.remove("loading");
      }
    };
    user.oninput = () => user.classList.remove("invalid");
    card.append(el("label", "label", "Login de desenvolvimento (só local)"), form);
  }
  if (!providers.google && !providers.discord && !providers.dev) card.append(el("p", "muted", "Nenhum login configurado no servidor."));
  card.append(error);

  const news = el("aside", "login-news frame glass", el("h2", "title", "Novidades"));
  for (const n of NEWS) {
    news.append(
      el("article", "news-item", el("div", "news-meta", el("span", `news-tag t-${n.tag.toLowerCase()}`, n.tag)), el("h3", null, n.title), el("p", null, n.text)),
    );
  }

  const logo = el(
    "header",
    "login-logo",
    el("div", "logo-crest", el("span")),
    el("h1", "logo-word", "EREVIA"),
    el("p", "logo-tag", "As terras partidas esperam por você"),
  );
  const layout = el(
    "div",
    "login-layout scroll",
    el("div", "login-main", logo, card),
    news,
    el("footer", "login-foot", el("span", null, "Modelos 3D: KayKit (CC0)"), el("span", null, "© 2026 Erevia")),
  );
  mount("login", layout);
}

// ---------------------------------------------------------------------------------------
// Personagens
// ---------------------------------------------------------------------------------------

const POSES: { role: "idle" | "attack" | "skill" | "buff"; label: string }[] = [
  { role: "idle", label: "Parado" },
  { role: "attack", label: "Ataque" },
  { role: "skill", label: "Skill" },
  { role: "buff", label: "Vitória" },
];

/** Skills iniciais de cada classe (as do servidor hoje; ícones reais). */
const START_SKILLS: Record<ClassId, { fx: string; name: string; meta: string }[]> = {
  knight: [
    { fx: "shield_bash", name: "Golpe de Escudo", meta: "Atordoa o alvo" },
    { fx: "fortress", name: "Fortaleza", meta: "+Defesa por 10 s" },
  ],
  barbarian: [
    { fx: "brutal_strike", name: "Golpe Brutal", meta: "Golpe muito forte" },
    { fx: "fury", name: "Fúria", meta: "+Ataque por 10 s" },
  ],
  mage: [
    { fx: "fireball", name: "Bola de Fogo", meta: "Dano à distância" },
    { fx: "ice_nova", name: "Nova de Gelo", meta: "Área com lentidão" },
  ],
  rogue: [
    { fx: "backstab", name: "Golpe Furtivo", meta: "Crítico garantido quase sempre" },
    { fx: "shadow_step", name: "Passo Sombrio", meta: "+Velocidade por 6 s" },
  ],
};

export async function showCharacters(onPlay: (c: CharacterInfo) => void, onLogout: () => void): Promise<void> {
  const [me, data] = await Promise.all([api.me(), api.characters()]);
  let roster = data.characters;
  const max = data.max;

  const root = el("div", "char-screen");
  const stage = el("div", "char-stage");
  const canvas = el("canvas", "char-canvas");
  canvas.setAttribute("aria-label", "Prévia 3D do personagem — arraste para girar");
  const loading = el("div", "stage-loading", el("span", "spinner"), "Carregando modelo…");
  const nameplate = el("div", "nameplate", el("span", "np-name"), el("span", "np-sub"));
  const spinL = button("◀", "btn btn-icon btn-ghost spin-btn l");
  const spinR = button("▶", "btn btn-icon btn-ghost spin-btn r");
  spinL.setAttribute("aria-label", "Girar para a esquerda");
  spinR.setAttribute("aria-label", "Girar para a direita");
  stage.append(canvas, loading, nameplate, spinL, spinR, el("p", "stage-hint", "Arraste para girar · role ou pince para aproximar"));

  // ---------- Topo ----------
  let mode: "select" | "create" = roster.length ? "select" : "create";
  const top = el("header", "char-top");
  const logout = button("", "btn btn-ghost btn-sm", async () => {
    await api.logout().catch(() => undefined);
    onLogout();
  });
  logout.innerHTML = `${ui("logout", 16)}<span>Sair</span>`;
  const modeTabs = tabs(
    [
      { id: "select", label: "Personagens" },
      { id: "create", label: "Criar novo" },
    ],
    mode,
    (id) => setMode(id as typeof mode),
    "tabs mode-tabs",
  );
  top.append(logout, modeTabs.root, el("span", "account small muted", me.displayName ? `Conectado como ${me.displayName}` : ""));

  // ---------- Seleção ----------
  const selectPanel = el("aside", "char-panel left frame glass select-panel");
  const selectFoot = el("div", "char-foot select-foot");
  let picked = 0;

  const renderSelect = () => {
    selectPanel.replaceChildren(el("h2", "title panel-title", "Personagens", el("span", "count", ` ${roster.length}/${max}`)));
    roster.forEach((c, i) => {
      const info = classInfo(c.class);
      const b = el("button", "roster-slot");
      b.type = "button";
      b.setAttribute("aria-pressed", String(i === picked));
      b.style.setProperty("--cc", info.color);
      b.append(el("span", "roster-crest", info.label[0]), el("span", "roster-info", el("b", null, c.name), el("span", null, `${info.label} · Nv ${c.level ?? 1}`)));
      b.onclick = () => {
        picked = i;
        renderSelect();
      };
      b.ondblclick = () => onPlay(c);
      selectPanel.append(b);
    });
    if (roster.length < max) {
      const empty = el("button", "roster-slot empty");
      empty.type = "button";
      empty.innerHTML = `${ui("plus", 22)}<span>Criar novo personagem</span>`;
      empty.onclick = () => modeTabs.select("create");
      selectPanel.append(empty);
    }
    const c = roster[picked];
    if (!c) return;
    void preview?.show(c.class, DEFAULT_LOOK[c.class]);
    setPlate(c.name, `${classInfo(c.class).label} · Nível ${c.level ?? 1}`, DEFAULT_LOOK[c.class].emblem);
    const play = button("Entrar no mundo", "btn btn-gold btn-play", () => onPlay(c));
    selectFoot.replaceChildren(el("div", "foot-info", el("b", null, c.name), el("span", "muted small", `${classInfo(c.class).label} · Nível ${c.level ?? 1}`)), play);
  };

  // ---------- Criação ----------
  let cls: ClassId = "knight";
  let name = "";
  const classPanel = el("aside", "char-panel left frame glass create-panel");
  const lookPanel = el("aside", "char-panel right frame glass create-panel");
  const createFoot = el("div", "char-foot create-foot");

  const renderClasses = () => {
    classPanel.replaceChildren(el("h2", "title panel-title", "Classe"));
    const grid = el("div", "class-grid");
    for (const c of CLASSES) {
      const x = CLASS_EXTRA[c.id];
      const b = el("button", "class-card2");
      b.type = "button";
      b.setAttribute("aria-pressed", String(c.id === cls));
      b.style.setProperty("--cc", c.color);
      b.append(
        el("span", "cc-crest", c.label[0]),
        el("span", "cc-text", el("b", null, c.label), el("small", null, x.role)),
        el("span", "cc-diff", ...[1, 2, 3].map((n) => el("i", n <= x.diff ? "on" : ""))),
      );
      b.title = `Dificuldade ${["", "fácil", "média", "difícil"][x.diff]}`;
      b.onclick = () => {
        cls = c.id;
        renderClasses();
        refreshPreview();
      };
      grid.append(b);
    }
    const info = classInfo(cls);
    const x = CLASS_EXTRA[cls];
    const bars = el("div", "class-bars");
    x.bars.forEach((v, i) => {
      const fill = el("i");
      fill.style.setProperty("--v", `${v}%`);
      bars.append(el("span", "cb-name", CLASS_BAR_NAMES[i]), el("span", "cb-bar", fill), el("span", "cb-val", String(v)));
    });
    const skills = el("div", "class-skills");
    for (const s of START_SKILLS[cls]) skills.append(el("div", "cs-skill", art(uniqueIds(skillIcon(s.fx)), "cs-icon"), el("span", null, el("b", null, s.name), el("small", null, s.meta))));
    classPanel.append(grid, el("div", "class-detail", el("p", "class-desc", info.desc), bars, el("div", "label", "Skills iniciais"), skills));
  };

  const renderLook = () => {
    const poses = el("div", "pose-row");
    for (const p of POSES) poses.append(button(p.label, "btn btn-sm", () => preview?.play(p.role)));
    lookPanel.replaceChildren(
      el("h2", "title panel-title", "Aparência"),
      el("div", "opt-block", el("span", "label", "Ver pose"), poses),
      el("p", "hint-box small", "Escolha de arma inicial e cor do emblema chegam na próxima atualização."),
    );
  };

  const nameInput = el("input", "input name-input");
  nameInput.placeholder = "Nome do personagem";
  nameInput.maxLength = 16;
  nameInput.autocomplete = "off";
  nameInput.setAttribute("aria-label", "Nome do personagem");
  const nameMsg = el("span", "name-msg small");
  const createBtn = button("Criar personagem", "btn btn-gold btn-play");
  const validate = () => {
    name = nameInput.value.trim();
    let msg = "3 a 16 letras ou números.";
    let ok = false;
    if (name && !/^[A-Za-z0-9]+$/.test(name)) msg = "Só letras e números, sem espaço ou acento.";
    else if (name && name.length < 3) msg = "Mínimo de 3 caracteres.";
    else if (name) {
      msg = "";
      ok = true;
    }
    nameMsg.textContent = msg;
    nameMsg.className = `name-msg small ${name && !ok ? "bad" : "muted"}`;
    nameInput.classList.toggle("invalid", !!name && !ok);
    createBtn.disabled = !ok || roster.length >= max;
    setPlate(name || "Seu nome", classInfo(cls).label, DEFAULT_LOOK[cls].emblem);
    return ok;
  };
  nameInput.oninput = validate;
  const create = async () => {
    if (!validate()) return;
    createBtn.disabled = true;
    createBtn.classList.add("loading");
    try {
      const c = await api.createCharacter(name, cls);
      roster = [...roster, { ...c, level: c.level ?? 1 }];
      picked = roster.length - 1;
      nameInput.value = "";
      toast(`${c.name} foi criado!`);
      modeTabs.select("select");
    } catch (err) {
      nameMsg.textContent = err instanceof ApiError ? err.message : "Erro ao criar personagem.";
      nameMsg.className = "name-msg small bad";
    } finally {
      createBtn.classList.remove("loading");
      createBtn.disabled = false;
    }
  };
  createBtn.onclick = () => void create();
  nameInput.onkeydown = (e) => {
    if (e.key === "Enter") void create();
  };
  createFoot.append(el("div", "name-field", nameInput, nameMsg), createBtn);

  const mobileTabs = tabs(
    [
      { id: "class", label: "Classe" },
      { id: "look", label: "Aparência" },
    ],
    "class",
    (id) => (root.dataset.mtab = id),
    "tabs mobile-tabs create-panel",
  );
  root.dataset.mtab = "class";

  function refreshPreview() {
    void preview?.show(cls, DEFAULT_LOOK[cls]);
    setPlate(name || "Seu nome", classInfo(cls).label, DEFAULT_LOOK[cls].emblem);
  }

  function setPlate(n: string, sub: string, color: string) {
    nameplate.querySelector(".np-name")!.textContent = n;
    nameplate.querySelector(".np-sub")!.textContent = sub;
    nameplate.style.setProperty("--emblem", color);
  }

  function setMode(m: typeof mode) {
    if (m === "create" && roster.length >= max) {
      toast(`Limite de ${max} personagens por conta.`, "warn");
      modeTabs.select("select", false);
      return;
    }
    mode = m;
    root.dataset.mode = m;
    if (m === "select") renderSelect();
    else {
      renderClasses();
      renderLook();
      validate();
      refreshPreview();
      if (!isCompact()) nameInput.focus({ preventScroll: true });
    }
  }

  root.append(stage, top, selectPanel, classPanel, mobileTabs.root, lookPanel, selectFoot, createFoot);
  mount("chars", root);
  // O palco 3D é criado depois de o canvas estar na página (precisa do tamanho).
  preview = createPreview(canvas);
  preview.ready.then(() => loading.remove()).catch(() => (loading.textContent = "Modelos 3D indisponíveis"));
  spinL.onclick = () => preview?.spin(-0.6);
  spinR.onclick = () => preview?.spin(0.6);
  modeTabs.select(mode, false);
  setMode(mode);
}
