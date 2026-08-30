// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// oxlint-disable jsx-a11y/prefer-tag-over-role -- SVG has no <button> or <fieldset>; a role is the only way to expose these.

import { Fragment, useEffect, useRef, useState, type ReactNode } from "react";
import { Box, Tooltip } from "@mui/material";
import { alpha, useTheme } from "@mui/material/styles";
import type { InterfacesInfo, VlanInfo } from "@/queries/interfaces";

export type InterfaceSegment = "n2" | "n3" | "n6" | "api";

const SEGMENTS: InterfaceSegment[] = ["n2", "n3", "n6", "api"];

const VIEWBOX_WIDTH = 1000;
const MIN_VIEWBOX_HEIGHT = 300;
/** Generous enough to cover the drawing at any panel height. */
const MASK_HEIGHT = 1000;

const CORE = { x: 380, y: 40, w: 240, h: 230 };
const USER_PLANE = { x: 394, y: 162, w: 212, h: 90 };

const RADIOS_CX = 88;
const RADIOS_CY = 144;
const INTERNET_CX = 920;

const N2_Y = 100;
const UPLINK_Y = 199;
const DOWNLINK_Y = 215;
const TRACK_WIDTH = 9;

const TRACK_START = 145;
const TRACK_END = 865;

const N3_EDGE = CORE.x;
const N6_EDGE = CORE.x + CORE.w;

const PORT_SPAN = 30;
const PORT_DEPTH = 20;

const HEAD_LENGTH = 16;
const HEAD_HALF_WIDTH = 7;

/** An arrow into a socket stops at its outer face rather than under it. */
const SOCKET_FACE = PORT_DEPTH / 2;

const LABEL_SIZE = 12;
const CAPTION_SIZE = 11;

/** Source Code Pro advances 600/1000 per glyph, so panel widths are known. */
const CHAR_ADVANCE = 0.6;

const PANEL_FONT = 11;
const PANEL_LINE = 18;
const PANEL_PAD_X = 12;
const PANEL_PAD_Y = 10;
const PANEL_EDIT_WIDTH = 24;
const PANEL_LABEL_GAP = 10;
const PANEL_MARGIN = 8;

/**
 * The band that reveals each interface. It reaches from the interface's own
 * drawing down to the top of its panel, so the pointer never crosses dead
 * space on the way.
 */
const ZONES: Record<
  InterfaceSegment,
  { x: number; y: number; w: number; h: number }
> = {
  n2: { x: 130, y: 58, w: 265, h: 58 },
  n3: { x: 130, y: 170, w: 265, h: 62 },
  n6: { x: 605, y: 170, w: 275, h: 62 },
  api: { x: 605, y: 58, w: 275, h: 58 },
};

const PANELS: Record<
  InterfaceSegment,
  { x: number; y: number; anchor: "start" | "end" }
> = {
  n2: { x: 356, y: 116, anchor: "end" },
  n3: { x: 356, y: 232, anchor: "end" },
  n6: { x: 644, y: 232, anchor: "start" },
  api: { x: 644, y: 116, anchor: "start" },
};

const SEGMENT_LABELS: Record<InterfaceSegment, string> = {
  n2: "N2, NGAP and S1AP signalling",
  n3: "N3, GTP-U user plane",
  n6: "N6, external network",
  api: "API, management",
};

/** A row of the reveal panel; a continuation row carries no label of its own. */
type DetailLine = { label?: string; text: string; editable?: boolean };

const formatVlan = (vlan: VlanInfo): string =>
  `VLAN ${vlan.vlan_id ?? "—"}${vlan.master_interface ? ` on ${vlan.master_interface}` : ""}`;

/** The mechanism the UPF actually attached with, which is empty until it is up. */
export const formatDatapath = (attachMode?: string): string => {
  switch (attachMode) {
    case "xdp-native":
      return "eBPF · XDP native";
    case "xdp-generic":
      return "eBPF · XDP generic";
    case "tcx":
      return "eBPF · TCX";
    default:
      return "eBPF";
  }
};

