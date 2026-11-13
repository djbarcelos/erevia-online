/** Tela de login: logo, entrar com Google/Discord/dev, escolha de servidor e novidades. */

import "../../styles/screens.css";
import { ui } from "../../ui/art";
import { NEWS, SERVERS } from "../data/mock";
import { button, el, toast } from "../../ui/kit";
import type { Ctx, Screen } from "../main";
import { animatedBackdrop } from "../../ui/backdrop";

export function mount(root: HTMLElement, ctx: Ctx): Screen {
  root.classList.add("login-screen");
  const stopBg = animatedBackdrop(root);

  const content = el("div", "login-layout scroll");

  // Logo
  const logo = el(
    "header",
    "login-logo",
    el("div", "logo-crest", el("span")),
    el("h1", "logo-word", "EREVIA"),
    el("p", "logo-tag", "As terras partidas esperam por você"),
  );

  // Cartão de entrada
  const card = el("section", "login-card frame");
  card.setAttribute("aria-label", "Entrar");
  const google = el("a", "btn btn-block btn-social google");
  google.href = "#personagem";
  google.innerHTML = `${ui("google", 20)}<span>Entrar com Google</span>`;
  const discord = el("a", "btn btn-block btn-social discord");
  discord.href = "#personagem";
  discord.innerHTML = `${ui("discord", 20)}<span>Entrar com Discord</span>`;
  for (const a of [google, discord]) {
    a.addEventListener("click", (ev) => {
      ev.preventDefault();
      enter(a);
    });
  }

  const form = el("form", "dev-login");
  const user = el("input", "input");
  user.placeholder = "Nome de usuário";
  user.minLength = 3;
  user.maxLength = 16;
  user.pattern = "[A-Za-z0-9]{3,16}";
  user.required = true;
  user.autocomplete = "off";
  user.setAttribute("aria-label", "Usuário de desenvolvimento");
  const devBtn = button("Entrar", "btn btn-gold");
  devBtn.type = "submit";
  form.append(user, devBtn);
  form.onsubmit = (ev) => {
    ev.preventDefault();
    if (!user.checkValidity()) {
      user.classList.add("invalid");
      toast("Use de 3 a 16 letras ou números.", "bad");
      return;
    }
    enter(devBtn);
  };
  user.oninput = () => user.classList.remove("invalid");

  // Servidores
  let chosen = SERVERS[0].id;
  const servers = el("div", "server-list");
  servers.setAttribute("role", "radiogroup");
  servers.setAttribute("aria-label", "Servidor");
  for (const s of SERVERS) {
    const b = el("button", "server");
    b.type = "button";
    b.setAttribute("role", "radio");
    b.setAttribute("aria-checked", String(s.id === chosen));
    const pop = el("span", `pop p${s.popLevel}`, el("i"), el("i"), el("i"));
    pop.title = `População ${s.pop.toLowerCase()}`;
    const ping = el("span", `ping ${s.ping < 60 ? "good" : s.ping < 120 ? "mid" : "bad"}`, `${s.ping} ms`);
    b.append(
      el("span", "srv-dot"),
      el("span", "srv-name", s.name, s.tag ? el("em", null, s.tag) : null),
      el("span", "srv-region", s.region),
      pop,
      ping,
    );
    b.onclick = () => {
      chosen = s.id;
      servers.querySelectorAll(".server").forEach((x) => x.setAttribute("aria-checked", String(x === b)));
    };
    servers.append(b);
  }

  card.append(
    el("h2", "title card-title", "Entrar"),
    el("div", "stack", google, discord),
    el("div", "divider", "ou"),
    el("label", "label", "Login de desenvolvimento"),
    form,
    el("div", "divider", "◆"),
    el("label", "label", "Servidor"),
    servers,
  );

  // Novidades
  const news = el("aside", "login-news frame glass");
  news.append(el("h2", "title", "Novidades"));
  for (const n of NEWS) {
    news.append(
      el(
        "article",
        "news-item",
        el("div", "news-meta", el("span", `news-tag t-${n.tag.toLowerCase()}`, n.tag), el("time", null, n.date)),
        el("h3", null, n.title),
        el("p", null, n.text),
      ),
    );
  }

  const footer = el(
    "footer",
    "login-foot",
    el("span", null, "v0.3.0 · protótipo"),
    el("span", null, "Modelos 3D: KayKit (CC0)"),
    el("span", null, "© 2026 Erevia"),
  );

  content.append(el("div", "login-main", logo, card), news, footer);
  root.append(content);

  function enter(btn: HTMLElement) {
    btn.classList.add("loading");
    const name = SERVERS.find((s) => s.id === chosen)!.name;
    toast(`Conectando a ${name}…`);
    setTimeout(() => ctx.go("personagem"), 650);
  }

  return { dispose: stopBg };
}
