// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Typography, Link as MuiLink } from "@mui/material";
import { Link } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { useStatusQuery } from "@/hooks/useStatus";
import { nodeLabel } from "@/queries/nodeId";

const DeploymentIdentity: React.FC<{ compact?: boolean }> = ({
  compact = false,
}) => {
  const { role } = useAuth();
  const { data: status } = useStatusQuery();

  if (!status) {
    return null;
  }

  const version = status.version ?? "—";

  if (compact) {
    return (
      <Typography variant="body2" noWrap sx={{ opacity: 0.8 }}>
        {version}
      </Typography>
    );
  }

  const mode = status.cluster.enabled
    ? `Cluster node ${nodeLabel(status.cluster.displayName, status.cluster.nodeId)}`
    : "Standalone";

  return (
    <Typography variant="body2" noWrap sx={{ opacity: 0.8 }}>
      {version}
      {" · "}
      {role === "Admin" ? (
        <MuiLink
          component={Link}
          to="/cluster"
          color="inherit"
          underline="hover"
        >
          {mode}
        </MuiLink>
      ) : (
        mode
      )}
    </Typography>
  );
};

export default DeploymentIdentity;
