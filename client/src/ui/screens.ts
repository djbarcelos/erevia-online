/**
 * Telas de entrada (fora do mundo 3D): carregando, login, personagens (seleção e criação
 * com o modelo 3D no pedestal) e mensagens. Tudo dentro de #overlay, sobre um fundo animado.
 */

import "../styles/tokens.css";
import "../styles/kit.css";
import "../styles/screens.css";
import { api, ApiError, type CharacterInfo, type ClassId, type CreationOption, type CreationOptions, type LookChoice, type Providers } from "../api";
import { CLASSES, classInfo } from "../classes";
import { ui, uniqueIds } from "./art";
import { animatedBackdrop } from "./backdrop";
import { skillIcon } from "./icons";
import { art, button, el, isCompact, tabs, toast } from "./kit";
import { CLASS_BAR_NAMES, CLASS_EXTRA, DEFAULT_LOOK } from "./looks";
import { createPreview, type Appearance, type Preview } from "./preview3d";
import { bodyEditor } from "./body-editor";
import { openWhatsNewAlone } from "./windows/whatsnew";
import { normalLook, ORIGINAL_LOOK, setBodyPalettes, toBodyLook, type BodyLook } from "../world/body-look";

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

/** Mostra a tela de carregando (com um texto opcional), ou esconde tudo (null = entrou no jogo). */
export function showScreen(screen: Screen, text = "Carregando…"): void {
  if (screen === null) {
    preview?.dispose();
    preview = null;
    stopBg?.();
    stopBg = null;
    overlay.hidden = true;
    content.replaceChildren();
    return;
  }
  mount("loading", el("div", "entry-loading", el("h1", "logo-word", "EREVIA"), el("span", "spinner"), el("p", "muted", text)));
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
  { tag: "Novo", title: "Visual do personagem", text: "Rosto, cabelo, pele, roupa e chapéu na criação. Mude depois com Teodoro, o Barbeiro da praça." },
  { tag: "Novo", title: "13 encaixes de equipamento", text: "Pernas, luvas, botas, capa, colar, dois brincos e dois anéis. Joias na Mercadora, armaduras no Ferreiro." },
  { tag: "Novo", title: "Escolha sua arma", text: "Na criação, escolha a arma inicial, o escudo e a cor do emblema que pinta seu nome." },
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
  card.append(error, el("p", "login-legal small muted", "Ao entrar, você concorda com os ", legalLink("termos"), " e com a ", legalLink("privacidade"), "."));

  const news = el("aside", "login-news frame glass", el("h2", "title", "Novidades"));
  for (const n of NEWS) {
    news.append(
      el("article", "news-item", el("div", "news-meta", el("span", `news-tag t-${n.tag.toLowerCase()}`, n.tag)), el("h3", null, n.title), el("p", null, n.text)),
    );
  }
  news.append(button("O que há de novo", "btn btn-sm btn-ghost news-all", openWhatsNewAlone));

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
    el(
      "footer",
      "login-foot",
      newsLink(),
      legalLink("privacidade"),
      legalLink("termos"),
      el("span", null, "Modelos 3D: KayKit (CC0)"),
      el("span", null, "© 2026 Erevia"),
    ),
  );
  mount("login", layout);
}

// ---------------------------------------------------------------------------------------
// Páginas legais (estáticas: client/privacidade.html e client/termos.html)
// ---------------------------------------------------------------------------------------

const LEGAL = {
  privacidade: { href: "/privacidade.html", label: "Política de Privacidade" },
  termos: { href: "/termos.html", label: "Termos de Uso" },
} as const;

/** Link para uma página legal (abre em outra aba para não perder a tela atual). */
function legalLink(kind: keyof typeof LEGAL, text: string = LEGAL[kind].label): HTMLAnchorElement {
  const a = el("a", "legal-link", text);
  a.href = LEGAL[kind].href;
  a.target = "_blank";
  a.rel = "noopener";
  return a;
}

