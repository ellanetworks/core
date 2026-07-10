// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import * as React from "react";
import { Box, IconButton, Tooltip, Divider, Collapse } from "@mui/material";
import {
  ExpandMore as ExpandMoreIcon,
  ChevronRight as ChevronRightIcon,
} from "@mui/icons-material";
import type { DecodedNGAPMessage } from "@/queries/radio_events";

const MONO_FONT =
  "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace";
const INDENT_PX = 16;
const CHEVRON_W = 24;
const ROW_H = 22;

// --- Enum helpers ---

type EnumLike = {
  type: "enum";
  value: unknown;
  label: string;
  unknown?: boolean;
};

const isEnumLike = (x: unknown): x is EnumLike =>
  !!x &&
  typeof x === "object" &&
  (x as any).type === "enum" &&
  "label" in (x as any);

const formatEnum = (e: EnumLike) => `${e.label} (${String(e.value)})`;

// --- IE helpers ---

type IEFields = {
  idEnum?: EnumLike;
  criticalityEnum?: EnumLike;
  value?: unknown;
  error?: string;
};

const extractIEFields = (x: any): IEFields => {
  if (!x || typeof x !== "object") return {};
  const idEnum = (x.ID ?? x.id) as EnumLike | undefined;
  const criticalityEnum = (x.Criticality ?? x.criticality) as
    | EnumLike
    | undefined;
  const value = (x.Value ?? x.value) as unknown;
  const error = (x.Error ?? x.error) as string | undefined;
  return { idEnum, criticalityEnum, value, error };
};

const isNgapIE = (x: unknown) => {
  const { idEnum, criticalityEnum } = extractIEFields(x as any);
  return isEnumLike(idEnum) && isEnumLike(criticalityEnum);
};

// --- Bitrate helpers ---

const formatBps = (bps: number): string => {
  if (bps >= 1_000_000_000)
    return `${(bps / 1_000_000_000).toFixed(bps % 1_000_000_000 === 0 ? 0 : 1)} Gbps`;
  if (bps >= 1_000_000)
    return `${(bps / 1_000_000).toFixed(bps % 1_000_000 === 0 ? 0 : 1)} Mbps`;
  if (bps >= 1_000)
    return `${(bps / 1_000).toFixed(bps % 1_000 === 0 ? 0 : 1)} Kbps`;
  return `${bps} bps`;
};

const isBpsObject = (obj: Record<string, unknown>): boolean =>
  obj.unit === "bps";

// --- Tree primitives ---

/**
 * Base tree row. Computes its own left padding from depth.
 * Renders either a clickable chevron (expandable) or an inert spacer (leaf).
 */
const TreeRow: React.FC<{
  depth: number;
  expandable?: boolean;
  open?: boolean;
  onToggle?: () => void;
  children: React.ReactNode;
}> = ({ depth, expandable = false, open = false, onToggle, children }) => (
  <Box
    sx={{
      display: "flex",
      alignItems: "center",
      minHeight: ROW_H,
      pl: `${depth * INDENT_PX}px`,
    }}
  >
    {expandable ? (
      <Tooltip title={open ? "Collapse" : "Expand"}>
        <IconButton
          size="small"
          onClick={onToggle}
          sx={{ p: 0.25, width: CHEVRON_W, flexShrink: 0 }}
        >
          {open ? (
            <ExpandMoreIcon fontSize="small" />
          ) : (
            <ChevronRightIcon fontSize="small" />
          )}
        </IconButton>
      </Tooltip>
    ) : (
      <Box sx={{ width: CHEVRON_W, flexShrink: 0 }} />
    )}
    {children}
  </Box>
);

/** Leaf key-value row. */
const KVLine: React.FC<{ depth: number; k: string; v: React.ReactNode }> = ({
  depth,
  k,
  v,
}) => (
  <TreeRow depth={depth}>
    <Box component="span" sx={{ color: "text.secondary", whiteSpace: "pre" }}>
      {k + ": "}
    </Box>
    <Box component="span" sx={{ wordBreak: "break-word", minWidth: 0 }}>
      {v}
    </Box>
  </TreeRow>
);

// --- Collapsible sections ---

const ChildSection: React.FC<{
  depth: number;
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}> = ({ depth, title, defaultOpen = false, children }) => {
  const [open, setOpen] = React.useState(defaultOpen);
  return (
    <>
      <TreeRow
        depth={depth}
        expandable
        open={open}
        onToggle={() => setOpen((s) => !s)}
      >
        <Box component="span" sx={{ color: "text.secondary" }}>
          {title}
        </Box>
      </TreeRow>
      <Collapse in={open}>{children}</Collapse>
    </>
  );
};

