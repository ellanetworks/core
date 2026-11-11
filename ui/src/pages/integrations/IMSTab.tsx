// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Box,
  Button,
  IconButton,
  Link as MuiLink,
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
import { useTheme, type Theme } from "@mui/material/styles";
import { Link as RouterLink } from "react-router-dom";
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
} from "@mui/icons-material";
import {
  deleteIMSPeer,
  getIMSSettings,
  listIMSPeers,
  primaryHomeDomain,
  type IMSPeer,
  type IMSSettings,
  type IMSVoiceStatus,
} from "@/queries/integrations";
import {
  deleteIMSApplicationServer,
  deleteIMSServiceProfile,
  listIMSApplicationServers,
  listIMSServiceProfiles,
  type IMSApplicationServer,
  type IMSServiceProfile,
} from "@/queries/ims";
import EditPCSCFAddressesModal from "@/components/EditPCSCFAddressesModal";
import EditIMSHomeDomainModal, {
  HOME_DOMAIN_HELP,
} from "@/components/EditIMSHomeDomainModal";
import EditAdvertiseVoiceModal, {
  ADVERTISE_VOICE_HELP,
} from "@/components/EditAdvertiseVoiceModal";
import EditRestrictPeersModal, {
  RESTRICT_PEERS_HELP,
} from "@/components/EditRestrictPeersModal";
import IMSPeerModal, { toCIDR } from "@/components/IMSPeerModal";
import ConnectionsStatusChip from "@/components/ConnectionsStatusChip";
import DomainName from "@/components/DomainName";
import IMSApplicationServerModal from "@/components/IMSApplicationServerModal";
import IMSServiceProfileModal from "@/components/IMSServiceProfileModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import {
  ACTION_CELL_SX,
  SETTING_CELL_SX,
  TABLE_CONTAINER_SX,
  VALUE_CELL_SX,
} from "@/utils/layout";
import { IMS_DATA_NETWORK } from "@/utils/voice";
import { useIntegrationsContext } from "./types";
import {
  ThisCoreSection,
  UnknownPeersTable,
  useDiameterStatus,
} from "./diameter";

export const OPEN_PEER_POLICY_WARNING =
  "Any system that can reach the Diameter port can query subscriber credentials. Turn on Restrict to Allowed Peers to block them.";

export const EMPTY_ALLOWLIST_WARNING =
  "No allowed peers: every IMS node is rejected.";

const linkSx = { color: (theme: Theme) => theme.palette.link };

const VoiceStatusAlert: React.FC<{
  status: IMSVoiceStatus;
  advertiseVoice: boolean;
}> = ({ status, advertiseVoice }) => {
  if (status === "enabled") {
    return advertiseVoice ? null : (
      <Alert severity="info" sx={{ mb: 2 }}>
        IMS voice is ready, but phones are not told it is available: Advertise
        IMS Voice is off.
      </Alert>
    );
  }

  if (!advertiseVoice) return null;

  return (
    <Alert severity="warning" sx={{ mb: 2 }}>
      Phones are told IMS voice is available, but calls fail until this is
      fixed:{" "}
      {status === "no_pcscf" && (
        <>
          no P-CSCF address is set. Set the P-CSCF addresses below so handsets
          can reach your IMS.
        </>
      )}
      {status === "no_ims_data_network" && (
        <>
          there is no IMS data network named {IMS_DATA_NETWORK}. Create it, or
          mark it as IMS, in{" "}
          <MuiLink
            component={RouterLink}
            to="/networking/data-networks"
            sx={linkSx}
          >
            Networking › Data Networks
          </MuiLink>
          .
        </>
      )}
      {status === "ip_family_mismatch" && (
        <>
          no P-CSCF address matches the {IMS_DATA_NETWORK} data network&apos;s
          IP family. Add a P-CSCF address of the same family as its IP pool, or
          give the data network a pool of the P-CSCF&apos;s family.
        </>
      )}
    </Alert>
  );
};

const errorMessage = (error: unknown) =>
  error instanceof Error ? error.message : "Unknown error occurred.";

