/**
 * "Mundo pronto": a tela de carregamento só sai quando o cenário, os personagens e os shaders
 * estão prontos. Lógica pura (sem Babylon): as etapas são promessas que já estão em andamento.
 * Um teto de tempo garante que nada trava a entrada: estourou, entra assim mesmo.
 */

export interface ReadyStage {
  /** Nome mostrado na barra ("Cenário", "Personagens"…). */
  label: string;
  /** Resolve quando a etapa termina (rejeitar conta como terminada, com a falha registrada). */
  wait: () => Promise<unknown>;
}

export interface ReadyOptions {
  /** Teto para o conjunto das etapas, em ms. */
  timeoutMs?: number;
  /** Chamado ao começar cada etapa (index 0-based) e ao fim de todas (index = total). */
  onStage?: (index: number, total: number, label: string) => void;
  /** Etapa que rejeitou (a entrada continua). */
  onStageError?: (label: string, err: unknown) => void;
  /** Teto estourado, com a etapa em que parou. */
  onTimeout?: (label: string) => void;
}

export interface ReadyResult {
  timedOut: boolean;
  failed: string[];
}

export const READY_TIMEOUT_MS = 25_000;

/** Espera as etapas em ordem, dentro do teto de tempo. Nunca rejeita. */
export async function waitWorldReady(stages: ReadyStage[], opts: ReadyOptions = {}): Promise<ReadyResult> {
  const timeoutMs = opts.timeoutMs ?? READY_TIMEOUT_MS;
  const result: ReadyResult = { timedOut: false, failed: [] };
  let current = stages[0]?.label ?? "";
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<"timeout">((resolve) => {
    timer = setTimeout(() => resolve("timeout"), timeoutMs);
  });
  const run = (async () => {
    for (let i = 0; i < stages.length; i++) {
      const st = stages[i];
      current = st.label;
      opts.onStage?.(i, stages.length, st.label);
      try {
        await st.wait();
      } catch (err) {
        result.failed.push(st.label);
        opts.onStageError?.(st.label, err);
      }
    }
    opts.onStage?.(stages.length, stages.length, "");
    return "done" as const;
  })();
  const winner = await Promise.race([run, timeout]);
  clearTimeout(timer);
  if (winner === "timeout") {
    result.timedOut = true;
    opts.onTimeout?.(current);
  }
  return result;
}

/** Resolve quando `n` quadros foram desenhados (observable = scene.onAfterRenderObservable). */
export function afterFrames(
  observable: { add(fn: () => void): unknown; remove(o: never): unknown },
  n: number,
): Promise<void> {
  return new Promise((resolve) => {
    let left = n;
    const obs = observable.add(() => {
      if (--left > 0) return;
      observable.remove(obs as never);
      resolve();
    });
  });
}
