// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useMemo } from "react";
import { Box } from "@mui/material";
import { alpha, useTheme } from "@mui/material/styles";
import type { InterfacesInfo } from "@/queries/interfaces";

export type InterfaceSegment = "n2" | "n3" | "n6" | "api";

const VIEWBOX_WIDTH = 1000;
const VIEWBOX_HEIGHT = 340;

const CORE = { x: 380, y: 40, w: 240, h: 230 };
const CONTROL_PLANE = { x: 394, y: 84, w: 212, h: 32 };
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

/** Where each wire meets its socket, and so where N3 ends and N6 begins. */
const N3_EDGE = CORE.x;
const N6_EDGE = CORE.x + CORE.w;

const PORT_SPAN = 30;
const PORT_DEPTH = 20;

const HEAD_LENGTH = 16;
const HEAD_HALF_WIDTH = 7;

/** An arrow into a socket stops at its outer face rather than under it. */
const SOCKET_FACE = PORT_DEPTH / 2;

const formatEndpoint = (addresses?: string[], port?: number): string => {
  if (!addresses || addresses.length === 0) return "—";
  const joined = addresses.join(", ");
  return port === undefined ? joined : `${joined} · port ${port}`;
};

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
      transform={`translate(${cx} ${cy}) scale(${scale}) translate(${-cx} ${-cy})`}
      stroke={color}
      strokeWidth={2}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path
        d={`M${cx - 11} ${cy + 26} L${cx} ${cy - 6} L${cx + 11} ${cy + 26}`}
      />
      <path d={`M${cx - 6} ${cy + 12} L${cx + 6} ${cy + 12}`} />
      <path d={`M${cx - 9} ${cy - 12} A 12 12 0 0 0 ${cx - 9} ${cy - 1}`} />
      <path d={`M${cx + 9} ${cy - 12} A 12 12 0 0 1 ${cx + 9} ${cy - 1}`} />
      <path
        d={`M${cx - 16} ${cy - 18} A 21 21 0 0 0 ${cx - 16} ${cy + 3}`}
        opacity={0.5}
      />
      <path
        d={`M${cx + 16} ${cy - 18} A 21 21 0 0 1 ${cx + 16} ${cy + 3}`}
        opacity={0.5}
      />
    </g>
  );
}

function GlobeGlyph({ cx, cy, scale, color }: GlyphProps) {
  return (
    <g
      transform={`translate(${cx} ${cy}) scale(${scale}) translate(${-cx} ${-cy})`}
      stroke={color}
      strokeWidth={2}
      fill="none"
    >
      <circle cx={cx} cy={cy} r={22} />
      <ellipse cx={cx} cy={cy} rx={22} ry={9} />
      <path
        d={`M${cx} ${cy - 22} A 11 22 0 0 0 ${cx} ${cy + 22} A 11 22 0 0 0 ${cx} ${cy - 22}`}
      />
    </g>
  );
}

function ConsoleGlyph({
  cx,
  cy,
  color,
}: {
  cx: number;
  cy: number;
  color: string;
}) {
  return (
    <g stroke={color} strokeWidth={1.6} fill="none" strokeLinejoin="round">
      <rect x={cx - 15} y={cy - 11} width={30} height={21} rx={2} />
      <path
        d={`M${cx - 21} ${cy + 15} L${cx - 17} ${cy + 10} L${cx + 17} ${cy + 10} L${cx + 21} ${cy + 15} Z`}
      />
    </g>
  );
}

export type NetworkTopologyProps = {
  interfaces: InterfacesInfo;
  datapathAttachMode?: string;
  active: InterfaceSegment | null;
  onActiveChange: (segment: InterfaceSegment | null) => void;
};

