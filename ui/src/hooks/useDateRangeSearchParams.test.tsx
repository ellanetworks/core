// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { useEffect } from "react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { defaultDateRange } from "@/utils/dates";
import { useDateRangeSearchParams } from "./useDateRangeSearchParams";

let setStart: (v: string) => void;
let setEnd: (v: string) => void;

const Probe = () => {
  const { startDate, endDate, handleStartChange, handleEndChange } =
    useDateRangeSearchParams();
  const location = useLocation();

  useEffect(() => {
    setStart = (v) =>
      handleStartChange({
        target: { value: v },
      } as React.ChangeEvent<HTMLInputElement>);
    setEnd = (v) =>
      handleEndChange({
        target: { value: v },
      } as React.ChangeEvent<HTMLInputElement>);
  });

  return (
    <>
      <span data-testid="start">{startDate}</span>
      <span data-testid="end">{endDate}</span>
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

describe("useDateRangeSearchParams", () => {
  it("falls back to the default range when the URL has none", () => {
    renderAt("");
    const { startDate, endDate } = defaultDateRange();
    expect(value("start")).toBe(startDate);
    expect(value("end")).toBe(endDate);
  });

  it("leaves the URL alone until a date is changed", () => {
    renderAt("");
    expect(value("search")).toBe("");
  });

  it("pins both ends of the range once one is changed", () => {
    renderAt("");
    const { endDate } = defaultDateRange();
    act(() => setStart("2026-01-15"));
    expect(value("search")).toContain("start=2026-01-15");
    act(() => setEnd(endDate));
    expect(value("search")).toContain(`end=${endDate}`);
  });

  it("seeds from the URL", () => {
    renderAt("?start=2026-01-01&end=2026-01-31");
    expect(value("start")).toBe("2026-01-01");
    expect(value("end")).toBe("2026-01-31");
  });

  it("writes a changed date back to the URL", () => {
    renderAt("?start=2026-01-01&end=2026-01-31");
    act(() => setStart("2026-01-15"));
    expect(value("start")).toBe("2026-01-15");
    expect(value("search")).toContain("start=2026-01-15");
    expect(value("search")).toContain("end=2026-01-31");
  });

  it("keeps a cleared date empty rather than restoring the default", () => {
    renderAt("?start=2026-01-01&end=2026-01-31");
    act(() => setEnd(""));
    expect(value("end")).toBe("");
    expect(value("search")).toContain("end=");
  });

  it("leaves other params untouched", () => {
    renderAt("?user=admin@ella.com");
    act(() => setStart("2026-01-15"));
    expect(value("search")).toContain("user=admin%40ella.com");
  });
});
