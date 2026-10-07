import { z } from "zod";
import { api } from "./client";

/** 13-Frontend §8 단일 사용자 로그인. 서버가 JWT를 HttpOnly 쿠키로 발급하므로 토큰은 다루지 않는다. */
export const Me = z.object({ user: z.string(), auth_enabled: z.boolean() });

export const fetchMe = () => api.get("/api/v1/me", Me);

export const login = (password: string) => api.post<unknown>("/api/v1/auth/login", { password }).then(() => undefined);

export const logout = () => api.post<void>("/api/v1/auth/logout");
