// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ThemeProvider, useTheme } from "@mui/material/styles";
import theme, { THEME_PROVIDER_PROPS } from "@/utils/theme";
import { dark, light } from "@/utils/tokens";

const Probe = () => {
  const t = useTheme();
  return (
    <>
      <span data-testid="primary">{t.palette.primary.main}</span>
      <span data-testid="link">{t.palette.link}</span>
      <span data-testid="info">{t.palette.info.main}</span>
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
      {...THEME_PROVIDER_PROPS}
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
    expect(value("info")).toBe(light.info);
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
    expect(value("info")).toBe(dark.info);
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
    ["info", dark.info],
    ["link", dark.link],
  ])("keeps %s readable on every dark surface", (_name, color) => {
    for (const surface of surfaces) {
      expect(contrast(color, surface)).toBeGreaterThanOrEqual(WCAG_AA);
    }
  });

  it.each([...Object.values(dark.chart.protocols), ...dark.chart.series])(
    "keeps a %s chip readable with the text MUI picks",
    (color) => {
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

  it("steps light paper to subtle as far as dark does", () => {
    expect(
      contrast(light.backgroundPaper, light.backgroundSubtle),
    ).toBeGreaterThanOrEqual(
      contrast(dark.backgroundPaper, dark.backgroundSubtle) - 0.01,
    );
  });

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
  const WCAG_NON_TEXT = 3;

  it.each([...Object.values(light.chart.protocols), ...light.chart.series])(
    "keeps a %s chip readable with the text MUI picks",
    (color) => {
      expect(bestTextContrast(color)).toBeGreaterThanOrEqual(WCAG_AA);
    },
  );

  it.each(light.chart.series.map((c, i) => [i, c] as const))(
    "keeps chart series %s above the non-text floor",
    (_index, color) => {
      expect(contrast(color, light.backgroundDefault)).toBeGreaterThanOrEqual(
        WCAG_NON_TEXT,
      );
    },
  );

  it("keeps the uplink and downlink marks above the non-text floor", () => {
    expect(
      contrast(light.chart.uplink, light.backgroundDefault),
    ).toBeGreaterThanOrEqual(WCAG_NON_TEXT);
    expect(
      contrast(light.chart.downlink, light.backgroundDefault),
    ).toBeGreaterThanOrEqual(WCAG_NON_TEXT);
  });

  it.each([
    ["primary", light.primary],
    ["success", light.success],
    ["error", light.error],
    ["info", light.info],
    ["link", light.link],
  ])("keeps %s readable as text on the page", (_name, color) => {
    expect(contrast(color, light.backgroundDefault)).toBeGreaterThanOrEqual(
      WCAG_AA,
    );
  });
});

describe("control boundaries", () => {
  const WCAG_NON_TEXT = 3;

  it("keeps the light input outline above the non-text floor", () => {
    expect(
      contrast(
        over("#000000", 0.42, light.backgroundPaper),
        light.backgroundPaper,
      ),
    ).toBeGreaterThanOrEqual(WCAG_NON_TEXT);
  });

  it.each([
    ["paper", dark.backgroundPaper],
    ["subtle", dark.backgroundSubtle],
  ])(
    "keeps the dark input outline above the non-text floor on %s",
    (_n, surface) => {
      expect(
        contrast(over("#FFFFFF", 0.36, surface), surface),
      ).toBeGreaterThanOrEqual(WCAG_NON_TEXT);
    },
  );
});
