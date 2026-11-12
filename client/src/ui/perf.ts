import { SceneInstrumentation, type Scene } from "@babylonjs/core";
import type { AdvancedDynamicTexture } from "@babylonjs/gui";

const STORAGE_KEY = "erevia.debug";

/**
 * Painel de desempenho (F3 ou ?debug=1): tempo de frame, draw calls, malhas ativas e
 * quanto tempo vai em animação e na interface. Serve para achar a causa de quedas de FPS.
 */
export function setupPerfPanel(scene: Scene, gui: AdvancedDynamicTexture, hud: HTMLElement): void {
  let on = new URLSearchParams(location.search).has("debug");
  try {
    on ||= localStorage.getItem(STORAGE_KEY) === "1";
  } catch {
    /* sem storage: só pela URL */
  }

  const rows = new Map<string, HTMLElement>();
  const box = document.createElement("div");
  box.className = "perf";
  for (const [key, label] of [
    ["frame", "Frame"],
    ["draws", "Draws"],
    ["meshes", "Malhas"],
    ["anim", "Anim"],
    ["gui", "GUI"],
    ["render", "Render"],
  ]) {
    const row = document.createElement("div");
    row.innerHTML = `<span class="k">${label}</span> <span>–</span>`;
    rows.set(key, row.lastElementChild as HTMLElement);
    box.appendChild(row);
  }
  hud.appendChild(box);

  let sceneInst: SceneInstrumentation | null = null;
  const apply = () => {
    box.hidden = !on;
    if (on && !sceneInst) {
      sceneInst = new SceneInstrumentation(scene);
      sceneInst.captureFrameTime = true;
      sceneInst.captureAnimationsTime = true;
      sceneInst.captureRenderTime = true;
    } else if (!on && sceneInst) {
      sceneInst.dispose();
      sceneInst = null;
    }
  };
  apply();

  window.addEventListener("keydown", (ev) => {
    if (ev.key !== "F3") return;
    ev.preventDefault();
    on = !on;
    try {
      localStorage.setItem(STORAGE_KEY, on ? "1" : "0");
    } catch {
      /* ignora */
    }
    apply();
  });

  // GUI 2D (nomes, barras, números de dano): tempo de layout + desenho no canvas e quantas
  // vezes por segundo ela foi redesenhada (só redesenha quando algo muda).
  let guiStart = 0;
  let guiMs = 0;
  let guiCount = 0;
  gui.onBeginLayoutObservable.add(() => (guiStart = performance.now()));
  gui.onEndRenderObservable.add(() => {
    guiMs += performance.now() - guiStart;
    guiCount++;
  });

  const ms = (v: number) => `${v.toFixed(1)} ms`;
  window.setInterval(() => {
    if (!on || !sceneInst) return;
    rows.get("frame")!.textContent = ms(sceneInst.frameTimeCounter.lastSecAverage);
    rows.get("draws")!.textContent = String(sceneInst.drawCallsCounter.current);
    rows.get("meshes")!.textContent = `${scene.getActiveMeshes().length} / ${scene.meshes.length}`;
    rows.get("anim")!.textContent = ms(sceneInst.animationsTimeCounter.lastSecAverage);
    rows.get("render")!.textContent = ms(sceneInst.renderTimeCounter.lastSecAverage);
    rows.get("gui")!.textContent = guiCount ? `${ms(guiMs / guiCount)} · ${guiCount * 2}/s` : "parada";
    guiMs = guiCount = 0;
  }, 500);
}
