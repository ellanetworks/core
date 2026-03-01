import React from "react";
import { Alert, AlertTitle } from "@mui/material";
import { useFleet } from "@/contexts/FleetContext";

const FleetBanner: React.FC = () => {
  const { isFleetManaged } = useFleet();

  if (!isFleetManaged) return null;

  return (
    <Alert
      severity="info"
      variant="filled"
      sx={{
        borderRadius: 0,
        mb: 0,
        "& .MuiAlert-message": { width: "100%" },
      }}
    >
      <AlertTitle sx={{ fontWeight: 700 }}>Fleet Managed</AlertTitle>
      This Ella Core instance is managed by Ella Fleet. Configuration changes
      must be made through Fleet.
    </Alert>
  );
};

export default FleetBanner;
