// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { useEffect } from "react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { useSearchParamState } from "./useSearchParamState";

let setUser: (v: string) => void;
let setAction: (v: string) => void;

const Probe = () => {
  const [user, setUserParam] = useSearchParamState("user");
  const [action, setActionParam] = useSearchParamState("action");
  const location = useLocation();

  useEffect(() => {
    setUser = setUserParam;
    setAction = setActionParam;
  });

  return (
    <>
      <span data-testid="user">{user}</span>
      <span data-testid="action">{action}</span>
      <span data-testid="search">{location.search}</span>
    </>
  );
};

const renderAt = (search: string) =>
  render(
    <MemoryRouter initialEntries={[`/audit-logs${search}`]}>
      <Probe />
    </MemoryRouter>,
  );

const value = (id: string) => screen.getByTestId(id).textContent;

describe("useSearchParamState", () => {
  it("is empty when the param is absent", () => {
    renderAt("");
    expect(value("user")).toBe("");
  });

  it("seeds from the URL", () => {
    renderAt("?user=admin@ella.com");
    expect(value("user")).toBe("admin@ella.com");
  });

  it("writes the new value back to the URL", () => {
    renderAt("");
    act(() => setUser("admin@ella.com"));
    expect(value("user")).toBe("admin@ella.com");
    expect(value("search")).toBe("?user=admin%40ella.com");
  });

  it("drops the param when cleared", () => {
    renderAt("?user=admin@ella.com");
    act(() => setUser(""));
    expect(value("user")).toBe("");
    expect(value("search")).toBe("");
  });

  it("leaves other params untouched", () => {
    renderAt("?event=42&user=admin@ella.com");
    act(() => setUser("operator@ella.com"));
    expect(value("search")).toContain("event=42");
  });

  it("keeps independent keys separate", () => {
    renderAt("");
    act(() => setUser("admin@ella.com"));
    act(() => setAction("create_subscriber"));
    expect(value("user")).toBe("admin@ella.com");
    expect(value("action")).toBe("create_subscriber");
  });
});
