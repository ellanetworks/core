// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  IconButton,
  Link as MuiLink,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  Autorenew as GenerateIcon,
  ContentCopy as CopyIcon,
  Download as DownloadIcon,
  OpenInNew as OpenInNewIcon,
  Upload as ImportIcon,
} from "@mui/icons-material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import ListPageHeader from "@/components/ListPageHeader";
import {
  GenerateSEPPCertificateModal,
  ImportSEPPCertificateModal,
  RegenerateSEPPCAModal,
} from "@/components/SEPPCertificateModals";
import {
  formatEndpoint,
  getSEPPCertificate,
  getSEPPStatus,
  plmnDomain,
  type SEPPCARegenerated,
  type SEPPCertificate,
  type SEPPNodeStatus,
  type SEPPSource,
  type SEPPStatus,
  type SEPPTrustAnchor,
} from "@/queries/roaming";
import { getOperator, type OperatorData } from "@/queries/operator";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { useNow } from "@/hooks/useNow";
import { formatDateTime } from "@/utils/formatters";
import { TABLE_CONTAINER_SX } from "@/utils/layout";
import { PRODUCT } from "@/utils/product";
import { useRoamingContext } from "./types";

const DAY_MS = 24 * 60 * 60 * 1000;
export const EXPIRY_WARNING_DAYS = 30;

