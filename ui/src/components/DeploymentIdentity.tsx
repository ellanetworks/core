// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Typography } from "@mui/material";
import { useStatusQuery } from "@/hooks/useStatus";
import { useDiameterStatus } from "@/hooks/useDiameterStatus";
import { nodeLabel } from "@/queries/nodeId";

const DeploymentIdentity: React.FC<{ compact?: boolean }> = ({
  compact = false,
}) => {
  const { data: status } = useStatusQuery();
  const { data: diameter } = useDiameterStatus(true, false);
  const originHost = diameter?.nodes.find((n) => n.local)?.originHost;

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

  const label = nodeLabel(status.cluster.displayName, status.cluster.nodeId);

  if (!label) {
    return (
      <Typography variant="body2" noWrap sx={{ opacity: 0.8 }}>
        {version}
      </Typography>
    );
  }

  return (
    <Typography
      variant="body2"
      noWrap
      title={originHost ? `Origin-Host: ${originHost}` : undefined}
      sx={{ opacity: 0.8 }}
    >
      {`${version} · Node ${label}`}
    </Typography>
  );
};

export default DeploymentIdentity;
