/**
 * "O que há de novo": a versão mais nova do changelog e, recolhidas, as anteriores.
 *
 * Abre sozinha UMA vez por versão (a última vista fica no localStorage; sem storage, não
 * abre). Reabre em Configurações → Sobre e no login. Nada roda por quadro: o json é
 * buscado uma vez e a janela só existe enquanto está aberta.
 */

import { loadNews, newsVersionEl, type News } from "../../news";
import { button, el, WindowManager, type WindowDef } from "../kit";
import "../../styles/news.css";

const SEEN_KEY = "erevia.novidades.vista";

/**
 * Diz se a janela deve abrir sozinha para a versão `latest` e já a marca como vista.
 * Só responde sim se conseguiu gravar: com o storage bloqueado ou cheio, a janela não
 * abriria uma vez, e sim a cada entrada.
 */
export function takeUnseenNews(latest: string): boolean {
  try {
    if (localStorage.getItem(SEEN_KEY) === latest) return false;
    localStorage.setItem(SEEN_KEY, latest);
    return localStorage.getItem(SEEN_KEY) === latest;
  } catch {
    return false;
  }
}

function buildWhatsNew(news: News | null, close: () => void): HTMLElement {
  const root = el("div", "whatsnew-win");
  if (!news) {
    root.append(el("p", "muted", "Não deu para carregar as novidades agora. Tente de novo daqui a pouco."));
  } else {
    const [latest, ...older] = news.versions;
    root.append(newsVersionEl(latest));
    if (older.length) {
      const prev = el("details", "news-older", el("summary", null, `Versões anteriores (${older.length})`));
      prev.append(...older.map((v) => newsVersionEl(v, "h3")));
      root.append(prev);
    }
  }
  const page = el("a", "btn btn-sm btn-ghost", "Página completa");
  page.href = "/novidades";
  page.target = "_blank";
  page.rel = "noopener";
  root.append(el("div", "whatsnew-foot", page, button("Fechar", "btn btn-gold btn-sm", close)));
  return root;
}

/** A janela para o WindowManager do jogo. */
export function whatsNewDef(): WindowDef {
  return {
    id: "news",
    title: "O que há de novo",
    width: 620,
    height: 640,
    build: async (close) => buildWhatsNew(await loadNews(), close),
  };
}

/**
 * Abre a janela fora do jogo (tela de login): um WindowManager só para ela, que some
 * com a camada ao fechar.
 */
export function openWhatsNewAlone(): void {
  if (document.querySelector('[data-win="news"]')) return;
  const wm = new WindowManager(new Map([["news", whatsNewDef()]]));
  wm.layer.style.zIndex = "60"; // acima do #overlay das telas de entrada (50)
  const onKey = (ev: KeyboardEvent) => {
    if (ev.key === "Escape") wm.closeTop();
  };
  wm.onChange = () => {
    if (wm.anyOpen()) return;
    removeEventListener("keydown", onKey);
    wm.layer.remove();
  };
  addEventListener("keydown", onKey);
  void wm.show("news");
}
