/**
 * Portal → Jogadores: busca (online e offline), ficha completa e as ações de GM.
 * Rotas: #jogadores (online agora / busca ?q) e #jogadores/<id> (ficha).
 *
 * Toda ação vai ao servidor, que decide onde ela acontece (na zona se o personagem está
 * online ou saiu há pouco, no banco se está offline) e registra no gm_log.
 */

import "./actions.css";
import { call } from "./api";
import {
  badge,
  button,
  card,
  type Child,
  errorText,
  field,
  fmtDateTime,
  fmtDuration,
  fmtRelative,
  h,
  nf,
  openDialog,
  table,
  toast,
  type ActionResult,
  type DialogField,
} from "./widgets";

interface Identity {
  provider: string;
  subject: string;
  displayName: string;
  createdAt: string;
}

interface PlayerRow {
  id: number;
  name: string;
  class: string;
  level: number;
  accountId: number;
  gm: boolean;
  banned: boolean;
  mutedUntil: string | null;
  lastSeenAt: string | null;
  identities: Identity[];
  online: boolean;
  region?: string;
}

interface ItemOut {
  slot: number;
  id: string;
  name: string;
  count: number;
  enchant: number;
  grade: number;
  where?: string;
}

interface GMLogRow {
  id: number;
  at: string;
  gmName: string;
  command: string;
  target: string;
  detail: string;
}

interface Sheet {
  id: number;
  name: string;
  class: string;
  className: string;
  level: number;
  xp: number;
  xpLevel: number;
  xpNext: number;
  gold: number;
  x: number;
  z: number;
  region: string;
  hp: number;
  mp: number;
  maxHp: number;
  maxMp: number;
  dead: boolean;
  karma: number;
  pks: number;
  pvpKills: number;
  title: string;
  spec: { id: string; name: string; color: string } | null;
  online: boolean;
  recent: boolean;
  held: boolean;
  zone: boolean;
  source: string;
  joinedAt: string | null;
  equipped: ItemOut[];
  bag: ItemOut[];
  storage: { gold: number; items: ItemOut[]; source: string };
  clan: { id: number; name: string; rank: string } | null;
  party: string[];
  quests: { active: { id: string; title: string; progress?: string }[]; done: { id: string; title: string }[] };
  achievements: { id: string; name: string; title: string; at: string }[];
  account: {
    id: number;
    createdAt: string;
    lastLoginAt: string;
    gm: boolean;
    banned: boolean;
    banReason: string;
    mutedUntil: string | null;
    identities: Identity[];
    characters: { id: number; name: string; class: string; level: number }[];
  };
  createdAt: string;
  lastSeenAt: string | null;
  sessions: { start: string; end: string | null; seconds: number }[];
  events: { at: string; kind: string; level: number; x: number; z: number; ref: string; amount: number; extra: Record<string, unknown> | null }[];
  history: GMLogRow[];
}

interface CatalogItem {
  id: string;
  name: string;
  grade: number;
  stack: number;
}

const CLASSES: Record<string, string> = { knight: "Cavaleiro", barbarian: "Bárbaro", mage: "Mago", rogue: "Ladino" };
const GRADES = ["", "is-common", "is-uncommon", "is-rare", "is-epic", "is-legend"];
const PROVIDERS: Record<string, string> = { google: "Google", discord: "Discord", dev: "Dev" };

let catalog: CatalogItem[] | null = null;

async function items(): Promise<CatalogItem[]> {
  if (!catalog) catalog = (await call<{ items: CatalogItem[] }>("GET", "/api/admin/items")).items;
  return catalog;
}

function playerPath(id: number, action = ""): string {
  return `/api/admin/players/${id}${action ? "/" + action : ""}`;
}

