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

const mount = () =>
  renderWithProviders(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
    { initialEntries: ["/dashboard"] },
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
  it("obtains its access token from the session cookie on mount", async () => {
    refreshMock.mockResolvedValue({ token: makeToken({ expiresInSec: 3600 }) });

    mount();
    await flush();

    expect(screen.getByTestId("token")).toHaveTextContent("present");
    expect(refreshMock).toHaveBeenCalledTimes(1);
  });

  it("never reads a token from browser history state", async () => {
    refreshMock.mockResolvedValue({ token: makeToken({ expiresInSec: 3600 }) });

    renderWithProviders(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
      {
        initialEntries: [
          {
            pathname: "/dashboard",
            state: { token: makeToken({ roleId: 1 }) },
          },
        ],
      },
    );
    await flush();

    expect(refreshMock).toHaveBeenCalledTimes(1);
  });

  it("maps role ids to labels", async () => {
    refreshMock.mockResolvedValue({
      token: makeToken({ expiresInSec: 3600, roleId: 3 }),
    });

    mount();
    await flush();

    expect(screen.getByTestId("role")).toHaveTextContent("Network Manager");
  });

  it("clears the session when the refresh call fails", async () => {
    refreshMock.mockRejectedValue(new Error("no session"));

    mount();
    await flush();

    expect(screen.getByTestId("token")).toHaveTextContent("absent");
    expect(screen.getByTestId("email")).toHaveTextContent("-");
  });

  it("arms exactly one refresh chain when a visibility change interrupts a pending timer", async () => {
    refreshMock.mockImplementation(async () => ({
      token: makeToken({ expiresInSec: 60 }),
    }));

    mount();
    await flush();
    expect(refreshMock).toHaveBeenCalledTimes(1);

    await flush(2000);
    fireVisible();
    await flush();
    expect(refreshMock).toHaveBeenCalledTimes(2);

    await flush(30_000);

    expect(refreshMock).toHaveBeenCalledTimes(8);
  });
});
