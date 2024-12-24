"use client";

import React, { useState, useEffect } from "react";
import { Button, Box, TextField, Alert, Typography } from "@mui/material";
import { updateNetwork, getNetwork } from "@/queries/network";
import { useQuery, useMutation } from "@tanstack/react-query";
import * as yup from "yup";
import { ValidationError } from "yup";

const NetworkConfiguration = () => {
  const { data: network } = useQuery({
    queryKey: ["network"],
    queryFn: getNetwork,
  });

  const [mcc, setMcc] = useState("");
  const [mnc, setMnc] = useState("");
  const [mccError, setMccError] = useState("");
  const [mncError, setMncError] = useState("");
  const [isSaveDisabled, setSaveDisabled] = useState(true);
  const [alert, setAlert] = useState<{ message: string; severity: "success" | "error" | null }>({
    message: "",
    severity: null,
  });

  const schema = yup.object().shape({
    mcc: yup.string().matches(/^\d{3}$/, "MCC must be a 3 decimal digit").required("MCC is required"),
    mnc: yup
      .string()
      .matches(/^\d{2,3}$/, "MNC must be a 2 or 3 decimal digit")
      .required("MNC is required"),
  });

  const validate = async () => {
    try {
      await schema.validate({ mcc, mnc }, { abortEarly: false });
      setMccError("");
      setMncError("");
      setSaveDisabled(false);
    } catch (err) {
      if (err instanceof ValidationError) {
        const errors = err.inner.reduce(
          (acc: any, curr: yup.ValidationError) => ({ ...acc, [curr.path!]: curr.message }),
          {}
        );
        setMccError(errors.mcc || "");
        setMncError(errors.mnc || "");
        setSaveDisabled(true);
      }
    }
  };

  const mutation = useMutation({
    mutationFn: (variables: { mcc: string; mnc: string }) =>
      updateNetwork(variables.mcc, variables.mnc),
    onSuccess: () => {
      setAlert({ message: "Network configuration updated successfully!", severity: "success" });
    },
    onError: (error: any) => {
      console.error("Error updating network configuration:", error);
      setAlert({
        message: `Failed to update network configuration: ${error.message}`,
        severity: "error",
      });
    },
  });

  useEffect(() => {
    if (network) {
      setMcc(network.mcc);
      setMnc(network.mnc);
    }
  }, [network]);

  useEffect(() => {
    validate();
  }, [mcc, mnc]);

  const handleSave = () => {
    mutation.mutate({ mcc, mnc });
  };

  return (
    <Box
      sx={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        justifyContent: "flex-start",
        alignItems: "center",
        paddingTop: 6,
        textAlign: "center",
      }}
    >
      <Box sx={{ marginBottom: 4 }}>
        <Typography variant="h4" component="h1" gutterBottom>
          Network Configuration
        </Typography>
      </Box>
      <Box sx={{ maxWidth: 400, width: "100%" }}>
        {alert.severity && (
          <Alert
            severity={alert.severity}
            onClose={() => setAlert({ message: "", severity: null })}
            sx={{ marginBottom: 2 }}
          >
            {alert.message}
          </Alert>
        )}
        <TextField
          fullWidth
          id="mcc"
          label="Mobile Country Code (MCC)"
          value={mcc}
          onChange={(e) => setMcc(e.target.value)}
          variant="outlined"
          margin="normal"
          error={!!mccError}
          helperText={mccError}
        />
        <TextField
          fullWidth
          id="mnc"
          label="Mobile Network Code (MNC)"
          value={mnc}
          onChange={(e) => setMnc(e.target.value)}
          variant="outlined"
          margin="normal"
          error={!!mncError}
          helperText={mncError}
        />
        <Button
          fullWidth
          variant="contained"
          color="success"
          onClick={handleSave}
          disabled={isSaveDisabled}
          sx={{ marginTop: 2 }}
        >
          Save
        </Button>
      </Box>
    </Box>
  );
};

export default NetworkConfiguration;
