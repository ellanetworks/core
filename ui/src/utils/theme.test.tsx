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
      <span data-testid="series0">{t.palette.chart.series[0]}</span>
      <span data-testid="protocol6">{t.palette.chart.protocols[6]}</span>
      <span data-testid="protocolText">{t.palette.chart.protocolText}</span>
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

const DARK_DEFAULT = "#121212";
const WCAG_AA = 4.5;

describe("theme color schemes", () => {
  it("resolves the light tokens", () => {
    renderIn("light");

    expect(value("mode")).toBe("light");
    expect(value("primary")).toBe(light.primary);
    expect(value("link")).toBe(light.link);
    expect(value("subtle")).toBe(light.backgroundSubtle);
    expect(value("series0")).toBe(light.chart.series[0]);
    expect(value("protocol6")).toBe(light.chart.protocols[6]);
    expect(value("protocolText")).toBe(light.chart.protocolText);
    expect(value("headerBg")).toBe(light.backgroundSubtle);
  });

  it("resolves the dark tokens", () => {
    renderIn("dark");

    expect(value("mode")).toBe("dark");
    expect(value("primary")).toBe(dark.primary);
    expect(value("link")).toBe(dark.link);
    expect(value("subtle")).toBe(dark.backgroundSubtle);
    expect(value("series0")).toBe(dark.chart.series[0]);
    expect(value("protocol6")).toBe(dark.chart.protocols[6]);
    expect(value("protocolText")).toBe(dark.chart.protocolText);
    expect(value("headerBg")).toBe(dark.backgroundSubtle);
  });
});

describe("dark palette contrast", () => {
  const surfaces = [DARK_DEFAULT, dark.backgroundSubtle];

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
      expect(contrast(color, dark.chart.protocolText)).toBeGreaterThanOrEqual(
        WCAG_AA,
      );
    },
  );

  it.each(dark.chart.series.map((c, i) => [i, c] as const))(
    "keeps chart series %s visible on the dark background",
    (_index, color) => {
      expect(contrast(color, DARK_DEFAULT)).toBeGreaterThanOrEqual(WCAG_AA);
    },
  );

  it("keeps the uplink and downlink series visible", () => {
    expect(contrast(dark.chart.uplink, DARK_DEFAULT)).toBeGreaterThanOrEqual(
      WCAG_AA,
    );
    expect(contrast(dark.chart.downlink, DARK_DEFAULT)).toBeGreaterThanOrEqual(
      WCAG_AA,
    );
  });
});
