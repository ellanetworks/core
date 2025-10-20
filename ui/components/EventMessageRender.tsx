import * as React from "react";
import {
  Box,
  Typography,
  IconButton,
  Tooltip,
  Chip,
  Divider,
  Collapse,
  Stack,
} from "@mui/material";
import {
  ExpandMore as ExpandMoreIcon,
  ChevronRight as ChevronRightIcon,
} from "@mui/icons-material";

const INDENT = 0.375;
const ROW_Y = 0.25;

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

/** Renders a chevron, or a hidden one to reserve the exact same width */
const ChevronSlot: React.FC<{
  present?: boolean;
  open?: boolean;
  onClick?: () => void;
}> = ({ present = false, open = false, onClick }) => (
  <Box sx={{ display: "inline-flex", alignItems: "center" }}>
    {present ? (
      <Tooltip title={open ? "Collapse" : "Expand"}>
        <IconButton size="small" onClick={onClick} sx={{ p: 0.25 }}>
          {open ? (
            <ExpandMoreIcon fontSize="small" />
          ) : (
            <ChevronRightIcon fontSize="small" />
          )}
        </IconButton>
      </Tooltip>
    ) : (
      // Hidden chevron to reserve width, perfectly matching the real one
      <IconButton
        size="small"
        sx={{ p: 0.25, visibility: "hidden" }}
        aria-hidden
        tabIndex={-1}
      >
        <ChevronRightIcon fontSize="small" />
      </IconButton>
    )}
  </Box>
);

const KVLine: React.FC<{ k: string; v: React.ReactNode }> = ({ k, v }) => (
  <Box
    sx={{
      display: "grid",
      gridTemplateColumns: "auto 1fr",
      columnGap: 1,
      alignItems: "baseline",
      py: ROW_Y,
    }}
  >
    <Box
      sx={{
        display: "inline-flex",
        alignItems: "baseline",
        gap: 0.5,
        minWidth: 0,
      }}
    >
      <ChevronSlot />
      <Typography
        variant="body2"
        sx={{ color: "text.secondary", whiteSpace: "pre" }}
      >
        {k + ":"}
      </Typography>
    </Box>
    <Typography variant="body2" sx={{ wordBreak: "break-word", minWidth: 0 }}>
      {v}
    </Typography>
  </Box>
);

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

const RowHeader: React.FC<{
  title: string;
  open: boolean;
  onToggle: () => void;
}> = ({ title, open, onToggle }) => (
  <Box
    sx={{
      display: "grid",
      gridTemplateColumns: "auto 1fr",
      columnGap: 1,
      alignItems: "center",
      py: ROW_Y,
    }}
  >
    <Box sx={{ display: "inline-flex", alignItems: "center" }}>
      <ChevronSlot present open={open} onClick={onToggle} />
    </Box>
    <Typography variant="body2" sx={{ color: "text.secondary", minWidth: 0 }}>
      {title}
    </Typography>
  </Box>
);
const ChildSection: React.FC<{
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}> = ({ title, defaultOpen = true, children }) => {
  const [open, setOpen] = React.useState(defaultOpen);
  return (
    <Box sx={{ ml: INDENT }}>
      <RowHeader
        title={title}
        open={open}
        onToggle={() => setOpen((s) => !s)}
      />
      <Collapse in={open}>
        {/* No extra margins here: children rows use the same ROW_Y spacing */}
        <Box sx={{ ml: INDENT }}>{children}</Box>
      </Collapse>
    </Box>
  );
};

const IEValueRow: React.FC<{ value: unknown; depth: number }> = ({
  value,
  depth,
}) => {
  const [open, setOpen] = React.useState(true);

  return (
    <Box>
      {/* Label line (chevron + "Value") */}
      <Box sx={{ display: "inline-flex", alignItems: "center", py: ROW_Y }}>
        <ChevronSlot present open={open} onClick={() => setOpen((s) => !s)} />
        <Typography variant="body2" sx={{ color: "text.secondary", ml: 0.5 }}>
          Value
        </Typography>
      </Box>

      {/* Content BELOW the label, full width */}
      <Collapse in={open}>
        <Box sx={{ ml: INDENT }}>
          <GenericNode value={value} depth={depth + 1} />
        </Box>
      </Collapse>
    </Box>
  );
};