// --- NAS-PDU helpers ---

const isNasPdu = (v: unknown): boolean =>
  !!v && typeof v === "object" && (v as any).protocol === "NAS";

const getNasHeader = (nasPdu: any): string => {
  const decoded = nasPdu?.decoded;
  if (!decoded) return "undecoded";
  if (decoded.error) return "decode error";
  if (decoded.encrypted) return "encrypted";

  const messageType =
    // 5GS NAS (NGAP): GMM / GSM
    decoded.gmm_message?.gmm_header?.message_type?.label ||
    decoded.gsm_message?.gsm_header?.message_type?.label ||
    // EPS NAS (S1AP): EMM / ESM
    decoded.emm_message?.emm_header?.message_type?.label ||
    decoded.esm_message?.esm_header?.message_type?.label ||
    "Unknown";

  const secHeader =
    decoded.security_header?.security_header_type?.label || "Plain NAS";
  return `${messageType} (${secHeader})`;
};

// --- NRPPa-PDU helpers ---

const isNrppaPdu = (v: unknown): boolean =>
  !!v && typeof v === "object" && (v as any).protocol === "NRPPa";

const getNrppaHeader = (nrppaPdu: any): string => {
  const decoded = nrppaPdu?.decoded;
  if (!decoded) return "undecoded";
  if (decoded.error) return "decode error";
  return decoded.kind?.label || "Unknown";
};

/**
 * Renders an embedded protocol PDU (NAS or NRPPa) with a distinct accent
 * border, a header line ("title — summary"), the raw hex, and the decoded tree.
 */
const ProtocolPduBlock: React.FC<{
  pdu: any;
  depth: number;
  title: string;
  header: string;
  accentColor: string;
}> = ({ pdu, depth, title, header, accentColor }) => {
  const [open, setOpen] = React.useState(true);

  return (
    <>
      <TreeRow
        depth={depth}
        expandable
        open={open}
        onToggle={() => setOpen((s) => !s)}
      >
        <Box component="span" sx={{ color: "text.secondary" }}>
          {title}
          {" \u2014\u00A0"}
        </Box>
        <Box component="span" sx={{ fontWeight: 600 }}>
          {header}
        </Box>
      </TreeRow>
      <Collapse in={open}>
        <Box
          sx={{
            borderLeft: 3,
            borderColor: accentColor,
            ml: `${(depth + 1) * INDENT_PX + CHEVRON_W / 2}px`,
            pl: 1.5,
          }}
        >
          {pdu.raw_hex && (
            <KVLine depth={0} k="raw_hex" v={String(pdu.raw_hex)} />
          )}
          {pdu.decoded && <GenericNode value={pdu.decoded} depth={0} />}
        </Box>
      </Collapse>
    </>
  );
};

const NasPduBlock: React.FC<{ nasPdu: any; depth: number; title: string }> = ({
  nasPdu,
  depth,
  title,
}) => (
  <ProtocolPduBlock
    pdu={nasPdu}
    depth={depth}
    title={title}
    header={getNasHeader(nasPdu)}
    accentColor="info.main"
  />
);

const NrppaPduBlock: React.FC<{
  nrppaPdu: any;
  depth: number;
  title: string;
}> = ({ nrppaPdu, depth, title }) => (
  <ProtocolPduBlock
    pdu={nrppaPdu}
    depth={depth}
    title={title}
    header={getNrppaHeader(nrppaPdu)}
    accentColor="secondary.main"
  />
);

// --- NGAP IE block ---

const NgapIEBlock: React.FC<{ ie: any; depth: number; label?: string }> = ({
  ie,
  depth,
  label,
}) => {
  const { idEnum, value, error } = extractIEFields(ie);
  const title = isEnumLike(idEnum)
    ? `${idEnum.label} (${String(idEnum.value)})`
    : (label ?? "Information Element");

  // Inline primitive/enum values on the header row (no expand/collapse needed)
  const isInline =
    value == null ||
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean" ||
    isEnumLike(value);

  const inlineDisplay = isInline
    ? value == null
      ? "\u2014"
      : isEnumLike(value)
        ? formatEnum(value)
        : String(value)
    : null;

  const [open, setOpen] = React.useState(true);

  if (isInline) {
    return (
      <>
        <KVLine depth={depth} k={title} v={inlineDisplay!} />
        {error && <KVLine depth={depth + 1} k="Error" v={String(error)} />}
      </>
    );
  }

  // NAS-PDU: render with protocol layer distinction
  if (isNasPdu(value)) {
    return (
      <>
        <NasPduBlock nasPdu={value} depth={depth} title={title} />
        {error && <KVLine depth={depth + 1} k="Error" v={String(error)} />}
      </>
    );
  }

  // NRPPa-PDU: render with protocol layer distinction
  if (isNrppaPdu(value)) {
    return (
      <>
        <NrppaPduBlock nrppaPdu={value} depth={depth} title={title} />
        {error && <KVLine depth={depth + 1} k="Error" v={String(error)} />}
      </>
    );
  }

  return (
    <>
      <TreeRow
        depth={depth}
        expandable
        open={open}
        onToggle={() => setOpen((s) => !s)}
      >
        <Box component="span" sx={{ color: "text.secondary" }}>
          {title}
        </Box>
      </TreeRow>
      <Collapse in={open}>
        {error && <KVLine depth={depth + 1} k="Error" v={String(error)} />}
        <GenericNode value={value} depth={depth + 1} />
      </Collapse>
    </>
  );
};

