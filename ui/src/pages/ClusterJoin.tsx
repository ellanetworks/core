// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Box, CircularProgress, Typography } from "@mui/material";
import PageTitle from "@/components/PageTitle";
import ClusterSetupCard from "@/components/ClusterSetupCard";
import { getStatus, type APIStatus } from "@/queries/status";
import {
  getClusterJoinStatus,
  type ClusterJoinStatus,
} from "@/queries/cluster";

const CLUSTER_JOIN_DESCRIPTION =
  "Join this node to an existing high-availability cluster.";

const ClusterJoinPage: React.FC = () => {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const backTo = pathname.startsWith("/initialize")
    ? "/initialize"
    : "/cluster";

  const statusQuery = useQuery<APIStatus>({
    queryKey: ["status"],
    queryFn: getStatus,
    refetchInterval: 5000,
  });

  const joinQuery = useQuery<ClusterJoinStatus>({
    queryKey: ["cluster-join-status"],
    queryFn: getClusterJoinStatus,
    enabled: statusQuery.isSuccess,
    refetchInterval: (query) => {
      const state = query.state.data?.state;
      return state === "waiting" || state === "joining" ? 2000 : false;
    },
  });

  const joinState = joinQuery.data?.state ?? "unavailable";
  const accepting = joinState === "waiting" || joinState === "joining";

  if (!statusQuery.isSuccess || !joinQuery.isSuccess) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!accepting) {
    return <Navigate to={backTo} replace />;
  }

  return (
    <Box
      sx={{
        flexGrow: 1,
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        p: 2,
      }}
    >
      <Box
        sx={{
          width: "100%",
          maxWidth: 360,
          display: "flex",
          flexDirection: "column",
          gap: 2,
          border: "1px solid",
          borderColor: "divider",
          borderRadius: 2,
          p: 3,
          boxShadow: 2,
        }}
      >
        <PageTitle
          title="Join a cluster"
          size="h5"
          gutterBottom
          sx={{ textAlign: "center" }}
        />
        <Typography variant="body1">{CLUSTER_JOIN_DESCRIPTION}</Typography>

        <ClusterSetupCard
          joinOnly
          state={joinState}
          failure={joinQuery.data?.error}
          onSubmitted={() => navigate(backTo)}
          onCancel={() => navigate(backTo)}
        />
      </Box>
    </Box>
  );
};

export default ClusterJoinPage;
