// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { dark } from "../src/utils/tokens";
import {
  DEFAULT_BRANDING_DIR,
  brandingDir,
  contrast,
  loadBranding,
} from "./branding";

const dirs: string[] = [];

const brandDir = (config: Record<string, unknown>, files = ["logo.svg"]) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "branding-"));
  dirs.push(dir);
  fs.writeFileSync(path.join(dir, "branding.json"), JSON.stringify(config));
  for (const f of files) {
    fs.writeFileSync(path.join(dir, f), "x");
  }
  return dir;
};

const valid = {
  appName: "Northwind",
  logo: "logo.svg",
  favicon: "favicon.ico",
  colorPrimary: "#3a5a40",
};

afterEach(() => {
  for (const dir of dirs.splice(0)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

describe("loadBranding", () => {
  it("loads the default Ella branding", () => {
    const b = loadBranding(DEFAULT_BRANDING_DIR);

    expect(b.client).toEqual({
      appName: "Ella Core",
      logoUrl: "/logo-mark.svg",
      colorPrimary: "#26374A",
      colorPrimaryDark: "#5B9DFF",
      showProductName: true,
      poweredBy: false,
      topBar: null,
    });
    expect(b.favicons.map((f) => f.fileName)).toEqual([
      "favicon.ico",
      "favicon.svg",
    ]);
    expect(b.warnings).toEqual([]);
  });

  it("loads a customer directory", () => {
    const b = loadBranding(brandDir(valid, ["logo.svg", "favicon.ico"]));

    expect(b.client.appName).toBe("Northwind");
    expect(b.client.colorPrimary).toBe("#3A5A40");
    expect(b.favicons.map((f) => f.fileName)).toEqual(["favicon.ico"]);
    expect(b.client.showProductName).toBe(true);
    expect(b.client.poweredBy).toBe(true);
  });

  it("can hide the product name next to the logo", () => {
    const b = loadBranding(
      brandDir({ ...valid, showProductName: false }, [
        "logo.svg",
        "favicon.ico",
      ]),
    );

    expect(b.client.showProductName).toBe(false);
  });

  it("picks readable text for the top bar", () => {
    const deep = loadBranding(
      brandDir({ ...valid, colorTopBar: "#3A5A40" }, [
        "logo.svg",
        "favicon.ico",
      ]),
    );
    const bright = loadBranding(
      brandDir({ ...valid, colorTopBar: "#E0E0E0" }, [
        "logo.svg",
        "favicon.ico",
      ]),
    );

    expect(deep.client.topBar).toEqual({
      background: "#3A5A40",
      text: "#FFFFFF",
    });
    expect(bright.client.topBar?.text).toBe("rgba(0, 0, 0, 0.87)");
  });

  it("rejects a non-boolean showProductName", () => {
    const dir = brandDir({ ...valid, showProductName: "yes" }, [
      "logo.svg",
      "favicon.ico",
    ]);

    expect(() => loadBranding(dir)).toThrow(
      /showProductName must be true or false/,
    );
  });

  it("derives a readable dark primary when none is given", () => {
    const b = loadBranding(brandDir(valid, ["logo.svg", "favicon.ico"]));

    for (const surface of [dark.backgroundDefault, dark.backgroundPaper]) {
      expect(
        contrast(b.client.colorPrimaryDark, surface),
      ).toBeGreaterThanOrEqual(4.5);
    }
  });

  it("uses the logo extension in the served file name", () => {
    const b = loadBranding(
      brandDir({ ...valid, logo: "brand.png" }, ["brand.png", "favicon.ico"]),
    );

    expect(b.client.logoUrl).toBe("/logo-mark.png");
  });

  it("rejects unsupported fields", () => {
    const dir = brandDir({ ...valid, colorAccent: "#b5651d" }, [
      "logo.svg",
      "favicon.ico",
    ]);

    expect(() => loadBranding(dir)).toThrow(
      /unsupported field\(s\) colorAccent/,
    );
  });

  it("rejects invalid colors", () => {
    const dir = brandDir({ ...valid, colorPrimary: "blue" }, [
      "logo.svg",
      "favicon.ico",
    ]);

    expect(() => loadBranding(dir)).toThrow(/colorPrimary must be a hex color/);
  });

  it("rejects missing files", () => {
    expect(() => loadBranding(brandDir(valid))).toThrow(
      /favicon favicon.ico does not exist/,
    );
  });

  it("rejects unsupported file types", () => {
    const dir = brandDir({ ...valid, logo: "logo.gif" }, [
      "logo.gif",
      "favicon.ico",
    ]);

    expect(() => loadBranding(dir)).toThrow(/logo logo.gif must be one of/);
  });

  it("requires an app name", () => {
    const dir = brandDir({ ...valid, appName: " " }, [
      "logo.svg",
      "favicon.ico",
    ]);

    expect(() => loadBranding(dir)).toThrow(/appName must be a non-empty/);
  });

  it("warns about low-contrast colors", () => {
    const b = loadBranding(
      brandDir({ ...valid, colorPrimary: "#FFEE00" }, [
        "logo.svg",
        "favicon.ico",
      ]),
    );

    expect(b.warnings).toHaveLength(1);
    expect(b.warnings[0]).toMatch(/^colorPrimary #FFEE00/);
  });

  it("resolves ELLA_BRANDING against the directory npm was run from", () => {
    expect(
      brandingDir({ ELLA_BRANDING: "northwind", INIT_CWD: "/home/me" }),
    ).toBe("/home/me/northwind");
    expect(brandingDir({})).toBe(DEFAULT_BRANDING_DIR);
  });
});
