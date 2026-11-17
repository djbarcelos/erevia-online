/**
 * Portal → Operação: anúncios (na hora ou agendados), aviso de manutenção, eventos
 * (invasão e XP em dobro, com a agenda dos automáticos) e os chefes. Tudo vai pela zona
 * e fica no gm_log; a página se atualiza a cada poucos segundos.
 */

import "./actions.css";
import { call } from "./api";
import { badge, button, card, errorText, field, fmtClock, fmtDateTime, fmtDuration, fmtRelative, h, nf, openDialog, table, toast, type ActionResult } from "./widgets";

interface Scheduled {
  id: number;
  at: string;
  title: string;
  text: string;
  by: string;
}

interface Boss {
  id: number;
  def: string;
  name: string;
  level: number;
  raid: boolean;
  alive: boolean;
  hpPct: number;
  phase: number;
  enraged: boolean;
  final: boolean;
  fighting: boolean;
  fightStart: string | null;
  windowStart: string | null;
  windowEnd: string | null;
  respawnAt: string | null;
  killedAt: string | null;
}

interface Ops {
  now: string;
  online: number;
  night: boolean;
  nextNight: string;
  nextDay: string;
  xp2: { on: boolean; until: string | null; gmUntil: string | null; offUntil: string | null; days: string[]; nextStart: string | null; nextEnd: string | null };
  invasion: { phase: "none" | "warning" | "active"; reason: string; startsAt: string | null; endsAt: string | null; wave: number; waves: number; alive: number; killed: number; chance: number };
  bosses: Boss[];
  scheduled: Scheduled[];
  maintenance: { at: string; message: string; by: string } | null;
}

const REFRESH_MS = 5000;

