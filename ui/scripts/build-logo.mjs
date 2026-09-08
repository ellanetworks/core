// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { optimize } from "svgo";
import svgpath from "svgpath";
import pc from "polygon-clipping";
import { chromium } from "@playwright/test";
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const SOURCE = path.join(root, "assets", "logo.source.svg");
const OUTPUT = path.join(root, "public", "logo.svg");
const DIFF_DIR = path.join(root, ".logo-diff");

const CENTRE = 30;
const DISC_R = 25.273438;
const OUTER_R = 25.167;
const OUTER_W = 1.101;
const INNER_R = 23.524;
const INNER_W = 0.717;

const BAND = { inner: 22.85, outer: 24.05 };
const WIDE_BAND = { inner: 22.3, outer: 24.35 };
const KEYLINE = 0.55;
const FLATTEN_TOLERANCE = 0.004;
const SIMPLIFY_TOLERANCE = 0.008;
const RING_SIMPLIFY_TOLERANCE = 0.004;
const GRID = 1e5;

const OUTER_RING = 0;
const TOP_ARC = 1;
const ARTWORK = 2;
const BLACK_PATH_COUNT = 19;

const PRECISION = Number(process.env.LOGO_PRECISION ?? 3);
const SIZES = [512, 150, 100, 50];
const BACKGROUNDS = ["#FFFFFF", "#14161B", "#26374A"];
const MATCH_THRESHOLD = 0.1;
const MAX_DIFF_PIXELS = 16;
const MAX_DIFF_COMPONENT = 8;
const MASK_INNER = 22.8;
const MASK_OUTER = 25.95;
const RING_TOLERANCE = { centre: 0.06, width: 0.09 };
const ARC_CHECK_SPANS = [
  { from: 10, to: 200 },
  { from: 234, to: 284 },
];

const round = (n) => Number(n.toFixed(PRECISION));

function blackPaths(svg) {
  const found = [];
  for (const m of svg.matchAll(/<path\b([^>]*)>/g)) {
    const attrs = m[1];
    const fill = /fill="([^"]*)"/.exec(attrs);
    if (!fill || fill[1].toLowerCase() !== "#000000") continue;
    const d = /\bd="([^"]*)"/.exec(attrs);
    if (d) found.push(d[1].trim());
  }
  return found;
}

function firstSubpath(d) {
  const starts = [];
  for (let i = 0; i < d.length; i++) if (d[i] === "M") starts.push(i);
  return d.slice(starts[0], starts[1]).trim();
}

function flatten(d, tolerance) {
  const rings = [];
  let cur = null;
  let px = 0;
  let py = 0;
  let sx = 0;
  let sy = 0;
  const cubic = (x0, y0, x1, y1, x2, y2, x3, y3) => {
    const n = Math.max(
      3,
      Math.ceil(Math.hypot(x3 - x0, y3 - y0) / tolerance / 6),
    );
    for (let i = 1; i <= n; i++) {
      const t = i / n;
      const u = 1 - t;
      cur.push([
        u * u * u * x0 +
          3 * u * u * t * x1 +
          3 * u * t * t * x2 +
          t * t * t * x3,
        u * u * u * y0 +
          3 * u * u * t * y1 +
          3 * u * t * t * y2 +
          t * t * t * y3,
      ]);
    }
  };
  svgpath(d)
    .abs()
    .unarc()
    .unshort()
    .iterate((s) => {
      const c = s[0];
      if (c === "M") {
        if (cur && cur.length > 2) rings.push(cur);
        cur = [[s[1], s[2]]];
        sx = s[1];
        sy = s[2];
        px = s[1];
        py = s[2];
      } else if (c === "L") {
        cur.push([s[1], s[2]]);
        px = s[1];
        py = s[2];
      } else if (c === "C") {
        cubic(px, py, s[1], s[2], s[3], s[4], s[5], s[6]);
        px = s[5];
        py = s[6];
      } else if (c === "Z") {
        cur.push([sx, sy]);
      }
    });
  if (cur && cur.length > 2) rings.push(cur);
  return rings.map((r) => {
    const q = r.slice();
    const a = q[0];
    const b = q[q.length - 1];
    if (a[0] !== b[0] || a[1] !== b[1]) q.push([a[0], a[1]]);
    return [q];
  });
}

