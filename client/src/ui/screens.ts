import { api, ApiError, type CharacterInfo, type ClassId, type Providers } from "../api";
import { CLASSES, classInfo } from "../classes";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

const SCREENS = ["screen-loading", "screen-login", "screen-chars", "screen-message"] as const;
type Screen = (typeof SCREENS)[number];

export function showScreen(screen: Screen | null): void {
  $("overlay").hidden = screen === null;
  for (const s of SCREENS) $(s).hidden = s !== screen;
}

/** Mensagens amigáveis para os códigos de erro do callback OAuth. */
const LOGIN_ERRORS: Record<string, string> = {
  cancelado: "Login cancelado.",
  sessao_expirada: "A tentativa de login expirou. Tente de novo.",
  state_invalido: "Não foi possível confirmar o login. Tente de novo.",
  troca_falhou: "O provedor recusou o login. Tente de novo.",
  perfil_falhou: "Não foi possível ler seu perfil no provedor.",
  erro_interno: "Erro no servidor. Tente de novo.",
};

export function showLogin(providers: Providers, loginError: string | null, onLoggedIn: () => void): void {
  $("login-google").hidden = !providers.google;
  $("login-discord").hidden = !providers.discord;
  $("no-login").hidden = providers.google || providers.discord || providers.dev;
  $("login-error").textContent = loginError ? (LOGIN_ERRORS[loginError] ?? "Falha no login.") : "";

  const form = $<HTMLFormElement>("dev-login");
  form.hidden = !providers.dev;
  form.onsubmit = async (ev) => {
    ev.preventDefault();
    const btn = form.querySelector("button")!;
    btn.disabled = true;
    try {
      await api.devLogin($<HTMLInputElement>("dev-user").value.trim());
      onLoggedIn();
    } catch (err) {
      $("login-error").textContent = (err as Error).message;
    } finally {
      btn.disabled = false;
    }
  };
  showScreen("screen-login");
}

export async function showCharacters(onPlay: (c: CharacterInfo) => void, onLogout: () => void): Promise<void> {
  const errorEl = $("chars-error");
  errorEl.textContent = "";

  const [me, data] = await Promise.all([api.me(), api.characters()]);
  $("account-name").textContent = me.displayName ? `Conectado como ${me.displayName}` : "";

  const render = (chars: CharacterInfo[], max: number) => {
    const list = $<HTMLUListElement>("char-list");
    list.replaceChildren(
      ...chars.map((c) => {
        const info = classInfo(c.class);
        const li = document.createElement("li");
        const dot = document.createElement("span");
        dot.className = "dot";
        dot.style.background = info.color;
        const text = document.createElement("div");
        text.className = "info";
        const name = document.createElement("div");
        name.className = "name";
        name.textContent = c.name;
        const cls = document.createElement("div");
        cls.className = "cls";
        cls.textContent = info.label;
        text.append(name, cls);
        const play = document.createElement("button");
        play.type = "button";
        play.className = "btn btn-gold";
        play.textContent = "Jogar";
        play.onclick = () => onPlay(c);
        li.append(dot, text, play);
        return li;
      }),
    );
    $("char-empty").hidden = chars.length > 0;
    $("char-create").hidden = chars.length >= max;
  };

  // Cartões de classe (montados uma vez).
  const picker = $<HTMLFieldSetElement>("class-picker");
  if (!picker.querySelector(".class-card")) {
    for (const [i, c] of CLASSES.entries()) {
      const label = document.createElement("label");
      label.className = "class-card";
      const radio = document.createElement("input");
      radio.type = "radio";
      radio.name = "class";
      radio.value = c.id;
      radio.checked = i === 0;
      const title = document.createElement("span");
      title.className = "title";
      const dot = document.createElement("span");
      dot.style.cssText = `width:10px;height:10px;border-radius:50%;background:${c.color}`;
      title.append(dot, c.label);
      const desc = document.createElement("span");
      desc.className = "desc";
      desc.textContent = c.desc;
      label.append(radio, title, desc);
      picker.append(label);
    }
  }

  let chars = data.characters;
  render(chars, data.max);

  const form = $<HTMLFormElement>("char-create");
  form.onsubmit = async (ev) => {
    ev.preventDefault();
    const btn = form.querySelector<HTMLButtonElement>("button[type=submit]")!;
    const nameInput = $<HTMLInputElement>("char-name");
    const cls = (form.querySelector<HTMLInputElement>("input[name=class]:checked")?.value ?? "knight") as ClassId;
    btn.disabled = true;
    errorEl.textContent = "";
    try {
      const created = await api.createCharacter(nameInput.value.trim(), cls);
      chars = [...chars, created];
      nameInput.value = "";
      render(chars, data.max);
    } catch (err) {
      errorEl.textContent = err instanceof ApiError ? err.message : "Erro ao criar personagem.";
    } finally {
      btn.disabled = false;
    }
  };

  $("logout").onclick = async () => {
    await api.logout().catch(() => undefined);
    onLogout();
  };

  showScreen("screen-chars");
}

export function showMessage(title: string, text: string, button: string, onClick: () => void): void {
  $("message-title").textContent = title;
  $("message-text").textContent = text;
  const btn = $<HTMLButtonElement>("message-btn");
  btn.textContent = button;
  btn.onclick = onClick;
  showScreen("screen-message");
}
