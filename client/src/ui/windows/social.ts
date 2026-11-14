/**
 * Social (tecla O): amigos e correio.
 * - Amigos: quem você segue (mão única), online primeiro, com a região onde está; adicionar
 *   pelo nome, sussurrar, escrever uma carta e remover.
 * - Correio: escrever (para quem estiver offline também), caixa de entrada com marcar como
 *   lida (ao abrir a carta) e apagar.
 * Só redesenha quando chega mensagem do servidor (ou o jogador clica): nada roda por quadro.
 * Os formulários são criados uma vez, então o texto digitado não se perde ao redesenhar.
 */

import { classInfo } from "../../classes";
import type { FriendList, Mail, MailBox } from "../../gen/erevia/v1/game_pb";
import { button, el, toast } from "../kit";

export type SocialTab = "friends" | "mail";

export interface SocialDeps {
  /** null = a lista ainda não chegou do servidor. */
  friends(): FriendList | null;
  /** Contadores do correio (null = ainda não chegaram). */
  mailBox(): MailBox | null;
  /** Cartas da última caixa listada (null = ainda não listou). */
  mails(): Mail[] | null;
  onChange(fn: () => void): () => void;
  regionName(x: number, z: number): string;
  tab: SocialTab;
  /** Aba escolhida (para reabrir na mesma). */
  onTab(tab: SocialTab): void;
  addFriend(name: string): void;
  removeFriend(id: number): void;
  /** Pede a lista de amigos com as posições de agora. */
  refreshFriends(): void;
  whisper(name: string): void;
  listMail(): void;
  readMail(id: bigint): void;
  deleteMail(id: bigint): void;
  sendMail(to: string, subject: string, body: string): void;
  /** Casa de leilões: retirar o anexo da carta para a bolsa. */
  claimMail(id: bigint): void;
  /** Nome do item (anexos), com a quantidade e o +N. */
  itemLabel(id: string, count: number, enchant: number): string;
}

/** Enquanto a janela está aberta, a posição dos amigos é atualizada a cada tanto (ms). */
const FRIENDS_REFRESH_MS = 10_000;
const NAME_RE = /^[A-Za-z0-9]{3,16}$/;

/** Botão que pede um segundo clique para confirmar. */
function confirmButton(label: string, confirmLabel: string, cls: string, run: () => void): HTMLButtonElement {
  let armed = 0;
  const b = button(label, cls, () => {
    if (armed) {
      window.clearTimeout(armed);
      armed = 0;
      b.textContent = label;
      run();
      return;
    }
    b.textContent = confirmLabel;
    armed = window.setTimeout(() => {
      armed = 0;
      b.textContent = label;
    }, 3000);
  });
  return b;
}