/** Monta a seção. Devolve a limpeza (ao sair da seção). */
export function mountPlayers(main: HTMLElement): () => void {
  const sub = location.hash.replace(/^#\/?/, "").split("/")[1] ?? "";
  const id = Number.parseInt(sub, 10);
  let alive = true;
  if (Number.isFinite(id) && id > 0) void sheetPage(main, id, () => alive);
  else listPage(main, () => alive);
  return () => {
    alive = false;
  };
}

// ---------- Lista e busca ----------

function listPage(main: HTMLElement, alive: () => boolean): void {
  const q0 = new URLSearchParams(location.hash.split("?")[1] ?? "").get("q") ?? sessionStorage.getItem("adm:q") ?? "";
  const input = h("input", {
    type: "search",
    name: "q",
    placeholder: "Nome, id da conta ou identidade (google:…)",
    "aria-label": "Buscar jogador",
    autocomplete: "off",
    enterkeyhint: "search",
    maxlength: "64",
  });
  input.value = q0;
  const go = h("button", { class: "adm-btn is-primary", type: "submit" }, "Buscar");
  const form = h("form", { class: "adm-search", role: "search" }, input, go);
  const status = h("p", { class: "adm-updated", "aria-live": "polite" });
  const results = h("section", { class: "adm-card adm-box" });
  main.append(h("header", { class: "adm-head" }, h("h1", {}, "Jogadores")), form, status, results);

  const load = async () => {
    const q = input.value.trim();
    try {
      sessionStorage.setItem("adm:q", q);
    } catch {
      /* sem storage: sem lembrança da busca */
    }
    go.disabled = true;
    try {
      const res = await call<{ players: PlayerRow[]; zone: boolean; online: number }>(
        "GET",
        `/api/admin/players${q ? "?q=" + encodeURIComponent(q) : ""}`,
      );
      if (!alive()) return;
      status.textContent = res.zone
        ? `${nf.format(res.online)} online agora${q ? ` · ${res.players.length} resultado(s) para “${q}”` : ""}`
        : "A zona não respondeu: o estado online pode estar desatualizado.";
      paintList(results, q ? "Resultados" : "Online agora", res.players, q);
    } catch (e) {
      results.replaceChildren(h("p", { class: "adm-error", role: "alert" }, errorText(e)));
    } finally {
      go.disabled = false;
    }
  };
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    void load();
  });
  void load();
  if (!q0) input.focus();
}

function paintList(box: HTMLElement, title: string, rows: PlayerRow[], q: string): void {
  const empty = q ? "Ninguém encontrado. Tente parte do nome ou o id da conta." : "Ninguém online agora. Use a busca para achar quem está offline.";
  box.replaceChildren(
    h("div", { class: "adm-box-head" }, h("h2", {}, title)),
    table(
      ["Personagem", "Nível", "Situação", "Conta", "Visto"],
      rows.map((p) => [
        h("a", { href: `#jogadores/${p.id}`, class: "adm-link" }, p.name, h("small", {}, ` ${CLASSES[p.class] ?? p.class}`)),
        String(p.level),
        h(
          "span",
          { class: "adm-badges" },
          p.online ? badge(p.region ? `Online · ${p.region}` : "Online", "ok") : badge("Offline"),
          p.gm ? badge("GM", "gold") : null,
          p.banned ? badge("Banido", "bad") : null,
          p.mutedUntil && new Date(p.mutedUntil) > new Date() ? badge("Silenciado", "warn") : null,
        ),
        h(
          "span",
          {},
          `#${p.accountId}`,
          p.identities.length ? h("small", {}, ` ${p.identities.map((i) => `${PROVIDERS[i.provider] ?? i.provider}: ${i.displayName}`).join(", ")}`) : null,
        ),
        p.online ? "agora" : fmtRelative(p.lastSeenAt),
      ]),
      empty,
    ),
  );
}

// ---------- Ficha ----------

async function sheetPage(main: HTMLElement, id: number, alive: () => boolean): Promise<void> {
  const back = h("a", { href: "#jogadores", class: "adm-back-link" }, "← Jogadores");
  const refresh = h("button", { class: "adm-btn", type: "button" }, "Atualizar");
  const title = h("h1", {}, "Ficha");
  const body = h("div", { class: "adm-sheet", "aria-busy": "true" }, h("div", { class: "adm-card adm-box is-loading" }));
  main.append(back, h("header", { class: "adm-head" }, title, h("div", { class: "adm-head-tools" }, refresh)), body);

  const load = async () => {
    refresh.disabled = true;
    try {
      const s = await call<Sheet>("GET", playerPath(id));
      if (!alive()) return;
      title.textContent = s.name;
      document.title = `${s.name} · Jogadores · Administração · Erevia`;
      paintSheet(body, s, load);
      body.setAttribute("aria-busy", "false");
    } catch (e) {
      body.replaceChildren(h("p", { class: "adm-error", role: "alert" }, errorText(e)));
    } finally {
      refresh.disabled = false;
    }
  };
  refresh.addEventListener("click", () => void load());
  await load();
}

