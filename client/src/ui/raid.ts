/**
 * Interface do raid boss: a barra larga no topo da tela (nome, vida, marcas das fases,
 * fúria e tempo de luta) e o banner dos anúncios para o servidor inteiro.
 */

import { AnnounceKind } from "../gen/erevia/v1/game_pb";
import "../styles/raid.css";
import { el } from "./kit";

export interface RaidBarState {
  name: string;
  hp: number;
  maxHp: number;
  phase: number;
  enraged: boolean;
  final: boolean;
  /** Início da luta (relógio do servidor, ms); 0 = parado. */
  fightStartMs: number;
}

/** Marcas das fases na barra (server/internal/zone/raid.go: raidPhase2At, raidPhase3At). */
const PHASE_MARKS = [0.7, 0.4];

export class RaidUi {
  readonly bar = el("div", "raid-bar glass");
  readonly banner = el("div", "raid-announce");
  private name = el("b", "rb-name");
  private phase = el("span", "rb-phase");
  private timer = el("span", "rb-timer");
  private fill = el("div", "rb-fill");
  private lag = el("div", "rb-lag");
  private pct = el("span", "rb-pct");
  private bannerTimer = 0;
  private shown = false;

  constructor(private serverNow: () => number) {
    const track = el("div", "rb-track", this.lag, this.fill);
    for (const m of PHASE_MARKS) {
      const tick = el("i", "rb-mark");
      tick.style.left = `${m * 100}%`;
      track.append(tick);
    }
    track.append(this.pct);
    this.bar.append(el("div", "rb-head", el("span", "rb-skull", "☠"), this.name, this.phase, this.timer), track);
    this.bar.hidden = true;
    this.banner.hidden = true;
  }

  /** Mostra (ou esconde, com null) a barra do chefe. */
  setBar(s: RaidBarState | null): void {
    const on = !!s;
    if (on !== this.shown) {
      this.shown = on;
      this.bar.hidden = !on;
      // O quadro do alvo desce enquanto a barra aparece (ver styles/raid.css).
      document.body.classList.toggle("raid-bar-on", on);
    }
    if (!s) return;
    const frac = s.maxHp > 0 ? Math.max(0, Math.min(1, s.hp / s.maxHp)) : 0;
    this.name.textContent = s.name;
    this.fill.style.width = `${frac * 100}%`;
    // A parte "perdida" acompanha devagar (transição no CSS).
    this.lag.style.width = `${frac * 100}%`;
    this.pct.textContent = `${(frac * 100).toFixed(frac < 0.1 ? 1 : 0)}%`;
    this.phase.textContent = s.final ? "Fúria final" : s.enraged ? "Fúria" : `Fase ${Math.max(1, s.phase)}`;
    this.bar.classList.toggle("enraged", s.enraged || s.final);
    if (s.fightStartMs > 0) {
      const secs = Math.max(0, Math.floor((this.serverNow() - s.fightStartMs) / 1000));
      this.timer.textContent = `${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, "0")}`;
    } else this.timer.textContent = "";
  }

  /** Banner grande no centro (anúncios do servidor). */
  announce(kind: AnnounceKind, title: string, text: string): void {
    window.clearTimeout(this.bannerTimer);
    this.banner.className = `raid-announce ${kindClass(kind)}`;
    this.banner.replaceChildren(el("div", "ra-title", title), el("div", "ra-text", text));
    this.banner.hidden = false;
    void this.banner.offsetWidth; // reinicia a animação
    this.banner.classList.add("show");
    this.bannerTimer = window.setTimeout(() => (this.banner.hidden = true), 6000);
  }

  dispose(): void {
    window.clearTimeout(this.bannerTimer);
    document.body.classList.remove("raid-bar-on");
    this.bar.remove();
    this.banner.remove();
  }
}

function kindClass(k: AnnounceKind): string {
  switch (k) {
    case AnnounceKind.BOSS_SOON:
      return "soon";
    case AnnounceKind.BOSS_DEFEAT:
      return "defeat";
    default:
      return "spawn";
  }
}
