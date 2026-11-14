/**
 * Grupo (tecla P): membros com vida e mana, quem é o líder, e as ações — convidar pelo
 * nome, expulsar e passar a liderança (só o líder) e sair do grupo.
 */

import { classInfo } from "../../classes";
import type { PartyState } from "../../gen/erevia/v1/game_pb";
import { bar, button, el } from "../kit";

export interface PartyDeps {
  localId: number;
  state(): PartyState;
  onChange(fn: () => void): () => void;
  invite(name: string): void;
  kick(id: number): void;
  promote(id: number): void;
  leave(): void;
  select(id: number): void;
}

export function buildParty(d: PartyDeps): HTMLElement {
  const root = el("div", "party-win");
  const list = el("div", "pw-list");
  const empty = el("p", "muted pw-empty", "Você não está em um grupo. Convide alguém pelo nome, pelo botão \"Grupo +\" no quadro do alvo ou com /convidar Nome.");
  const name = el("input", "input");
  name.placeholder = "Nome do jogador";
  name.maxLength = 16;
  name.setAttribute("aria-label", "Nome do jogador para convidar");
  const send = () => {
    const v = name.value.trim();
    if (!v) return;
    d.invite(v);
    name.value = "";
  };
  name.onkeydown = (ev) => {
    ev.stopPropagation(); // letras aqui não abrem janelas
    if (ev.key === "Enter") send();
  };
  const inviteRow = el("div", "pw-invite", name, button("Convidar", "btn btn-gold btn-sm", send));
  const leave = button("Sair do grupo", "btn btn-danger btn-sm", () => d.leave());
  const rules = el(
    "p",
    "small muted pw-rules",
    "Até 6 pessoas. A XP dos monstros é dividida entre quem está perto (até 30 m), pelo nível de cada um, com bônus de 10% por pessoa a mais. Moedas pegas por um membro são divididas com os que estão perto, e o saque reservado vale para o grupo inteiro.",
  );
  root.append(list, empty, inviteRow, el("div", "pw-foot", leave), rules);

  const render = () => {
    const st = d.state();
    const amLeader = st.leaderId === d.localId;
    const solo = st.members.length === 0;
    empty.hidden = !solo;
    leave.hidden = solo;
    inviteRow.hidden = !solo && !amLeader;
    list.replaceChildren();
    for (const m of st.members) {
      const info = classInfo(m.class);
      const hp = bar("hp", "thin");
      hp.set(m.hp, m.maxHp, "");
      const mp = bar("mp", "thin");
      mp.set(m.mp, m.maxMp, "");
      const isMe = m.id === d.localId;
      const title = el(
        "div",
        "pw-name",
        m.id === st.leaderId ? el("span", "m-lead", "♛") : "",
        el("b", null, m.name),
        el("small", null, ` ${info.label} · Nv ${m.level}${isMe ? " · você" : ""}${m.dead ? " · morto" : ""}${m.map ? ` · em ${m.map}` : ""}`),
      );
      const acts = el("div", "pw-acts");
      if (amLeader && !isMe) {
        acts.append(
          button("Líder", "btn btn-sm", () => d.promote(m.id)),
          button("Expulsar", "btn btn-sm btn-danger", () => d.kick(m.id)),
        );
      }
      const row = el("div", "pw-row", el("span", "m-crest", info.label[0]), el("div", "pw-body", title, hp.root, mp.root), acts);
      row.style.setProperty("--cc", info.color);
      if (!isMe) {
        row.classList.add("clickable");
        row.onclick = (ev) => {
          if ((ev.target as HTMLElement).closest("button")) return;
          d.select(m.id);
        };
      }
      list.append(row);
    }
  };
  render();
  const off = d.onChange(render);
  root.addEventListener("win-close", off);
  return root;
}
