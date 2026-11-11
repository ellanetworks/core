// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Button,
  IconButton,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  ContentCopy as CopyIcon,
  PersonAdd as AddPeerIcon,
} from "@mui/icons-material";
import type { DiameterPeer, DiameterStatus } from "@/queries/integrations";
import DiameterPeerStatusChip from "@/components/DiameterPeerStatusChip";
import DomainName from "@/components/DomainName";
import NotListening from "@/components/NotListening";
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
            {(loading || nodes.length === 0) && (
              <IdentityRow
                label="Origin-Host"
                help={ORIGIN_HOST_HELP}
                loading={loading}
              >
                <Typography variant="body2" color="textSecondary">
                  Assigned when this node joins the cluster.
                </Typography>
              </IdentityRow>
            )}
            {nodes.map((node) => (
              <IdentityRow
                key={node.nodeId}
                label={
                  cluster
                    ? `Origin-Host (${node.local ? "this node" : node.displayName || shortNodeId(node.nodeId)})`
                    : "Origin-Host"
                }
                help={
                  cluster
                    ? `${ORIGIN_HOST_HELP} Each cluster node has its own: add every node as a peer.`
                    : ORIGIN_HOST_HELP
                }
                value={node.originHost}
                copyAs="Origin-Host"
              />
            ))}
            {showAddress &&
              (addresses.length === 0 ? (
                <IdentityRow
                  label="Diameter Address"
                  help="Where your IMS connects to Ella Core, over SCTP."
                  loading={loading}
                >
                  <NotListening setting="interfaces.diameter" />
                </IdentityRow>
              ) : (
                addresses.map((address) => (
                  <IdentityRow
                    key={address}
                    label="Diameter Address"
                    help="Where your IMS connects to Ella Core, over SCTP."
                    value={address}
                    copyAs="Diameter address"
                  >
                    {address} (SCTP)
                  </IdentityRow>
                ))
              ))}
            {after}
          </TableBody>
        </Table>
      </TableContainer>
    </>
  );
};

export const UnknownPeersTable: React.FC<{
  peers: DiameterPeer[];
  canEdit: boolean;
  onAdd: (peer: DiameterPeer) => void;
}> = ({ peers, canEdit, onAdd }) => {
  const theme = useTheme();

  return (
    <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 3 }}>
      <Table aria-label="Unknown peers">
        <TableBody
          sx={{
            "& .MuiTableRow-root:first-of-type .MuiTableCell-root": {
              fontWeight: 600,
              backgroundColor: theme.palette.backgroundSubtle,
            },
          }}
        >
          <TableRow>
            <TableCell>Origin-Host</TableCell>
            <TableCell>Realm</TableCell>
            <TableCell>Address</TableCell>
            <TableCell>Status</TableCell>
            <TableCell align="right">Actions</TableCell>
          </TableRow>
          {peers.map((peer) => (
            <TableRow key={`${peer.id}-${peer.address}`}>
              <TableCell>
                {peer.originHost ? <DomainName name={peer.originHost} /> : "—"}
              </TableCell>
              <TableCell>
                {peer.realm ? <DomainName name={peer.realm} /> : "—"}
              </TableCell>
              <TableCell sx={{ whiteSpace: "nowrap" }}>
                {peer.address}
              </TableCell>
              <TableCell>
                <DiameterPeerStatusChip
                  status={{
                    state: peer.state,
                    originHost: peer.originHost,
                    realm: peer.realm,
                    since: peer.since,
                    error: peer.error,
                  }}
                />
              </TableCell>
              <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                {canEdit && (
                  <Stack direction="row" sx={{ justifyContent: "flex-end" }}>
                    <Button
                      size="small"
                      startIcon={<AddPeerIcon />}
                      onClick={() => onAdd(peer)}
                      aria-label={`Add ${peer.originHost || peer.address} as allowed peer`}
                    >
                      Add as allowed peer
                    </Button>
                  </Stack>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
};
