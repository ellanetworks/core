// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { useEffect, useState } from "react";
import { useDebouncedValue } from "./useDebouncedValue";

let setValue: (v: string) => void;

const Probe = ({ delay }: { delay?: number }) => {
  const [value, setState] = useState("");

  useEffect(() => {
    setValue = setState;
  }, []);

  return <span data-testid="debounced">{useDebouncedValue(value, delay)}</span>;
};

const debounced = () => screen.getByTestId("debounced").textContent;

const type = (v: string) => act(() => setValue(v));

const advance = (ms: number) =>
  act(() => {
    vi.advanceTimersByTime(ms);
  });

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useDebouncedValue", () => {
  it("starts at the initial value", () => {
    render(<Probe />);
    expect(debounced()).toBe("");
  });

  it("withholds the new value until the delay elapses", () => {
    render(<Probe delay={400} />);

    type("00101");
    advance(399);
    expect(debounced()).toBe("");

    advance(1);
    expect(debounced()).toBe("00101");
  });

  it("only emits the last value of a burst", () => {
    render(<Probe delay={400} />);

    type("0");
    advance(100);
    type("00");
    advance(100);
    type("001");
    advance(400);

    expect(debounced()).toBe("001");
  });

  it("emits a cleared value like any other", () => {
    render(<Probe delay={400} />);

    type("001");
    advance(400);
    type("");
    advance(400);

    expect(debounced()).toBe("");
  });
});