/** Absent values drop out rather than leaving an em dash behind. */
export function detailsFor(
  segment: InterfaceSegment,
  interfaces: InterfacesInfo,
): DetailLine[] {
  const nic = (name?: string) =>
    name ? [{ label: "interface", text: name }] : [];

  const port = (value?: number) =>
    value === undefined ? [] : [{ label: "port", text: String(value) }];

  /** The label sits on the first address only; the rest read as a list. */
  const addresses = (values?: string[]) =>
    (values ?? []).map((text, index) => ({
      label: index === 0 ? "address" : undefined,
      text,
    }));

  const vlan = (value?: VlanInfo) =>
    value ? [{ label: "vlan", text: formatVlan(value) }] : [];

  switch (segment) {
    case "n2":
      return [
        ...nic(interfaces.n2?.interface),
        ...port(interfaces.n2?.port),
        ...addresses(interfaces.n2?.addresses),
      ];
    case "n3":
      return [
        ...nic(interfaces.n3?.name),
        ...addresses(interfaces.n3?.addresses),
        {
          label: "external",
          text: interfaces.n3?.external_address || "not set",
          editable: true,
        },
        ...vlan(interfaces.n3?.vlan),
      ];
    case "n6":
      return [
        ...nic(interfaces.n6?.name),
        ...addresses(interfaces.n6?.addresses),
        ...vlan(interfaces.n6?.vlan),
      ];
    case "api":
      return [
        ...port(interfaces.api?.port),
        ...addresses(interfaces.api?.addresses),
      ];
  }
}

const panelSize = (lines: DetailLine[], withEdit: boolean) => {
  const chars = (longest: number, text = "") => Math.max(longest, text.length);
  const labelColumn =
    lines.reduce((longest, l) => chars(longest, l.label), 0) *
    PANEL_FONT *
    CHAR_ADVANCE;
  const valueColumn =
    lines.reduce((longest, l) => chars(longest, l.text), 0) *
    PANEL_FONT *
    CHAR_ADVANCE;
  const edit = withEdit && lines.some((l) => l.editable) ? PANEL_EDIT_WIDTH : 0;

  return {
    labelColumn,
    w: labelColumn + PANEL_LABEL_GAP + valueColumn + edit + PANEL_PAD_X * 2,
    h: lines.length * PANEL_LINE + PANEL_PAD_Y * 2,
  };
};

type NicPortProps = {
  cx: number;
  cy: number;
  rotate: number;
  paper: string;
  housing: string;
  slot: string;
};

/**
 * A socket straddling the Ella Core boundary: one physical port per interface,
 * drawn as an RJ45 face. `rotate` turns the clip notch to face out of the box.
 */
function NicPort({ cx, cy, rotate, paper, housing, slot }: NicPortProps) {
  return (
    <g transform={`rotate(${rotate} ${cx} ${cy})`}>
      <rect
        x={cx - PORT_SPAN / 2}
        y={cy - PORT_DEPTH / 2}
        width={PORT_SPAN}
        height={PORT_DEPTH}
        rx={3}
        fill={paper}
        stroke={housing}
        strokeWidth={1.2}
      />
      <rect
        x={cx - PORT_SPAN / 2 + 4}
        y={cy - PORT_DEPTH / 2 + 5}
        width={PORT_SPAN - 8}
        height={PORT_DEPTH - 8}
        rx={1.5}
        fill={alpha(slot, 0.4)}
      />
      <rect
        x={cx - 4}
        y={cy - PORT_DEPTH / 2 + 1.5}
        width={8}
        height={4}
        rx={1}
        fill={alpha(slot, 0.4)}
      />
    </g>
  );
}

type GlyphProps = { cx: number; cy: number; scale: number; color: string };

