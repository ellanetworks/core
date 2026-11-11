// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Box,
  Button,
  Chip,
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
  Add as AddIcon,
  Autorenew as RegenerateIcon,
  Delete as DeleteIcon,
  Download as DownloadIcon,
  Edit as EditIcon,
  OpenInNew as OpenInNewIcon,
  Sync as SyncIcon,
  Upload as ImportIcon,
} from "@mui/icons-material";
import { Link } from "react-router-dom";
import {
  deleteRoamingDiameterPeer,
  deleteRoamingPartner,
  formatEndpoint,
  deleteSEPPCertificate,
  formatPLMN,
  getSEPPStatus,
  listRoamingDiameterPeers,
  listRoamingPartners,
  plmnDomain,
  regenerateSEPPCA,
  rehandshakeRoamingPartner,
  type RoamingDiameterPeer,
  type RoamingPartner,
  type SEPPCertificate,
  type SEPPSource,
  type SEPPStatus,
  type SEPPTrustAnchor,
} from "@/queries/roaming";
import RoamingPartnerModal from "@/components/RoamingPartnerModal";
import RoamingDiameterPeerModal from "@/components/RoamingDiameterPeerModal";
import ImportSEPPCertificateModal from "@/components/ImportSEPPCertificateModal";
import N32StatusChip from "@/components/N32StatusChip";
import DiameterPeerStatusChip from "@/components/DiameterPeerStatusChip";
import DomainName from "@/components/DomainName";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import ConfirmDialog from "@/components/form/ConfirmDialog";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useNow } from "@/hooks/useNow";
import { formatDateTime } from "@/utils/formatters";
import {
  ACTION_CELL_SX,
  SETTING_CELL_SX,
  TABLE_CONTAINER_SX,
  VALUE_CELL_SX,
} from "@/utils/layout";
import { PRODUCT } from "@/utils/product";
import { getOperator, type OperatorData } from "@/queries/operator";
import { useIntegrationsContext } from "./types";
import { IdentityRow, ThisCoreSection, useDiameterStatus } from "./diameter";
import NotListening from "@/components/NotListening";

const DAY_MS = 24 * 60 * 60 * 1000;
const EXPIRY_WARNING_DAYS = 30;

const N32_ADDRESS_HELP =
  "Where your 5G roaming partners' SEPPs connect to Ella Core over N32.";

const TRUST_ANCHOR_CHANGED_WARNING =
  "The trust anchor of this node changed: every roaming partner must import the new trust anchor before N32 with this node works again";

const errorMessage = (error: unknown) =>
  error instanceof Error ? error.message : "Unknown error occurred.";

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

const anchorFilename = (anchor: SEPPTrustAnchor) =>
  `sepp-trust-anchor-${anchor.fingerprint.replace(/^sha256:/, "").slice(0, 12)}.pem`;

const sourceLabels: Record<SEPPSource, string> = {
  generated: "Generated",
  imported: "Imported",
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

const InlineValue: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
    {children}
  </Box>
);

const AnchorTooltip = ({ anchor }: { anchor: SEPPTrustAnchor }) => (
  <>
    <div>Download trust anchor</div>
    <div>{anchor.subject}</div>
    <Box sx={{ fontFamily: "monospace" }}>{anchor.fingerprint}</Box>
  </>
);

const AddButton: React.FC<{ label: string; onClick: () => void }> = ({
  label,
  onClick,
}) => (
  <Button
    variant="contained"
    color="success"
    size="small"
    startIcon={<AddIcon />}
    onClick={onClick}
    aria-label={label}
    sx={{ ml: 2, flexShrink: 0 }}
  >
    {label}
  </Button>
);

const EmptyRow: React.FC<{
  colSpan: number;
  loading: boolean;
  text: string;
}> = ({ colSpan, loading, text }) => (
  <TableRow>
    <TableCell colSpan={colSpan}>
      {loading ? (
        <Skeleton variant="text" width={240} />
      ) : (
        <Typography variant="body2" color="textSecondary">
          {text}
        </Typography>
      )}
    </TableCell>
  </TableRow>
);

