/** Registro das seções de leitura do portal: Relatos (#107), Métricas (#108) e Economia (#109). */

import "./panels.css";
import { renderEconomy } from "./economy";
import { renderMetrics } from "./metrics";
import { mountReportsBadge, renderReports } from "./reports";

/** Monta a seção em main e devolve a limpeza (timers) para quando sair dela. */
export type Panel = (main: HTMLElement) => () => void;

export const PANELS: Record<string, Panel> = {
  relatos: renderReports,
  metricas: renderMetrics,
  economia: renderEconomy,
};

/** Extras do menu (o contador de relatos abertos). */
export function mountPanelsNav(nav: HTMLElement): void {
  mountReportsBadge(nav);
}
