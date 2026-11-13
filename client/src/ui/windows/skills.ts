/**
 * Skills (tecla K): as skills da classe com a descrição completa. Arraste para a barra de
 * atalhos (ou toque e escolha "Pôr na barra"). A árvore com ramos e ranks chega na Fase B.
 */

import type { SkillInfo } from "../../gen/erevia/v1/game_pb";
import { uniqueIds } from "../art";
import { beginDrag } from "../dragdrop";
import { skillTipHtml } from "../hud";
import { skillIcon } from "../icons";
import { art, button, el, isTouch } from "../kit";

export interface SkillsDeps {
  skills: SkillInfo[];
  /** Põe a skill num atalho (índice opcional = primeiro livre). */
  toHotbar(skillId: number, index?: number): void;
}

export function buildSkills(d: SkillsDeps): HTMLElement {
  const root = el("div", "skills-win");
  const list = el("div", "skill-list");
  for (const s of [...d.skills].sort((a, b) => a.key - b.key)) {
    const icon = art(uniqueIds(skillIcon(s.fx)), "sk-icon");
    const body = el("div", "sk-body");
    body.innerHTML = skillTipHtml(s);
    const add = button("Pôr na barra", "btn btn-sm", () => d.toHotbar(s.id));
    const card = el("article", "sk-card frame", icon, body, add);
    icon.title = isTouch() ? "" : "Arraste para a barra de atalhos";
    icon.addEventListener("pointerdown", (e) => {
      beginDrag(e, {
        icon: uniqueIds(skillIcon(s.fx)),
        onDrop(t) {
          const hb = t?.closest<HTMLElement>("[data-hotbar]");
          if (hb) d.toHotbar(s.id, Number(hb.dataset.hotbar));
        },
      });
    });
    list.append(card);
  }
  root.append(
    list,
    el("p", "hint-box small", "Em breve: árvore de skills com 3 ramos por classe, ranks e skills novas a cada 5 níveis."),
  );
  return root;
}