function simplifyOpen(points, tolerance) {
  if (points.length < 3) return points;
  const keep = new Uint8Array(points.length);
  keep[0] = 1;
  keep[points.length - 1] = 1;
  const stack = [[0, points.length - 1]];
  while (stack.length) {
    const [lo, hi] = stack.pop();
    const [x1, y1] = points[lo];
    const [x2, y2] = points[hi];
    const dx = x2 - x1;
    const dy = y2 - y1;
    const len = Math.hypot(dx, dy) || 1;
    let best = -1;
    let bestD = tolerance;
    for (let i = lo + 1; i < hi; i++) {
      const [x, y] = points[i];
      const dist = Math.abs(dy * x - dx * y + x2 * y1 - y2 * x1) / len;
      if (dist > bestD) {
        bestD = dist;
        best = i;
      }
    }
    if (best > 0) {
      keep[best] = 1;
      stack.push([lo, best], [best, hi]);
    }
  }
  return points.filter((_, i) => keep[i]);
}

function simplify(ring, tolerance) {
  const pts = ring.slice();
  const a = pts[0];
  const b = pts[pts.length - 1];
  const closed = a[0] === b[0] && a[1] === b[1];
  if (closed) pts.pop();
  if (pts.length < 4) return ring;
  let far = 0;
  let farD = -1;
  for (let i = 1; i < pts.length; i++) {
    const d = Math.hypot(pts[i][0] - pts[0][0], pts[i][1] - pts[0][1]);
    if (d > farD) {
      farD = d;
      far = i;
    }
  }
  const head = simplifyOpen(pts.slice(0, far + 1), tolerance);
  const tail = simplifyOpen(pts.slice(far), tolerance);
  const out = head.concat(tail.slice(1));
  if (closed) out.push([out[0][0], out[0][1]]);
  return out;
}

const snap = (v) => Math.round(v * GRID) / GRID;

function snapRing(ring) {
  const out = [];
  for (const [x, y] of ring) {
    const p = [snap(x), snap(y)];
    const last = out[out.length - 1];
    if (!last || last[0] !== p[0] || last[1] !== p[1]) out.push(p);
  }
  const f = out[0];
  const l = out[out.length - 1];
  if (out.length && (f[0] !== l[0] || f[1] !== l[1])) out.push([f[0], f[1]]);
  return out;
}

function circlePoly(r, n = 1440, cx = CENTRE, cy = CENTRE) {
  const p = [];
  for (let i = 0; i < n; i++) {
    const t = (2 * Math.PI * i) / n;
    p.push([snap(cx + r * Math.cos(t)), snap(cy + r * Math.sin(t))]);
  }
  p.push([p[0][0], p[0][1]]);
  return [p];
}

function dilate(rings, margin) {
  let acc = null;
  for (const ring of rings) {
    const parts = [[snapRing(ring)]];
    for (let i = 0; i + 1 < ring.length; i++) {
      const [x1, y1] = ring[i];
      const [x2, y2] = ring[i + 1];
      const dx = x2 - x1;
      const dy = y2 - y1;
      const len = Math.hypot(dx, dy);
      if (len < 1e-9) continue;
      const nx = (-dy / len) * margin;
      const ny = (dx / len) * margin;
      parts.push([
        snapRing([
          [x1 + nx, y1 + ny],
          [x2 + nx, y2 + ny],
          [x2 - nx, y2 - ny],
          [x1 - nx, y1 - ny],
          [x1 + nx, y1 + ny],
        ]),
      ]);
      parts.push(circlePoly(margin, 16, x1, y1));
    }
    const last = ring[ring.length - 1];
    parts.push(circlePoly(margin, 16, last[0], last[1]));
    let sub = parts[0];
    for (let i = 1; i < parts.length; i++) sub = pc.union(sub, parts[i]);
    acc = acc === null ? sub : pc.union(acc, sub);
  }
  return acc;
}