const dateFmt = new Intl.DateTimeFormat("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });

/** Letras digitadas nos campos não abrem janelas; Enter envia (no texto da carta, Ctrl+Enter). */
function stopKeys(input: HTMLInputElement | HTMLTextAreaElement, enter?: () => void): void {
  input.onkeydown = (ev) => {
    ev.stopPropagation();
    if (ev.key !== "Enter" || !enter) return;
    if (input instanceof HTMLTextAreaElement && !ev.ctrlKey && !ev.metaKey) return;
    ev.preventDefault();
    enter();
  };
}

export function buildSocial(d: SocialDeps): HTMLElement {
  const root = el("div", "social-win");
  let tab: SocialTab = d.tab;
  /** Carta aberta na caixa de entrada. */
  let openMail: bigint | null = null;

  // ---------- Abas ----------
  const tabsRow = el("div", "tabs");
  tabsRow.setAttribute("role", "tablist");
  const tabBtn = (id: SocialTab) => {
    const b = el("button", "tab");
    b.type = "button";
    b.setAttribute("role", "tab");
    b.onclick = () => showTab(id);
    tabsRow.append(b);
    return b;
  };
  const friendsTab = tabBtn("friends");
  const mailTab = tabBtn("mail");

  // ---------- Amigos ----------
  const friendsPane = el("div", "sw-pane");
  const addName = el("input", "input");
  addName.placeholder = "Nome do personagem";
  addName.maxLength = 16;
  addName.setAttribute("aria-label", "Nome do amigo para adicionar");
  const add = () => {
    const v = addName.value.trim();
    if (!NAME_RE.test(v)) return toast("Use o nome do personagem (3 a 16 letras ou números).", "warn");
    d.addFriend(v);
    addName.value = "";
  };
  stopKeys(addName, add);
  const friendList = el("div", "sw-list");
  const friendsFoot = el("p", "small muted sw-foot");
  friendsPane.append(el("div", "sw-row-form", addName, button("Adicionar", "btn btn-gold btn-sm", add)), friendList, friendsFoot);

  // ---------- Correio ----------
  const mailPane = el("div", "sw-pane");
  const to = el("input", "input");
  to.placeholder = "Para (nome)";
  to.maxLength = 16;
  to.setAttribute("aria-label", "Destinatário da carta");
  const subject = el("input", "input");
  subject.placeholder = "Assunto (opcional)";
  subject.setAttribute("aria-label", "Assunto da carta");
  const body = el("textarea", "input sw-body-input");
  body.rows = 3;
  body.placeholder = "Escreva a carta (o destinatário pode estar offline)";
  body.setAttribute("aria-label", "Texto da carta");
  const counter = el("span", "small muted");
  const send = () => {
    const name = to.value.trim();
    const text = body.value.trim();
    if (!NAME_RE.test(name)) return toast("Escreva o nome de quem vai receber.", "warn");
    if (!text) return toast("A carta está vazia.", "warn");
    d.sendMail(name, subject.value.trim(), text);
    subject.value = "";
    body.value = "";
    updateCounter();
  };
  const updateCounter = () => {
    const max = d.mailBox()?.bodyMax || 200;
    counter.textContent = `${body.value.length}/${max}`;
  };
  body.oninput = updateCounter;
  stopKeys(to);
  stopKeys(subject);
  stopKeys(body, send);
  const compose = el(
    "div",
    "sw-compose",
    el("div", "sw-row-form", to, subject),
    body,
    el("div", "sw-compose-foot", counter, button("Enviar carta", "btn btn-gold btn-sm", send)),
  );
  const mailList = el("div", "sw-list sw-mail");
  const mailFoot = el("p", "small muted sw-foot");
  mailPane.append(compose, mailList, mailFoot);

  root.append(tabsRow, friendsPane, mailPane);

  // ---------- Desenho ----------
  const renderFriends = () => {
    const list = d.friends();
    const online = list?.friends.filter((f) => f.online).length ?? 0;
    friendsTab.textContent = "Amigos";
    friendsTab.append(el("span", "count", list ? `${online}/${list.friends.length}` : ""));
    friendList.replaceChildren();
    if (!list) {
      friendList.append(el("p", "muted", "Carregando…"));
      return;
    }
    if (!list.friends.length) {
      friendList.append(el("p", "muted", "Nenhum amigo ainda. Adicione pelo nome, pelo botão \"Amigo +\" no quadro do alvo ou com /amigo Nome."));
    }
    for (const f of list.friends) {
      const info = classInfo(f.class);
      const where = f.online ? d.regionName(f.x, f.z) : "offline";
      const acts = el("div", "pw-acts");
      if (f.online) acts.append(button("Sussurrar", "btn btn-sm", () => d.whisper(f.name)));
      acts.append(
        button("Carta", "btn btn-sm", () => {
          to.value = f.name;
          showTab("mail");
          body.focus();
        }),
        confirmButton("Remover", "Confirmar?", "btn btn-sm btn-danger", () => d.removeFriend(f.characterId)),
      );
      const row = el(
        "div",
        `cl-row${f.online ? " online" : ""}`,
        el("span", "m-crest", info.label[0]),
        el("div", "cl-body", el("b", null, f.name), el("small", null, `${info.label} · Nv ${f.level}`)),
        el("span", "cl-dot sw-where", where),
        acts,
      );
      row.style.setProperty("--cc", info.color);
      friendList.append(row);
    }
    friendsFoot.textContent = `${list.friends.length}/${list.max} amigos. A amizade é de mão única: quem você adiciona não é avisado. Você é avisado quando um amigo entra no jogo.`;
  };

  const renderMail = () => {
    const box = d.mailBox();
    const mails = d.mails();
    mailTab.textContent = "Correio";
    if (box?.unread) mailTab.append(el("span", "count sw-unread", String(box.unread)));
    subject.maxLength = box?.subjectMax || 40;
    body.maxLength = box?.bodyMax || 200;
    updateCounter();
    mailList.replaceChildren();
    if (!mails) {
      mailList.append(el("p", "muted", "Carregando…"));
    } else if (!mails.length) {
      mailList.append(el("p", "muted", "Sua caixa de entrada está vazia."));
    }
    for (const m of mails ?? []) {
      const opened = openMail === m.id;
      const head = el(
        "button",
        "sw-mail-head",
        el("span", "sw-dot", m.read ? "" : "●"),
        el("b", null, m.fromName),
        el("span", "sw-subj", m.attachment && !m.attachment.claimed ? el("span", "sw-clip", "[anexo] ") : "", m.subject || "(sem assunto)"),
        el("small", "muted", dateFmt.format(new Date(Number(m.sentAtMs)))),
      );
      head.type = "button";
      head.setAttribute("aria-expanded", String(opened));
      head.onclick = () => {
        openMail = opened ? null : m.id;
        if (!opened && !m.read) d.readMail(m.id);
        renderMail();
      };
      const item = el("div", `sw-mail-item${m.read ? "" : " unread"}`, head);
      if (opened) {
        // Anexo da casa de leilões: retirar põe na bolsa; antes disso a carta não se apaga.
        const a = m.attachment;
        const parts = a ? [a.itemId ? d.itemLabel(a.itemId, a.count, a.enchant) : "", a.gold ? `${a.gold} moedas` : ""].filter(Boolean) : [];
        const locked = !!a && !a.claimed;
        const del = confirmButton("Apagar", "Confirmar?", "btn btn-sm btn-danger", () => {
          openMail = null;
          d.deleteMail(m.id);
        });
        del.disabled = locked;
        if (locked) del.title = "Retire o anexo antes de apagar a carta";
        item.append(
          el("p", "sw-mail-body", m.body),
          a
            ? el(
                "div",
                `sw-attach${a.claimed ? " claimed" : ""}`,
                el("span", "muted", "Anexo:"),
                el("b", null, parts.join(" e ")),
                a.claimed ? el("span", "small muted", "retirado") : button("Retirar", "btn btn-gold btn-sm", () => d.claimMail(m.id)),
              )
            : "",
          el(
            "div",
            "pw-acts sw-mail-acts",
            m.fromName === "Casa de Leilões"
              ? ""
              : button("Responder", "btn btn-sm", () => {
                  to.value = m.fromName;
                  if (m.subject) subject.value = m.subject.startsWith("Re: ") ? m.subject : `Re: ${m.subject}`.slice(0, subject.maxLength);
                  body.focus();
                }),
            del,
          ),
        );
      }
      mailList.append(item);
    }
    mailFoot.textContent = box ? `${box.total}/${box.max} cartas guardadas. Até 10 cartas por hora. As cartas da Casa de Leilões trazem anexos (moedas ou itens) para retirar.` : "";
  };

  const showTab = (id: SocialTab) => {
    tab = id;
    d.onTab(id);
    friendsTab.setAttribute("aria-selected", String(id === "friends"));
    mailTab.setAttribute("aria-selected", String(id === "mail"));
    friendsPane.hidden = id !== "friends";
    mailPane.hidden = id !== "mail";
  };

  renderFriends();
  renderMail();
  showTab(tab);
  d.refreshFriends();
  d.listMail();
  const timer = window.setInterval(() => d.refreshFriends(), FRIENDS_REFRESH_MS);
  const off = d.onChange(() => {
    renderFriends();
    renderMail();
  });
  root.addEventListener("win-close", () => {
    off();
    window.clearInterval(timer);
  });
  return root;
}
