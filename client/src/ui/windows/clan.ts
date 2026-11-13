/**
 * Clã (tecla N). Sem clã: criar (nível e moedas) e quanto falta para poder entrar em
 * outro. Com clã: aviso (líder e oficiais editam), membros com cargo, classe, nível e
 * online, e as ações de cada cargo — promover/rebaixar e passar a liderança (líder),
 * expulsar (quem tem cargo maior), sair e desfazer.
 */

import { classInfo } from "../../classes";
import { ClanRank, type ClanState } from "../../gen/erevia/v1/game_pb";
import { button, el, fmt, toast } from "../kit";

export interface ClanDeps {
  localId: number;
  state(): ClanState;
  onChange(fn: () => void): () => void;
  gold(): number;
  level(): number;
  create(name: string): void;
  invite(name: string): void;
  kick(id: number): void;
  setRank(id: number, rank: ClanRank): void;
  promoteLeader(id: number): void;
  notice(text: string): void;
  leave(): void;
  disband(): void;
}

const RANK_NAME: Record<number, string> = { [ClanRank.LEADER]: "Líder", [ClanRank.OFFICER]: "Oficial", [ClanRank.MEMBER]: "Membro" };

/** Botão que pede um segundo clique para confirmar (ações sem volta). */
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

const hours = (ms: number) => {
  const h = Math.floor(ms / 3_600_000);
  const m = Math.ceil((ms % 3_600_000) / 60_000);
  return h > 0 ? `${h} h ${m} min` : `${m} min`;
};