function where(s: Sheet): HTMLElement {
  if (s.online) return badge("Online", "ok");
  if (s.recent) return badge("Saiu há pouco", "warn");
  return badge("Offline");
}

function itemName(it: ItemOut): HTMLElement {
  return h("span", { class: `adm-item ${GRADES[it.grade] ?? ""}` }, it.name, it.enchant > 0 ? ` +${it.enchant}` : "", it.count > 1 ? h("small", {}, ` ×${it.count}`) : null);
}

function paintSheet(body: HTMLElement, s: Sheet, reload: () => Promise<void>): void {
  const pct = s.xpNext > 0 ? Math.min(100, Math.max(0, ((s.xp - s.xpLevel) / s.xpNext) * 100)) : 100;
  const muted = s.account.mutedUntil && new Date(s.account.mutedUntil) > new Date() ? s.account.mutedUntil : null;
  const sourceNote: Record<string, string> = {
    zona: "Dados ao vivo da zona.",
    recente: "Saiu há pouco: os dados vêm da memória da zona (valem no próximo login).",
    banco: s.zone ? "Offline: dados do banco." : "A zona não respondeu: dados do banco, podem estar atrasados.",
  };

  // Resumo
  const summary = card("Resumo", "adm-span-2", h("span", { class: "adm-badges" }, where(s), s.account.gm ? badge("GM", "gold") : null, s.account.banned ? badge("Banido", "bad") : null, muted ? badge("Silenciado", "warn") : null, s.held ? badge("Ação em andamento", "warn") : null));
  summary.append(
    h("p", { class: "adm-note" }, sourceNote[s.source] ?? ""),
    h(
      "div",
      { class: "adm-kpis" },
      kpi("Nível", String(s.level), h("span", { class: "adm-bar", role: "img", "aria-label": `${Math.round(pct)}% do nível` }, styled(h("span"), "width", `${pct.toFixed(1)}%`))),
      kpi("Moedas", nf.format(s.gold)),
      kpi("Região", s.region, `${Math.round(s.x)}, ${Math.round(s.z)}`),
      kpi("Karma / PKs", `${nf.format(s.karma)} / ${nf.format(s.pks)}`, `${nf.format(s.pvpKills)} vitórias PvP`),
    ),
    h(
      "dl",
      { class: "adm-dl" },
      ...field("Classe", [s.className, s.spec ? styled(h("span", { class: "adm-spec" }, ` · ${s.spec.name}`), "color", s.spec.color) : " · sem especialização"]),
      ...field("XP", `${nf.format(s.xp)} (${Math.round(pct)}% do nível)`),
      ...(s.online ? field("Vida / mana", `${nf.format(s.hp)}/${nf.format(s.maxHp)} · ${nf.format(s.mp)}/${nf.format(s.maxMp)}${s.dead ? " · morto" : ""}`) : []),
      ...field("Título", s.title || "—"),
      ...field("Clã", s.clan ? `${s.clan.name} (${s.clan.rank})` : "—"),
      ...field("Grupo", s.party.length ? s.party.join(", ") : s.online ? "sozinho" : "—"),
      ...field("Criado", fmtDateTime(s.createdAt)),
      ...field(s.online ? "Na sessão desde" : "Visto por último", s.online ? fmtDateTime(s.joinedAt) : fmtRelative(s.lastSeenAt)),
    ),
  );

  // Conta
  const acc = card("Conta", "", h("small", { class: "adm-mono" }, `#${s.account.id}`));
  acc.append(
    h(
      "dl",
      { class: "adm-dl" },
      ...field("Identidades", s.account.identities.length ? s.account.identities.map((i) => h("span", { class: "adm-ident" }, `${PROVIDERS[i.provider] ?? i.provider}: ${i.displayName}`, h("small", { class: "adm-mono" }, ` ${i.subject}`))) : "—"),
      ...field("Criada", fmtDateTime(s.account.createdAt)),
      ...field("Último login", fmtDateTime(s.account.lastLoginAt)),
      ...field("Banida", s.account.banned ? `sim — ${s.account.banReason || "sem motivo"}` : "não"),
      ...field("Silêncio", muted ? `até ${fmtDateTime(muted)} (${fmtRelative(muted)})` : "não"),
      ...field(
        "Personagens",
        s.account.characters.map((c) => (c.id === s.id ? h("strong", {}, `${c.name} (${c.level})`) : h("a", { href: `#jogadores/${c.id}`, class: "adm-link" }, `${c.name} (${c.level})`))),
      ),
    ),
  );

  // Ações
  const act = card("Ações", "adm-actions-card");
  act.append(h("p", { class: "adm-note" }, s.online ? "Online: a ação vai pela zona, na hora." : s.recent ? "Saiu há pouco: a ação vai na memória da zona (vale no próximo login)." : "Offline: a ação vai pelo banco, numa transação."), actions(s, reload));

  // Inventário
  const inv = card("Inventário", "adm-span-2", h("small", {}, `${s.bag.length} de 40 espaços`));
  inv.append(
    h("h3", {}, "Equipado"),
    table(["Encaixe", "Item"], s.equipped.map((it) => [it.where ?? String(it.slot), itemName(it)]), "Nada equipado."),
    h("h3", {}, "Bolsa"),
    h("ul", { class: "adm-items" }, ...s.bag.map((it) => h("li", {}, itemName(it)))),
    s.bag.length === 0 ? h("p", { class: "adm-empty" }, "Bolsa vazia.") : "",
  );

  const storage = card("Armazém", "", h("small", {}, s.storage.source === "zona" ? "da zona" : "do banco"));
  storage.append(
    h("p", {}, h("strong", {}, nf.format(s.storage.gold)), " moedas guardadas"),
    s.storage.items.length ? h("ul", { class: "adm-items" }, ...s.storage.items.map((it) => h("li", {}, itemName(it)))) : h("p", { class: "adm-empty" }, "Armazém vazio."),
  );

  const quests = card("Missões", "", h("small", {}, `${s.quests.done.length} concluída(s)`));
  quests.append(
    s.quests.active.length
      ? h("ul", { class: "adm-list" }, ...s.quests.active.map((q) => h("li", {}, h("strong", {}, q.title), q.progress ? h("small", {}, q.progress) : null)))
      : h("p", { class: "adm-empty" }, "Nenhuma missão ativa."),
    s.quests.done.length ? h("details", {}, h("summary", {}, "Concluídas"), h("p", { class: "adm-note" }, s.quests.done.map((q) => q.title).join(" · "))) : "",
  );

  const ach = card("Conquistas", "", h("small", {}, String(s.achievements.length)));
  ach.append(
    s.achievements.length
      ? h("ul", { class: "adm-list" }, ...s.achievements.map((a) => h("li", {}, h("strong", {}, a.name), h("small", {}, `${a.title ? `título “${a.title}” · ` : ""}${fmtDateTime(a.at)}`))))
      : h("p", { class: "adm-empty" }, "Nenhuma ainda."),
  );

  const sessions = card("Últimas sessões");
  sessions.append(
    table(
      ["Início", "Fim", "Duração"],
      s.sessions.map((x) => [fmtDateTime(x.start), x.end ? fmtDateTime(x.end) : s.online ? "em andamento" : "—", fmtDuration(x.seconds)]),
      "Nenhuma sessão registrada.",
    ),
  );

  const events = card("Eventos recentes", "adm-span-2");
  events.append(table(["Quando", "Evento", "Detalhe", "Onde"], s.events.map((e) => [fmtDateTime(e.at), eventLabel(e.kind), eventDetail(e), `nv ${e.level} · ${Math.round(e.x)}, ${Math.round(e.z)}`]), "Nada registrado."));

  const hist = card("Histórico de moderação", "adm-span-3");
  hist.append(
    table(
      ["Quando", "GM", "Ação", "Alvo", "Detalhe"],
      s.history.map((g) => [fmtDateTime(g.at), g.gmName, h("code", {}, g.command), g.target || "—", h("span", { class: "adm-detail" }, g.detail || "—")]),
      "Nenhuma ação de GM contra esta conta.",
    ),
  );

  body.replaceChildren(summary, act, acc, inv, storage, quests, ach, sessions, events, hist);
}

