// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { optimize } from "svgo";
import pc from "polygon-clipping";
import { chromium } from "@playwright/test";
import { PNG } from "pngjs";
import {
  ARTWORK,
  BAND,
  BLACK_PATH_COUNT,
  CENTRE,
  FLATTEN_TOLERANCE,
  PRECISION,
  blackPaths,
  circlePoly,
  flatten,
  polysToPath,
  simplify,
  snapRing,
} from "./lib/geometry.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const SOURCE = path.join(root, "assets", "logo.source.svg");
const PUBLIC = path.join(root, "public");

const FAVICON_SVG = path.join(PUBLIC, "favicon.svg");
const FAVICON_ICO = path.join(PUBLIC, "favicon.ico");
const APPLE_TOUCH = path.join(PUBLIC, "apple-touch-icon.png");

const MIN_HOLE = 6;
const OUTLINE_TOLERANCE = 0.08;
const THICKEN = 0.8;
const FILL = 0.95;
const ICO_SIZES = [16, 32, 48];
const APPLE_SIZE = 180;

function catShape(sourceSvg) {
  const black = blackPaths(sourceSvg);
  if (black.length !== BLACK_PATH_COUNT) {
    throw new Error(`expected ${BLACK_PATH_COUNT} black paths`);
  }
  const rings = flatten(black[ARTWORK], FLATTEN_TOLERANCE)
    .map((poly) => snapRing(poly[0]))
    .filter((r) => r.length > 3);
  let shape = [[rings[0]]];
  for (let i = 1; i < rings.length; i++) shape = pc.xor(shape, [[rings[i]]]);

  const band = pc.difference(
    circlePoly(BAND.outer),
    circlePoly(BAND.inner),
  );
  const inBand = pc.intersection(shape, band);
  const area = (r) =>
    Math.abs(
      r.reduce((s, pt, j) => {
        const q = r[(j + 1) % r.length];
        return s + (pt[0] * q[1] - q[0] * pt[1]);
      }, 0) / 2,
    );
  const scored = inBand
    .map((poly) => ({ poly, a: area(poly[0]) }))
    .sort((x, y) => y.a - x.a);
  return { cat: pc.difference(shape, scored[0].poly), area };
}

function simplifyCat(cat, area) {
  const out = [];
  for (const poly of cat) {
    if (area(poly[0]) < MIN_HOLE * 1.5) continue;
    const keep = [poly[0]];
    for (let i = 1; i < poly.length; i++) {
      if (area(poly[i]) >= MIN_HOLE) keep.push(poly[i]);
    }
    out.push(keep);
  }
  if (out.length === 0) throw new Error("simplification removed everything");
  return out;
}

function fitToFrame(mp) {
  let x0 = Infinity;
  let y0 = Infinity;
  let x1 = -Infinity;
  let y1 = -Infinity;
  for (const poly of mp) {
    for (const ring of poly) {
      for (const [x, y] of ring) {
        x0 = Math.min(x0, x);
        y0 = Math.min(y0, y);
        x1 = Math.max(x1, x);
        y1 = Math.max(y1, y);
      }
    }
  }
  const target = 60 * FILL;
  const s = Math.min(target / (x1 - x0), target / (y1 - y0));
  const tx = CENTRE - ((x0 + x1) / 2) * s;
  const ty = CENTRE - ((y0 + y1) / 2) * s;
  return mp.map((poly) =>
    poly.map((ring) => ring.map(([x, y]) => [x * s + tx, y * s + ty])),
  );
}

function buildFaviconSvg(sourceSvg) {
  const { cat, area } = catShape(sourceSvg);
  const mp = fitToFrame(simplifyCat(cat, area)).map((poly) =>
    poly.map((ring) => simplify(ring, OUTLINE_TOLERANCE)),
  );
  const raw =
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 60 60" role="img">` +
    `<title>Ella Core</title>` +
    `<style>@media(prefers-color-scheme:dark){` +
    `path{fill:#fff;stroke:#fff}}</style>` +
    `<path fill="#000" stroke="#000" fill-rule="evenodd" ` +
    `stroke-width="${THICKEN}" stroke-linejoin="round" stroke-linecap="round" ` +
    `d="${polysToPath(mp)}"/>` +
    `</svg>`;
  const { data } = optimize(raw, {
    multipass: true,
    floatPrecision: PRECISION,
    plugins: [
      {
        name: "preset-default",
        params: {
          overrides: {
            mergePaths: false,
            convertShapeToPath: false,
            removeUselessStrokeAndFill: false,
          },
        },
      },
    ],
  });
  return data;
}

async function rasterise(browser, svg, size, background) {
  const page = await browser.newPage({
    viewport: { width: size, height: size },
    deviceScaleFactor: 1,
  });
  const encoded = Buffer.from(svg).toString("base64");
  await page.setContent(
    `<style>html,body{margin:0;padding:0;background:${background}}` +
      `img{display:block;width:${size}px;height:${size}px}</style>` +
      `<img src="data:image/svg+xml;base64,${encoded}">`,
  );
  await page.waitForLoadState("networkidle");
  const buffer = await page.screenshot({ type: "png", omitBackground: background === "transparent" });
  await page.close();
  return buffer;
}

function buildIco(pngs) {
  const count = pngs.length;
  const header = Buffer.alloc(6);
  header.writeUInt16LE(0, 0);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(count, 4);
  const entries = [];
  let offset = 6 + count * 16;
  for (const { size, data } of pngs) {
    const e = Buffer.alloc(16);
    e.writeUInt8(size >= 256 ? 0 : size, 0);
    e.writeUInt8(size >= 256 ? 0 : size, 1);
    e.writeUInt8(0, 2);
    e.writeUInt8(0, 3);
    e.writeUInt16LE(1, 4);
    e.writeUInt16LE(32, 6);
    e.writeUInt32LE(data.length, 8);
    e.writeUInt32LE(offset, 12);
    entries.push(e);
    offset += data.length;
  }
  return Buffer.concat([header, ...entries, ...pngs.map((p) => p.data)]);
}

function inkCoverage(buffer) {
  const png = PNG.sync.read(buffer);
  let dark = 0;
  for (let i = 0; i < png.data.length; i += 4) {
    if (png.data[i + 3] > 128 && png.data[i] < 140) dark++;
  }
  return dark / (png.width * png.height);
}

const source = fs.readFileSync(SOURCE, "utf8");
const favicon = buildFaviconSvg(source);
console.log(`favicon.svg ${favicon.length} B`);

const browser = await chromium.launch();
let ok = true;
try {
  const pngs = [];
  for (const size of ICO_SIZES) {
    const data = await rasterise(browser, favicon, size, "transparent");
    const coverage = inkCoverage(data);
    const pass = coverage > 0.12 && coverage < 0.6;
    if (!pass) ok = false;
    console.log(
      `  ${pass ? "ok  " : "FAIL"} ${size}px ink coverage ${(coverage * 100).toFixed(1)}%`,
    );
    pngs.push({ size, data });
  }
  const apple = await rasterise(browser, favicon, APPLE_SIZE, "#ffffff");
  fs.writeFileSync(APPLE_TOUCH, apple);
  fs.writeFileSync(FAVICON_ICO, buildIco(pngs));
} finally {
  await browser.close();
}

if (!ok) {
  console.error("refusing to write icons");
  process.exit(1);
}

fs.writeFileSync(FAVICON_SVG, favicon);
console.log(
  `\nwrote public/favicon.svg, public/favicon.ico (${ICO_SIZES.join("/")}), ` +
    `public/apple-touch-icon.png (${APPLE_SIZE})`,
);
