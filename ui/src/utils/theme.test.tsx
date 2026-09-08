// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ThemeProvider, useTheme } from "@mui/material/styles";
import theme from "@/utils/theme";
import { dark, light } from "@/utils/tokens";

const Probe = () => {
  const t = useTheme();
  return (
    <>
      <span data-testid="primary">{t.palette.primary.main}</span>
      <span data-testid="link">{t.palette.link}</span>
      <span data-testid="subtle">{t.palette.backgroundSubtle}</span>
      <span data-testid="canvas">{t.palette.background.default}</span>
      <span data-testid="paper">{t.palette.background.paper}</span>
      <span data-testid="text">{t.palette.text.primary}</span>
      <span data-testid="series0">{t.palette.chart.series[0]}</span>
      <span data-testid="protocol6">{t.palette.chart.protocols[6]}</span>
      <span data-testid="headerBg">{t.palette.DataGrid.headerBg}</span>
      <span data-testid="mode">{t.palette.mode}</span>
    </>
  );
};

const renderIn = (mode: "light" | "dark") =>
  render(
    <ThemeProvider
      theme={theme}
      noSsr
      forceThemeRerender
      defaultMode={mode}
      storageManager={null}
    >
      <Probe />
    </ThemeProvider>,
  );

const value = (id: string) => screen.getByTestId(id).textContent;

const relativeLuminance = (hex: string) => {
  const channels = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
};

const contrast = (a: string, b: string) => {
  const [hi, lo] = [relativeLuminance(a), relativeLuminance(b)].sort(
    (x, y) => y - x,
  );
  return (hi + 0.05) / (lo + 0.05);
};

const WCAG_AA = 4.5;

const over = (hex: string, alpha: number, background: string) => {
  const channel = (i: number) =>
    Math.round(
      alpha * parseInt(hex.slice(i, i + 2), 16) +
        (1 - alpha) * parseInt(background.slice(i, i + 2), 16),
    );
  return `#${[1, 3, 5].map((i) => channel(i).toString(16).padStart(2, "0")).join("")}`;
};

const bestTextContrast = (background: string) =>
  Math.max(
    contrast("#FFFFFF", background),
    contrast(over("#000000", 0.87, background), background),
  );

describe("theme color schemes", () => {
  it("resolves the light tokens", () => {
    renderIn("light");

    expect(value("mode")).toBe("light");
    expect(value("primary")).toBe(light.primary);
    expect(value("link")).toBe(light.link);
    expect(value("subtle")).toBe(light.backgroundSubtle);
    expect(value("canvas")).toBe(light.backgroundDefault);
    expect(value("paper")).toBe(light.backgroundPaper);
    expect(value("text")).toBe(light.textPrimary);
    expect(value("series0")).toBe(light.chart.series[0]);
    expect(value("protocol6")).toBe(light.chart.protocols[6]);
    expect(value("headerBg")).toBe(light.backgroundSubtle);
  });

  it("resolves the dark tokens", () => {
    renderIn("dark");

    expect(value("mode")).toBe("dark");
    expect(value("primary")).toBe(dark.primary);
    expect(value("link")).toBe(dark.link);
    expect(value("subtle")).toBe(dark.backgroundSubtle);
    expect(value("canvas")).toBe(dark.backgroundDefault);
    expect(value("paper")).toBe(dark.backgroundPaper);
    expect(value("text")).toBe(dark.textPrimary);
    expect(value("series0")).toBe(dark.chart.series[0]);
    expect(value("protocol6")).toBe(dark.chart.protocols[6]);
    expect(value("headerBg")).toBe(dark.backgroundSubtle);
  });
});

describe("dark palette contrast", () => {
  const surfaces = [
    dark.backgroundDefault,
    dark.backgroundPaper,
    dark.backgroundSubtle,
  ];

  it.each([
    ["primary", dark.primary],
    ["success", dark.success],
    ["error", dark.error],
    ["warning", dark.warning],
    ["link", dark.link],
  ])("keeps %s readable on every dark surface", (_name, color) => {
    for (const surface of surfaces) {
      expect(contrast(color, surface)).toBeGreaterThanOrEqual(WCAG_AA);
    }
  });

  it.each(Object.entries(dark.chart.protocols))(
    "keeps the protocol %s chip readable",
    (_protocol, color) => {
      expect(bestTextContrast(color)).toBeGreaterThanOrEqual(WCAG_AA);
    },
  );

  it.each(dark.chart.series.map((c, i) => [i, c] as const))(
    "keeps chart series %s visible on the surfaces charts sit on",
    (_index, color) => {
      for (const surface of [dark.backgroundDefault, dark.backgroundPaper]) {
        expect(contrast(color, surface)).toBeGreaterThanOrEqual(WCAG_AA);
      }
    },
  );

  it("separates the dark surfaces from each other", () => {
    expect(
      contrast(dark.backgroundDefault, dark.backgroundPaper),
    ).toBeGreaterThan(1.08);
    expect(
      contrast(dark.backgroundPaper, dark.backgroundSubtle),
    ).toBeGreaterThan(1.2);
  });

  it("keeps body text readable on every dark surface", () => {
    for (const surface of surfaces) {
      expect(contrast(dark.textPrimary, surface)).toBeGreaterThanOrEqual(7);
    }
  });

  it("keeps the uplink and downlink series visible", () => {
    for (const surface of [dark.backgroundDefault, dark.backgroundPaper]) {
      expect(contrast(dark.chart.uplink, surface)).toBeGreaterThanOrEqual(
        WCAG_AA,
      );
      expect(contrast(dark.chart.downlink, surface)).toBeGreaterThanOrEqual(
        WCAG_AA,
      );
    }
  });
});

describe("light palette contrast", () => {
  it.each(Object.entries(light.chart.protocols))(
    "keeps the protocol %s chip readable",
    (_protocol, color) => {
      expect(bestTextContrast(color)).toBeGreaterThanOrEqual(WCAG_AA);
    },
  );
});