export default function RoamingTab() {
  const { canEdit } = useIntegrationsContext();
  const theme = useTheme();
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();
  const now = useNow(60_000);

  const [partnerModal, setPartnerModal] = useState<{
    partner?: RoamingPartner;
  } | null>(null);
  const [partnerToDelete, setPartnerToDelete] = useState<RoamingPartner | null>(
    null,
  );
  const [peerModal, setPeerModal] = useState<{
    peer?: RoamingDiameterPeer;
  } | null>(null);
  const [peerToDelete, setPeerToDelete] = useState<RoamingDiameterPeer | null>(
    null,
  );
  const [importSEPPID, setImportSEPPID] = useState<number | null>(null);
  const [regenerate, setRegenerate] = useState<
    { kind: "certificate"; seppID: number } | { kind: "ca" } | null
  >(null);
  const [warnings, setWarnings] = useState<string[]>([]);

  const anyModalOpen =
    partnerModal !== null ||
    partnerToDelete !== null ||
    peerModal !== null ||
    peerToDelete !== null ||
    importSEPPID !== null ||
    regenerate !== null;

  const queriesEnabled = authReady && !!accessToken && !anyModalOpen;
  const diameterQuery = useDiameterStatus(!anyModalOpen);

  const operatorQuery = useQuery<OperatorData>({
    queryKey: ["operator"],
    enabled: queriesEnabled,
    queryFn: () => getOperator(accessToken!),
    placeholderData: (prev) => prev,
  });
  const plmn = operatorQuery.data?.id;

  const partnersQuery = useQuery<RoamingPartner[]>({
    queryKey: ["roaming-partners"],
    enabled: queriesEnabled,
    refetchInterval: 5000,
    queryFn: () => listRoamingPartners(accessToken!),
    placeholderData: (prev) => prev,
  });

  const peersQuery = useQuery<RoamingDiameterPeer[]>({
    queryKey: ["roaming-diameter-peers"],
    enabled: queriesEnabled,
    refetchInterval: 5000,
    queryFn: () => listRoamingDiameterPeers(accessToken!),
    placeholderData: (prev) => prev,
  });

  const seppQuery = useQuery<SEPPStatus>({
    queryKey: ["sepp-status"],
    enabled: queriesEnabled,
    refetchInterval: 5000,
    queryFn: () => getSEPPStatus(accessToken!),
    placeholderData: (prev) => prev,
  });

  const partners = useMemo(
    () => partnersQuery.data ?? [],
    [partnersQuery.data],
  );
  const peers = useMemo(() => peersQuery.data ?? [], [peersQuery.data]);
  const partnersLoading = partnersQuery.isLoading && !partnersQuery.data;
  const peersLoading = peersQuery.isLoading && !peersQuery.data;
  const sepp = seppQuery.data;
  const listener = sepp?.listener;
  const ownCertificate = sepp?.certificates.find(
    (cert) => cert.seppID === listener?.seppID,
  );

  const peersByID = useMemo(
    () => new Map(peers.map((peer) => [peer.id, peer])),
    [peers],
  );

  const usedBy = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const partner of partners) {
      if (!partner.diameterPeerID) continue;
      map.set(partner.diameterPeerID, [
        ...(map.get(partner.diameterPeerID) ?? []),
        partner.name,
      ]);
    }
    return map;
  }, [partners]);

  const domain = plmn ? plmnDomain(plmn.mcc, plmn.mnc) : "";

  const { mutate: handshake } = useMutation<unknown, unknown, RoamingPartner>({
    mutationFn: (partner) =>
      rehandshakeRoamingPartner(accessToken || "", partner.id),
    onSuccess: (_n32, partner) =>
      showSnackbar(`N32 handshake with ${partner.name} started.`, "success"),
    onError: (error) =>
      showSnackbar(
        `Failed to start the N32 handshake: ${errorMessage(error)}`,
        "error",
      ),
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: ["roaming-partners"] }),
  });

  const handleDeletePartner = async () => {
    if (!partnerToDelete || !accessToken) return;
    try {
      await deleteRoamingPartner(accessToken, partnerToDelete.id);
      queryClient.invalidateQueries({ queryKey: ["roaming-partners"] });
      showSnackbar("Roaming partner deleted successfully.", "success");
    } catch (error) {
      showSnackbar(
        `Failed to delete roaming partner: ${errorMessage(error)}`,
        "error",
      );
    } finally {
      setPartnerToDelete(null);
    }
  };

  const handleDeletePeer = async () => {
    if (!peerToDelete || !accessToken) return;
    try {
      await deleteRoamingDiameterPeer(accessToken, peerToDelete.id);
      queryClient.invalidateQueries({ queryKey: ["roaming-diameter-peers"] });
      showSnackbar("Diameter peer deleted successfully.", "success");
    } catch (error) {
      showSnackbar(
        `Failed to delete Diameter peer: ${errorMessage(error)}`,
        "error",
      );
    } finally {
      setPeerToDelete(null);
    }
  };

  const handleImported = (cert: SEPPCertificate) => {
    void queryClient.invalidateQueries({ queryKey: ["sepp-status"] });
    setWarnings(cert.warnings ?? []);
    showSnackbar("SEPP certificate imported successfully.", "success");
  };

  const handleRegenerate = async () => {
    if (!regenerate || !accessToken) return;
    if (regenerate.kind === "certificate") {
      await deleteSEPPCertificate(accessToken, regenerate.seppID);
      setWarnings(
        ownCertificate?.source === "imported"
          ? [TRUST_ANCHOR_CHANGED_WARNING]
          : [],
      );
    } else {
      const result = await regenerateSEPPCA(accessToken);
      setWarnings(result?.warnings ?? []);
    }
    void queryClient.invalidateQueries({ queryKey: ["sepp-status"] });
    showSnackbar(
      regenerate.kind === "certificate"
        ? "SEPP certificate regenerated successfully."
        : "SEPP trust anchor regenerated successfully.",
      "success",
    );
    setRegenerate(null);
  };

  const headerRowSx = {
    "& .MuiTableRow-root:first-of-type .MuiTableCell-root": {
      fontWeight: 600,
      backgroundColor: theme.palette.backgroundSubtle,
    },
  };

  const anchors = sepp?.trustAnchors ?? [];
  const seppEnabled = !!listener?.configured;
  const n32Endpoint =
    listener?.listening && listener.address
      ? formatEndpoint(listener.address, listener.port)
      : "";
  const fqdnOutsideDomain =
    seppEnabled &&
    !!domain &&
    !!ownCertificate?.fqdn &&
    !ownCertificate.fqdn.endsWith(`.${domain}`);

  return (
    <Box sx={{ mt: 3 }}>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        Let subscribers of partner networks use your network. 5G roamers reach
        their home network through your SEPP, 4G roamers through a Diameter
        peer.
      </Typography>

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
      {fqdnOutsideDomain && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {`The SEPP FQDN of this node is not under ${domain}. Import a new certificate.`}
        </Alert>
      )}
      <ThisCoreSection
        diameter={diameterQuery.data}
        loading={diameterQuery.isLoading && !diameterQuery.data}
        description="Ella Core's settings for your roaming partners. Give them its N32 Address, SEPP certificate and trust anchor for 5G, and its Diameter Realm and Origin-Host for 4G."
        realmHelp="Ella Core's Diameter realm, set by the Operator ID (MCC/MNC). Your partners' HSS answers S6a requests to it."
        before={
          <>
            {n32Endpoint ? (
              <IdentityRow
                label="N32 Address"
                help={N32_ADDRESS_HELP}
                value={n32Endpoint}
                copyAs="N32 address"
              >
                {n32Endpoint}
              </IdentityRow>
            ) : (
              <TableRow>
                <TableCell sx={SETTING_CELL_SX}>
                  <Tooltip title={N32_ADDRESS_HELP} arrow>
                    <span>N32 Address</span>
                  </Tooltip>
                </TableCell>
                <TableCell sx={VALUE_CELL_SX}>
                  {!listener ? (
                    <Skeleton variant="text" width={240} />
                  ) : (
                    <NotListening
                      setting="interfaces.n32"
                      configured={seppEnabled}
                      error={listener.error}
                    />
                  )}
                </TableCell>
                <TableCell sx={ACTION_CELL_SX}>
                  {listener && !seppEnabled && (
                    <Tooltip title="Learn how to enable roaming" arrow>
                      <IconButton
                        size="small"
                        href={PRODUCT.roamingDocsUrl}
                        target="_blank"
                        rel="noreferrer"
                        aria-label="Learn how to enable roaming"
                      >
                        <OpenInNewIcon fontSize="small" color="primary" />
                      </IconButton>
                    </Tooltip>
                  )}
                </TableCell>
              </TableRow>
            )}
            {listener && seppEnabled && (
              <>
                <TableRow>
                  <TableCell sx={SETTING_CELL_SX}>Certificate</TableCell>
                  <TableCell sx={{ ...VALUE_CELL_SX, wordBreak: "break-word" }}>
                    {ownCertificate?.notAfter ? (
                      <InlineValue>
                        <span>
                          {[
                            ownCertificate.fqdn,
                            sourceLabels[ownCertificate.source],
                            `Expires ${formatDateTime(ownCertificate.notAfter)}`,
                          ]
                            .filter(Boolean)
                            .join(" · ")}
                        </span>
                        {now > 0 && (
                          <ExpiryChip
                            notAfter={ownCertificate.notAfter}
                            now={now}
                          />
                        )}
                      </InlineValue>
                    ) : (
                      <Muted />
                    )}
                  </TableCell>
                  <TableCell sx={{ ...ACTION_CELL_SX, whiteSpace: "nowrap" }}>
                    {canEdit && listener.seppID !== undefined && (
                      <>
                        <Tooltip title="Regenerate certificate" arrow>
                          <IconButton
                            size="small"
                            onClick={() =>
                              setRegenerate({
                                kind: "certificate",
                                seppID: listener.seppID!,
                              })
                            }
                            aria-label="Regenerate certificate"
                          >
                            <RegenerateIcon fontSize="small" color="primary" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Import certificate" arrow>
                          <IconButton
                            size="small"
                            onClick={() => setImportSEPPID(listener.seppID!)}
                            aria-label="Import certificate"
                          >
                            <ImportIcon fontSize="small" color="primary" />
                          </IconButton>
                        </Tooltip>
                      </>
                    )}
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={SETTING_CELL_SX}>Trust anchor</TableCell>
                  <TableCell sx={VALUE_CELL_SX}>
                    {anchors.length ? (
                      <Typography
                        variant="body2"
                        color="textSecondary"
                        component="span"
                      >
                        Give to your partners
                      </Typography>
                    ) : (
                      <Muted />
                    )}
                  </TableCell>
                  <TableCell sx={{ ...ACTION_CELL_SX, whiteSpace: "nowrap" }}>
                    {canEdit && (
                      <Tooltip title="Regenerate trust anchor" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setRegenerate({ kind: "ca" })}
                          aria-label="Regenerate trust anchor"
                        >
                          <RegenerateIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    )}
                    {anchors.map((anchor) => (
                      <Tooltip
                        key={anchor.fingerprint}
                        title={<AnchorTooltip anchor={anchor} />}
                        arrow
                      >
                        <IconButton
                          size="small"
                          onClick={() =>
                            downloadText(anchorFilename(anchor), anchor.pem)
                          }
                          aria-label={
                            anchors.length > 1
                              ? `Download trust anchor ${anchor.fingerprint}`
                              : "Download trust anchor"
                          }
                        >
                          <DownloadIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    ))}
                  </TableCell>
                </TableRow>
              </>
            )}
          </>
        }
      />

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Partners
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          Networks whose subscribers can roam on your network.
        </Typography>
        {canEdit && (
          <AddButton label="Add Partner" onClick={() => setPartnerModal({})} />
        )}
      </Stack>
      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 4 }}>
        <Table sx={{ tableLayout: "fixed" }} aria-label="Roaming partners">
          <TableBody sx={headerRowSx}>
            <TableRow>
              <TableCell sx={{ width: "18%" }}>Name</TableCell>
              <TableCell sx={{ width: "14%" }}>PLMNs</TableCell>
              <TableCell sx={{ width: "14%" }}>Profile</TableCell>
              <TableCell>Diameter Peer</TableCell>
              <TableCell sx={{ width: 150 }}>N32 Status</TableCell>
              <TableCell align="right" sx={{ width: 140 }}>
                Actions
              </TableCell>
            </TableRow>
            {partners.length === 0 && (
              <EmptyRow
                colSpan={6}
                loading={partnersLoading}
                text="No roaming partners yet."
              />
            )}
            {partners.map((partner) => {
              const peer = partner.diameterPeerID
                ? peersByID.get(partner.diameterPeerID)
                : undefined;
              return (
                <TableRow key={partner.id}>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {partner.name}
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {partner.plmns.map(formatPLMN).join(", ")}
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    <Link
                      to={`/profiles/${encodeURIComponent(partner.profileName)}`}
                      style={{
                        color: theme.palette.link,
                        textDecoration: "underline",
                      }}
                    >
                      {partner.profileName}
                    </Link>
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {partner.diameterPeerID ? (
                      peer ? (
                        peer.originHost ? (
                          <DomainName name={peer.originHost} />
                        ) : (
                          formatEndpoint(peer.address, peer.port)
                        )
                      ) : (
                        partner.diameterPeerID
                      )
                    ) : (
                      <Muted />
                    )}
                  </TableCell>
                  <TableCell>
                    {partner.n32 ? (
                      <N32StatusChip n32={partner.n32} />
                    ) : (
                      <Muted />
                    )}
                  </TableCell>
                  <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                    {canEdit && (
                      <>
                        {partner.n32 && (
                          <Tooltip title="Restart N32 handshake" arrow>
                            <IconButton
                              size="small"
                              onClick={() => handshake(partner)}
                              aria-label={`Restart N32 handshake with ${partner.name}`}
                            >
                              <SyncIcon fontSize="small" color="primary" />
                            </IconButton>
                          </Tooltip>
                        )}
                        <Tooltip title="Edit partner" arrow>
                          <IconButton
                            size="small"
                            onClick={() => setPartnerModal({ partner })}
                            aria-label={`Edit roaming partner ${partner.name}`}
                          >
                            <EditIcon fontSize="small" color="primary" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Delete partner" arrow>
                          <IconButton
                            size="small"
                            onClick={() => setPartnerToDelete(partner)}
                            aria-label={`Delete roaming partner ${partner.name}`}
                          >
                            <DeleteIcon fontSize="small" color="primary" />
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

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Diameter Peers
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          Diameter edge agents or IPX nodes that connect Ella Core to the HSS of
          4G partners.
        </Typography>
        {canEdit && (
          <AddButton
            label="Add Diameter Peer"
            onClick={() => setPeerModal({})}
          />
        )}
      </Stack>
      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 4 }}>
        <Table sx={{ tableLayout: "fixed" }} aria-label="Diameter peers">
          <TableBody sx={headerRowSx}>
            <TableRow>
              <TableCell sx={{ width: "32%" }}>Origin-Host</TableCell>
              <TableCell sx={{ width: "22%" }}>Address</TableCell>
              <TableCell>Partners</TableCell>
              <TableCell sx={{ width: 150 }}>Status</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {peers.length === 0 && (
              <EmptyRow
                colSpan={5}
                loading={peersLoading}
                text="No Diameter peers yet."
              />
            )}
            {peers.map((peer) => {
              const endpoint = formatEndpoint(peer.address, peer.port);
              const names = usedBy.get(peer.id) ?? [];
              return (
                <TableRow key={peer.id}>
                  <TableCell>
                    {peer.originHost ? (
                      <DomainName name={peer.originHost} />
                    ) : (
                      <Typography variant="body2" color="textSecondary">
                        Any
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {endpoint}
                  </TableCell>
                  <TableCell sx={{ wordBreak: "break-word" }}>
                    {names.length ? names.join(", ") : <Muted />}
                  </TableCell>
                  <TableCell>
                    <DiameterPeerStatusChip status={peer.status} />
                  </TableCell>
                  <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                    {canEdit && (
                      <>
                        <Tooltip title="Edit Diameter peer" arrow>
                          <IconButton
                            size="small"
                            onClick={() => setPeerModal({ peer })}
                            aria-label={`Edit Diameter peer ${endpoint}`}
                          >
                            <EditIcon fontSize="small" color="primary" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip
                          title={
                            names.length
                              ? "Used by a partner"
                              : "Delete Diameter peer"
                          }
                          arrow
                        >
                          <span>
                            <IconButton
                              size="small"
                              disabled={names.length > 0}
                              onClick={() => setPeerToDelete(peer)}
                              aria-label={`Delete Diameter peer ${endpoint}`}
                            >
                              <DeleteIcon
                                fontSize="small"
                                color={names.length ? "disabled" : "primary"}
                              />
                            </IconButton>
                          </span>
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

      {partnerModal && (
        <RoamingPartnerModal
          open
          onClose={() => setPartnerModal(null)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["roaming-partners"] });
            showSnackbar(
              partnerModal.partner
                ? "Roaming partner updated successfully."
                : "Roaming partner added successfully.",
              "success",
            );
          }}
          partner={partnerModal.partner}
          diameterPeers={peers}
        />
      )}
      {partnerToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setPartnerToDelete(null)}
          onConfirm={handleDeletePartner}
          title="Delete Roaming Partner"
          description={`Delete the roaming partner ${partnerToDelete.name}? Its roamers will be deregistered.`}
        />
      )}
      {peerModal && (
        <RoamingDiameterPeerModal
          open
          onClose={() => setPeerModal(null)}
          onSuccess={() => {
            queryClient.invalidateQueries({
              queryKey: ["roaming-diameter-peers"],
            });
            queryClient.invalidateQueries({
              queryKey: ["integrations-diameter"],
            });
            showSnackbar(
              peerModal.peer
                ? "Diameter peer updated successfully."
                : "Diameter peer added successfully.",
              "success",
            );
          }}
          peer={peerModal.peer}
        />
      )}
      {peerToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setPeerToDelete(null)}
          onConfirm={handleDeletePeer}
          title="Delete Diameter Peer"
          description={`Delete the Diameter peer ${formatEndpoint(peerToDelete.address, peerToDelete.port)}? Its connection is closed.`}
        />
      )}
      {importSEPPID !== null && (
        <ImportSEPPCertificateModal
          open
          onClose={() => setImportSEPPID(null)}
          onSuccess={handleImported}
          seppID={importSEPPID}
          mcc={plmn?.mcc}
          mnc={plmn?.mnc}
        />
      )}
      {regenerate && (
        <ConfirmDialog
          open
          onClose={() => setRegenerate(null)}
          onConfirm={handleRegenerate}
          title={
            regenerate.kind === "certificate"
              ? "Regenerate Certificate"
              : "Regenerate Trust Anchor"
          }
          description={
            regenerate.kind === "certificate"
              ? "Replace this node's SEPP certificate with a new one signed by the SEPP CA?"
              : "Partners must import the new trust anchor before roaming works again."
          }
          confirmLabel="Regenerate"
          confirmingLabel="Regenerating…"
        />
      )}
    </Box>
  );
}