const downloadText = (filename: string, text: string) => {
  const url = window.URL.createObjectURL(
    new Blob([text], { type: "application/x-pem-file" }),
  );
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

const headCellSx = { fontWeight: 600 } as const;

const sourceLabels: Record<SEPPSource, string> = {
  generated: "Generated",
  imported: "Imported",
};

const anchorSourceLabels: Record<SEPPSource, string> = {
  generated: "Cluster SEPP CA",
  imported: "Imported CA",
};

const Muted = () => (
  <Typography variant="body2" color="textSecondary" component="span">
    —
  </Typography>
);

const ExpiryChip = ({ notAfter, now }: { notAfter: string; now: number }) => {
  const daysLeft = Math.floor((new Date(notAfter).getTime() - now) / DAY_MS);
  if (daysLeft < 0) {
    return <Chip size="small" color="error" label="Expired" />;
  }
  if (daysLeft <= EXPIRY_WARNING_DAYS) {
    return (
      <Chip
        size="small"
        color="warning"
        label={
          daysLeft === 1 ? "Expires in 1 day" : `Expires in ${daysLeft} days`
        }
      />
    );
  }
  return null;
};

const Expiry = ({ notAfter, now }: { notAfter: string; now: number }) => (
  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
    <span>{formatDateTime(notAfter)}</span>
    {now > 0 && <ExpiryChip notAfter={notAfter} now={now} />}
  </Box>
);

const isOrphan = (node: SEPPNodeStatus) => !node.nodeID && !node.reachable;

const nodeStatus = (
  node: SEPPNodeStatus,
): {
  label: string;
  color: "success" | "warning" | "error" | "default";
  tooltip: string;
} => {
  if (isOrphan(node)) {
    return {
      label: "No node",
      color: "warning",
      tooltip: "No node has this SEPP ID.",
    };
  }
  if (!node.reachable) {
    return { label: "Unknown", color: "default", tooltip: node.error ?? "" };
  }
  if (!node.n32Configured) {
    return {
      label: "Not configured",
      color: "default",
      tooltip: "interfaces.n32 is not set.",
    };
  }
  if (!node.listening) {
    return {
      label: "Not listening",
      color: "error",
      tooltip: node.error ?? "",
    };
  }
  if (node.error) {
    return { label: "Listening", color: "warning", tooltip: node.error };
  }
  return { label: "Listening", color: "success", tooltip: "" };
};

const NodeStatusChip = ({ node }: { node: SEPPNodeStatus }) => {
  const { label, color, tooltip } = nodeStatus(node);
  return (
    <Tooltip title={tooltip} arrow>
      <Chip label={label} color={color} size="small" variant="outlined" />
    </Tooltip>
  );
};

const nodeEndpoint = (node: SEPPNodeStatus) => {
  if (!node.address) return "—";
  return node.port ? formatEndpoint(node.address, node.port) : node.address;
};

const nodeKey = (node: SEPPNodeStatus) =>
  node.seppID ? `sepp-${node.seppID}` : `node-${node.nodeID ?? "local"}`;

const anchorFilename = (anchor: SEPPTrustAnchor) =>
  `sepp-trust-anchor-${anchor.fingerprint.replace(/^sha256:/, "").slice(0, 12)}.pem`;

type SEPPDialog =
  | { kind: "import"; seppID: number }
  | { kind: "generate"; seppID: number; imported: boolean }
  | { kind: "regenerate-ca" };

export default function SEPPTab() {
  const { accessToken, canEdit, showSnackbar } = useRoamingContext();
  const queryClient = useQueryClient();
  const copy = useCopyToClipboard();
  const now = useNow(60_000);

  const [dialog, setDialog] = useState<SEPPDialog | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);

  const statusQuery = useQuery<SEPPStatus>({
    queryKey: ["sepp-status"],
    queryFn: () => getSEPPStatus(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
    refetchInterval: 10_000,
  });

  const operatorQuery = useQuery<OperatorData>({
    queryKey: ["operator"],
    queryFn: () => getOperator(accessToken || ""),
    enabled: !!accessToken,
  });

  const mcc = operatorQuery.data?.id.mcc;
  const mnc = operatorQuery.data?.id.mnc;
  const domain = mcc && mnc ? plmnDomain(mcc, mnc) : "";

  const handleReplaced = (cert: SEPPCertificate, action: string) => {
    setDialog(null);
    void queryClient.invalidateQueries({ queryKey: ["sepp-status"] });
    setWarnings(cert.warnings ?? []);
    showSnackbar(
      `SEPP ${cert.seppID} certificate ${action} successfully.`,
      "success",
    );
  };

  const handleCARegenerated = (result: SEPPCARegenerated) => {
    setDialog(null);
    void queryClient.invalidateQueries({ queryKey: ["sepp-status"] });
    setWarnings(result.warnings ?? []);
    showSnackbar("SEPP CA regenerated successfully.", "success");
  };

  const downloadChain = async (seppID: number) => {
    try {
      const cert = await getSEPPCertificate(accessToken || "", seppID);
      if (!cert) {
        showSnackbar(`SEPP ${seppID} has no certificate.`, "error");
        return;
      }
      downloadText(`${cert.fqdn}-chain.pem`, cert.certificateChainPEM);
    } catch (error) {
      const msg =
        error instanceof Error ? error.message : "Unknown error occurred.";
      showSnackbar(`Failed to download the certificate chain: ${msg}`, "error");
    }
  };

  const nodeAlerts = (status: SEPPStatus) =>
    status.nodes
      .filter(
        (node) => domain && node.fqdn && !node.fqdn.endsWith(`.${domain}`),
      )
      .map((node) => (
        <Alert key={nodeKey(node)} severity="error" sx={{ mb: 2 }}>
          {`The FQDN of SEPP ${node.seppID ?? ""} is not under ${domain}. Generate or import a new certificate.`}
        </Alert>
      ));

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <ListPageHeader
        title="SEPP"
        description="Manage the certificates your SEPPs use with 5G roaming partners."
        variant="section"
        action={
          canEdit &&
          statusQuery.data !== undefined && (
            <Button
              variant="outlined"
              color="error"
              onClick={() => setDialog({ kind: "regenerate-ca" })}
            >
              Regenerate CA
            </Button>
          )
        }
      />

      {warnings.map((warning) => (
        <Alert
          key={warning}
          severity="warning"
          sx={{ mb: 2 }}
          onClose={() =>
            setWarnings((prev) => prev.filter((w) => w !== warning))
          }
        >
          {warning}
        </Alert>
      ))}

      <QueryState
        query={statusQuery}
        resource="SEPP status"
        isEmpty={(data) =>
          data.nodes.length === 0 && data.trustAnchors.length === 0
        }
        empty={
          <EmptyState
            primaryText="No SEPP yet"
            secondaryText={
              <>
                Set interfaces.n32 in a node&apos;s configuration file to enable
                its SEPP.{" "}
                <MuiLink
                  href={PRODUCT.roamingDocsUrl}
                  target="_blank"
                  rel="noreferrer"
                  underline="hover"
                  sx={{ display: "inline-flex", alignItems: "center" }}
                >
                  Learn more
                  <OpenInNewIcon sx={{ fontSize: 16, ml: 0.5 }} />
                </MuiLink>
              </>
            }
          />
        }
      >
        {(status) => (
          <>
            {nodeAlerts(status)}

            <Typography variant="h6" sx={{ mb: 1 }}>
              Trust Anchors
            </Typography>
            <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
              Give these CA certificates to your roaming partners.
            </Typography>
            {status.trustAnchors.length === 0 ? (
              <Typography variant="body2" color="textSecondary">
                No trust anchor yet.
              </Typography>
            ) : (
              <TableContainer sx={TABLE_CONTAINER_SX}>
                <Table aria-label="Trust anchors">
                  <TableHead>
                    <TableRow>
                      <TableCell sx={headCellSx}>Source</TableCell>
                      <TableCell sx={headCellSx}>Subject</TableCell>
                      <TableCell sx={headCellSx}>Fingerprint</TableCell>
                      <TableCell sx={headCellSx}>Expires</TableCell>
                      <TableCell sx={headCellSx}>Used By</TableCell>
                      <TableCell sx={headCellSx} align="right">
                        Actions
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {status.trustAnchors.map((anchor) => (
                      <TableRow key={anchor.fingerprint}>
                        <TableCell>
                          {anchorSourceLabels[anchor.source]}
                        </TableCell>
                        <TableCell sx={{ wordBreak: "break-all" }}>
                          {anchor.subject}
                        </TableCell>
                        <TableCell>
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              gap: 0.5,
                            }}
                          >
                            <Typography
                              variant="body2"
                              sx={{
                                fontFamily: "monospace",
                                wordBreak: "break-all",
                              }}
                            >
                              {anchor.fingerprint}
                            </Typography>
                            <Tooltip title="Copy fingerprint" arrow>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  copy(anchor.fingerprint, "Fingerprint")
                                }
                                aria-label={`Copy fingerprint of ${anchor.subject}`}
                              >
                                <CopyIcon fontSize="small" color="primary" />
                              </IconButton>
                            </Tooltip>
                          </Box>
                        </TableCell>
                        <TableCell>
                          <Expiry notAfter={anchor.notAfter} now={now} />
                        </TableCell>
                        <TableCell>
                          {anchor.seppIDs.length > 0 ? (
                            anchor.seppIDs.map((id) => `SEPP ${id}`).join(", ")
                          ) : (
                            <Muted />
                          )}
                        </TableCell>
                        <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                          <Tooltip title="Download trust anchor" arrow>
                            <IconButton
                              size="small"
                              onClick={() =>
                                downloadText(anchorFilename(anchor), anchor.pem)
                              }
                              aria-label={`Download trust anchor ${anchor.subject}`}
                            >
                              <DownloadIcon fontSize="small" color="primary" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="Copy trust anchor" arrow>
                            <IconButton
                              size="small"
                              onClick={() => copy(anchor.pem, "Trust anchor")}
                              aria-label={`Copy trust anchor ${anchor.subject}`}
                            >
                              <CopyIcon fontSize="small" color="primary" />
                            </IconButton>
                          </Tooltip>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}

            <Typography variant="h6" sx={{ mt: 4, mb: 1 }}>
              Nodes
            </Typography>
            <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
              Partners must be able to reach every node.
            </Typography>
            {status.nodes.length === 0 ? (
              <Typography variant="body2" color="textSecondary">
                No nodes yet.
              </Typography>
            ) : (
              <TableContainer sx={TABLE_CONTAINER_SX}>
                <Table aria-label="SEPP nodes">
                  <TableHead>
                    <TableRow>
                      <TableCell sx={headCellSx}>SEPP ID</TableCell>
                      <TableCell sx={headCellSx}>Node</TableCell>
                      <TableCell sx={headCellSx}>FQDN</TableCell>
                      <TableCell sx={headCellSx}>Source</TableCell>
                      <TableCell sx={headCellSx}>N32</TableCell>
                      <TableCell sx={headCellSx}>Address</TableCell>
                      <TableCell sx={headCellSx}>Certificate Expires</TableCell>
                      <TableCell sx={headCellSx} align="right">
                        Actions
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {status.nodes.map((node) => {
                      const seppID = node.seppID;
                      const orphan = isOrphan(node);
                      return (
                        <TableRow key={nodeKey(node)}>
                          <TableCell>{seppID ?? <Muted />}</TableCell>
                          <TableCell sx={{ wordBreak: "break-all" }}>
                            {node.nodeID || (orphan ? <Muted /> : "This node")}
                          </TableCell>
                          <TableCell sx={{ wordBreak: "break-all" }}>
                            {node.fqdn || <Muted />}
                          </TableCell>
                          <TableCell>
                            {node.source ? (
                              sourceLabels[node.source]
                            ) : (
                              <Muted />
                            )}
                          </TableCell>
                          <TableCell>
                            <NodeStatusChip node={node} />
                          </TableCell>
                          <TableCell>{nodeEndpoint(node)}</TableCell>
                          <TableCell>
                            {node.certificate ? (
                              <Expiry
                                notAfter={node.certificate.notAfter}
                                now={now}
                              />
                            ) : (
                              <Muted />
                            )}
                          </TableCell>
                          <TableCell
                            align="right"
                            sx={{ whiteSpace: "nowrap" }}
                          >
                            {seppID !== undefined && node.certificate && (
                              <Tooltip title="Download certificate chain" arrow>
                                <IconButton
                                  size="small"
                                  onClick={() => void downloadChain(seppID)}
                                  aria-label={`Download certificate chain of SEPP ${seppID}`}
                                >
                                  <DownloadIcon
                                    fontSize="small"
                                    color="primary"
                                  />
                                </IconButton>
                              </Tooltip>
                            )}
                            {canEdit && seppID !== undefined && !orphan && (
                              <>
                                <Tooltip title="Import certificate" arrow>
                                  <IconButton
                                    size="small"
                                    onClick={() =>
                                      setDialog({ kind: "import", seppID })
                                    }
                                    aria-label={`Import certificate for SEPP ${seppID}`}
                                  >
                                    <ImportIcon
                                      fontSize="small"
                                      color="primary"
                                    />
                                  </IconButton>
                                </Tooltip>
                                <Tooltip title="Generate certificate" arrow>
                                  <IconButton
                                    size="small"
                                    onClick={() =>
                                      setDialog({
                                        kind: "generate",
                                        seppID,
                                        imported: node.source === "imported",
                                      })
                                    }
                                    aria-label={`Generate certificate for SEPP ${seppID}`}
                                  >
                                    <GenerateIcon
                                      fontSize="small"
                                      color="primary"
                                    />
                                  </IconButton>
                                </Tooltip>
                              </>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </>
        )}
      </QueryState>

      {dialog?.kind === "import" && (
        <ImportSEPPCertificateModal
          open
          onClose={() => setDialog(null)}
          onSuccess={(cert) => handleReplaced(cert, "imported")}
          seppID={dialog.seppID}
          mcc={mcc}
          mnc={mnc}
        />
      )}
      {dialog?.kind === "generate" && (
        <GenerateSEPPCertificateModal
          open
          onClose={() => setDialog(null)}
          onSuccess={(cert) => handleReplaced(cert, "generated")}
          seppID={dialog.seppID}
          imported={dialog.imported}
          mcc={mcc}
          mnc={mnc}
        />
      )}
      {dialog?.kind === "regenerate-ca" && (
        <RegenerateSEPPCAModal
          open
          onClose={() => setDialog(null)}
          onSuccess={handleCARegenerated}
        />
      )}
    </Box>
  );
}
