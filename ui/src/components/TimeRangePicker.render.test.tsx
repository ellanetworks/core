// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, beforeAll, afterAll } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { useState } from "react";
import TimeRangePicker, {
  CUSTOM_RANGE,
  DAILY_RANGES,
  RELATIVE_RANGES,
  type TimeRangeValue,
} from "./TimeRangePicker";

const ERROR_ID = "time-range-error";

const originalTz = process.env.TZ;

beforeAll(() => {
  process.env.TZ = "America/Toronto";
});

afterAll(() => {
  process.env.TZ = originalTz;
});

const Harness = ({
  initial,
  onChange,
  allowAnyTime = true,
  ranges = RELATIVE_RANGES,
}: {
  initial: TimeRangeValue;
  onChange?: (next: TimeRangeValue) => void;
  allowAnyTime?: boolean;
  ranges?: typeof RELATIVE_RANGES;
}) => {
  const [value, setValue] = useState(initial);
  return (
    <TimeRangePicker
      value={value}
      onChange={(next) => {
        setValue(next);
        onChange?.(next);
      }}
      errorId={ERROR_ID}
      ranges={ranges}
      allowAnyTime={allowAnyTime}
    />
  );
};

const trigger = () => screen.getByRole("button", { name: /^Time range:/ });

const open = async () => {
  fireEvent.click(trigger());
  await screen.findByLabelText("From");
};

const close = async () => {
  fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
  await waitFor(() =>
    expect(screen.queryByRole("menu")).not.toBeInTheDocument(),
  );
};

describe("TimeRangePicker", () => {
  it("applies a preset and closes the panel", async () => {
    const onChange = vi.fn();
    render(
      <Harness
        initial={{ preset: "1h", from: "", to: "" }}
        onChange={onChange}
      />,
    );
    await open();

    fireEvent.click(screen.getByRole("menuitem", { name: "Last 15 minutes" }));

    await waitFor(() =>
      expect(screen.queryByRole("menu")).not.toBeInTheDocument(),
    );
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ preset: "15m" }),
    );
    expect(trigger()).toHaveTextContent("Last 15 minutes");
  });

  it("does not apply a range whose end precedes its start", async () => {
    const onChange = vi.fn();
    render(
      <Harness
        initial={{ preset: "1h", from: "", to: "" }}
        onChange={onChange}
      />,
    );
    await open();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    onChange.mockClear();
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /must be on or after/i,
    );
    expect(onChange).not.toHaveBeenCalled();
  });

  it("describes both bounds with the reason the range is invalid", async () => {
    render(<Harness initial={{ preset: "1h", from: "", to: "" }} />);
    await open();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findByRole("alert");

    for (const label of ["From", "To"]) {
      expect(screen.getByLabelText(label)).toHaveAccessibleDescription(
        /must be on or after/i,
      );
    }
  });

  it("discards an invalid edit when the panel is closed", async () => {
    render(<Harness initial={{ preset: "1h", from: "", to: "" }} />);
    await open();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findByRole("alert");
    await close();

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(trigger()).toHaveTextContent(/^Aug 10, 10:00 → /);
    expect(trigger()).not.toHaveTextContent("Aug 1, 10:00");
  });

  it("stops the picker offering an end before the start", async () => {
    render(<Harness initial={{ preset: "1h", from: "", to: "" }} />);
    await open();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });

    await waitFor(() =>
      expect(screen.getByLabelText("To")).toHaveAttribute(
        "min",
        "2026-08-10T10:00",
      ),
    );
  });

  it("requires both bounds when no open range is allowed", async () => {
    const onChange = vi.fn();
    render(
      <Harness
        initial={{ preset: "7d", from: "", to: "" }}
        onChange={onChange}
        allowAnyTime={false}
        ranges={DAILY_RANGES}
      />,
    );
    await open();
    onChange.mockClear();

    fireEvent.change(screen.getByLabelText("From"), { target: { value: "" } });

    expect(await screen.findByRole("alert")).toHaveTextContent(/enter a date/i);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.queryByRole("menuitem", { name: "Any time" })).toBeNull();
  });

  it("switches to a custom range once a bound is typed", async () => {
    const onChange = vi.fn();
    render(
      <Harness
        initial={{ preset: "1h", from: "", to: "" }}
        onChange={onChange}
      />,
    );
    await open();

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ preset: CUSTOM_RANGE }),
    );
  });

  it("names the browser time zone in the footer", async () => {
    render(<Harness initial={{ preset: "1h", from: "", to: "" }} />);
    await open();

    expect(screen.getByText(/^Browser time:/)).toBeInTheDocument();
  });
});
