/** Cliente da API HTTP do servidor (mesma origem; a sessão vai no cookie). */

export type ClassId = "knight" | "barbarian" | "mage" | "rogue";

export interface CharacterInfo {
  id: number;
  name: string;
  class: ClassId;
  level?: number;
  /** Cor do emblema (#rrggbb; o servidor manda a padrão da classe se não houver). */
  emblem?: string;
  /** Modelos 3D das armas que estão nas mãos agora e a raridade da principal. */
  look?: { right: string; left: string; grade: number };
}

/** O que o jogador escolhe na criação (ids de item do kit inicial; "" = mão vazia). */
export interface LookChoice {
  weapon: string;
  offhand: string;
  emblem: string;
}

export interface CreationOption {
  id: string;
  name: string;
  model: string;
  offModel?: string;
}

/** Opções de aparência de cada classe, vindas do servidor (a primeira é a padrão). */
export interface CreationOptions {
  classes: Record<ClassId, { weapons: CreationOption[]; offhands: CreationOption[]; emblem: string }>;
  emblems: string[];
}

export interface Providers {
  google: boolean;
  discord: boolean;
  dev: boolean;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, "Sem conexão com o servidor.");
  }
  const data = (await res.json().catch(() => ({}))) as { error?: string };
  if (!res.ok) throw new ApiError(res.status, data.error ?? `Erro ${res.status}`);
  return data as T;
}

export const api = {
  providers: () => request<Providers>("GET", "/api/auth/providers"),
  me: () => request<{ accountId: number; displayName: string }>("GET", "/api/me"),
  devLogin: (username: string) => request<{ ok: boolean }>("POST", "/api/auth/dev", { username }),
  logout: () => request<{ ok: boolean }>("POST", "/api/auth/logout", {}),
  characters: () => request<{ characters: CharacterInfo[]; max: number }>("GET", "/api/characters"),
  createCharacter: (name: string, cls: ClassId, look: LookChoice) =>
    request<CharacterInfo>("POST", "/api/characters", { name, class: cls, look }),
  creation: () => request<CreationOptions>("GET", "/api/creation"),
};