function catPieces(artworkPath, bounds, tolerance) {
  const band = pc.difference(
    circlePoly(bounds.outer),
    circlePoly(bounds.inner),
  );
  const shape = flatten(firstSubpath(artworkPath), FLATTEN_TOLERANCE).map(
    (poly) => poly.map(snapRing),
  );
  const pieces = pc.intersection(shape, band);
  if (pieces.length < 2) {
    throw new Error(`expected ring plus cat pieces, got ${pieces.length}`);
  }
  const scored = pieces.map((poly) => {
    const p = poly[0];
    const area = Math.abs(
      p.reduce((s, pt, j) => {
        const q = p[(j + 1) % p.length];
        return s + (pt[0] * q[1] - q[0] * pt[1]);
      }, 0) / 2,
    );
    return { poly, area };
  });
  scored.sort((a, b) => b.area - a.area);
  return scored
    .slice(1)
    .map(({ poly }) => poly.map((ring) => snapRing(simplify(ring, tolerance))));
}

function occludedRing(cats) {
  const halo = dilate(cats.flat(), KEYLINE);
  const annulus = pc.difference(
    circlePoly(INNER_R + INNER_W / 2),
    circlePoly(INNER_R - INNER_W / 2),
  );
  const cut = pc.difference(annulus, halo);
  if (cut.length !== 2) {
    throw new Error(`expected ring cut into 2 arcs, got ${cut.length}`);
  }
  return cut.map((poly) =>
    poly.map((ring) => simplify(ring, RING_SIMPLIFY_TOLERANCE)),
  );
}

function polysToPath(polys) {
  return polys
    .map((poly) =>
      poly
        .map(
          (ring) =>
            "M " +
            ring.map(([x, y]) => `${round(x)} ${round(y)}`).join(" L ") +
            " Z",
        )
        .join(" "),
    )
    .join(" ");
}

function annulusMask() {
  const o = BAND.outer;
  const i = BAND.inner;
  return (
    `M ${CENTRE + o} ${CENTRE} A ${o} ${o} 0 1 1 ${CENTRE - o} ${CENTRE} ` +
    `A ${o} ${o} 0 1 1 ${CENTRE + o} ${CENTRE} Z ` +
    `M ${CENTRE + i} ${CENTRE} A ${i} ${i} 0 1 1 ${CENTRE - i} ${CENTRE} ` +
    `A ${i} ${i} 0 1 1 ${CENTRE + i} ${CENTRE} Z`
  );
}

function finalize(svg) {
  const out = svg.replace("<svg ", '<svg role="img" ');
  if (!out.includes('role="img"')) {
    throw new Error("failed to reapply role");
  }
  return out;
}

function build(sourceSvg) {
  const black = blackPaths(sourceSvg);
  if (black.length !== BLACK_PATH_COUNT) {
    throw new Error(
      `expected ${BLACK_PATH_COUNT} black paths, found ${black.length}`,
    );
  }
  const artwork = black
    .filter((_, i) => i !== OUTER_RING && i !== TOP_ARC)
    .join(" ");
  const haloSource = catPieces(black[ARTWORK], WIDE_BAND, SIMPLIFY_TOLERANCE);
  const cats = catPieces(black[ARTWORK], BAND, RING_SIMPLIFY_TOLERANCE);
  const ring = occludedRing(haloSource);

  const raw =
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 60 60">` +
    `<title>Ella Core</title>` +
    `<circle cx="${CENTRE}" cy="${CENTRE}" r="${DISC_R}" fill="#ffffff"/>` +
    `<path fill="#000" fill-rule="nonzero" d="${artwork}"/>` +
    `<path fill="#ffffff" fill-rule="evenodd" d="${annulusMask()}"/>` +
    `<path fill="#000" fill-rule="nonzero" d="${polysToPath(ring)} ${polysToPath(cats)}"/>` +
    `<circle cx="${CENTRE}" cy="${CENTRE}" r="${OUTER_R}" fill="none" ` +
    `stroke="#000" stroke-width="${OUTER_W}"/>` +
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
  return finalize(data);
}

async function render(browser, svg, size, background) {
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
  const buffer = await page.screenshot({ type: "png" });
  await page.close();
  return PNG.sync.read(buffer);
}