function RadioGlyph({ cx, cy, scale, color }: GlyphProps) {
  return (
    <g
      transform={`translate(${cx} ${cy}) scale(${scale / 40}) translate(-480 480)`}
      fill={color}
    >
      <path d="M197.69-140q-23.53 0-40.61-17.08T140-197.69v-167.46q0-23.53 17.08-40.61t40.61-17.08h424.08v-171.77h45.38v171.77h95.16q23.53 0 40.61 17.08T820-365.15v167.46q0 23.53-17.08 40.61T762.31-140H197.69Zm0-45.39h564.62q5.38 0 8.84-3.46t3.46-8.84v-167.46q0-5.39-3.46-8.85t-8.84-3.46H197.69q-5.38 0-8.84 3.46t-3.46 8.85v167.46q0 5.38 3.46 8.84t8.84 3.46Zm110.08-69.83q10.46-10.6 10.46-26.35 0-15.74-10.6-26.2-10.61-10.46-26.35-10.46-15.74 0-26.2 10.6-10.46 10.61-10.46 26.35 0 15.74 10.6 26.2 10.6 10.46 26.35 10.46 15.74 0 26.2-10.6Zm146.08 0q10.46-10.6 10.46-26.35 0-15.74-10.57-26.2-10.57-10.46-26.54-10.46-15.97 0-26.43 10.6-10.46 10.61-10.46 26.35 0 15.74 10.56 26.2 10.57 10.46 26.54 10.46 15.97 0 26.44-10.6Zm145.69 0Q610-265.82 610-281.57q0-15.74-10.6-26.2-10.61-10.46-26.35-10.46-15.74 0-26.2 10.6-10.47 10.61-10.47 26.35 0 15.74 10.61 26.2 10.6 10.46 26.34 10.46 15.75 0 26.21-10.6Zm-39.31-398.7-31-31q24.33-24 52.87-36.85 28.53-12.84 62.36-12.84t62.15 12.84q28.31 12.85 53.08 36.85l-31 31q-15.92-15.54-37.31-25.43-21.38-9.88-46.61-9.88t-47.12 9.88q-21.88 9.89-37.42 25.43ZM471-743.15l-33.23-33.23q34.54-34.93 88.38-59.27Q580-860 644.46-860q64.46 0 118.12 24.35 53.65 24.34 88.57 59.27l-33.61 33.23q-27.92-29.77-71.96-50.62-44.04-20.84-101.12-20.84-57.08 0-101.31 20.84-44.23 20.85-72.15 50.62ZM185.39-185.39v-192.07 192.07Z" />
    </g>
  );
}

function GlobeGlyph({ cx, cy, scale, color }: GlyphProps) {
  return (
    <g
      transform={`translate(${cx} ${cy}) scale(${scale / 40}) translate(-480 480)`}
      fill={color}
    >
      <path d="M331.86-129.92q-69.37-29.92-120.68-81.21-51.31-51.29-81.25-120.63Q100-401.1 100-479.93q0-78.84 29.92-148.21t81.21-120.68q51.29-51.31 120.63-81.25Q401.1-860 479.93-860q78.84 0 148.21 29.92t120.68 81.21q51.31 51.29 81.25 120.63Q860-558.9 860-480.07q0 78.84-29.92 148.21t-81.21 120.68q-51.29 51.31-120.63 81.25Q558.9-100 480.07-100q-78.84 0-148.21-29.92Zm105.91-16.85v-80.85q-34.68 0-58.46-25.23-23.77-25.23-23.77-59.84v-42.85L154-557.08q-4.23 19.23-6.42 38.38-2.19 19.14-2.19 38.79 0 127.6 83.15 222.87 83.15 95.27 209.23 110.27Zm289.38-106.46q42.85-46.85 65.16-105.35 22.3-58.5 22.3-121.56 0-103.93-56.84-189.28-56.85-85.35-152.69-124.19v18.51q0 34.49-23.78 59.72-23.77 25.23-58.45 25.23h-85.08v85.07q0 17.17-12.92 28.28-12.93 11.11-29.79 11.11h-82.37V-480h253q17.17 0 28.28 12.62 11.11 12.61 11.11 29.61v125.08h42.06q28.4 0 50.24 16.57 21.84 16.58 29.77 42.89Z" />
    </g>
  );
}

function ConsoleGlyph({ cx, cy, scale, color }: GlyphProps) {
  return (
    <g
      transform={`translate(${cx} ${cy}) scale(${scale / 40}) translate(-480 480)`}
      fill={color}
    >
      <path d="M55.39-150.77v-45.39h849.22v45.39H55.39Zm102.3-100q-23.53 0-40.61-17.08T100-308.46v-444.61q0-23.53 17.08-40.62 17.08-17.08 40.61-17.08h644.62q23.53 0 40.61 17.08Q860-776.6 860-753.07v444.61q0 23.53-17.08 40.61t-40.61 17.08H157.69Zm0-45.38h644.62q4.61 0 8.46-3.85 3.84-3.85 3.84-8.46v-444.61q0-4.62-3.84-8.47-3.85-3.84-8.46-3.84H157.69q-4.61 0-8.46 3.84-3.84 3.85-3.84 8.47v444.61q0 4.61 3.84 8.46 3.85 3.85 8.46 3.85Zm-12.3 0v-469.23 469.23Z" />
    </g>
  );
}