function kpi(label: string, value: string, extra?: Child): HTMLElement {
  return h("div", { class: "adm-kpi" }, h("span", { class: "adm-stat-label" }, label), h("strong", {}, value), typeof extra === "string" ? h("small", {}, extra) : extra);
}

const EVENT_LABELS: Record<string, string> = {
  gold: "Moedas",
  item_in: "Item entrou",
  item_out: "Item saiu",
  kill: "Abate",
  death: "Morte",
  level: "Subiu de nível",
  invasion: "Invasão",
  invariant: "Correção de estado",
  achievement: "Conquista",
  quest: "Missão",
  server_error: "Erro do servidor",
};

function eventLabel(k: string): string {
  return EVENT_LABELS[k] ?? k;
}

function eventDetail(e: Sheet["events"][number]): string {
  const how = typeof e.extra?.how === "string" ? ` (${e.extra.how})` : "";
  switch (e.kind) {
    case "gold":
      return `${e.amount > 0 ? "+" : ""}${nf.format(e.amount)} · ${e.ref}`;
    case "item_in":
    case "item_out":
      return `${e.ref} ×${e.amount}${how}`;
    case "kill":
      return `${e.ref} (nv ${e.amount})`;
    case "level":
      return `nível ${e.amount}`;
    default:
      return [e.ref, e.amount ? String(e.amount) : ""].filter(Boolean).join(" · ") || "—";
  }
}