const PeerConnections: React.FC<{ peer: IMSPeer }> = ({ peer }) => (
  <ConnectionsStatusChip
    connections={peer.connections.map((status) => ({
      label: status.originHost || "Unknown host",
      status,
    }))}
  />
);

export default function IMSTab() {
  const { canEdit } = useIntegrationsContext();
  const theme = useTheme();
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [domainOpen, setDomainOpen] = useState(false);
  const [advertiseOpen, setAdvertiseOpen] = useState(false);
  const [restrictOpen, setRestrictOpen] = useState(false);
  const [peerModal, setPeerModal] = useState<{
    peer?: IMSPeer;
    initialValues?: { name: string; originHost: string; addresses: string[] };
  } | null>(null);
  const [peerToDelete, setPeerToDelete] = useState<IMSPeer | null>(null);
  const [asModal, setASModal] = useState<{
    applicationServer?: IMSApplicationServer;
  } | null>(null);
  const [asToDelete, setASToDelete] = useState<IMSApplicationServer | null>(
    null,
  );

  const [spModal, setSPModal] = useState<{
    serviceProfile?: IMSServiceProfile;
  } | null>(null);
  const [spToDelete, setSPToDelete] = useState<IMSServiceProfile | null>(null);

  const anyModalOpen =
    editOpen ||
    domainOpen ||
    advertiseOpen ||
    restrictOpen ||
    peerModal !== null ||
    peerToDelete !== null ||
    asModal !== null ||
    asToDelete !== null ||
    spModal !== null ||
    spToDelete !== null;

  const settingsQuery = useQuery<IMSSettings>({
    queryKey: ["integrations-ims"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    queryFn: () => getIMSSettings(accessToken!),
    placeholderData: (prev) => prev,
  });
  const ims = settingsQuery.data;

  const settingsLoading = settingsQuery.isLoading && !ims;

  const peersQuery = useQuery<IMSPeer[]>({
    queryKey: ["ims-peers"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listIMSPeers(accessToken!),
    placeholderData: (prev) => prev,
  });
  const peers = peersQuery.data ?? [];
  const peersLoading = peersQuery.isLoading && !peersQuery.data;

  const diameterQuery = useDiameterStatus(!anyModalOpen);
  const diameter = diameterQuery.data;
  const diameterLoading = diameterQuery.isLoading && !diameter;
  const unknownPeers = (diameter?.peers ?? []).filter(
    (p) => p.direction === "inbound" && p.role === "unknown",
  );

  const serversQuery = useQuery<IMSApplicationServer[]>({
    queryKey: ["ims-application-servers"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listIMSApplicationServers(accessToken!),
    placeholderData: (prev) => prev,
  });

  const servers = serversQuery.data ?? [];
  const serversLoading = serversQuery.isLoading && !serversQuery.data;

  const serviceProfilesQuery = useQuery<IMSServiceProfile[]>({
    queryKey: ["ims-service-profiles"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listIMSServiceProfiles(accessToken!),
    placeholderData: (prev) => prev,
  });

  const serviceProfiles = serviceProfilesQuery.data ?? [];
  const serviceProfilesLoading =
    serviceProfilesQuery.isLoading && !serviceProfilesQuery.data;

  const addresses = ims?.pcscfAddresses ?? [];
  const homeDomain = primaryHomeDomain(ims);

  const refreshPeers = () => {
    queryClient.invalidateQueries({ queryKey: ["ims-peers"] });
    queryClient.invalidateQueries({ queryKey: ["integrations-ims"] });
    queryClient.invalidateQueries({ queryKey: ["integrations-diameter"] });
  };

  const handleDeletePeer = async () => {
    if (!peerToDelete || !accessToken) return;
    try {
      await deleteIMSPeer(accessToken, peerToDelete.name);
      refreshPeers();
      showSnackbar("Allowed peer deleted successfully.", "success");
    } catch (error) {
      showSnackbar(
        `Failed to delete allowed peer: ${errorMessage(error)}`,
        "error",
      );
    } finally {
      setPeerToDelete(null);
    }
  };

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["ims-application-servers"] });
  };

  const handleDeleteSP = async () => {
    if (!spToDelete || !accessToken) return;
    try {
      await deleteIMSServiceProfile(accessToken, spToDelete.name);
      queryClient.invalidateQueries({ queryKey: ["ims-service-profiles"] });
      showSnackbar("Service profile deleted successfully.", "success");
    } catch (error) {
      showSnackbar(
        `Failed to delete service profile: ${errorMessage(error)}`,
        "error",
      );
    } finally {
      setSPToDelete(null);
    }
  };

  const handleDeleteAS = async () => {
    if (!asToDelete || !accessToken) return;
    try {
      await deleteIMSApplicationServer(accessToken, asToDelete.name);
      refresh();
      showSnackbar("Application server deleted successfully.", "success");
    } catch (error) {
      showSnackbar(
        `Failed to delete application server: ${errorMessage(error)}`,
        "error",
      );
    } finally {
      setASToDelete(null);
    }
  };

  const headerRowSx = {
    "& .MuiTableRow-root:first-of-type .MuiTableCell-root": {
      fontWeight: 600,
      backgroundColor: theme.palette.backgroundSubtle,
    },
  };

  return (
    <Box sx={{ mt: 3 }}>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        Let 4G and 5G subscribers make voice and video calls through your IMS.
        Each subscriber needs a phone number and a policy on the{" "}
        {IMS_DATA_NETWORK} data network, marked as IMS.
      </Typography>

      {settingsQuery.isError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          Failed to load IMS settings.
        </Alert>
      )}
      {ims && (
        <VoiceStatusAlert
          status={ims.voiceStatus}
          advertiseVoice={ims.advertiseVoice}
        />
      )}

      <ThisCoreSection
        diameter={diameter}
        loading={diameterLoading}
        showAddress
        showServedRealms
        description="Ella Core's settings for your IMS. Enter its Diameter Realm, Origin-Host and Diameter Address in your IMS, which connects to Ella Core as its HSS and PCRF."
        realmHelp="Ella Core's Diameter realm, set by the Operator ID (MCC/MNC). Ella Core answers Cx, Sh and Rx requests sent to this realm or to the IMS home domain."
        after={
          <>
            <TableRow>
              <TableCell sx={SETTING_CELL_SX}>
                <Tooltip title="The IMS entry points handsets call" arrow>
                  <span>P-CSCF Addresses</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={{ ...VALUE_CELL_SX, wordBreak: "break-word" }}>
                {settingsLoading ? (
                  <Skeleton variant="text" width={160} />
                ) : addresses.length > 0 ? (
                  addresses.join(", ")
                ) : (
                  "N/A"
                )}
              </TableCell>
              <TableCell sx={ACTION_CELL_SX}>
                {canEdit && (
                  <Tooltip title="Edit P-CSCF addresses" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setEditOpen(true)}
                      aria-label="Edit P-CSCF addresses"
                      disabled={!ims}
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell sx={SETTING_CELL_SX}>
                <Tooltip title={HOME_DOMAIN_HELP} arrow>
                  <span>Home Domain</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={{ ...VALUE_CELL_SX, wordBreak: "break-word" }}>
                {settingsLoading ? (
                  <Skeleton variant="text" width={240} />
                ) : !ims ? (
                  "N/A"
                ) : ims.homeDomain ? (
                  <>
                    <Typography variant="body2">{ims.homeDomain}</Typography>
                    {ims.derivedHomeDomain &&
                      ims.derivedHomeDomain !== ims.homeDomain && (
                        <Typography variant="body2" color="textSecondary">
                          Also accepted: {ims.derivedHomeDomain}
                        </Typography>
                      )}
                  </>
                ) : (
                  <>
                    <Typography variant="body2">
                      {ims.derivedHomeDomain || "N/A"}
                    </Typography>
                    <Typography variant="body2" color="textSecondary">
                      Default
                    </Typography>
                  </>
                )}
              </TableCell>
              <TableCell sx={ACTION_CELL_SX}>
                {canEdit && (
                  <Tooltip title="Edit home domain" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setDomainOpen(true)}
                      aria-label="Edit home domain"
                      disabled={!ims}
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell sx={SETTING_CELL_SX}>
                <Tooltip title={ADVERTISE_VOICE_HELP} arrow>
                  <span>Advertise IMS Voice</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={VALUE_CELL_SX}>
                {settingsLoading ? (
                  <Skeleton variant="text" width={60} />
                ) : !ims ? (
                  "N/A"
                ) : ims.advertiseVoice ? (
                  "On"
                ) : (
                  "Off"
                )}
              </TableCell>
              <TableCell sx={ACTION_CELL_SX}>
                {canEdit && (
                  <Tooltip title="Edit IMS voice advertisement" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setAdvertiseOpen(true)}
                      aria-label="Edit IMS voice advertisement"
                      disabled={!ims}
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell sx={SETTING_CELL_SX}>
                <Tooltip title={RESTRICT_PEERS_HELP} arrow>
                  <span>Restrict to Allowed Peers</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={VALUE_CELL_SX}>
                {settingsLoading ? (
                  <Skeleton variant="text" width={60} />
                ) : !ims ? (
                  "N/A"
                ) : ims.restrictPeers ? (
                  "On"
                ) : (
                  "Off"
                )}
              </TableCell>
              <TableCell sx={ACTION_CELL_SX}>
                {canEdit && (
                  <Tooltip title="Edit IMS peer restriction" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setRestrictOpen(true)}
                      aria-label="Edit IMS peer restriction"
                      disabled={!ims}
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
          </>
        }
      />

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Allowed IMS Peers
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          IMS nodes allowed to connect to Ella Core.
        </Typography>
        {canEdit && (
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<AddIcon />}
            onClick={() => setPeerModal({})}
            aria-label="Add allowed peer"
            sx={{ ml: 2, flexShrink: 0 }}
          >
            Add Allowed Peer
          </Button>
        )}
      </Stack>
      {ims && !ims.restrictPeers && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {OPEN_PEER_POLICY_WARNING}
        </Alert>
      )}
      {ims?.restrictPeers && !peersLoading && peers.length === 0 && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {EMPTY_ALLOWLIST_WARNING}
        </Alert>
      )}
      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 3 }}>
        <Table sx={{ tableLayout: "fixed" }} aria-label="Allowed IMS peers">
          <TableBody sx={headerRowSx}>
            <TableRow>
              <TableCell sx={{ width: "15%" }}>Name</TableCell>
              <TableCell>Origin-Host</TableCell>
              <TableCell sx={{ width: "20%" }}>Realm</TableCell>
              <TableCell sx={{ width: "17%" }}>Source Addresses</TableCell>
              <TableCell sx={{ width: 160 }}>Status</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {peersLoading && (
              <TableRow>
                <TableCell colSpan={6}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!peersLoading && peers.length === 0 && (
              <TableRow>
                <TableCell colSpan={6}>
                  <Typography variant="body2" color="textSecondary">
                    No allowed peers yet.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {peers.map((peer) => (
              <TableRow key={peer.name}>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {peer.name}
                </TableCell>
                <TableCell>
                  {peer.originHost ? (
                    <DomainName name={peer.originHost} />
                  ) : (
                    <Typography variant="body2" color="textSecondary">
                      Any
                    </Typography>
                  )}
                </TableCell>
                <TableCell>
                  {peer.realm ? (
                    <DomainName name={peer.realm} />
                  ) : (
                    <Typography variant="body2" color="textSecondary">
                      Any
                    </Typography>
                  )}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {(peer.addresses ?? []).join(", ")}
                </TableCell>
                <TableCell>
                  <PeerConnections peer={peer} />
                </TableCell>
                <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                  {canEdit && (
                    <>
                      <Tooltip title="Edit allowed peer" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setPeerModal({ peer })}
                          aria-label={`Edit allowed peer ${peer.name}`}
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete allowed peer" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setPeerToDelete(peer)}
                          aria-label={`Delete allowed peer ${peer.name}`}
                        >
                          <DeleteIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    </>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {unknownPeers.length > 0 && (
        <>
          <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
            Unknown Peers ({unknownPeers.length})
          </Typography>
          <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
            Diameter peers that connected without matching any allowed peer.
            They are accepted only while the list above is empty: add the ones
            you trust.
          </Typography>
          <UnknownPeersTable
            peers={unknownPeers}
            canEdit={canEdit}
            onAdd={(peer) =>
              setPeerModal({
                initialValues: {
                  name: peer.originHost?.split(".")[0] ?? "",
                  originHost: peer.originHost ?? "",
                  addresses: [toCIDR(peer.address)],
                },
              })
            }
          />
        </>
      )}

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Application Servers
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          SIP servers that receive requests matching the filter criteria of a
          service profile.
        </Typography>
        {canEdit && (
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<AddIcon />}
            onClick={() => setASModal({})}
            aria-label="Add application server"
            sx={{ ml: 2, flexShrink: 0 }}
          >
            Add Application Server
          </Button>
        )}
      </Stack>
      <TableContainer sx={TABLE_CONTAINER_SX}>
        <Table sx={{ tableLayout: "fixed" }}>
          <TableBody sx={headerRowSx}>
            <TableRow>
              <TableCell sx={{ width: "20%" }}>Name</TableCell>
              <TableCell sx={{ width: "30%" }}>Server Name</TableCell>
              <TableCell>Public Service Identities</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {serversLoading && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!serversLoading && servers.length === 0 && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Typography variant="body2" color="textSecondary">
                    No application servers yet.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {servers.map((as) => (
              <TableRow key={as.name}>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {as.name}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {as.serverName}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {as.publicServiceIdentities.length > 0
                    ? as.publicServiceIdentities.join(", ")
                    : "None"}
                </TableCell>
                <TableCell align="right">
                  {canEdit && (
                    <>
                      <Tooltip title="Edit application server" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setASModal({ applicationServer: as })}
                          aria-label={`Edit application server ${as.name}`}
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete application server" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setASToDelete(as)}
                          aria-label={`Delete application server ${as.name}`}
                        >
                          <DeleteIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    </>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mt: 4, mb: 2 }}>
        Service Profiles
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          Ordered initial filter criteria. Assign one to a profile's MMTel or
          MCPTT identities on the Profiles page.
        </Typography>
        {canEdit && (
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<AddIcon />}
            onClick={() => setSPModal({})}
            aria-label="Add service profile"
            sx={{ ml: 2, flexShrink: 0 }}
          >
            Add Service Profile
          </Button>
        )}
      </Stack>
      <TableContainer sx={TABLE_CONTAINER_SX}>
        <Table sx={{ tableLayout: "fixed" }}>
          <TableBody sx={headerRowSx}>
            <TableRow>
              <TableCell sx={{ width: "20%" }}>Name</TableCell>
              <TableCell>
                <Tooltip
                  title="The application servers of its initial filter criteria, by priority"
                  arrow
                >
                  <span>Application Servers</span>
                </Tooltip>
              </TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {serviceProfilesLoading && (
              <TableRow>
                <TableCell colSpan={3}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!serviceProfilesLoading && serviceProfiles.length === 0 && (
              <TableRow>
                <TableCell colSpan={3}>
                  <Typography variant="body2" color="textSecondary">
                    No service profiles yet.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {serviceProfiles.map((sp) => (
              <TableRow key={sp.name}>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {sp.name}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {sp.initialFilterCriteria.length === 0
                    ? "None"
                    : [
                        ...new Set(
                          [...sp.initialFilterCriteria]
                            .sort((a, b) => a.priority - b.priority)
                            .map((ifc) => ifc.applicationServerName),
                        ),
                      ].join(", ")}
                </TableCell>
                <TableCell align="right">
                  {canEdit && (
                    <>
                      <Tooltip title="Edit service profile" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setSPModal({ serviceProfile: sp })}
                          aria-label={`Edit service profile ${sp.name}`}
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete service profile" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setSPToDelete(sp)}
                          aria-label={`Delete service profile ${sp.name}`}
                        >
                          <DeleteIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    </>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {editOpen && ims && (
        <EditPCSCFAddressesModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["integrations-ims"] });
            showSnackbar("P-CSCF addresses updated successfully.", "success");
          }}
          initialData={{
            pcscfAddresses: addresses,
            homeDomain: ims.homeDomain,
            advertiseVoice: ims.advertiseVoice,
            restrictPeers: ims.restrictPeers,
          }}
        />
      )}
      {domainOpen && ims && (
        <EditIMSHomeDomainModal
          open
          onClose={() => setDomainOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["integrations-ims"] });
            queryClient.invalidateQueries({
              queryKey: ["ims-application-servers"],
            });
            showSnackbar("Home domain updated successfully.", "success");
          }}
          initialData={{
            homeDomain: ims.homeDomain,
            derivedHomeDomain: ims.derivedHomeDomain,
            pcscfAddresses: ims.pcscfAddresses,
            advertiseVoice: ims.advertiseVoice,
            restrictPeers: ims.restrictPeers,
          }}
        />
      )}
      {advertiseOpen && ims && (
        <EditAdvertiseVoiceModal
          open
          onClose={() => setAdvertiseOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["integrations-ims"] });
            showSnackbar(
              "IMS voice advertisement updated successfully.",
              "success",
            );
          }}
          initialData={{
            pcscfAddresses: ims.pcscfAddresses,
            homeDomain: ims.homeDomain,
            advertiseVoice: ims.advertiseVoice,
            restrictPeers: ims.restrictPeers,
          }}
        />
      )}
      {restrictOpen && ims && (
        <EditRestrictPeersModal
          open
          onClose={() => setRestrictOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["integrations-ims"] });
            showSnackbar(
              "IMS peer restriction updated successfully.",
              "success",
            );
          }}
          initialData={{
            pcscfAddresses: ims.pcscfAddresses,
            homeDomain: ims.homeDomain,
            advertiseVoice: ims.advertiseVoice,
            restrictPeers: ims.restrictPeers,
          }}
        />
      )}
      {peerModal && (
        <IMSPeerModal
          open
          onClose={() => setPeerModal(null)}
          onSuccess={() => {
            refreshPeers();
            showSnackbar(
              peerModal.peer
                ? "Allowed peer updated successfully."
                : "Allowed peer added successfully.",
              "success",
            );
          }}
          peer={peerModal.peer}
          initialValues={peerModal.initialValues}
        />
      )}
      {peerToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setPeerToDelete(null)}
          onConfirm={handleDeletePeer}
          title="Delete Allowed Peer"
          description={
            peers.length === 1
              ? `Delete the allowed peer ${peerToDelete.name}? It is your last one: any system that can reach the Diameter port will be accepted again.`
              : `Delete the allowed peer ${peerToDelete.name}? Its connections are closed.`
          }
        />
      )}
      {asModal && (
        <IMSApplicationServerModal
          open
          onClose={() => setASModal(null)}
          onSuccess={() => {
            refresh();
            showSnackbar(
              asModal.applicationServer
                ? "Application server updated successfully."
                : "Application server added successfully.",
              "success",
            );
          }}
          applicationServer={asModal.applicationServer}
          homeDomain={homeDomain || undefined}
        />
      )}
      {spModal && (
        <IMSServiceProfileModal
          open
          onClose={() => setSPModal(null)}
          onSuccess={() => {
            queryClient.invalidateQueries({
              queryKey: ["ims-service-profiles"],
            });
            showSnackbar(
              spModal.serviceProfile
                ? "Service profile updated successfully."
                : "Service profile added successfully.",
              "success",
            );
          }}
          serviceProfile={spModal.serviceProfile}
        />
      )}
      {spToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setSPToDelete(null)}
          onConfirm={handleDeleteSP}
          title="Delete Service Profile"
          description={`Delete the service profile ${spToDelete.name}?`}
        />
      )}
      {asToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setASToDelete(null)}
          onConfirm={handleDeleteAS}
          title="Delete Application Server"
          description={`Delete the application server ${asToDelete.name}?`}
        />
      )}
    </Box>
  );
}
