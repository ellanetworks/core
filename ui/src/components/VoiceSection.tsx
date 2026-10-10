// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import RadioButtonUncheckedIcon from "@mui/icons-material/RadioButtonUnchecked";
import { Edit as EditIcon } from "@mui/icons-material";
import type { OperatorIMS } from "@/queries/operator";
import { getDataNetwork } from "@/queries/data_networks";
import { listPolicies } from "@/queries/policies";
import { getDiameterStatus } from "@/queries/diameter";
import EditOperatorIMSModal from "@/components/EditOperatorIMSModal";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { TABLE_CONTAINER_SX } from "@/utils/layout";
import { ipv4Regex } from "@/utils/ip";
import { IMS_DATA_NETWORK } from "@/utils/voice";

const POLICIES_PER_PAGE = 100;

interface Check {
  label: string;
  done: boolean;
  detail: string;
  to?: string;
}

const linkSx = {
  color: "link",
  textDecoration: "underline",
  "&:hover": { textDecoration: "underline" },
};

const voicePolicyCount = async (authToken: string) => {
  let count = 0;

  for (let page = 1; ; page++) {
    const result = await listPolicies(authToken, page, POLICIES_PER_PAGE);
    const items = result.items ?? [];
    count += items.filter(
      (p) => p.data_network_name === IMS_DATA_NETWORK,
    ).length;

    if (
      items.length < POLICIES_PER_PAGE ||
      page * POLICIES_PER_PAGE >= (result.total_count ?? 0)
    ) {
      return count;
    }
  }
};

interface VoiceSectionProps {
  ims?: OperatorIMS;
  canEdit: boolean;
  onModalOpenChange: (open: boolean) => void;
}

const VoiceSection: React.FC<VoiceSectionProps> = ({
  ims,
  canEdit,
  onModalOpenChange,
}) => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);

  useEffect(() => {
    onModalOpenChange(editOpen);
  }, [editOpen, onModalOpenChange]);

  const enabled = authReady && !!accessToken && !editOpen;

  const dataNetworkQuery = useQuery({
    queryKey: ["data-networks", IMS_DATA_NETWORK],
    queryFn: () => getDataNetwork(accessToken!, IMS_DATA_NETWORK),
    enabled,
    retry: false,
  });

  const policiesQuery = useQuery({
    queryKey: ["voice-policies"],
    queryFn: () => voicePolicyCount(accessToken!),
    enabled,
  });

  const diameterQuery = useQuery({
    queryKey: ["diameter-status"],
    queryFn: () => getDiameterStatus(accessToken!),
    enabled,
    refetchInterval: 5000,
    retry: false,
  });

  const addresses = ims?.pcscfAddresses ?? [];
  const dataNetwork = dataNetworkQuery.data;
  const reachable =
    !dataNetwork ||
    addresses.some((a) =>
      ipv4Regex.test(a) ? !!dataNetwork.ipv4_pool : !!dataNetwork.ipv6_pool,
    );
  const imsConnected = (diameterQuery.data?.peers ?? []).some(
    (p) => p.role === "ims" && p.state === "open",
  );

  const checks: Check[] = [
    {
      label: "Voice data network",
      done: !!dataNetwork,
      detail: dataNetwork
        ? `The ${IMS_DATA_NETWORK} data network exists.`
        : "Create a data network of type Voice (IMS).",
      to: dataNetwork
        ? `/networking/data-networks/${IMS_DATA_NETWORK}`
        : "/networking/data-networks",
    },
    {
      label: "Voice policy",
      done: (policiesQuery.data ?? 0) > 0,
      detail:
        (policiesQuery.data ?? 0) > 0
          ? `At least one profile has a policy on the ${IMS_DATA_NETWORK} data network.`
          : `Add a policy on the ${IMS_DATA_NETWORK} data network to the profiles of subscribers who get voice.`,
      to: "/profiles",
    },
    {
      label: "P-CSCF addresses",
      done: addresses.length > 0 && reachable,
      detail:
        addresses.length === 0
          ? "Set the addresses of Ella IMS's P-CSCF."
          : reachable
            ? "Handsets on the voice data network receive these addresses."
            : `No address matches an IP family of the ${IMS_DATA_NETWORK} data network's pools.`,
    },
    {
      label: "Ella IMS",
      done: imsConnected,
      detail: imsConnected
        ? "Connected to this node over Diameter."
        : "Not connected to this node. Add this node as a peer in Ella IMS.",
      to: "/networking/interfaces",
    },
  ];

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        Voice
      </Typography>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        Let 4G and 5G subscribers make voice and video calls through Ella IMS.
        Each subscriber needs a phone number and a policy on the{" "}
        {IMS_DATA_NETWORK} data network.
      </Typography>

      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 3 }}>
        <Table>
          <TableBody>
            <TableRow>
              <TableCell sx={{ fontWeight: 600, width: "35%" }}>
                <Tooltip title="The IMS entry points handsets call" arrow>
                  <span>P-CSCF Addresses</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={{ width: "55%", wordBreak: "break-word" }}>
                {addresses.length > 0 ? addresses.join(", ") : "N/A"}
              </TableCell>
              <TableCell sx={{ width: "10%", textAlign: "right" }}>
                {canEdit && (
                  <Tooltip title="Edit P-CSCF addresses" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setEditOpen(true)}
                      aria-label="Edit P-CSCF addresses"
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableContainer>

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Setup
      </Typography>
      <TableContainer sx={TABLE_CONTAINER_SX}>
        <Table aria-label="Voice setup">
          <TableBody>
            {checks.map((check) => (
              <TableRow key={check.label}>
                <TableCell sx={{ width: 40 }}>
                  {check.done ? (
                    <CheckCircleIcon
                      color="success"
                      fontSize="small"
                      titleAccess="Done"
                    />
                  ) : (
                    <RadioButtonUncheckedIcon
                      color="warning"
                      fontSize="small"
                      titleAccess="To do"
                    />
                  )}
                </TableCell>
                <TableCell sx={{ fontWeight: 600, width: "30%" }}>
                  {check.to ? (
                    <Typography
                      variant="body2"
                      component={RouterLink}
                      to={check.to}
                      sx={linkSx}
                    >
                      {check.label}
                    </Typography>
                  ) : (
                    check.label
                  )}
                </TableCell>
                <TableCell>
                  <Typography variant="body2" color="textSecondary">
                    {check.detail}
                  </Typography>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {editOpen && (
        <EditOperatorIMSModal
          open
          onClose={() => setEditOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["operator"] });
            showSnackbar("Voice settings updated successfully.", "success");
          }}
          initialData={{ pcscfAddresses: addresses }}
        />
      )}
    </Box>
  );
};

export default VoiceSection;