const summarizeObject = (
  obj: Record<string, unknown>,
  maxFields = 3,
): string => {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(obj)) {
    if (parts.length >= maxFields) break;
    if (v == null) continue;
    if (
      typeof v === "string" ||
      typeof v === "number" ||
      typeof v === "boolean"
    ) {
      parts.push(`${k}: ${String(v)}`);
    } else if (isEnumLike(v)) {
      parts.push(`${k}: ${v.label}`);
    }
  }
  return parts.join(", ");
};

const CollapsibleArray: React.FC<{
  items: unknown[];
  depth: number;
  label?: string;
}> = ({ items, depth, label }) => {
  const [open, setOpen] = React.useState(true);
  const childDepth = label ? depth + 1 : depth;

  // Single-element arrays: render the item directly without extra nesting
  if (
    items.length === 1 &&
    items[0] != null &&
    typeof items[0] === "object" &&
    !Array.isArray(items[0]) &&
    !isEnumLike(items[0])
  ) {
    return (
      <CollapsibleObject
        obj={items[0] as Record<string, unknown>}
        depth={depth}
        label={label}
      />
    );
  }

  return (
    <>
      {label && (
        <TreeRow
          depth={depth}
          expandable
          open={open}
          onToggle={() => setOpen((s) => !s)}
        >
          <Box component="span" sx={{ color: "text.secondary" }}>
            {label}
          </Box>
        </TreeRow>
      )}
      <Collapse in={open || !label}>
        {items.map((item, i) => {
          if (isNgapIE(item)) {
            return <NgapIEBlock key={i} ie={item} depth={childDepth} />;
          }
          if (item == null || typeof item !== "object" || isEnumLike(item)) {
            const display =
              item == null
                ? "\u2014"
                : isEnumLike(item)
                  ? formatEnum(item)
                  : String(item);
            return (
              <KVLine key={i} depth={childDepth} k={`#${i + 1}`} v={display} />
            );
          }
          const obj = item as Record<string, unknown>;
          const summary = summarizeObject(obj);
          const itemLabel = summary
            ? `#${i + 1} \u2014 ${summary}`
            : `#${i + 1}`;
          return (
            <CollapsibleObject
              key={i}
              obj={obj}
              depth={childDepth}
              label={itemLabel}
            />
          );
        })}
      </Collapse>
    </>
  );
};

const CollapsibleObject: React.FC<{
  obj: Record<string, unknown>;
  depth: number;
  label?: string;
}> = ({ obj, depth, label }) => {
  const keys = Object.keys(obj);
  const [open, setOpen] = React.useState(true);

  if (isNgapIE(obj))
    return <NgapIEBlock ie={obj} depth={depth} label={label} />;

  const isBpsObj = isBpsObject(obj);
  const childDepth = label ? depth + 1 : depth;

  return (
    <>
      {label && (
        <TreeRow
          depth={depth}
          expandable
          open={open}
          onToggle={() => setOpen((s) => !s)}
        >
          <Box component="span" sx={{ color: "text.secondary" }}>
            {label}
          </Box>
        </TreeRow>
      )}
      <Collapse in={open || !label}>
        {keys.length === 0 ? (
          <TreeRow depth={childDepth}>
            <Box component="span">{"\u2014"}</Box>
          </TreeRow>
        ) : (
          keys
            .filter((k) => !(isBpsObj && k === "unit"))
            .map((k) => {
              const v = obj[k];
              if (isEnumLike(v))
                return (
                  <KVLine key={k} depth={childDepth} k={k} v={formatEnum(v)} />
                );
              if (
                v == null ||
                typeof v === "string" ||
                typeof v === "number" ||
                typeof v === "boolean"
              ) {
                const display =
                  v == null
                    ? "\u2014"
                    : isBpsObj && typeof v === "number"
                      ? formatBps(v)
                      : String(v);
                return <KVLine key={k} depth={childDepth} k={k} v={display} />;
              }
              // F11: NAS-PDU detection in nested structs
              if (isNasPdu(v)) {
                return (
                  <NasPduBlock
                    key={k}
                    nasPdu={v}
                    depth={childDepth}
                    title={k}
                  />
                );
              }
              // NRPPa-PDU detection in nested structs
              if (isNrppaPdu(v)) {
                return (
                  <NrppaPduBlock
                    key={k}
                    nrppaPdu={v}
                    depth={childDepth}
                    title={k}
                  />
                );
              }
              return (
                <ChildSection key={k} depth={childDepth} title={k} defaultOpen>
                  <GenericNode value={v} depth={childDepth + 1} />
                </ChildSection>
              );
            })
        )}
      </Collapse>
    </>
  );
};

