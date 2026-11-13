/** Protótipo: a janela de configurações é a mesma do jogo, com dados falsos. */

import { buildSettings } from "../../../ui/windows/settings";
import { game } from "../../data/mock";

export function build(opts: { close(): void; exit(): void; logout(): void }): HTMLElement {
  return buildSettings({
    ...opts,
    info: () => ({ account: "Conta Google", character: game.hero.name, level: game.hero.level, ping: 24, online: 1284 }),
  });
}
