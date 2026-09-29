// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import fs from "node:fs";
import path from "node:path";
import type { Plugin } from "vite";
import { dark, light } from "../src/utils/tokens";

export interface ClientBranding {
  appName: string;
  logoUrl: string;
  colorPrimary: string;
  colorPrimaryDark: string;
  showProductName: boolean;
  poweredBy: boolean;
  topBar: Surface | null;
}

export interface Surface {
  background: string;
  text: string;
}

export interface BrandAsset {
  fileName: string;
  source: string;
}

export interface Branding {
  client: ClientBranding;
  logo: BrandAsset;
  favicons: BrandAsset[];
  warnings: string[];
}

export const DEFAULT_BRANDING_DIR = path.resolve(
  import.meta.dirname,
  "default",
);
const FIELDS = [
  "appName",
  "logo",
  "favicon",
  "colorPrimary",
  "colorPrimaryDark",
  "showProductName",
  "colorTopBar",
];
const HEX = /^#[0-9a-fA-F]{6}$/;
const MIME: Record<string, string> = {
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".ico": "image/x-icon",
};
const LOGO_TYPES = [".svg", ".png"];
const FAVICON_TYPES = [".ico", ".svg", ".png"];
const WCAG_AA = 4.5;

export const brandingDir = (env: NodeJS.ProcessEnv = process.env) =>
  env.ELLA_BRANDING
    ? path.resolve(env.INIT_CWD ?? process.cwd(), env.ELLA_BRANDING)
    : DEFAULT_BRANDING_DIR;

const channels = (hex: string) =>
  [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));

const luminance = (hex: string) => {
  const [r, g, b] = channels(hex).map((v) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};

export const contrast = (a: string, b: string) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};

const lowestContrast = (color: string, surfaces: string[]) =>
  Math.min(...surfaces.map((s) => contrast(color, s)));

const towardWhite = (hex: string, t: number) =>
  `#${channels(hex)
    .map((v) =>
      Math.round(v + (255 - v) * t)
        .toString(16)
        .padStart(2, "0"),
    )
    .join("")
    .toUpperCase()}`;

const LIGHT_SURFACES = [light.backgroundDefault, light.backgroundPaper];
const DARK_TEXT_ON_LIGHT = "rgba(0, 0, 0, 0.87)";

export const readableText = (background: string) =>
  contrast("#FFFFFF", background) >= WCAG_AA ? "#FFFFFF" : DARK_TEXT_ON_LIGHT;

export const deriveDarkPrimary = (
  hex: string,
  surfaces: string[] = [dark.backgroundDefault, dark.backgroundPaper],
) => {
  for (let step = 0; step <= 20; step++) {
    const candidate = towardWhite(hex, step / 20);
    if (lowestContrast(candidate, surfaces) >= WCAG_AA) {
      return candidate;
    }
  }
  return "#FFFFFF";
};