const NgapIEBlock: React.FC<{ ie: any; depth: number; label?: string }> = ({
  ie,
  depth,
  label,
}) => {
  const { idEnum, criticalityEnum, value, error } = extractIEFields(ie);
  const [open, setOpen] = React.useState(true);
  const title = isEnumLike(idEnum)
    ? `${idEnum.label} (${String(idEnum.value)})`
    : (label ?? "Information Element");

  return (
    <Box sx={{ ml: INDENT }}>
      <RowHeader
        title={title}
        open={open}
        onToggle={() => setOpen((s) => !s)}
      />
      <Collapse in={open}>
        {isEnumLike(criticalityEnum) && (
          <KVLine k="Criticality" v={formatEnum(criticalityEnum)} />
        )}
        {error && <KVLine k="Error" v={String(error)} />}
        {value == null ||
        typeof value === "string" ||
        typeof value === "number" ||
        typeof value === "boolean" ? (
          <KVLine k="Value" v={value == null ? "—" : String(value)} />
        ) : isEnumLike(value) ? (
          <KVLine k="Value" v={formatEnum(value)} />
        ) : (
          <IEValueRow value={value} depth={depth} />
        )}
      </Collapse>
    </Box>
  );
};

const CollapsibleArray: React.FC<{
  items: unknown[];
  depth: number;
  label?: string;
}> = ({ items, depth, label }) => {
  const [open, setOpen] = React.useState(true);
  return (
    <Box sx={{ ml: INDENT }}>
      {label && (
        <RowHeader
          title={label}
          open={open}
          onToggle={() => setOpen((s) => !s)}
        />
      )}
      <Collapse in={open || !label}>
        {items.map((item, i) => (
          <Box key={i} sx={{ ml: INDENT }}>
            {isNgapIE(item) ? (
              <NgapIEBlock ie={item} depth={depth} />
            ) : (
              <>
                <Typography
                  variant="body2"
                  sx={{ color: "text.secondary", py: ROW_Y }}
                >
                  #{i + 1}
                </Typography>
                <GenericNode value={item} depth={depth + 1} />
              </>
            )}
          </Box>
        ))}
      </Collapse>
    </Box>
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

  return (
    <Box sx={{ ml: INDENT }}>
      {label && (
        <RowHeader
          title={label}
          open={open}
          onToggle={() => setOpen((s) => !s)}
        />
      )}
      <Collapse in={open || !label}>
        {keys.length === 0 ? (
          <Typography variant="body2" sx={{ py: ROW_Y }}>
            —
          </Typography>
        ) : (
          keys.map((k) => {
            const v = obj[k];
            if (isEnumLike(v))
              return <KVLine key={k} k={k} v={formatEnum(v)} />;
            if (
              v == null ||
              typeof v === "string" ||
              typeof v === "number" ||
              typeof v === "boolean"
            ) {
              return <KVLine key={k} k={k} v={v == null ? "—" : String(v)} />;
            }
            return (
              <ChildSection key={k} title={k} defaultOpen>
                <GenericNode value={v} depth={depth + 1} />
              </ChildSection>
            );
          })
        )}
      </Collapse>
    </Box>
  );
};

type GenericNodeProps = {
  value: unknown;
  depth?: number;
  labelOverride?: string;
};
export const GenericNode: React.FC<GenericNodeProps> = ({
  value,
  depth = 0,
  labelOverride,
}) => {
  if (value == null) return <Typography variant="body2">—</Typography>;
  if (isEnumLike(value))
    return <Typography variant="body2">{formatEnum(value)}</Typography>;
  const t = typeof value;
  if (t === "string" || t === "number" || t === "boolean")
    return <Typography variant="body2">{String(value)}</Typography>;
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
  return <Typography variant="body2">{String(value)}</Typography>;
};

/** ---------- NEW: Top-level NGAP shell so all four keys look alike ---------- */
type NgapRoot = {
  pdu_type?: unknown;
  procedure_code?: unknown;
  criticality?: unknown;
  value?: unknown;
};

const isNgapRoot = (x: unknown): x is NgapRoot =>
  !!x &&
  typeof x === "object" &&
  "pdu_type" in (x as any) &&
  "procedure_code" in (x as any) &&
  "criticality" in (x as any) &&
  "value" in (x as any);

/** Renders PDU Type, Procedure Code, Criticality, and Value as uniform rows.
 *  - For Value: if complex, put a chevron in the KEY column and the content in VALUE column.
 */
const TopLevelNgapView: React.FC<{ decoded: NgapRoot }> = ({ decoded }) => {
  const { pdu_type, procedure_code, criticality, value } = decoded;

  const renderTopLevelValue = () => {
    // primitive/enum → same as others
    if (
      value == null ||
      typeof value === "string" ||
      typeof value === "number" ||
      typeof value === "boolean"
    ) {
      return <KVLine k="Value" v={value == null ? "—" : String(value)} />;
    }
    if (isEnumLike(value as any)) {
      return <KVLine k="Value" v={formatEnum(value as any)} />;
    }

    // object/array → chevron in key column, content in value column
    return <TopLevelValueRow value={value} />;
  };

  return (
    <Box>
      <KVLine k="PDU Type" v={String(pdu_type ?? "—")} />
      <KVLine
        k="Procedure Code"
        v={
          isEnumLike(procedure_code as any)
            ? formatEnum(procedure_code as any)
            : String(procedure_code ?? "—")
        }
      />
      <KVLine
        k="Criticality"
        v={
          isEnumLike(criticality as any)
            ? formatEnum(criticality as any)
            : String(criticality ?? "—")
        }
      />
      {renderTopLevelValue()}
    </Box>
  );
};

const TopLevelValueRow: React.FC<{ value: unknown }> = ({ value }) => {
  const [open, setOpen] = React.useState(true);

  return (
    <Box>
      {/* Label line (chevron + "Value") */}
      <Box sx={{ display: "inline-flex", alignItems: "center", py: ROW_Y }}>
        <ChevronSlot present open={open} onClick={() => setOpen((s) => !s)} />
        <Typography variant="body2" sx={{ color: "text.secondary", ml: 0.5 }}>
          Value
        </Typography>
      </Box>

      {/* Content BELOW the label, full width */}
      <Collapse in={open}>
        <Box sx={{ ml: INDENT }}>
          <GenericNode value={value} depth={1} />
        </Box>
      </Collapse>
    </Box>
  );
};

/** ---------- Container ---------- */
export const GenericMessageView: React.FC<{
  decoded: unknown;
  headerChips?: Array<{ label: string }>;
  title?: string;
}> = ({ decoded, headerChips, title }) => {
  return (
    <Box
      sx={{
        p: 1.25,
        border: (t) => `1px solid ${t.palette.divider}`,
        borderRadius: 1,
      }}
    >
      {title && (
        <>
          <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
            {title}
          </Typography>
          <Divider sx={{ mb: 1 }} />
        </>
      )}
      {headerChips && headerChips.length > 0 && (
        <Stack direction="row" spacing={1} sx={{ mb: 1 }}>
          {headerChips.map((c, i) => (
            <Chip key={i} size="small" variant="outlined" label={c.label} />
          ))}
        </Stack>
      )}

      {/* If it looks like an NGAP root, use the uniform top-level view;
          otherwise just render generically. */}
      {isNgapRoot(decoded) ? (
        <TopLevelNgapView decoded={decoded} />
      ) : (
        <GenericNode value={decoded} />
      )}
    </Box>
  );
};
