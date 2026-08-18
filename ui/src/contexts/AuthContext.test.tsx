// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { makeToken } from "@/test/jwt";
import { AuthProvider, useAuth } from "./AuthContext";

vi.mock("@/queries/auth", () => ({ refresh: vi.fn() }));
const { refresh } = await import("@/queries/auth");
const refreshMock = vi.mocked(refresh);

const Probe = () => {
  const { email, role, accessToken } = useAuth();
  return (
    <div>
      <span data-testid="email">{email ?? "-"}</span>
      <span data-testid="role">{role ?? "-"}</span>
      <span data-testid="token">{accessToken ? "present" : "absent"}</span>
    </div>
  );
};

const mountWithNavToken = (token: string) =>
  renderWithProviders(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
    { initialEntries: [{ pathname: "/dashboard", state: { token } }] },
  );

const flush = async (ms = 0) => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
};

const fireVisible = () => {
  Object.defineProperty(document, "visibilityState", {
    value: "visible",
    configurable: true,
  });
  document.dispatchEvent(new Event("visibilitychange"));
};

beforeEach(() => {
  vi.useFakeTimers();
  refreshMock.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("AuthProvider token handling", () => {
  it("adopts a token passed through navigation state without calling refresh", async () => {
    mountWithNavToken(makeToken({ expiresInSec: 3600, roleId: 1 }));
    await flush();

    expect(screen.getByTestId("token")).toHaveTextContent("present");
    expect(screen.getByTestId("role")).toHaveTextContent("Admin");
    expect(refreshMock).not.toHaveBeenCalled();
  });

  it("maps role ids to labels", async () => {
    mountWithNavToken(makeToken({ expiresInSec: 3600, roleId: 3 }));
    await flush();

    expect(screen.getByTestId("role")).toHaveTextContent("Network Manager");
  });

  it("refreshes on mount when no token is supplied", async () => {
    refreshMock.mockResolvedValue({ token: makeToken({ expiresInSec: 3600 }) });

    renderWithProviders(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await flush();

    expect(screen.getByTestId("token")).toHaveTextContent("present");
    expect(refreshMock).toHaveBeenCalledTimes(1);
  });

  it("arms exactly one refresh chain when a visibility change interrupts a pending timer", async () => {
    refreshMock.mockImplementation(async () => ({
      token: makeToken({ expiresInSec: 60 }),
    }));

    mountWithNavToken(makeToken({ expiresInSec: 60 }));
    await flush();
    expect(refreshMock).toHaveBeenCalledTimes(0);

    await flush(2000);
    fireVisible();
    await flush();
    expect(refreshMock).toHaveBeenCalledTimes(1);

    await flush(30_000);
    expect(refreshMock).toHaveBeenCalledTimes(7);
  });
});