export function loadBranding(dir: string = brandingDir()): Branding {
  const file = path.join(dir, "branding.json");
  const fail = (message: string): never => {
    throw new Error(`Invalid branding (${file}): ${message}`);
  };

  let raw: unknown;
  try {
    raw = JSON.parse(fs.readFileSync(file, "utf8"));
  } catch (err) {
    return fail((err as Error).message);
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return fail("must be a JSON object");
  }
  const config = raw as Record<string, unknown>;

  const unsupported = Object.keys(config).filter((k) => !FIELDS.includes(k));
  if (unsupported.length > 0) {
    fail(
      `unsupported field(s) ${unsupported.join(", ")}; supported fields are ${FIELDS.join(", ")}`,
    );
  }

  const appName = config.appName;
  if (typeof appName !== "string" || appName.trim() === "") {
    fail("appName must be a non-empty string");
  }

  const asset = (field: string, value: unknown, types: string[]) => {
    if (typeof value !== "string" || value === "") {
      return fail(`${field} must be a file name`);
    }
    const source = path.resolve(dir, value);
    const ext = path.extname(source).toLowerCase();
    if (!types.includes(ext)) {
      fail(`${field} ${value} must be one of ${types.join(", ")}`);
    }
    if (!fs.statSync(source, { throwIfNoEntry: false })?.isFile()) {
      fail(`${field} ${value} does not exist`);
    }
    return { ext, source };
  };

  const color = (field: string, value: unknown) => {
    if (typeof value !== "string" || !HEX.test(value)) {
      return fail(`${field} must be a hex color like #26374A`);
    }
    return value.toUpperCase();
  };

  const logo = asset("logo", config.logo, LOGO_TYPES);

  const faviconList = Array.isArray(config.favicon)
    ? config.favicon
    : [config.favicon];
  if (faviconList.length === 0) {
    fail("favicon must list at least one file");
  }
  const favicons = faviconList.map((f) => asset("favicon", f, FAVICON_TYPES));
  if (new Set(favicons.map((f) => f.ext)).size !== favicons.length) {
    fail("favicon must not list two files of the same type");
  }

  const showProductName = config.showProductName ?? true;
  if (typeof showProductName !== "boolean") {
    fail("showProductName must be true or false");
  }

  const darkBackgrounds = [dark.backgroundDefault, dark.backgroundPaper];

  const colorPrimary = color("colorPrimary", config.colorPrimary);
  const colorPrimaryDark =
    config.colorPrimaryDark === undefined
      ? deriveDarkPrimary(colorPrimary, darkBackgrounds)
      : color("colorPrimaryDark", config.colorPrimaryDark);

  const topBar =
    config.colorTopBar === undefined
      ? null
      : (() => {
          const background = color("colorTopBar", config.colorTopBar);
          return { background, text: readableText(background) };
        })();

  const warnings: string[] = [];
  const checkContrast = (field: string, value: string, surfaces: string[]) => {
    const ratio = lowestContrast(value, surfaces);
    if (ratio < WCAG_AA) {
      warnings.push(
        `${field} ${value} has a contrast of ${ratio.toFixed(2)}:1 against the background; ${WCAG_AA}:1 is recommended for readable text`,
      );
    }
  };
  checkContrast("colorPrimary", colorPrimary, LIGHT_SURFACES);
  checkContrast("colorPrimaryDark", colorPrimaryDark, darkBackgrounds);

  const logoFileName = `logo-mark${logo.ext}`;

  return {
    client: {
      appName: (appName as string).trim(),
      logoUrl: `/${logoFileName}`,
      colorPrimary,
      colorPrimaryDark,
      showProductName: showProductName as boolean,
      poweredBy: path.resolve(dir) !== DEFAULT_BRANDING_DIR,
      topBar,
    },
    logo: { fileName: logoFileName, source: logo.source },
    favicons: favicons.map((f) => ({
      fileName: `favicon${f.ext}`,
      source: f.source,
    })),
    warnings,
  };
}

const faviconTag = ({ fileName }: BrandAsset) => {
  const ext = path.extname(fileName);
  return ext === ".ico"
    ? `<link rel="icon" href="/${fileName}" sizes="32x32" />`
    : `<link rel="icon" href="/${fileName}" type="${MIME[ext]}" />`;
};

export function brandingPlugin(branding: Branding): Plugin {
  const assets = [branding.logo, ...branding.favicons];

  return {
    name: "ella-branding",
    configResolved(config) {
      for (const warning of branding.warnings) {
        config.logger.warn(`Branding: ${warning}`);
      }
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const found = assets.find((a) => req.url === `/${a.fileName}`);
        if (!found) {
          next();
          return;
        }
        res.setHeader("Content-Type", MIME[path.extname(found.fileName)]);
        res.end(fs.readFileSync(found.source));
      });
    },
    generateBundle() {
      for (const a of assets) {
        this.emitFile({
          type: "asset",
          fileName: a.fileName,
          source: fs.readFileSync(a.source),
        });
      }
    },
    transformIndexHtml(html) {
      return html
        .replace(
          /<title>[^<]*<\/title>/,
          () => `<title>${escapeHtml(branding.client.appName)}</title>`,
        )
        .replace("%FAVICONS%", () =>
          branding.favicons.map(faviconTag).join("\n    "),
        )
        .replaceAll("%CANVAS_LIGHT%", light.backgroundDefault)
        .replaceAll("%CANVAS_DARK%", dark.backgroundDefault);
    },
  };
}

const escapeHtml = (value: string) =>
  value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
