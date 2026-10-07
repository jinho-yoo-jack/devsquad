import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LoginForm } from "@/components/auth/LoginForm";
import { SessionMenu } from "@/components/auth/SessionMenu";
import { ApiError, api, loginRedirect, safeNext, session } from "@/lib/api/client";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

const respond = (status: number, body: unknown) => vi.fn(async () => new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));

describe("unauthorized responses", () => {
  it("send the user to login and come back afterwards", () => {
    expect(loginRedirect("/tasks/abc", "?x=1")).toBe("/login?next=%2Ftasks%2Fabc%3Fx%3D1");
    expect(loginRedirect("/login", "?next=%2Ftasks")).toBeNull();
  });

  it("only return to pages of this site", () => {
    expect(safeNext("/tasks/abc?x=1")).toBe("/tasks/abc?x=1");
    for (const target of [null, "", "https://evil.example", "//evil.example", "/\\evil.example", "javascript:alert(1)"]) expect(safeNext(target)).toBe("/tasks");
  });

  it("are reported through the session hook and still reject the request", async () => {
    vi.stubGlobal("fetch", respond(401, { code: "UNAUTHORIZED", message: "authentication required", details: {} }));
    const hook = vi.spyOn(session, "onUnauthorized").mockImplementation(() => {});
    await expect(api.get("/api/v1/tasks")).rejects.toMatchObject({ status: 401, code: "UNAUTHORIZED" });
    expect(hook).toHaveBeenCalledOnce();
  });
});

describe("LoginForm", () => {
  it("submits the password and shows a rejected login", async () => {
    const onLogin = vi.fn().mockRejectedValueOnce(new ApiError(401, "UNAUTHORIZED", "invalid password")).mockResolvedValueOnce(undefined);
    render(<LoginForm onLogin={onLogin} />);
    const submit = screen.getByRole("button", { name: "로그인" }) as HTMLButtonElement;
    expect(submit.disabled).toBe(true);
    fireEvent.change(screen.getByLabelText("관리자 비밀번호"), { target: { value: "secret" } });
    fireEvent.click(submit);
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toContain("비밀번호가 올바르지 않습니다");
    fireEvent.click(submit);
    await waitFor(() => expect(onLogin).toHaveBeenCalledTimes(2));
    expect(onLogin).toHaveBeenLastCalledWith("secret");
  });
});

describe("SessionMenu", () => {
  const menu = () => render(<QueryClientProvider client={new QueryClient()}><SessionMenu onLoggedOut={vi.fn()} /></QueryClientProvider>);

  it("shows the user and logs out when authentication is on", async () => {
    const fetch = vi.fn(async (path: string) => path.endsWith("/logout") ? new Response(null, { status: 204 })
      : new Response(JSON.stringify({ user: "admin", auth_enabled: true }), { status: 200 }));
    vi.stubGlobal("fetch", fetch);
    menu();
    fireEvent.click(await screen.findByRole("button", { name: "로그아웃" }));
    await waitFor(() => expect(fetch).toHaveBeenCalledWith("/api/v1/auth/logout", expect.objectContaining({ method: "POST" })));
    expect(screen.getByText("admin")).toBeTruthy();
  });

  it("stays hidden when authentication is off", async () => {
    vi.stubGlobal("fetch", respond(200, { user: "local-user", auth_enabled: false }));
    menu();
    await waitFor(() => expect(vi.mocked(fetch)).toHaveBeenCalled());
    expect(screen.queryByRole("button", { name: "로그아웃" })).toBeNull();
  });
});
