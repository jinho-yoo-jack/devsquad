import type { ZodType, ZodTypeDef } from "zod";

/** 입력(스네이크·optional)과 출력(default 적용) 타입이 다른 스키마를 받기 위해 Input 을 unknown 으로 둔다. */
type Schema<T> = ZodType<T, ZodTypeDef, unknown>;

/** 15-API 명세 공통 오류 응답. */
export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public details: Record<string, unknown> = {}) {
    super(message);
  }
}

const USER_HEADER = { "X-User": "local-user" }; // Phase 1 단일 사용자. JWT 는 CP-10.

async function request<T>(method: string, path: string, body?: unknown, schema?: Schema<T>): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json", ...USER_HEADER },
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: "no-store",
  });
  if (!res.ok) {
    let code = `HTTP_${res.status}`;
    let message = res.statusText;
    let details: Record<string, unknown> = {};
    try {
      const j = await res.json();
      code = j.code ?? code;
      message = j.message ?? message;
      details = j.details ?? {};
    } catch {
      /* 본문 없음 */
    }
    throw new ApiError(res.status, code, message, details);
  }
  if (res.status === 204) return undefined as T;
  const json = await res.json();
  return schema ? schema.parse(json) : (json as T);
}

export const api = {
  get: <T>(path: string, schema?: Schema<T>) => request<T>("GET", path, undefined, schema),
  post: <T>(path: string, body?: unknown, schema?: Schema<T>) => request<T>("POST", path, body, schema),
};
