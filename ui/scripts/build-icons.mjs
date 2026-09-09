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
  DISC_R,
  OUTER_W,
  FLATTEN_TOLERANCE,
  PRECISION,
  blackPaths,
  circlePoly,
  dilate,
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
const LOGO_MARK = path.join(PUBLIC, "logo-mark.svg");

const MIN_OUTER = 9;
const MIN_HOLE = Infinity;
const OUTLINE_TOLERANCE = 0.08;
const THICKEN = 0.6;
const FILL = 0.95;
const ICO_SIZES = [16, 32, 48];
const APPLE_SIZE = 180;

const GROUND_CUT_Y = 45.7;

const MARK = {
  minHole: 1.0,
  thicken: 0.25,
  outerWidth: 1.9,
  innerWidth: 1.9,
  ringGap: 1.5,
  keyline: 2.2,
  haloBelowY: CENTRE + 6,
  reachFraction: 1.03,
  floorLines: 3,
  floorThicken: 0.55,
  floorMinGap: 1.5,
  tolerance: 0.02,
  haloTolerance: 0.1,
};
const MARK_SIZES = [50];
const MARK_MIN_RING_PX = 1.5;
const MARK_INNER_ARCS = 3;
const EAR_RAW_RADIUS = 23.48;

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

  const band = pc.difference(circlePoly(BAND.outer), circlePoly(BAND.inner));
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
    if (area(poly[0]) < MIN_OUTER) continue;
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
  const { cat: catNoGround } = unweld(cat, area);
  const mp = fitToFrame(simplifyCat(catNoGround, area)).map((poly) =>
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
  const buffer = await page.screenshot({
    type: "png",
    omitBackground: background === "transparent",
  });
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

function scaleAboutCentre(mp, s) {
  return mp.map((poly) =>
    poly.map((ring) =>
      ring.map(([x, y]) => [
        CENTRE + (x - CENTRE) * s,
        CENTRE + (y - CENTRE) * s,
      ]),
    ),
  );
}

function halfPlane(lo, hi) {
  return [
    [
      [-10, lo],
      [70, lo],
      [70, hi],
      [-10, hi],
      [-10, lo],
    ],
  ];
}

function spaceFloor(lines) {
  const extent = (poly) => {
    const ys = poly.flat().map((p) => p[1]);
    return [Math.min(...ys), Math.max(...ys)];
  };
  const ordered = lines
    .map((poly) => ({ poly, e: extent(poly) }))
    .sort((a, b) => a.e[0] - b.e[0]);
  const out = [];
  let prevBottom = GROUND_CUT_Y;
  for (const { poly, e } of ordered) {
    const need = prevBottom + MARK.floorMinGap - e[0];
    const dy = need > 0 ? need : 0;
    out.push(poly.map((ring) => ring.map(([x, y]) => [x, y + dy])));
    prevBottom = e[1] + dy;
  }
  return out;
}

function unweld(cat, area) {
  const kept = [];
  for (const poly of cat) {
    if (area(poly[0]) < MARK.minHole * 1.5) continue;
    const keep = [poly[0]];
    for (let i = 1; i < poly.length; i++) {
      if (area(poly[i]) >= MARK.minHole) keep.push(poly[i]);
    }
    kept.push(keep);
  }
  kept.sort((a, b) => area(b[0]) - area(a[0]));
  const body = [kept[0]];
  const cutTop = pc.intersection(body, halfPlane(-10, GROUND_CUT_Y));
  const cutBottom = pc.intersection(body, halfPlane(GROUND_CUT_Y, 70));
  const loose = kept.slice(1);
  const floor = [...cutBottom, ...loose]
    .map((poly) => {
      const xs = poly[0].map((q) => q[0]);
      return { poly, width: Math.max(...xs) - Math.min(...xs) };
    })
    .sort((a, b) => b.width - a.width)
    .slice(0, MARK.floorLines)
    .map((f) => f.poly);
  return { cat: cutTop, floor: spaceFloor(floor) };
}

function buildMarkSvg(sourceSvg) {
  const { cat, area } = catShape(sourceSvg);
  const { cat: catPolys, floor } = unweld(cat, area);

  const rOuter = DISC_R - MARK.outerWidth / 2;
  const rInner =
    rOuter - MARK.outerWidth / 2 - MARK.ringGap - MARK.innerWidth / 2;
  const ringOuterEdge = rInner + MARK.innerWidth / 2;
  const ringInnerEdge = rInner - MARK.innerWidth / 2;

  const scale = (ringInnerEdge * MARK.reachFraction) / EAR_RAW_RADIUS;
  const place = (mp, tolerance) =>
    scaleAboutCentre(mp, scale).map((poly) =>
      poly.map((ring) => snapRing(simplify(ring, tolerance))),
    );

  const catMp = place(catPolys, MARK.tolerance);
  const floorMp = pc.intersection(
    place(floor, MARK.tolerance),
    circlePoly(ringOuterEdge),
  );
  const halo = pc.intersection(
    place(catPolys, MARK.haloTolerance),
    halfPlane(-10, MARK.haloBelowY),
  );
  const band = pc.difference(
    circlePoly(ringOuterEdge),
    circlePoly(ringInnerEdge),
  );
  const innerRing = pc.difference(band, dilate(halo.flat(), MARK.keyline));
  if (innerRing.length !== MARK_INNER_ARCS) {
    throw new Error(
      `expected the inner ring in ${MARK_INNER_ARCS} arcs, got ${innerRing.length}`,
    );
  }
  const innerPath = innerRing.map((poly) =>
    poly.map((ring) => simplify(ring, MARK.tolerance)),
  );

  const raw =
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 60 60" role="img">` +
    `<title>Ella Core</title>` +
    `<circle cx="${CENTRE}" cy="${CENTRE}" r="${DISC_R}" fill="#fff"/>` +
    `<circle cx="${CENTRE}" cy="${CENTRE}" r="${rOuter.toFixed(3)}" fill="none" ` +
    `stroke="#000" stroke-width="${MARK.outerWidth}"/>` +
    `<path fill="#000" fill-rule="nonzero" d="${polysToPath(innerPath)}"/>` +
    `<path fill="#000" stroke="#000" fill-rule="evenodd" ` +
    `stroke-width="${MARK.floorThicken}" stroke-linejoin="round" ` +
    `stroke-linecap="round" d="${polysToPath(floorMp)}"/>` +
    `<path fill="#000" stroke="#000" fill-rule="evenodd" ` +
    `stroke-width="${MARK.thicken}" stroke-linejoin="round" ` +
    `d="${polysToPath(catMp)}"/>` +
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
  return {
    svg: data,
    rOuter,
    rInner,
    arcs: innerRing.length,
    ringPx: (MARK.innerWidth * 50) / 60,
  };
}

const source = fs.readFileSync(SOURCE, "utf8");
const favicon = buildFaviconSvg(source);
const mark = buildMarkSvg(source);
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

  console.log("\nmark rings must survive rasterisation:");
  for (const size of MARK_SIZES) {
    const px = (MARK.innerWidth * size) / 60;
    const pass = px >= MARK_MIN_RING_PX;
    if (!pass) ok = false;
    console.log(
      `  ${pass ? "ok  " : "FAIL"} at ${size}px the rings are ${px.toFixed(2)} device px`,
    );
  }
  const legacy = (OUTER_W * 50) / 60;
  console.log(
    `  (the full logo's outer ring is ${legacy.toFixed(2)} px at 50px, which is the problem)`,
  );
} finally {
  await browser.close();
}

if (!ok) {
  console.error("refusing to write icons");
  process.exit(1);
}

fs.writeFileSync(FAVICON_SVG, favicon);
fs.writeFileSync(LOGO_MARK, mark.svg);
console.log(
  `logo-mark.svg ${mark.svg.length} B  rings r=${mark.rOuter.toFixed(2)}/${mark.rInner.toFixed(2)}, inner ring in ${mark.arcs} arcs\n` +
    `  inner ring ${mark.arcs} arcs, ${mark.ringPx.toFixed(2)}px at 50px, ` +
    `keyline ${MARK.keyline} units = ${((MARK.keyline * 50) / 60).toFixed(2)}px`,
);
console.log(
  `\nwrote public/favicon.svg, public/favicon.ico (${ICO_SIZES.join("/")}), ` +
    `public/apple-touch-icon.png (${APPLE_SIZE})`,
);
