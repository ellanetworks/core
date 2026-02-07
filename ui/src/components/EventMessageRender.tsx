import * as React from "react";
import {
  Box,
  Typography,
  IconButton,
  Tooltip,
  Divider,
  Collapse,
} from "@mui/material";
import {
  ExpandMore as ExpandMoreIcon,
  ChevronRight as ChevronRightIcon,
} from "@mui/icons-material";

const INDENT = 0.375;
const ROW_H = 20;

const RowGroup: React.FC<{ indent?: number; children: React.ReactNode }> = ({
  indent = 0,
  children,
}) => (
  <Box sx={{ ml: indent, display: "flex", flexDirection: "column" }}>
    {children}
  </Box>
);

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
      alignItems: "center",
      minHeight: ROW_H,
    }}
  >
    <Box
      sx={{
        display: "inline-flex",
        alignItems: "center",
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
      minHeight: ROW_H,
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
    <RowGroup indent={INDENT}>
      <RowHeader
        title={title}
        open={open}
        onToggle={() => setOpen((s) => !s)}
      />
      <Collapse in={open}>
        <RowGroup indent={INDENT}>{children}</RowGroup>
      </Collapse>
    </RowGroup>
  );
};

const IEValueRow: React.FC<{ value: unknown; depth: number }> = ({
  value,
  depth,
}) => {
  const [open, setOpen] = React.useState(true);
  return (
    <RowGroup indent={INDENT}>
      <Box
        sx={{ display: "inline-flex", alignItems: "center", minHeight: ROW_H }}
      >
        <ChevronSlot present open={open} onClick={() => setOpen((s) => !s)} />
        <Typography variant="body2" sx={{ color: "text.secondary", ml: 0.5 }}>
          Value
        </Typography>
      </Box>
      <Collapse in={open}>
        <RowGroup indent={INDENT}>
          <GenericNode value={value} depth={depth + 1} />
        </RowGroup>
      </Collapse>
    </RowGroup>
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
    <RowGroup indent={INDENT}>
      <RowHeader
        title={title}
        open={open}
        onToggle={() => setOpen((s) => !s)}
      />
      <Collapse in={open}>
        <RowGroup indent={INDENT}>
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
        </RowGroup>
      </Collapse>
    </RowGroup>
  );
};

const CollapsibleArray: React.FC<{
  items: unknown[];
  depth: number;
  label?: string;
}> = ({ items, depth, label }) => {
  const [open, setOpen] = React.useState(true);
  return (
    <RowGroup indent={INDENT}>
      {label && (
        <RowHeader
          title={label}
          open={open}
          onToggle={() => setOpen((s) => !s)}
        />
      )}
      <Collapse in={open || !label}>
        <RowGroup indent={INDENT}>
          {items.map((item, i) => (
            <React.Fragment key={i}>
              {isNgapIE(item) ? (
                <NgapIEBlock ie={item} depth={depth} />
              ) : (
                <RowGroup>
                  <Typography
                    variant="body2"
                    sx={{
                      color: "text.secondary",
                      minHeight: ROW_H,
                      display: "flex",
                      alignItems: "center",
                    }}
                  >
                    #{i + 1}
                  </Typography>
                  <GenericNode value={item} depth={depth + 1} />
                </RowGroup>
              )}
            </React.Fragment>
          ))}
        </RowGroup>
      </Collapse>
    </RowGroup>
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
    <RowGroup indent={INDENT}>
      {label && (
        <RowHeader
          title={label}
          open={open}
          onToggle={() => setOpen((s) => !s)}
        />
      )}
      <Collapse in={open || !label}>
        <RowGroup indent={INDENT}>
          {keys.length === 0 ? (
            <Typography
              variant="body2"
              sx={{ minHeight: ROW_H, display: "flex", alignItems: "center" }}
            >
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
        </RowGroup>
      </Collapse>
    </RowGroup>
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

type NgapRoot = {
  pdu_type?: unknown;
  message_type?: unknown;
  procedure_code?: unknown;
  criticality?: unknown;
  value?: unknown;
};

const isNgapRoot = (x: unknown): x is NgapRoot =>
  !!x &&
  typeof x === "object" &&
  "pdu_type" in (x as any) &&
  "message_type" in (x as any) &&
  "procedure_code" in (x as any) &&
  "criticality" in (x as any) &&
  "value" in (x as any);

const TopLevelNgapView: React.FC<{ decoded: NgapRoot }> = ({ decoded }) => {
  const { pdu_type, message_type, procedure_code, criticality, value } =
    decoded;

  return (
    <RowGroup>
      <KVLine k="PDU Type" v={String(pdu_type ?? "—")} />
      <KVLine k="Message Type" v={String(message_type ?? "—")} />
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
      <TopLevelValueRow value={value} />
    </RowGroup>
  );
};

const TopLevelValueRow: React.FC<{ value: unknown }> = ({ value }) => {
  const [open, setOpen] = React.useState(true);
  return (
    <RowGroup>
      <Box
        sx={{ display: "inline-flex", alignItems: "center", minHeight: ROW_H }}
      >
        <ChevronSlot present open={open} onClick={() => setOpen((s) => !s)} />
        <Typography variant="body2" sx={{ color: "text.secondary", ml: 0.5 }}>
          Value
        </Typography>
      </Box>
      <Collapse in={open}>
        <RowGroup indent={INDENT}>
          <GenericNode value={value} depth={1} />
        </RowGroup>
      </Collapse>
    </RowGroup>
  );
};

export const GenericMessageView: React.FC<{
  decoded: unknown;
  title?: string;
}> = ({ decoded, title }) => {
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
      {isNgapRoot(decoded) ? (
        <TopLevelNgapView decoded={decoded} />
      ) : (
        <GenericNode value={decoded} />
      )}
    </Box>
  );
};
