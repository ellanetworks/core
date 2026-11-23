// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Box,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { ContentCopy as CopyIcon } from "@mui/icons-material";
import type { DiameterStatus } from "@/queries/integrations";
import DomainName from "@/components/DomainName";
import NotListening from "@/components/NotListening";
import OneLine from "@/components/OneLine";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { formatEndpoint } from "@/utils/ip";
import {
  ACTION_CELL_SX,
  SETTING_CELL_SX,
  TABLE_CONTAINER_SX,
  VALUE_CELL_SX,
} from "@/utils/layout";

export {
  DIAMETER_STATUS_KEY,
  useDiameterStatus,
} from "@/hooks/useDiameterStatus";

export { formatEndpoint };

export const shortNodeId = (nodeId: string) =>
  nodeId.split("-").pop() ?? nodeId;

export const CopyButton: React.FC<{ value: string; what: string }> = ({
  value,
  what,
}) => {
  const copy = useCopyToClipboard();

  return (
    <Tooltip title={`Copy ${what}`} arrow>
      <IconButton
        size="small"
        onClick={() => copy(value, what)}
        aria-label={`Copy ${what}`}
      >
        <CopyIcon fontSize="small" color="primary" />
      </IconButton>
    </Tooltip>
  );
};

export const IdentityRow: React.FC<{
  label: string;
  help: string;
  loading?: boolean;
  value?: string;
  copyAs?: string;
  children?: React.ReactNode;
}> = ({ label, help, loading, value, copyAs, children }) => (
  <TableRow>
    <TableCell sx={SETTING_CELL_SX}>
      <Tooltip title={help} arrow>
        <span>{label}</span>
      </Tooltip>
    </TableCell>
    <TableCell sx={{ ...VALUE_CELL_SX, wordBreak: "break-word" }}>
      {loading ? (
        <Skeleton variant="text" width={240} />
      ) : (
        (children ?? (value ? <DomainName name={value} /> : "N/A"))
      )}
    </TableCell>
    <TableCell sx={ACTION_CELL_SX}>
      {!loading && value && <CopyButton value={value} what={copyAs ?? label} />}
    </TableCell>
  </TableRow>
);

export const otherServedRealms = (diameter?: DiameterStatus) =>
  (diameter?.servedRealms ?? []).filter(
    (realm) => realm.toLowerCase() !== diameter?.realm?.toLowerCase(),
  );

export const RealmRow: React.FC<{
  diameter?: DiameterStatus;
  loading: boolean;
  label?: string;
  help: string;
  showServedRealms?: boolean;
}> = ({
  diameter,
  loading,
  label = "Diameter Realm",
  help,
  showServedRealms = false,
}) => {
  const others = showServedRealms ? otherServedRealms(diameter) : [];

  return (
    <IdentityRow
      label={label}
      help={help}
      loading={loading}
      value={diameter?.realm}
    >
      {diameter?.realm && others.length > 0 ? (
        <>
          <DomainName name={diameter.realm} />
          <Typography variant="body2" color="textSecondary">
            Also accepts: {others.join(", ")}
          </Typography>
        </>
      ) : undefined}
    </IdentityRow>
  );
};

const LINE_SX = {
  minHeight: 30,
  display: "flex",
  alignItems: "center",
  justifyContent: "flex-end",
  minWidth: 0,
} as const;

export interface Line {
  key: string;
  text: string;
  note?: string;
  copy?: string;
}

export const LinesRow: React.FC<{
  label: string;
  help: string;
  lines: Line[];
  copyAs: string;
}> = ({ label, help, lines, copyAs }) => (
  <TableRow>
    <TableCell sx={SETTING_CELL_SX}>
      <Tooltip title={help} arrow>
        <span>{label}</span>
      </Tooltip>
    </TableCell>
    <TableCell sx={VALUE_CELL_SX}>
      <Box sx={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr)" }}>
        {lines.map((line) => (
          <Box
            key={line.key}
            sx={{ ...LINE_SX, justifyContent: "flex-start", gap: 1 }}
          >
            <OneLine text={line.text} />
            {line.note && (
              <Typography
                variant="body2"
                color="textSecondary"
                noWrap
                sx={{ flexShrink: 0 }}
              >
                {line.note}
              </Typography>
            )}
          </Box>
        ))}
      </Box>
    </TableCell>
    <TableCell sx={ACTION_CELL_SX}>
      {lines.map((line) => (
        <Box key={line.key} sx={LINE_SX}>
          {line.copy && <CopyButton value={line.copy} what={copyAs} />}
        </Box>
      ))}
    </TableCell>
  </TableRow>
);

const DIAMETER_ADDRESS_HELP =
  "Where your IMS connects to Ella Core, over SCTP.";

const ORIGIN_HOST_HELP =
  "Ella Core's Diameter identity. It uses this one identity on every Diameter interface, as HSS, PCRF, MME and SMSF.";

export const ThisCoreSection: React.FC<{
  diameter?: DiameterStatus;
  loading: boolean;
  description: string;
  realmHelp: string;
  showServedRealms?: boolean;
  showAddress?: boolean;
  before?: React.ReactNode;
  after?: React.ReactNode;
}> = ({
  diameter,
  loading,
  description,
  realmHelp,
  showServedRealms = false,
  showAddress = false,
  before,
  after,
}) => {
  const nodes = diameter?.nodes ?? [];
  const cluster = nodes.length > 1;
  const listener = diameter?.listener;
  const addresses = listener?.enabled
    ? listener.addresses.map((a) => formatEndpoint(a, listener.port))
    : [];

  return (
    <>
      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        This Core
      </Typography>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        {description}
      </Typography>
      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 3 }}>
        <Table aria-label="This core">
          <TableBody>
            {before}
            <RealmRow
              diameter={diameter}
              loading={loading}
              help={realmHelp}
              showServedRealms={showServedRealms}
            />
            {loading || nodes.length === 0 ? (
              <IdentityRow
                label="Origin-Host"
                help={ORIGIN_HOST_HELP}
                loading={loading}
              >
                <Typography variant="body2" color="textSecondary">
                  Assigned when this node joins the cluster.
                </Typography>
              </IdentityRow>
            ) : (
              <LinesRow
                label="Origin-Host"
                help={
                  cluster
                    ? `${ORIGIN_HOST_HELP} Each cluster node has its own: add every node as a peer.`
                    : ORIGIN_HOST_HELP
                }
                lines={nodes.map((node) => ({
                  key: node.nodeId,
                  text: node.originHost,
                  note: cluster
                    ? node.local
                      ? "this node"
                      : node.displayName || shortNodeId(node.nodeId)
                    : undefined,
                  copy: node.originHost,
                }))}
                copyAs="Origin-Host"
              />
            )}
            {showAddress &&
              (addresses.length === 0 ? (
                <IdentityRow
                  label="Diameter Address"
                  help={DIAMETER_ADDRESS_HELP}
                  loading={loading}
                >
                  <NotListening setting="interfaces.diameter" />
                </IdentityRow>
              ) : (
                <LinesRow
                  label="Diameter Address"
                  help={DIAMETER_ADDRESS_HELP}
                  lines={addresses.map((address) => ({
                    key: address,
                    text: `${address} (SCTP)`,
                    copy: address,
                  }))}
                  copyAs="Diameter address"
                />
              ))}
            {after}
          </TableBody>
        </Table>
      </TableContainer>
    </>
  );
};