export function mountOps(main: HTMLElement): () => void {
  const updated = h("span", { class: "adm-updated" });
  const refresh = h("button", { class: "adm-btn", type: "button" }, "Atualizar");
  const error = h("p", { class: "adm-error", role: "alert", hidden: "" });
  const grid = h("div", { class: "adm-ops", "aria-busy": "true" }, h("div", { class: "adm-card adm-box is-loading" }));
  main.append(h("header", { class: "adm-head" }, h("h1", {}, "Operação"), h("div", { class: "adm-head-tools" }, updated, refresh)), error, grid);

  let alive = true;
  let last: Ops | null = null;
  let skew = 0; // relógio do servidor − o daqui (para as contagens)
  const ticking: { el: HTMLElement; at: string; prefix: string }[] = [];
  // Campos criados uma vez: a página se redesenha a cada 5 s sem apagar o que o GM digita.
  const title = h("input", { type: "text", maxlength: "60", placeholder: "Título (opcional): Anúncio", "aria-label": "Título do anúncio" });
  const text = h("textarea", { rows: "3", maxlength: "300", placeholder: "O texto aparece no banner e no chat de todos os jogadores.", "aria-label": "Texto do anúncio" });
  const when = h("input", { type: "datetime-local", "aria-label": "Agendar para" });
  const hours = h("input", { type: "number", min: "0.5", max: "168", step: "0.5", value: "2", "aria-label": "Horas de XP em dobro", inputmode: "decimal" });

  const load = async () => {
    refresh.disabled = true;
    try {
      const ops = await call<Ops>("GET", "/api/admin/ops");
      if (!alive) return;
      skew = new Date(ops.now).getTime() - Date.now();
      last = ops;
      paint(ops);
      error.hidden = true;
      updated.textContent = `Atualizado às ${new Date().toLocaleTimeString("pt-BR")}`;
      grid.setAttribute("aria-busy", "false");
    } catch (e) {
      error.textContent = errorText(e);
      error.hidden = false;
    } finally {
      refresh.disabled = false;
    }
  };

  const act = async (path: string, body: unknown): Promise<string> => {
    const r = await call<ActionResult>("POST", `/api/admin/ops/${path}`, body);
    void load();
    return r.message;
  };
  const confirmAct = (spec: Parameters<typeof openDialog>[0]) => {
    void openDialog(spec).then((msg) => {
      if (msg) toast(msg);
    });
  };
  const quick = async (path: string, body: unknown, btn: HTMLButtonElement) => {
    btn.disabled = true;
    try {
      toast(await act(path, body));
    } catch (e) {
      toast(errorText(e), "bad");
    } finally {
      btn.disabled = false;
    }
  };

  const countdown = (at: string, prefix = "") => {
    const el = h("strong", { class: "adm-count" });
    ticking.push({ el, at, prefix });
    return el;
  };
  const tick = () => {
    const now = Date.now() + skew;
    for (const t of ticking) t.el.textContent = t.prefix + fmtDuration((new Date(t.at).getTime() - now) / 1000);
  };

  function paint(o: Ops): void {
    ticking.length = 0;
    grid.replaceChildren(
      statusRow(o),
      announceCard(o),
      maintenanceCard(o),
      eventsCard(o),
      bossesCard(o),
    );
    tick();
  }

  function statusRow(o: Ops): HTMLElement {
    const inv = o.invasion;
    const invText = inv.phase === "none" ? "Nenhuma" : inv.phase === "warning" ? "Anunciada" : `Onda ${inv.wave}/${inv.waves}`;
    const invNote: HTMLElement =
      inv.phase === "none"
        ? h("span", {}, `Próxima noite em `, countdown(o.nextNight), ` · ${Math.round(inv.chance * 100)}% de chance`)
        : inv.phase === "warning" && inv.startsAt
          ? h("span", {}, "Começa em ", countdown(inv.startsAt))
          : h("span", {}, `${inv.alive} vivos · ${inv.killed} derrotados · acaba em `, inv.endsAt ? countdown(inv.endsAt) : "—");
    const xpNote = o.xp2.on ? h("span", {}, o.xp2.until ? ["Até ", fmtDateTime(o.xp2.until)] : "Sem previsão de fim") : h("span", {}, o.xp2.nextStart ? `Próxima: ${fmtDateTime(o.xp2.nextStart)}` : "Só por GM");
    const m = o.maintenance;
    return h(
      "div",
      { class: "adm-stats adm-span-3" },
      stat("Online agora", nf.format(o.online), h("span", {}, o.night ? "Noite · amanhece em " : "Dia · anoitece em ", countdown(o.night ? o.nextDay : o.nextNight)), true),
      stat("XP em dobro", o.xp2.on ? "Ligada" : "Desligada", xpNote),
      stat("Invasão", invText, invNote),
      stat("Manutenção", m ? "Em contagem" : "Nenhuma", m ? h("span", {}, "Reinicia em ", countdown(m.at)) : h("span", {}, "O servidor segue normal")),
    );
  }

  function stat(label: string, value: string, note: HTMLElement, accent = false): HTMLElement {
    return h("div", { class: accent ? "adm-card adm-stat is-accent" : "adm-card adm-stat" }, h("span", { class: "adm-stat-label" }, label), h("strong", { class: "adm-stat-value" }, value), h("span", { class: "adm-stat-note" }, note));
  }

  function announceCard(o: Ops): HTMLElement {
    const c = card("Anúncio para todos", "adm-span-2");
    const send = h("button", { class: "adm-btn is-primary", type: "button" }, "Enviar agora");
    const sched = h("button", { class: "adm-btn", type: "button" }, "Agendar");
    send.addEventListener("click", async () => {
      if (!text.value.trim()) return text.focus();
      send.disabled = true;
      try {
        toast(await act("announce", { title: title.value, text: text.value }));
        text.value = "";
      } catch (e) {
        toast(errorText(e), "bad");
      } finally {
        send.disabled = false;
      }
    });
    sched.addEventListener("click", async () => {
      if (!text.value.trim()) return text.focus();
      if (!when.value) {
        toast("Escolha o dia e a hora do agendamento.", "bad");
        return when.focus();
      }
      sched.disabled = true;
      try {
        toast(await act("announce", { title: title.value, text: text.value, at: new Date(when.value).toISOString() }));
        text.value = "";
        when.value = "";
      } catch (e) {
        toast(errorText(e), "bad");
      } finally {
        sched.disabled = false;
      }
    });
    c.append(
      h("div", { class: "adm-form" }, h("label", { class: "adm-field" }, h("span", {}, "Título"), title), h("label", { class: "adm-field" }, h("span", {}, "Texto"), text)),
      h("div", { class: "adm-form-row" }, send, h("span", { class: "adm-or" }, "ou"), h("label", { class: "adm-field is-inline" }, h("span", {}, "Agendar para"), when), sched),
      h("h3", {}, `Agendados (${o.scheduled.length})`),
      table(
        ["Quando", "Anúncio", "Por", ""],
        o.scheduled.map((a) => [
          h("span", {}, fmtDateTime(a.at), h("small", {}, ` ${fmtRelative(a.at)}`)),
          h("span", {}, h("strong", {}, a.title), ` ${a.text}`),
          a.by || "—",
          button(
            "Cancelar",
            () =>
              confirmAct({
                title: "Cancelar o anúncio agendado?",
                text: `“${a.text}” — ${fmtDateTime(a.at)}`,
                confirm: "Cancelar anúncio",
                danger: true,
                submit: () => act(`announce/${a.id}/cancel`, {}),
              }),
            "ghost",
          ),
        ]),
        "Nenhum anúncio agendado. Agendados sobrevivem a um reinício.",
      ),
    );
    return c;
  }

  function maintenanceCard(o: Ops): HTMLElement {
    const c = card("Aviso de manutenção");
    const m = o.maintenance;
    if (m) {
      c.append(
        h("p", { class: "adm-big" }, "Reinicia em ", countdown(m.at)),
        h("dl", { class: "adm-dl" }, ...field("Hora", fmtClock(m.at)), ...field("Mensagem", m.message || "—"), ...field("Por", m.by || "—")),
        h("p", { class: "adm-note" }, "Todos recebem o aviso de novo em 10, 5, 3, 2 e 1 minuto, e quem entra durante a contagem também é avisado."),
        button(
          "Cancelar manutenção",
          () => confirmAct({ title: "Cancelar a manutenção?", text: "Todos recebem o aviso de que o servidor não vai mais reiniciar.", confirm: "Cancelar manutenção", danger: true, submit: () => act("maintenance/cancel", {}) }),
          "danger",
        ),
      );
      return c;
    }
    c.append(
      h("p", { class: "adm-note" }, "Contagem regressiva para todos: “o servidor reinicia em N min”, no banner e no chat, repetida nas marcas. Não reinicia sozinho: é o aviso do deploy."),
      button(
        "Avisar manutenção",
        () =>
          confirmAct({
            title: "Avisar manutenção",
            fields: [
              { name: "minutes", label: "Reinicia em (minutos)", type: "number", value: "10", min: "1", max: "1440", required: true },
              { name: "message", label: "Mensagem (opcional)", placeholder: "ex.: Volta em 5 minutos com a atualização." },
            ],
            confirm: "Avisar todos",
            danger: true,
            submit: (v) => act("maintenance", { minutes: Number(v.minutes), message: v.message }),
          }),
        "primary",
      ),
    );
    return c;
  }

  function eventsCard(o: Ops): HTMLElement {
    const c = card("Eventos", "adm-span-2");
    const inv = o.invasion;
    const invBtns =
      inv.phase === "none"
        ? [
            button("Anunciar (2 min)", () => confirmAct({ title: "Anunciar a invasão?", text: "Todos são avisados; as ondas começam em 2 minutos.", confirm: "Anunciar", submit: () => act("invasion", { action: "anunciar" }) }), "primary"),
            button("Começar agora", () => confirmAct({ title: "Começar a invasão agora?", text: "Sem o aviso de 2 minutos: os esqueletos já saem dos portões.", confirm: "Começar", danger: true, submit: () => act("invasion", { action: "agora" }) })),
          ]
        : [
            inv.phase === "warning" ? button("Começar agora", () => confirmAct({ title: "Pular o aviso?", confirm: "Começar", danger: true, submit: () => act("invasion", { action: "agora" }) })) : null,
            button("Parar", () => confirmAct({ title: "Parar a invasão?", text: "Os esqueletos somem e ninguém recebe recompensa.", confirm: "Parar", danger: true, submit: () => act("invasion", { action: "parar" }) }), "danger"),
          ];
    const on = h("button", { class: "adm-btn is-primary", type: "button" }, o.xp2.on ? "Trocar o prazo" : "Ligar");
    on.addEventListener("click", () => void quick("xp2", { hours: Number(hours.value) }, on));
    const off = button("Desligar", () => confirmAct({ title: "Desligar a XP em dobro?", text: "Desliga também a do horário de hoje (até o fim da sequência de dias).", confirm: "Desligar", danger: true, submit: () => act("xp2", { hours: 0 }) }), "ghost");
    off.disabled = !o.xp2.on;
    const reasonText: Record<string, string> = { gm: "por GM", noite: "sorteada à noite" };
    c.append(
      h(
        "div",
        { class: "adm-event-grid" },
        h(
          "div",
          { class: "adm-event" },
          h("h3", {}, "Invasão da cidade ", inv.phase === "none" ? badge("parada") : inv.phase === "warning" ? badge("anunciada", "warn") : badge("em andamento", "bad")),
          inv.phase === "none"
            ? h("p", { class: "adm-note" }, `Automática: a cada anoitecer, ${Math.round(inv.chance * 100)}% de chance (só com alguém online).`)
            : h("p", { class: "adm-note" }, `${reasonText[inv.reason] ?? inv.reason}. ${inv.phase === "active" ? `Onda ${inv.wave} de ${inv.waves}, ${inv.alive} vivos, ${inv.killed} derrotados.` : ""}`),
          h("div", { class: "adm-btns" }, ...invBtns),
        ),
        h(
          "div",
          { class: "adm-event" },
          h("h3", {}, "XP em dobro ", o.xp2.on ? badge("ligada", "ok") : badge("desligada")),
          h("div", { class: "adm-form-row" }, h("label", { class: "adm-field is-inline" }, h("span", {}, "Horas"), hours), on, off),
          o.xp2.on ? h("p", { class: "adm-note" }, o.xp2.until ? `Até ${fmtDateTime(o.xp2.until)} (${fmtRelative(o.xp2.until)}).` : "Sem previsão de fim.") : null,
        ),
      ),
      h("h3", {}, "Agenda dos automáticos"),
      h(
        "dl",
        { class: "adm-dl" },
        ...field("XP em dobro pelo horário", o.xp2.days.length ? `${o.xp2.days.join(", ")} (horário de Brasília)` : "nenhum dia (só por GM)"),
        ...field("Próxima janela", o.xp2.nextStart ? `${fmtDateTime(o.xp2.nextStart)} → ${o.xp2.nextEnd ? fmtDateTime(o.xp2.nextEnd) : "sem fim"}` : "—"),
        ...(o.xp2.offUntil ? field("Horário desligado por GM até", fmtDateTime(o.xp2.offUntil)) : []),
        ...field("Próximo anoitecer", h("span", {}, fmtClock(o.nextNight), " (", countdown(o.nextNight, "em "), `) · invasão com ${Math.round(inv.chance * 100)}% de chance`)),
      ),
    );
    return c;
  }

  function bossesCard(o: Ops): HTMLElement {
    const c = card("Chefes", "adm-span-3");
    c.append(
      table(
        ["Chefe", "Situação", "Detalhe", ""],
        o.bosses.map((b) => {
          let state: HTMLElement;
          let detail: HTMLElement;
          if (b.alive) {
            state = b.fighting ? badge("Em luta", "bad") : badge("Vivo", "ok");
            const bits = [`${b.hpPct}% de vida`];
            if (b.raid) bits.push(`fase ${b.phase}`);
            if (b.enraged) bits.push("fúria");
            if (b.final) bits.push("fúria final");
            detail = h("span", {}, bits.join(" · "), b.fighting && b.fightStart ? h("small", {}, ` · luta há ${fmtDuration((Date.now() + skew - new Date(b.fightStart).getTime()) / 1000)}`) : null);
          } else {
            state = badge("Morto");
            detail = h(
              "span",
              {},
              b.respawnAt ? h("span", {}, "Renasce em ", countdown(b.respawnAt), ` (${fmtDateTime(b.respawnAt)})`) : "—",
              b.windowStart && b.windowEnd && b.windowStart !== b.windowEnd ? h("small", {}, ` · janela ${fmtClock(b.windowStart)}–${fmtClock(b.windowEnd)}`) : null,
              b.killedAt ? h("small", {}, ` · morto ${fmtRelative(b.killedAt)}`) : null,
            );
          }
          const respawn = button(
            "Renascer agora",
            () =>
              confirmAct({
                title: `Fazer ${b.name} renascer agora?`,
                text: b.raid ? "Todos ouvem o anúncio do raid boss, e o renascimento fica salvo (um reinício não o esconde de novo)." : "Ele volta ao ponto de origem, inteiro.",
                confirm: "Renascer",
                danger: true,
                submit: () => act(`bosses/${b.id}/respawn`, {}),
              }),
            "ghost",
          );
          respawn.disabled = b.alive;
          return [h("span", {}, h("strong", {}, b.name), h("small", {}, ` nv ${b.level}${b.raid ? " · raid" : ""}`)), state, detail, respawn];
        }),
        "Nenhum chefe nesta zona.",
      ),
    );
    return c;
  }

  refresh.addEventListener("click", () => void load());
  void load();
  const refreshTimer = window.setInterval(() => {
    const typing = document.activeElement instanceof HTMLElement && grid.contains(document.activeElement) && /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement.tagName);
    if (document.visibilityState === "visible" && !typing && !document.querySelector("dialog[open]")) void load();
  }, REFRESH_MS);
  const tickTimer = window.setInterval(() => {
    if (last) tick();
  }, 1000);
  return () => {
    alive = false;
    clearInterval(refreshTimer);
    clearInterval(tickTimer);
  };
}