function maskRings(png, size) {
  const scale = size / 60;
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      const r = Math.hypot(x / scale - CENTRE, y / scale - CENTRE);
      if (r >= MASK_INNER && r <= MASK_OUTER) {
        const i = (y * size + x) * 4;
        png.data[i] = 128;
        png.data[i + 1] = 128;
        png.data[i + 2] = 128;
      }
    }
  }
  return png;
}

function largestDiffComponent(diff, width, height) {
  const isDiff = (i) =>
    diff.data[i] > 200 && diff.data[i + 1] < 100 && diff.data[i + 2] < 100;
  const seen = new Uint8Array(width * height);
  let largest = 0;
  for (let start = 0; start < width * height; start++) {
    if (seen[start] || !isDiff(start * 4)) continue;
    const stack = [start];
    seen[start] = 1;
    let size = 0;
    while (stack.length > 0) {
      const p = stack.pop();
      size++;
      const px = p % width;
      const py = (p / width) | 0;
      for (let dy = -1; dy <= 1; dy++) {
        for (let dx = -1; dx <= 1; dx++) {
          const nx = px + dx;
          const ny = py + dy;
          if (nx < 0 || ny < 0 || nx >= width || ny >= height) continue;
          const n = ny * width + nx;
          if (seen[n] || !isDiff(n * 4)) continue;
          seen[n] = 1;
          stack.push(n);
        }
      }
    }
    largest = Math.max(largest, size);
  }
  return largest;
}

async function verifyArtworkUnchanged(browser, before, after) {
  const failures = [];
  for (const background of BACKGROUNDS) {
    for (const size of SIZES) {
      const a = maskRings(
        await render(browser, before, size, background),
        size,
      );
      const b = maskRings(await render(browser, after, size, background), size);
      const diff = new PNG({ width: size, height: size });
      const changed = pixelmatch(a.data, b.data, diff.data, size, size, {
        threshold: MATCH_THRESHOLD,
      });
      const component = largestDiffComponent(diff, size, size);
      const label = `${size}px on ${background}`;
      const detail = `${changed} px differ, largest run ${component}`;
      if (changed > MAX_DIFF_PIXELS || component > MAX_DIFF_COMPONENT) {
        failures.push({ label, diff });
        console.error(`  FAIL ${label}: ${detail}`);
      } else {
        console.log(`  ok   ${label}: ${detail}`);
      }
    }
  }
  return failures;
}

function scanRing(png, size, lo, hi) {
  const scale = size / 60;
  const bands = new Map();
  for (let deg = 0; deg < 3600; deg++) {
    const t = ((deg / 10) * Math.PI) / 180;
    let prev = false;
    let start = 0;
    let last = null;
    for (let r = lo; r <= hi; r += 0.004) {
      const x = Math.round((CENTRE + r * Math.cos(t)) * scale);
      const y = Math.round((CENTRE + r * Math.sin(t)) * scale);
      let dark = false;
      if (x >= 0 && y >= 0 && x < size && y < size) {
        dark = png.data[(y * size + x) * 4] < 128;
      }
      if (dark && !prev) start = r;
      if (!dark && prev) last = [start, r];
      prev = dark;
    }
    if (prev) last = [start, hi];
    if (last) bands.set(deg / 10, last);
  }
  return bands;
}

function checkRing(png, size, name, radius, weight, lo, hi, minSamples, spans) {
  const bands = scanRing(png, size, lo, hi);
  const centres = [];
  const widths = [];
  for (const [angle, [a, b]] of bands) {
    if (spans && !spans.some((s) => angle >= s.from && angle <= s.to)) continue;
    const w = b - a;
    if (w < weight * 0.8 || w > weight * 1.25) continue;
    centres.push((a + b) / 2);
    widths.push(w);
  }
  if (centres.length < minSamples) {
    console.error(`  FAIL ${name}: only ${centres.length} clean samples`);
    return false;
  }
  const dev = Math.max(...centres.map((c) => Math.abs(c - radius)));
  const wLo = Math.min(...widths);
  const wHi = Math.max(...widths);
  const wDev = Math.max(Math.abs(wLo - weight), Math.abs(wHi - weight));
  const ok = dev <= RING_TOLERANCE.centre && wDev <= RING_TOLERANCE.width;
  console[ok ? "log" : "error"](
    `  ${ok ? "ok  " : "FAIL"} ${name}: ${centres.length} samples, ` +
      `max centre deviation ${dev.toFixed(3)}, ` +
      `width ${wLo.toFixed(3)}-${wHi.toFixed(3)}`,
  );
  return ok;
}