export default function NetworkTopology({
  interfaces,
  datapathAttachMode,
  active,
  onActiveChange,
}: NetworkTopologyProps) {
  const theme = useTheme();

  const ink = theme.palette.text.primary;
  const muted = theme.palette.text.secondary;
  const line = theme.palette.divider;
  const paper = theme.palette.background.paper;
  const control = theme.palette.primary.main;
  const uplink = theme.palette.chart.uplink;
  const downlink = theme.palette.chart.downlink;

  const summary = useMemo(
    () =>
      [
        `Network topology.`,
        `Radios signal Ella Core over N2 at ${formatEndpoint(interfaces.n2?.addresses, interfaces.n2?.port)}.`,
        `Subscriber traffic reaches the user plane over N3 (GTP-U) at ${formatEndpoint(interfaces.n3?.addresses)} and leaves to the internet over N6 at ${formatEndpoint(interfaces.n6?.addresses)}.`,
        `The user plane runs on ${formatDatapath(datapathAttachMode)}.`,
        `The API is served on ${formatEndpoint(interfaces.api?.addresses, interfaces.api?.port)}.`,
      ].join(" "),
    [interfaces, datapathAttachMode],
  );

  const segment = (id: InterfaceSegment) => ({
    onMouseEnter: () => onActiveChange(id),
    onMouseLeave: () => onActiveChange(null),
    opacity: active !== null && active !== id ? 0.2 : 1,
    style: { cursor: "pointer", transition: "opacity 200ms" },
  });

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
  const caption = { fill: muted, fontSize: 11 };
  const edgeLabel = { fill: ink, fontSize: 12 };

  const fades: [InterfaceSegment, number, number, string, string][] = [
    ["n3", N3_EDGE - 20, N3_EDGE + 20, "#ffffff", "#333333"],
    ["n6", N6_EDGE - 20, N6_EDGE + 20, "#333333", "#ffffff"],
  ];

  return (
    <Box
      component="svg"
      viewBox={`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`}
      preserveAspectRatio="xMidYMid meet"
      // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- an inline <svg> carries role="img" so its summary is read in place of the shapes.
      role="img"
      aria-label={summary}
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
            height={VIEWBOX_HEIGHT}
          >
            <rect
              x={0}
              y={0}
              width={VIEWBOX_WIDTH}
              height={VIEWBOX_HEIGHT}
              fill={`url(#ella-${id}-fade)`}
            />
          </mask>
        ))}
      </defs>

      <g>
        <RadioGlyph cx={RADIOS_CX} cy={RADIOS_CY} scale={1.5} color={control} />
        <text x={RADIOS_CX} y={208} textAnchor="middle" {...heading}>
          Radios
        </text>
      </g>

      <g>
        <GlobeGlyph
          cx={INTERNET_CX}
          cy={(UPLINK_Y + DOWNLINK_Y) / 2}
          scale={1.35}
          color={muted}
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
        {planeBar(CONTROL_PLANE)}
        <text
          x={500}
          y={CONTROL_PLANE.y + CONTROL_PLANE.h / 2}
          textAnchor="middle"
          dominantBaseline="central"
          fill={control}
          fontSize={12}
          fontWeight={600}
        >
          Control Plane
        </text>
        {planeBar(USER_PLANE)}
        <text
          x={500}
          y={180}
          textAnchor="middle"
          fill={control}
          fontSize={12}
          fontWeight={600}
        >
          User Plane
        </text>
        <text x={500} y={240} textAnchor="middle" {...caption}>
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
        <g opacity={0.2}>
          {arrow(N3_EDGE, N6_EDGE, UPLINK_Y, uplink, false)}
          {arrow(N6_EDGE, N3_EDGE, DOWNLINK_Y, downlink, false)}
        </g>
      </g>

      <g {...segment("n2")}>
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
          slot={control}
        />
        <text x={253} y={88} textAnchor="middle" {...edgeLabel}>
          N2 · NGAP / S1AP
        </text>
        <text x={253} y={120} textAnchor="middle" {...caption}>
          {interfaces.n2?.interface ?? "—"}
        </text>
      </g>

      <g {...segment("n3")}>
        <g opacity={0.2}>
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
        <text x={262} y={180} textAnchor="middle" {...edgeLabel}>
          N3 · GTP-U
        </text>
        <text x={262} y={244} textAnchor="middle" {...caption}>
          {interfaces.n3?.name ?? "—"}
        </text>
      </g>

      <g {...segment("n6")}>
        <g opacity={0.2}>
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
        <text x={742} y={180} textAnchor="middle" {...edgeLabel}>
          N6
        </text>
        <text x={742} y={244} textAnchor="middle" {...caption}>
          {interfaces.n6?.name ?? "—"}
        </text>
      </g>

      <g {...segment("api")}>
        <NicPort
          cx={500}
          cy={CORE.y + CORE.h}
          rotate={180}
          paper={paper}
          housing={muted}
          slot={muted}
        />
        <line
          x1={500}
          y1={CORE.y + CORE.h + 10}
          x2={500}
          y2={290}
          stroke={muted}
          strokeWidth={1.4}
          strokeDasharray="3 5"
        />
        <text x={526} y={274} {...edgeLabel}>
          API
        </text>
        <ConsoleGlyph cx={500} cy={303} color={muted} />
        <text x={500} y={334} textAnchor="middle" {...caption}>
          management
        </text>
      </g>
    </Box>
  );
}