export function buildClan(d: ClanDeps): HTMLElement {
  const root = el("div", "clan-win");
  const stopKeys = (input: HTMLInputElement | HTMLTextAreaElement, enter?: () => void) => {
    input.onkeydown = (ev) => {
      ev.stopPropagation(); // letras aqui não abrem janelas
      if (ev.key === "Enter" && enter && !(ev.shiftKey && input instanceof HTMLTextAreaElement)) {
        ev.preventDefault();
        enter();
      }
    };
  };
  let openedAt = performance.now();

  const render = () => {
    const st = d.state();
    // Quem está digitando (aviso, nome) não perde o texto quando a lista atualiza.
    const active = document.activeElement as HTMLInputElement | HTMLTextAreaElement | null;
    const typing = active && root.contains(active) && "value" in active ? { label: active.getAttribute("aria-label") ?? active.placeholder, value: active.value } : null;
    root.replaceChildren();
    queueMicrotask(() => {
      if (!typing) return;
      const again = [...root.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>("input, textarea")].find(
        (x) => (x.getAttribute("aria-label") ?? x.placeholder) === typing.label,
      );
      if (again) {
        again.value = typing.value;
        again.focus();
      }
    });
    if (!st.id) {
      // ---------- Sem clã ----------
      const name = el("input", "input");
      name.placeholder = "Nome do clã (3 a 16 letras ou números)";
      name.maxLength = 16;
      name.setAttribute("aria-label", "Nome do clã");
      const okLevel = d.level() >= st.createLevel;
      const okGold = d.gold() >= st.createCost;
      const create = button("Criar clã", "btn btn-gold", () => {
        const v = name.value.trim();
        if (!/^[A-Za-z0-9]{3,16}$/.test(v)) return toast("Use de 3 a 16 letras ou números.", "warn");
        d.create(v);
      });
      create.disabled = !okLevel || !okGold;
      stopKeys(name, () => create.click());
      const wait = st.joinWaitMs - (performance.now() - openedAt);
      root.append(
        el("h3", "title", "Você não está em um clã"),
        el(
          "p",
          "muted",
          "Membros do clã são aliados: não se acertam em PvP e recebem os buffs de grupo uns dos outros. O nome do clã aparece embaixo do seu nome.",
        ),
        el(
          "div",
          "clan-req",
          el("span", okLevel ? "ok" : "bad", `Nível ${st.createLevel} ou mais`),
          el("span", okGold ? "ok" : "bad", `${fmt.format(st.createCost)} moedas`),
        ),
        el("div", "clan-create", name, create),
        el("p", "small muted", "Para entrar no clã de alguém, peça um convite a um líder ou oficial."),
        wait > 0 ? el("p", "small bad", `Você saiu de um clã há pouco: pode entrar em outro em ${hours(wait)}.`) : "",
      );
      return;
    }

    // ---------- Com clã ----------
    const me = st.myRank;
    const online = st.members.filter((m) => m.online).length;
    root.append(
      el(
        "div",
        "clan-head",
        el("h3", "title clan-name", `‹${st.name}›`),
        el("span", "muted", `${st.members.length}/${st.maxMembers} membros · ${online} online · você é ${RANK_NAME[me]}`),
      ),
    );

    // Aviso
    const notice = el("div", "clan-notice");
    if (me >= ClanRank.OFFICER) {
      const ta = el("textarea", "input clan-notice-edit");
      ta.maxLength = 200;
      ta.rows = 2;
      ta.value = st.notice;
      ta.placeholder = "Aviso para o clã (todos veem)";
      ta.setAttribute("aria-label", "Aviso do clã");
      const save = button("Salvar aviso", "btn btn-sm", () => d.notice(ta.value));
      stopKeys(ta, () => save.click());
      notice.append(ta, save);
    } else notice.append(el("p", st.notice ? "" : "muted", st.notice || "Sem aviso."));
    root.append(notice);

    // Membros
    const list = el("div", "clan-list");
    for (const m of st.members) {
      const info = classInfo(m.class);
      const isMe = m.characterId === d.localId;
      const acts = el("div", "cl-acts");
      if (!isMe && me === ClanRank.LEADER) {
        if (m.rank === ClanRank.MEMBER) acts.append(button("Promover", "btn btn-sm", () => d.setRank(m.characterId, ClanRank.OFFICER)));
        if (m.rank === ClanRank.OFFICER) acts.append(button("Rebaixar", "btn btn-sm", () => d.setRank(m.characterId, ClanRank.MEMBER)));
        acts.append(confirmButton("Líder", "Confirmar?", "btn btn-sm", () => d.promoteLeader(m.characterId)));
      }
      if (!isMe && me >= ClanRank.OFFICER && m.rank < me) acts.append(confirmButton("Expulsar", "Confirmar?", "btn btn-sm btn-danger", () => d.kick(m.characterId)));
      const row = el(
        "div",
        `cl-row${m.online ? " online" : ""}`,
        el("span", "m-crest", info.label[0]),
        el(
          "div",
          "cl-body",
          el("b", null, m.name),
          el("small", null, `${RANK_NAME[m.rank]} · ${info.label} · Nv ${m.level}${isMe ? " · você" : ""}`),
        ),
        el("span", "cl-dot", m.online ? "online" : "offline"),
        acts,
      );
      row.style.setProperty("--cc", info.color);
      list.append(row);
    }
    root.append(list);

    // Convidar
    if (me >= ClanRank.OFFICER) {
      const name = el("input", "input");
      name.placeholder = "Convidar pelo nome";
      name.maxLength = 16;
      name.setAttribute("aria-label", "Nome do jogador para convidar ao clã");
      const send = () => {
        const v = name.value.trim();
        if (v) d.invite(v);
        name.value = "";
      };
      stopKeys(name, send);
      root.append(el("div", "clan-invite", name, button("Convidar", "btn btn-gold btn-sm", send)));
    }

    const foot = el("div", "clan-foot");
    if (me === ClanRank.LEADER) foot.append(confirmButton("Desfazer clã", "Clique de novo para desfazer", "btn btn-danger btn-sm", d.disband));
    if (me !== ClanRank.LEADER || st.members.length === 1) foot.append(confirmButton("Sair do clã", "Confirmar saída?", "btn btn-sm", d.leave));
    root.append(foot, el("p", "small muted", "Quem sai ou é expulso espera 24 h para entrar em outro clã. Chat do clã: /c mensagem."));
  };

  render();
  const off = d.onChange(() => {
    openedAt = performance.now();
    render();
  });
  root.addEventListener("win-close", off);
  return root;
}
