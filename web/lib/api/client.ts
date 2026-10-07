import type { ZodType, ZodTypeDef } from "zod";

/** 입력(스네이크·optional)과 출력(default 적용) 타입이 다른 스키마를 받기 위해 Input 을 unknown 으로 둔다. */
type Schema<T> = ZodType<T, ZodTypeDef, unknown>;

/** 15-API 명세 공통 오류 응답. */
export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public details: Record<string, unknown> = {}) {
    super(message);
  }
}

const USER_HEADER = { "X-User": "local-user" }; // 인증이 꺼진 로컬 개발에서만 쓰인다. 켜져 있으면 서버는 쿠키의 JWT 사용자를 쓴다.

/** 401이면 로그인 화면으로 보내고, 로그인 후 원래 화면으로 돌아온다. 로그인 화면 자신은 제외한다. */
export function loginRedirect(pathname: string, search: string): string | null {
  if (pathname === "/login") return null;
  return `/login?next=${encodeURIComponent(pathname + search)}`;
}

/** 로그인 후 이동할 경로. 이 사이트의 경로만 허용해 외부로 보내지 않는다. */
export function safeNext(next: string | null): string {
  return next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/\\") ? next : "/tasks";
}

export const session = {
  onUnauthorized() {
    if (typeof window === "undefined") return;
    const target = loginRedirect(window.location.pathname, window.location.search);
    if (target) window.location.assign(target);
  },
};

async function request<T>(method: string, path: string, body?: unknown, schema?: Schema<T>): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json", ...USER_HEADER },
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: "no-store",
  });
  if (!res.ok) {
    if (res.status === 401) session.onUnauthorized();
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