const CAT_SECTORS = [
  { name: "ear", from: 216, to: 230 },
  { name: "tail", from: 288, to: 322 },
];
const CAT_REACH_TOLERANCE = 0.12;

function catReach(png, size, angle) {
  const scale = size / 60;
  const t = (angle * Math.PI) / 180;
  const darkAt = (r) => {
    const x = Math.round((CENTRE + r * Math.cos(t)) * scale);
    const y = Math.round((CENTRE + r * Math.sin(t)) * scale);
    if (x < 0 || y < 0 || x >= size || y >= size) return false;
    return png.data[(y * size + x) * 4] < 128;
  };
  const seed = BAND.inner - 0.25;
  if (!darkAt(seed)) return null;
  let r = seed;
  while (r <= 24.45 && darkAt(r)) r += 0.004;
  return r - 0.004;
}

function checkCatReach(beforePng, afterPng, size) {
  let ok = true;
  for (const sector of CAT_SECTORS) {
    let worst = 0;
    let worstAngle = null;
    for (let a = sector.from; a <= sector.to; a += 0.5) {
      const ra = catReach(beforePng, size, a);
      const rb = catReach(afterPng, size, a);
      if (ra === null && rb === null) continue;
      if (ra === null || rb === null) {
        worst = Infinity;
        worstAngle = a;
        break;
      }
      if (Math.abs(ra - rb) > worst) {
        worst = Math.abs(ra - rb);
        worstAngle = a;
      }
    }
    const pass = worst <= CAT_REACH_TOLERANCE;
    if (!pass) ok = false;
    console[pass ? "log" : "error"](
      `  ${pass ? "ok  " : "FAIL"} ${sector.name} reaches as far as the original ` +
        `(worst delta ${worst === Infinity ? "missing" : worst.toFixed(3)} at ${worstAngle}deg)`,
    );
  }
  return ok;
}

async function verifyRings(browser, before, after) {
  const size = 1800;
  const png = await render(browser, after, size, "#FFFFFF");
  const originalPng = await render(browser, before, size, "#FFFFFF");
  const outer = checkRing(
    png,
    size,
    "outer ring",
    OUTER_R,
    OUTER_W,
    23.5,
    27.0,
    3400,
    null,
  );
  const inner = checkRing(
    png,
    size,
    "inner ring",
    INNER_R,
    INNER_W,
    22.9,
    24.2,
    1500,
    ARC_CHECK_SPANS,
  );
  const reach = checkCatReach(originalPng, png, size);
  return outer && inner && reach;
}

fs.rmSync(DIFF_DIR, { recursive: true, force: true });

const source = fs.readFileSync(SOURCE, "utf8");
const built = build(source);

console.log(
  `source ${source.length} B -> built ${built.length} B ` +
    `(${Math.round((1 - built.length / source.length) * 100)}% smaller)`,
);

const browser = await chromium.launch();
let failures = [];
let ringsOk = false;
try {
  console.log("\nartwork outside the rings must be unchanged:");
  failures = await verifyArtworkUnchanged(browser, source, built);
  console.log("\nrings must be true:");
  ringsOk = await verifyRings(browser, source, built);
} finally {
  await browser.close();
}

if (failures.length > 0) {
  fs.mkdirSync(DIFF_DIR, { recursive: true });
  for (const f of failures) {
    fs.writeFileSync(
      path.join(DIFF_DIR, `${f.label.replace(/[^a-z0-9]+/gi, "-")}.png`),
      PNG.sync.write(f.diff),
    );
  }
  console.error(`\n${failures.length} render(s) differ; diffs in ${DIFF_DIR}`);
}

if (failures.length > 0 || !ringsOk) {
  console.error("refusing to write logo.svg");
  process.exit(1);
}

fs.writeFileSync(OUTPUT, built);
console.log(`\nwrote ${path.relative(root, OUTPUT)}`);
