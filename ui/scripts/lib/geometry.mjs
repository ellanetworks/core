// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import svgpath from "svgpath";
import pc from "polygon-clipping";

export const CENTRE = 30;
export const DISC_R = 25.273438;
export const OUTER_R = 25.167;
export const OUTER_W = 1.101;
export const INNER_R = 23.524;
export const INNER_W = 0.717;
export const BAND = { inner: 22.85, outer: 24.05 };
export const WIDE_BAND = { inner: 22.3, outer: 24.35 };
export const KEYLINE = 0.55;
export const FLATTEN_TOLERANCE = 0.004;
export const SIMPLIFY_TOLERANCE = 0.008;
export const RING_SIMPLIFY_TOLERANCE = 0.004;
export const GRID = 1e5;
export const OUTER_RING = 0;
export const TOP_ARC = 1;
export const ARTWORK = 2;
export const BLACK_PATH_COUNT = 19;
export const PRECISION = Number(process.env.LOGO_PRECISION ?? 3);

export const round = (n) => Number(n.toFixed(PRECISION));

export function blackPaths(svg) {
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

export function firstSubpath(d) {
  const starts = [];
  for (let i = 0; i < d.length; i++) if (d[i] === "M") starts.push(i);
  return d.slice(starts[0], starts[1]).trim();
}

export function flatten(d, tolerance) {
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

export function simplifyOpen(points, tolerance) {
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

export function simplify(ring, tolerance) {
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

export const snap = (v) => Math.round(v * GRID) / GRID;

export function snapRing(ring) {
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

export function circlePoly(r, n = 1440, cx = CENTRE, cy = CENTRE) {
  const p = [];
  for (let i = 0; i < n; i++) {
    const t = (2 * Math.PI * i) / n;
    p.push([snap(cx + r * Math.cos(t)), snap(cy + r * Math.sin(t))]);
  }
  p.push([p[0][0], p[0][1]]);
  return [p];
}

export function dilate(rings, margin) {
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

export function catPieces(artworkPath, bounds, tolerance) {
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

export function occludedRing(cats) {
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

export function polysToPath(polys) {
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