/** Link para a página pública de novidades (client/novidades.html). */
function newsLink(): HTMLAnchorElement {
  const a = el("a", "legal-link", "Novidades");
  a.href = "/novidades";
  a.target = "_blank";
  a.rel = "noopener";
  return a;
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

/** Primeira opção de cada lista e a cor padrão da classe; o visual do corpo vem de `body`. */
function defaultChoice(opts: CreationOptions, cls: ClassId, body: BodyLook = ORIGINAL_LOOK): LookChoice {
  const o = opts.classes[cls];
  return { weapon: o.weapons[0]?.id ?? "", offhand: o.offhands[0]?.id ?? "", emblem: o.emblem, ...normalLook(cls, body, opts) };
}

function bodyOf(c: LookChoice): BodyLook {
  return toBodyLook(c);
}

/** O que o palco 3D mostra para uma escolha da criação. */
function choiceLook(opts: CreationOptions, cls: ClassId, c: LookChoice): Appearance {
  const o = opts.classes[cls];
  const w = o.weapons.find((x) => x.id === c.weapon);
  const off = o.offhands.find((x) => x.id === c.offhand);
  return { right: w?.model || null, left: off?.model || w?.offModel || null, emblem: c.emblem, body: bodyOf(c) };
}

/** Aparência de um personagem salvo: as armas que estão nas mãos dele agora. */
function rosterLook(c: CharacterInfo): Appearance {
  const def = DEFAULT_LOOK[c.class];
  const emblem = c.emblem ?? def.emblem;
  const body = toBodyLook(c.appearance);
  if (!c.look) return { ...def, emblem, body };
  return { right: c.look.right || null, left: c.look.left || null, emblem, body };
}

export async function showCharacters(onPlay: (c: CharacterInfo) => void, onLogout: () => void): Promise<void> {
  const [me, data, opts] = await Promise.all([api.me(), api.characters(), api.creation()]);
  setBodyPalettes(opts);
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
  const account = button("", "btn btn-ghost btn-sm btn-account", () => openAccountDialog(me.displayName, roster.length, onLogout));
  account.innerHTML = `${ui("user", 16)}<span>Conta</span>`;
  account.setAttribute("aria-label", "Conta: privacidade, termos e apagar a conta");
  top.append(
    logout,
    account,
    modeTabs.root,
    el(
      "span",
      "account small muted",
      me.displayName ? `Conectado como ${me.displayName}` : "",
      el("span", "account-legal", legalLink("privacidade", "Privacidade"), legalLink("termos", "Termos")),
    ),
  );

  // ---------- Seleção ----------
  const selectPanel = el("aside", "char-panel left frame glass select-panel");
  const selectFoot = el("div", "char-foot select-foot");
  let picked = 0;

  /** "Mago · Nv 20" (a classe ativa) e, com subclasse, "Cavaleiro 30 · Mago 20". */
  const classLine = (c: CharacterInfo, nv: string) => `${classInfo(c.activeClass ?? c.class).label} · ${nv} ${c.level ?? 1}`;
  const classesLine = (c: CharacterInfo) => (c.classes && c.classes.length > 1 ? c.classes.map((k) => `${classInfo(k.class).label} ${k.level}`).join(" · ") : "");
  const renderSelect = () => {
    selectPanel.replaceChildren(el("h2", "title panel-title", "Personagens", el("span", "count", ` ${roster.length}/${max}`)));
    roster.forEach((c, i) => {
      const info = classInfo(c.activeClass ?? c.class);
      const b = el("button", "roster-slot");
      b.type = "button";
      b.setAttribute("aria-pressed", String(i === picked));
      b.style.setProperty("--cc", info.color);
      b.style.setProperty("--emblem", c.emblem ?? DEFAULT_LOOK[c.class].emblem);
      b.append(el("span", "roster-crest", info.label[0]), el("span", "roster-info", el("b", null, c.name), el("span", null, classLine(c, "Nv"))));
      const subs = classesLine(c);
      if (subs) b.title = subs;
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
    const look = rosterLook(c);
    void preview?.show(c.class, look);
    const subs = classesLine(c);
    setPlate(c.name, classLine(c, "Nível") + (subs ? ` (${subs})` : ""), look.emblem);
    const play = button("Entrar no mundo", "btn btn-gold btn-play", () => onPlay(c));
    selectFoot.replaceChildren(el("div", "foot-info", el("b", null, c.name), el("span", "muted small", classLine(c, "Nível") + (subs ? ` · ${subs}` : ""))), play);
  };

  // ---------- Criação ----------
  let cls: ClassId = "knight";
  let name = "";
  let choice: LookChoice = defaultChoice(opts, cls);
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
        choice = defaultChoice(opts, cls, bodyOf(choice)); // mantém o visual escolhido
        renderClasses();
        renderLook();
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

  /** Linha "‹ valor ›" que percorre as opções. */
  const optionRow = (label: string, list: CreationOption[], value: string, onPick: (id: string) => void) => {
    let idx = Math.max(0, list.findIndex((o) => o.id === value));
    const text = el("span", "opt-value", list[idx].name);
    const step = (dir: number) => {
      idx = (idx + dir + list.length) % list.length;
      text.textContent = list[idx].name;
      onPick(list[idx].id);
    };
    const prev = button("‹", "btn btn-icon btn-sm opt-step", () => step(-1));
    const next = button("›", "btn btn-icon btn-sm opt-step", () => step(1));
    prev.setAttribute("aria-label", `${label}: anterior`);
    next.setAttribute("aria-label", `${label}: próximo`);
    return el("div", "opt-row", el("span", "label", label), el("div", "opt-ctrl", prev, text, next));
  };

  const renderLook = () => {
    const o = opts.classes[cls];
    const paired = o.weapons.some((w) => w.offModel);
    const rows: HTMLElement[] = [
      optionRow(paired ? "Armas" : "Arma inicial", o.weapons, choice.weapon, (id) => {
        choice.weapon = id;
        refreshPreview();
      }),
    ];
    if (o.offhands.length > 1) {
      rows.push(
        optionRow("Mão secundária", o.offhands, choice.offhand, (id) => {
          choice.offhand = id;
          refreshPreview();
        }),
      );
    }
    const sw = el("div", "swatches");
    sw.setAttribute("role", "radiogroup");
    sw.setAttribute("aria-label", "Cor do emblema");
    for (const c of opts.emblems) {
      const b = el("button", "swatch");
      b.type = "button";
      b.style.setProperty("--c", c);
      b.setAttribute("role", "radio");
      b.setAttribute("aria-checked", String(c === choice.emblem));
      b.setAttribute("aria-label", `Cor ${c}`);
      b.onclick = () => {
        choice.emblem = c;
        sw.querySelectorAll(".swatch").forEach((x) => x.setAttribute("aria-checked", String(x === b)));
        preview?.setEmblem(c);
        setPlate(name || "Seu nome", classInfo(cls).label, c);
      };
      sw.append(b);
    }
    const poses = el("div", "pose-row");
    for (const p of POSES) poses.append(button(p.label, "btn btn-sm", () => preview?.play(p.role)));
    const body = bodyEditor(opts, cls, bodyOf(choice), (l) => {
      choice = { ...choice, ...l };
      preview?.setBody(l);
    });
    lookPanel.replaceChildren(
      el("h2", "title panel-title", "Aparência"),
      ...rows,
      el("div", "opt-block", el("span", "label", "Cor do emblema"), sw),
      el("h3", "title panel-subtitle", "Visual"),
      body.root,
      el("div", "opt-block", el("span", "label", "Ver pose"), poses),
      el(
        "p",
        "hint-box small",
        o.offhands.length > 1
          ? "A arma escolhida é a sua arma inicial. Sem escudo na mão, ele vai para a bolsa. A cor do emblema pinta seu nome e o anel no chão. O visual pode ser mudado depois no Barbeiro da cidade."
          : "A arma escolhida é a sua arma inicial. A cor do emblema pinta seu nome e o anel no chão. O visual pode ser mudado depois no Barbeiro da cidade.",
      ),
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
    setPlate(name || "Seu nome", classInfo(cls).label, choice.emblem);
    return ok;
  };
  nameInput.oninput = validate;
  const create = async () => {
    if (!validate()) return;
    createBtn.disabled = true;
    createBtn.classList.add("loading");
    try {
      const c = await api.createCharacter(name, cls, { ...choice });
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
    void preview?.show(cls, choiceLook(opts, cls, choice));
    setPlate(name || "Seu nome", classInfo(cls).label, choice.emblem);
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

// ---------------------------------------------------------------------------------------
// Conta: privacidade, termos e apagar a conta
// ---------------------------------------------------------------------------------------

/** Palavra que o jogador digita para apagar a conta (o servidor confere de novo). */
const DELETE_WORD = "APAGAR";

/** Janela da conta sobre a tela de personagens. Apagar é para sempre e exige digitar APAGAR. */
function openAccountDialog(displayName: string, characters: number, onDeleted: () => void): void {
  document.querySelector(".account-modal")?.remove();
  const scrim = el("div", "account-modal");
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") close();
  };
  const close = () => {
    scrim.remove();
    document.removeEventListener("keydown", onKey, true);
  };
  document.addEventListener("keydown", onKey, true);
  scrim.onclick = (e) => {
    if (e.target === scrim) close();
  };

  const x = button("✕", "btn btn-icon btn-ghost btn-sm account-x", close);
  x.setAttribute("aria-label", "Fechar");

  const confirm = el("input", "input account-confirm");
  confirm.id = "account-confirm";
  confirm.autocomplete = "off";
  confirm.spellcheck = false;
  confirm.placeholder = DELETE_WORD;
  const label = el("label", "label", `Digite ${DELETE_WORD} para confirmar`);
  label.htmlFor = confirm.id;
  const error = el("p", "account-error bad small");
  error.setAttribute("role", "alert");
  const del = button("", "btn btn-danger btn-block");
  del.innerHTML = `${ui("trash", 16)}<span>Apagar minha conta</span>`;
  del.disabled = true;
  const typed = () => confirm.value.trim() === DELETE_WORD;
  confirm.oninput = () => {
    del.disabled = !typed();
    error.textContent = "";
  };
  const run = async () => {
    if (!typed() || del.classList.contains("loading")) return;
    del.disabled = true;
    confirm.disabled = true;
    del.classList.add("loading");
    try {
      await api.deleteAccount(DELETE_WORD);
      forgetLocalData();
      close();
      showMessage(
        "Conta apagada",
        "Sua conta, os personagens, os itens e o armazém foram apagados para sempre. Obrigado por ter jogado Erevia.",
        "Voltar ao início",
        onDeleted,
      );
    } catch (err) {
      error.textContent = err instanceof ApiError ? err.message : "Não foi possível apagar a conta. Tente de novo.";
      confirm.disabled = false;
      del.disabled = !typed();
    } finally {
      del.classList.remove("loading");
    }
  };
  del.onclick = () => void run();
  confirm.onkeydown = (e) => {
    if (e.key === "Enter") void run();
  };

  const chars = characters === 1 ? "o seu personagem" : characters > 1 ? `os seus ${characters} personagens` : "";
  const box = el(
    "section",
    "account-dialog frame",
    x,
    el("h2", "title", "Conta"),
    displayName ? el("p", "muted small", `Conectado como ${displayName}`) : null,
    el("nav", "account-links", legalLink("privacidade"), legalLink("termos")),
    el(
      "div",
      "account-danger",
      el("h3", "title", "Apagar a conta"),
      el(
        "p",
        null,
        `Apaga para sempre a conta${chars ? `, ${chars}` : ""}, os itens, as moedas, o armazém e a ligação com o seu login do Google ou do Discord. Não dá para desfazer.`,
      ),
      el(
        "p",
        "small muted",
        "Se você lidera um clã, a liderança passa para o membro de cargo mais alto (ou o clã é desfeito, se ficar vazio). Quem estiver jogando com esta conta é desconectado. As cópias de segurança somem em até 27 dias.",
      ),
      label,
      confirm,
      error,
      del,
    ),
  );
  box.setAttribute("role", "dialog");
  box.setAttribute("aria-modal", "true");
  box.setAttribute("aria-label", "Conta");
  scrim.append(box);
  document.body.append(scrim);
  if (!isCompact()) confirm.focus({ preventScroll: true });
}

/** Apaga o que o jogo guardou neste navegador (preferências e barras antigas, "erevia.*"). */
function forgetLocalData(): void {
  try {
    for (const key of Object.keys(localStorage)) if (key.startsWith("erevia.")) localStorage.removeItem(key);
  } catch {
    // Navegador sem localStorage (modo privado restrito): nada a apagar.
  }
}