// ---------- Ações ----------

function actions(s: Sheet, reload: () => Promise<void>): HTMLElement {
  const run = (spec: Parameters<typeof openDialog>[0]) => {
    void openDialog(spec).then((msg) => {
      if (msg === null) return;
      if (msg) toast(msg);
      void reload();
    });
  };
  const post = async (action: string, body: unknown): Promise<string> => {
    const r = await call<ActionResult>("POST", playerPath(s.id, action), body);
    return r.message;
  };
  const reason: DialogField = { name: "reason", label: "Motivo", placeholder: "Vai para o gm_log (e para o jogador, quando ele vê)" };
  const muted = s.account.mutedUntil && new Date(s.account.mutedUntil) > new Date();

  const groups: { title: string; buttons: HTMLButtonElement[] }[] = [
    {
      title: "Moderação",
      buttons: [
        button(
          "Expulsar",
          () =>
            run({
              title: `Expulsar ${s.name}`,
              text: "Derruba a conexão agora. O jogador pode entrar de novo.",
              fields: [reason],
              confirm: "Expulsar",
              danger: true,
              submit: (v) => post("kick", { reason: v.reason }),
            }),
          "danger",
        ),
        muted
          ? button("Tirar silêncio", () =>
              run({ title: `Tirar o silêncio de ${s.name}`, fields: [reason], confirm: "Tirar silêncio", submit: (v) => post("unmute", { reason: v.reason }) }),
            )
          : button("Silenciar", () =>
              run({
                title: `Silenciar ${s.name}`,
                text: "Vale para a conta inteira, em todos os canais, e continua depois de relogar.",
                fields: [{ name: "minutes", label: "Minutos", type: "number", value: "30", min: "1", max: "10080", required: true, hint: "Até 10080 (7 dias)." }, reason],
                confirm: "Silenciar",
                submit: (v) => post("mute", { minutes: Number(v.minutes), reason: v.reason }),
              }),
            ),
        s.account.banned
          ? button("Desbanir", () =>
              run({ title: `Desbanir a conta de ${s.name}`, fields: [{ ...reason, required: true }], confirm: "Desbanir", submit: (v) => post("unban", { reason: v.reason }) }),
            )
          : button(
              "Banir",
              () =>
                run({
                  title: `Banir a conta de ${s.name}`,
                  text: `Vale para todos os personagens da conta #${s.account.id} e derruba quem estiver online.`,
                  fields: [{ ...reason, required: true }],
                  confirm: "Banir",
                  danger: true,
                  submit: (v) => post("ban", { reason: v.reason }),
                }),
              "danger",
            ),
        button(s.account.gm ? "Tirar GM" : "Marcar como GM", () =>
          run({
            title: s.account.gm ? `Tirar o GM da conta de ${s.name}` : `Tornar a conta de ${s.name} GM`,
            text: s.account.gm ? "Perde os comandos de GM e o portal na hora." : "Ganha os comandos de GM no jogo e o acesso a este portal.",
            confirm: s.account.gm ? "Tirar GM" : "Marcar como GM",
            danger: !s.account.gm,
            submit: () => post("gm", { gm: !s.account.gm }),
          }),
        ),
      ],
    },
    {
      title: "Personagem",
      buttons: [
        button("Teleportar", () =>
          run({
            title: `Teleportar ${s.name}`,
            text: `Hoje em ${Math.round(s.x)}, ${Math.round(s.z)} (${s.region}). A cidade é 0, 0.`,
            fields: [
              { name: "x", label: "X", type: "number", value: "0", step: "any", required: true },
              { name: "z", label: "Z", type: "number", value: "0", step: "any", required: true },
            ],
            confirm: "Teleportar",
            submit: (v) => post("teleport", { x: Number(v.x), z: Number(v.z) }),
          }),
        ),
        button("Dar item", () => void itemDialog(s, "give", run, post)),
        button("Tirar item", () => void itemDialog(s, "take", run, post), "danger"),
        button("Dar moedas", () =>
          run({
            title: `Dar moedas a ${s.name}`,
            fields: [{ name: "amount", label: "Moedas", type: "number", value: "100", min: "1", required: true }, reason],
            confirm: "Dar",
            submit: (v) => post("gold", { amount: Math.abs(Math.trunc(Number(v.amount))), reason: v.reason }),
          }),
        ),
        button(
          "Tirar moedas",
          () =>
            run({
              title: `Tirar moedas de ${s.name}`,
              text: `Tem ${nf.format(s.gold)} moedas.`,
              fields: [{ name: "amount", label: "Moedas", type: "number", value: "", min: "1", required: true }, reason],
              confirm: "Tirar",
              danger: true,
              submit: (v) => post("gold", { amount: -Math.abs(Math.trunc(Number(v.amount))), reason: v.reason }),
            }),
          "danger",
        ),
      ],
    },
  ];
  if (!s.online) groups[0].buttons[0].disabled = true; // expulsar: só online
  return h(
    "div",
    { class: "adm-action-groups" },
    ...groups.map((g) => h("div", { class: "adm-action-group" }, h("h3", {}, g.title), h("div", { class: "adm-btns" }, ...g.buttons))),
  );
}

