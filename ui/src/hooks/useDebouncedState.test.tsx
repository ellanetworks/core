// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { useEffect } from "react";
import { useDebouncedState } from "./useDebouncedState";

let setValue: (v: string) => void;
let applyNow: (v: string) => void;

const Probe = ({ delay }: { delay?: number }) => {
  const [, applied, setState, apply] = useDebouncedState("", delay);

  useEffect(() => {
    setValue = setState;
    applyNow = apply;
  }, [setState, apply]);

  return <span data-testid="debounced">{applied}</span>;
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

describe("useDebouncedState", () => {
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

  it("applies a value at once when asked, without waiting for the delay", () => {
    render(<Probe delay={400} />);

    act(() => applyNow("001"));

    expect(debounced()).toBe("001");
    expect(vi.getTimerCount()).toBe(0);
  });

  it("supersedes a pending debounce with the value applied at once", () => {
    render(<Probe delay={400} />);

    type("001");
    advance(399);
    act(() => applyNow("999"));
    advance(400);

    expect(debounced()).toBe("999");
  });

  it("cancels a pending debounce when it unmounts", () => {
    const view = render(<Probe delay={400} />);

    type("001");
    expect(vi.getTimerCount()).toBe(1);

    view.unmount();

    expect(vi.getTimerCount()).toBe(0);
  });
});
