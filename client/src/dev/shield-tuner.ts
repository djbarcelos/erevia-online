/**
 * Painel de ajuste do escudo nos heróis v4 (lab.html?herois=v4&ajuste=escudo). Ferramenta de arte, fora do jogo.
 * Posição em centímetros no referencial da mão (para fora, ao longo do braço, para o lado do polegar) e giro em
 * graus nos eixos do próprio escudo, por cima do encaixe atual (V4_GRIPS). "Copiar" leva o encaixe pronto.
 */
import { Quaternion } from "@babylonjs/core";
import { V4_GRIPS, type Grip } from "../world/models";

type Lab = { show(spec: unknown): Promise<string> };
const SHIELDS: Record<string, string> = { shield_round_barbarian: "Redondo (bossa)", shield_badge_color: "Alto (leão)" };
const POSES: [string, string, number][] = [
  ["idle", "Parado", 0.1], ["stance", "Guarda", 0.3], ["block", "Bloqueio", 0.35], ["attack", "Golpe 1", 0.35],
  ["attack2", "Golpe 2", 0.42], ["attack3", "Escudo (golpe 3)", 0.46], ["run", "Correndo", 0.25], ["walk", "Andando", 0.25],
];
const CM = 40; // 1 unidade do encaixe = 0,4 m (escala do handslot) = 40 cm

export function start(): void {
  const lab = (window as unknown as { lab: Lab }).lab;
  const st = { shield: "shield_round_barbarian", role: "idle", t: 0.1, x: 0, y: 0, z: 0, rx: 0, ry: 0, rz: 0, base: [0, 0, 0, 1] as number[] };
  const box = document.createElement("div");
  box.style.cssText = "position:fixed;top:12px;right:12px;width:300px;background:#1d2230ee;color:#eef;font:13px system-ui;padding:12px;border-radius:10px;z-index:9;box-shadow:0 4px 18px #0006";
  document.body.appendChild(box);
  const loadBase = () => {
    const g = V4_GRIPS[st.shield];
    st.base = g?.q ? [...g.q] : [0, 0, 0, 1];
    const p = g?.pos ?? [0, 0, 0];
    st.x = Math.round(-p[0] * CM * 10) / 10; // positivo = para fora
    st.y = Math.round(p[1] * CM * 10) / 10;
    st.z = Math.round(p[2] * CM * 10) / 10;
    st.rx = st.ry = st.rz = 0;
  };
  const grip = (): Grip => {
    const d = Math.PI / 180;
    const q = new Quaternion(st.base[0], st.base[1], st.base[2], st.base[3]).multiply(Quaternion.FromEulerAngles(st.rx * d, st.ry * d, st.rz * d));
    const r = (v: number) => Math.round(v * 10000) / 10000;
    return { rot: [0, 0, 0], q: [r(q.x), r(q.y), r(q.z), r(q.w)], pos: [r(-st.x / CM), r(st.y / CM), r(st.z / CM)], hand: "l" };
  };
  let timer = 0;
  let first = true;
  const apply = () => {
    clearTimeout(timer);
    timer = window.setTimeout(() => {
      const items = [{ model: "knight", right: "sword_1handed", left: st.shield, role: st.role, t: st.t }];
      void lab.show({ grips: { [st.shield]: grip() }, keepCam: !first, cam: "three", radius: 2.4, height: 1.05, settle: 30, items });
      first = false;
      out.value = JSON.stringify({ escudo: st.shield, ...grip() });
    }, 90);
  };
  const row = (label: string, el: HTMLElement) => {
    const d = document.createElement("label");
    d.style.cssText = "display:block;margin:6px 0";
    d.textContent = label;
    d.appendChild(el);
    box.appendChild(d);
    return el;
  };
  const sel = (opts: [string, string][], val: string, on: (v: string) => void) => {
    const s = document.createElement("select");
    s.style.cssText = "display:block;width:100%;margin-top:3px";
    for (const [v, l] of opts) s.add(new Option(l, v, false, v === val));
    s.onchange = () => on(s.value);
    return s;
  };
  const sliders: Record<string, HTMLInputElement> = {};
  const slider = (key: "x" | "y" | "z" | "rx" | "ry" | "rz" | "t", label: string, min: number, max: number, step: number) => {
    const s = document.createElement("input");
    s.type = "range";
    Object.assign(s, { min, max, step });
    s.style.cssText = "width:100%";
    const v = document.createElement("span");
    v.style.cssText = "float:right;color:#9cf";
    const d = row(label, s);
    d.parentElement!.insertBefore(v, d.parentElement!.firstChild);
    const sync = () => { s.value = String(st[key]); v.textContent = st[key].toFixed(key === "t" ? 2 : 1); };
    s.oninput = () => { st[key] = +s.value; sync(); apply(); };
    sliders[key] = s;
    (s as unknown as { sync: () => void }).sync = sync;
    sync();
  };
  const title = document.createElement("div");
  title.innerHTML = "<b>Ajuste do escudo</b><br><span style='color:#aab'>Gire a câmera com o mouse (roda = zoom).</span>";
  box.appendChild(title);
  loadBase();
  row("Escudo", sel(Object.entries(SHIELDS), st.shield, (v) => { st.shield = v; loadBase(); syncAll(); apply(); }));
  row("Pose", sel(POSES.map(([v, l]) => [v, l]), st.role, (v) => { st.role = v; st.t = POSES.find((p) => p[0] === v)![2]; syncAll(); apply(); }));
  slider("t", "Momento da animação", 0, 1, 0.01);
  slider("x", "Para fora do braço (cm)", -15, 25, 0.5);
  slider("y", "Ao longo do braço (cm, + = para os dedos)", -30, 20, 0.5);
  slider("z", "Para o lado do polegar (cm)", -20, 20, 0.5);
  slider("rx", "Girar escudo X (graus)", -180, 180, 1);
  slider("ry", "Girar escudo Y (graus)", -180, 180, 1);
  slider("rz", "Girar escudo Z (graus)", -180, 180, 1);
  const syncAll = () => { for (const s of Object.values(sliders)) (s as unknown as { sync: () => void }).sync(); };
  const out = document.createElement("textarea");
  out.readOnly = true;
  out.style.cssText = "width:100%;height:64px;margin-top:6px;font:11px monospace;background:#0b0e14;color:#9f9;border:0;border-radius:6px";
  box.appendChild(out);
  const btn = document.createElement("button");
  btn.textContent = "Copiar valores (cole no chat)";
  btn.style.cssText = "width:100%;margin-top:6px;padding:7px;background:#3a6df0;color:#fff;border:0;border-radius:6px;cursor:pointer";
  btn.onclick = () => { out.select(); void navigator.clipboard?.writeText(out.value); btn.textContent = "Copiado!"; setTimeout(() => (btn.textContent = "Copiar valores (cole no chat)"), 1500); };
  box.appendChild(btn);
  apply();
}