async function itemDialog(
  s: Sheet,
  mode: "give" | "take",
  run: (spec: Parameters<typeof openDialog>[0]) => void,
  post: (action: string, body: unknown) => Promise<string>,
): Promise<void> {
  let list: CatalogItem[];
  try {
    list = await items();
  } catch (e) {
    toast(errorText(e), "bad");
    return;
  }
  const own = [...s.bag, ...s.equipped];
  const suggestions =
    mode === "take"
      ? own.map((it) => ({ value: it.id, label: `${it.name}${it.count > 1 ? ` ×${it.count}` : ""}${it.where ? ` (${it.where})` : ""}` }))
      : list.map((it) => ({ value: it.id, label: it.name }));
  run({
    title: mode === "give" ? `Dar item a ${s.name}` : `Tirar item de ${s.name}`,
    text: mode === "take" ? "Tira da bolsa primeiro e, se faltar, do equipamento." : "Vai para a bolsa (se não couber tudo, entra o que couber).",
    fields: [
      { name: "item", label: "Item (id)", required: true, suggestions, placeholder: mode === "give" ? "ex.: potion_hp_s" : own[0]?.id },
      { name: "count", label: "Quantidade", type: "number", value: "1", min: "1", max: "9999", required: true },
      { name: "reason", label: "Motivo", placeholder: "Vai para o gm_log" },
    ],
    confirm: mode === "give" ? "Dar" : "Tirar",
    danger: mode === "take",
    submit: (v) => {
      const known = list.find((it) => it.id === v.item || it.name.toLowerCase() === v.item.toLowerCase());
      if (!known) return Promise.reject(new Error("Item desconhecido. Escolha da lista."));
      return post("item", { action: mode, item: known.id, count: Math.trunc(Number(v.count)), reason: v.reason });
    },
  });
}

/** Estilo pelo CSSOM: a CSP do portal bloqueia o atributo style. */
function styled<T extends HTMLElement>(el: T, prop: "width" | "color", value: string): T {
  el.style[prop] = value;
  return el;
}