type GenericNodeProps = {
  value: unknown;
  depth?: number;
  labelOverride?: string;
};

const GenericNode: React.FC<GenericNodeProps> = ({
  value,
  depth = 0,
  labelOverride,
}) => {
  if (value == null)
    return (
      <TreeRow depth={depth}>
        <Box component="span">{"\u2014"}</Box>
      </TreeRow>
    );
  if (isEnumLike(value))
    return (
      <TreeRow depth={depth}>
        <Box component="span">{formatEnum(value)}</Box>
      </TreeRow>
    );
  const t = typeof value;
  if (t === "string" || t === "number" || t === "boolean")
    return (
      <TreeRow depth={depth}>
        <Box component="span">{String(value)}</Box>
      </TreeRow>
    );
  if (Array.isArray(value))
    return (
      <CollapsibleArray items={value} depth={depth} label={labelOverride} />
    );
  if (t === "object")
    return (
      <CollapsibleObject
        obj={value as Record<string, unknown>}
        depth={depth}
        label={labelOverride}
      />
    );
  return (
    <TreeRow depth={depth}>
      <Box component="span">{String(value)}</Box>
    </TreeRow>
  );
};

// --- Top-level views ---

const TopLevelNgapView: React.FC<{ decoded: DecodedNGAPMessage }> = ({
  decoded,
}) => {
  const { summary, pdu_type, procedure_code, criticality, value } = decoded;

  return (
    <>
      {summary && (
        <Box
          sx={{
            color: "text.secondary",
            fontSize: 13,
            fontFamily: MONO_FONT,
            mb: 0.5,
            pb: 0.5,
            borderBottom: (t) => `1px solid ${t.palette.divider}`,
          }}
        >
          {summary}
        </Box>
      )}
      <KVLine depth={0} k="PDU Type" v={String(pdu_type ?? "\u2014")} />
      <KVLine
        depth={0}
        k="Procedure Code"
        v={
          isEnumLike(procedure_code as any)
            ? formatEnum(procedure_code as any)
            : String(procedure_code ?? "\u2014")
        }
      />
      <KVLine
        depth={0}
        k="Criticality"
        v={
          isEnumLike(criticality as any)
            ? formatEnum(criticality as any)
            : String(criticality ?? "\u2014")
        }
      />
      <TopLevelValueRow value={value} />
    </>
  );
};

const TopLevelValueRow: React.FC<{ value: unknown }> = ({ value }) => {
  const [open, setOpen] = React.useState(true);
  return (
    <>
      <TreeRow
        depth={0}
        expandable
        open={open}
        onToggle={() => setOpen((s) => !s)}
      >
        <Box component="span" sx={{ color: "text.secondary" }}>
          Value
        </Box>
      </TreeRow>
      <Collapse in={open}>
        <GenericNode value={value} depth={1} />
      </Collapse>
    </>
  );
};

export const NGAPMessageView: React.FC<{
  decoded: DecodedNGAPMessage;
  title?: string;
}> = ({ decoded, title }) => {
  return (
    <Box
      sx={{
        p: 1.25,
        border: (t) => `1px solid ${t.palette.divider}`,
        borderRadius: 1,
        fontFamily: MONO_FONT,
        fontSize: 13,
        lineHeight: 1.5,
      }}
    >
      {title && (
        <>
          <Box sx={{ fontWeight: 600, mb: 0.5 }}>{title}</Box>
          <Divider sx={{ mb: 1 }} />
        </>
      )}
      <TopLevelNgapView decoded={decoded} />
    </Box>
  );
};