function EditPencil({
  x,
  y,
  color,
  onClick,
}: {
  x: number;
  y: number;
  color: string;
  onClick: () => void;
}) {
  const [focused, setFocused] = useState(false);

  return (
    <Tooltip title="Edit external address">
      <g
        role="button"
        tabIndex={0}
        aria-label="Edit N3 external address"
        onClick={onClick}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            onClick();
          }
        }}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        style={{ cursor: "pointer", outline: "none" }}
      >
        <rect
          x={x - 9}
          y={y - 9}
          width={18}
          height={18}
          rx={4}
          fill="transparent"
          stroke={focused ? color : "none"}
          strokeWidth={1.5}
        />
        <g
          transform={`translate(${x - 6} ${y - 6})`}
          stroke={color}
          strokeWidth={1.3}
          fill="none"
          strokeLinejoin="round"
        >
          <path d="M0 9 L0.5 6.5 L7 0 L9 2 L2.5 8.5 Z" />
          <path d="M6 1 L8 3" />
        </g>
      </g>
    </Tooltip>
  );
}

export type NetworkTopologyProps = {
  interfaces: InterfacesInfo;
  datapathAttachMode?: string;
  canEdit: boolean;
  onEditN3: () => void;
};

export default function NetworkTopology({
  interfaces,
  datapathAttachMode,
  canEdit,
  onEditN3,
}: NetworkTopologyProps) {
  const theme = useTheme();
  const [active, setActive] = useState<InterfaceSegment | null>(null);

  const clearTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (clearTimer.current) clearTimeout(clearTimer.current);
    },
    [],
  );

  const reveal = (segment: InterfaceSegment) => {
    if (clearTimer.current) clearTimeout(clearTimer.current);
    setActive(segment);
  };

  const conceal = () => {
    if (clearTimer.current) clearTimeout(clearTimer.current);
    clearTimer.current = setTimeout(() => setActive(null), 90);
  };

  const ink = theme.palette.text.primary;
  const muted = theme.palette.text.secondary;
  const line = theme.palette.divider;
  const paper = theme.palette.background.paper;
  const control = theme.palette.primary.main;
  const uplink = theme.palette.chart.uplink;
  const downlink = theme.palette.chart.downlink;

  const details = Object.fromEntries(
    SEGMENTS.map((id) => [id, detailsFor(id, interfaces)]),
  ) as Record<InterfaceSegment, DetailLine[]>;

  const viewBoxHeight = SEGMENTS.reduce((tallest, id) => {
    if (details[id].length === 0) return tallest;
    return Math.max(
      tallest,
      PANELS[id].y + panelSize(details[id], canEdit).h + 10,
    );
  }, MIN_VIEWBOX_HEIGHT);

  const planeBar = (bar: typeof USER_PLANE) => (
    <rect
      x={bar.x}
      y={bar.y}
      width={bar.w}
      height={bar.h}
      rx={10}
      fill={alpha(control, 0.06)}
      stroke={alpha(control, 0.35)}
    />
  );

  /**
   * The bar stops where the head begins: run it to the tip instead and it fills
   * the triangle in, leaving a blunt end with two wings rather than a point.
   */
  const arrow = (
    from: number,
    tip: number,
    y: number,
    color: string,
    headed: boolean,
  ) => {
    const base = headed ? tip - Math.sign(tip - from) * HEAD_LENGTH : tip;
    return (
      <>
        <line
          x1={from}
          y1={y}
          x2={base}
          y2={y}
          stroke={color}
          strokeWidth={TRACK_WIDTH}
          strokeLinecap="butt"
        />
        {headed && (
          <path
            d={`M${base} ${y - HEAD_HALF_WIDTH} L${tip} ${y} L${base} ${y + HEAD_HALF_WIDTH} Z`}
            fill={color}
          />
        )}
      </>
    );
  };

  const heading = { fill: ink, fontSize: 13, fontWeight: 600 };
  const title = {
    fill: ink,
    fontSize: LABEL_SIZE,
    textAnchor: "middle" as const,
  };

  const panel = (id: InterfaceSegment) => {
    const lines = details[id];
    if (lines.length === 0) return null;

    const { x, y, anchor } = PANELS[id];
    const { w, h, labelColumn } = panelSize(lines, canEdit);
    // A long IPv6 can outgrow the space on its side, so the panel gives way
    // rather than running off the drawing.
    const left =
      anchor === "end"
        ? Math.max(PANEL_MARGIN, x - w)
        : Math.min(x, VIEWBOX_WIDTH - PANEL_MARGIN - w);
    return (
      <g>
        <rect
          x={left}
          y={y}
          width={w}
          height={h}
          rx={8}
          fill={paper}
          stroke={line}
          filter="url(#ella-panel-shadow)"
        />
        {lines.map((detail, index) => {
          const baseline =
            y + PANEL_PAD_Y + PANEL_FONT + index * PANEL_LINE - 4;
          return (
            <Fragment key={`${detail.label ?? ""}-${detail.text}`}>
              {detail.label && (
                <text
                  x={left + PANEL_PAD_X}
                  y={baseline}
                  fill={muted}
                  fontSize={PANEL_FONT}
                >
                  {detail.label}
                </text>
              )}
              <text
                x={left + PANEL_PAD_X + labelColumn + PANEL_LABEL_GAP}
                y={baseline}
                fill={ink}
                fontSize={PANEL_FONT}
              >
                {detail.text}
              </text>
              {detail.editable && canEdit && (
                <EditPencil
                  x={left + w - PANEL_PAD_X - 6}
                  y={baseline - 4}
                  color={control}
                  onClick={onEditN3}
                />
              )}
            </Fragment>
          );
        })}
      </g>
    );
  };

  const graphics: Record<InterfaceSegment, ReactNode> = {
    n2: (
      <>
        <line
          x1={TRACK_START}
          y1={N2_Y}
          x2={CORE.x - 22}
          y2={N2_Y}
          stroke={control}
          strokeWidth={1.6}
          markerEnd="url(#ella-arrow)"
        />
        <NicPort
          cx={CORE.x}
          cy={N2_Y}
          rotate={-90}
          paper={paper}
          housing={muted}
          slot={ink}
        />
        <text x={253} y={88} {...title}>
          N2 · NGAP / S1AP
        </text>
      </>
    ),
    n3: (
      <>
        <g opacity={0.45}>
          {arrow(
            TRACK_START,
            active === "n3" ? N3_EDGE - SOCKET_FACE : N3_EDGE,
            UPLINK_Y,
            uplink,
            active === "n3",
          )}
          {arrow(N3_EDGE, TRACK_START, DOWNLINK_Y, downlink, true)}
        </g>
        <NicPort
          cx={N3_EDGE}
          cy={(UPLINK_Y + DOWNLINK_Y) / 2}
          rotate={-90}
          paper={paper}
          housing={muted}
          slot={ink}
        />
        <text x={262} y={180} {...title}>
          N3 · GTP-U
        </text>
      </>
    ),
    n6: (
      <>
        <g opacity={0.45}>
          {arrow(N6_EDGE, TRACK_END, UPLINK_Y, uplink, true)}
          {arrow(
            TRACK_END,
            active === "n6" ? N6_EDGE + SOCKET_FACE : N6_EDGE,
            DOWNLINK_Y,
            downlink,
            active === "n6",
          )}
        </g>
        <NicPort
          cx={N6_EDGE}
          cy={(UPLINK_Y + DOWNLINK_Y) / 2}
          rotate={90}
          paper={paper}
          housing={muted}
          slot={ink}
        />
        <text x={742} y={180} {...title}>
          N6
        </text>
      </>
    ),
    api: (
      <>
        <NicPort
          cx={N6_EDGE}
          cy={N2_Y}
          rotate={90}
          paper={paper}
          housing={muted}
          slot={ink}
        />
        <line
          x1={TRACK_END}
          y1={N2_Y}
          x2={N6_EDGE + 22}
          y2={N2_Y}
          stroke={control}
          strokeWidth={1.6}
          markerEnd="url(#ella-arrow)"
        />
        <text x={753} y={88} {...title}>
          API
        </text>
        <ConsoleGlyph cx={INTERNET_CX} cy={N2_Y} scale={2.5} color={ink} />
      </>
    ),
  };

  const fades: [InterfaceSegment, number, number, string, string][] = [
    ["n3", N3_EDGE - 20, N3_EDGE + 20, "#ffffff", "#333333"],
    ["n6", N6_EDGE - 20, N6_EDGE + 20, "#333333", "#ffffff"],
  ];

  return (
    <Box
      component="svg"
      viewBox={`0 0 ${VIEWBOX_WIDTH} ${viewBoxHeight}`}
      preserveAspectRatio="xMidYMid meet"
      role="group"
      aria-label="Ella Core network interfaces"
      sx={{
        width: "100%",
        height: "auto",
        display: "block",
        fontFamily: theme.typography.fontFamily,
      }}
    >
      <defs>
        <marker
          id="ella-arrow"
          markerWidth={7}
          markerHeight={7}
          refX={6}
          refY={3.5}
          orient="auto"
        >
          <path d="M0 0 L7 3.5 L0 7 z" fill={control} />
        </marker>
        <filter
          id="ella-panel-shadow"
          x="-20%"
          y="-20%"
          width="140%"
          height="140%"
        >
          <feDropShadow dx="0" dy="2" stdDeviation="3" floodOpacity="0.18" />
        </filter>
        {fades.map(([id, x1, x2, from, to]) => (
          <linearGradient
            key={id}
            id={`ella-${id}-fade`}
            gradientUnits="userSpaceOnUse"
            x1={x1}
            x2={x2}
          >
            <stop offset="0" stopColor={from} />
            <stop offset="1" stopColor={to} />
          </linearGradient>
        ))}
        {fades.map(([id]) => (
          <mask
            key={id}
            id={`ella-${id}-mask`}
            maskUnits="userSpaceOnUse"
            x={0}
            y={0}
            width={VIEWBOX_WIDTH}
            height={MASK_HEIGHT}
          >
            <rect
              x={0}
              y={0}
              width={VIEWBOX_WIDTH}
              height={MASK_HEIGHT}
              fill={`url(#ella-${id}-fade)`}
            />
          </mask>
        ))}
      </defs>

      <g>
        <RadioGlyph cx={RADIOS_CX} cy={RADIOS_CY} scale={2.5} color={ink} />
        <text x={RADIOS_CX} y={208} textAnchor="middle" {...heading}>
          Radios
        </text>
      </g>

      <g>
        <GlobeGlyph
          cx={INTERNET_CX}
          cy={(UPLINK_Y + DOWNLINK_Y) / 2}
          scale={2.5}
          color={ink}
        />
        <text x={INTERNET_CX} y={264} textAnchor="middle" {...heading}>
          Internet
        </text>
      </g>

      <g>
        <rect
          x={CORE.x}
          y={CORE.y}
          width={CORE.w}
          height={CORE.h}
          rx={10}
          fill={paper}
          stroke={line}
        />
        <text x={500} y={66} textAnchor="middle" {...heading}>
          Ella Core
        </text>
        {planeBar(USER_PLANE)}
        <text x={500} y={180} {...title} fill={control} fontWeight={600}>
          User Plane
        </text>
        <text
          x={500}
          y={240}
          textAnchor="middle"
          fill={muted}
          fontSize={CAPTION_SIZE}
        >
          {formatDatapath(datapathAttachMode)}
        </text>
      </g>

      <g
        mask={
          active === "n3" || active === "n6"
            ? `url(#ella-${active}-mask)`
            : undefined
        }
        opacity={active === "n2" || active === "api" ? 0.2 : 1}
        style={{ transition: "opacity 200ms" }}
      >
        <g opacity={0.45}>
          {arrow(N3_EDGE, N6_EDGE, UPLINK_Y, uplink, false)}
          {arrow(N6_EDGE, N3_EDGE, DOWNLINK_Y, downlink, false)}
        </g>
      </g>

      <g style={{ pointerEvents: "none" }}>
        {SEGMENTS.map((id) => (
          <g
            key={id}
            data-segment={id}
            opacity={active !== null && active !== id ? 0.2 : 1}
            style={{ transition: "opacity 200ms" }}
          >
            {graphics[id]}
          </g>
        ))}
      </g>

      {SEGMENTS.filter((id) => details[id].length > 0).map((id) => (
        <g
          key={id}
          onPointerEnter={(event) => {
            if (event.pointerType !== "touch") reveal(id);
          }}
          onPointerLeave={(event) => {
            if (event.pointerType !== "touch") conceal();
          }}
          onBlur={(event) => {
            if (!event.currentTarget.contains(event.relatedTarget as Node)) {
              conceal();
            }
          }}
        >
          <rect
            x={ZONES[id].x}
            y={ZONES[id].y}
            width={ZONES[id].w}
            height={ZONES[id].h}
            fill="transparent"
            role="button"
            tabIndex={0}
            aria-label={SEGMENT_LABELS[id]}
            aria-expanded={active === id}
            onFocus={() => reveal(id)}
            onClick={() => reveal(id)}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                reveal(id);
              }
            }}
            style={{ cursor: "pointer", outline: "none", pointerEvents: "all" }}
          />
          {active === id && panel(id)}
        </g>
      ))}
    </Box>
  );
}
