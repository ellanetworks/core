"use client";

import React, { useState, useEffect } from "react";
import { Button, Box, TextField, Alert, Typography } from "@mui/material";
import { updateNetwork, getNetwork } from "@/queries/network";
import { useMutation } from "@tanstack/react-query";
import * as yup from "yup";
import { ValidationError } from "yup";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"
import Grid from "@mui/material/Grid2";


const NetworkConfiguration = () => {
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

  if (!cookies.user_token) {
    router.push("/login")
  }

  const [network, setNetwork] = useState<{ mcc: string; mnc: string } | null>(null);

  const fetchNetwork = async () => {
    try {
      const data = await getNetwork(cookies.user_token);
      setNetwork(data);
    } catch (error) {
      console.error("Error fetching network:", error);
    } finally {
    }
  };

  useEffect(() => {
    fetchNetwork();
  }, []);

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
      updateNetwork(cookies.user_token, variables.mcc, variables.mnc),
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
    <Box sx={{ padding: 4, maxWidth: "1200px", margin: "0 auto" }}>
      <Typography
        variant="h4"
        component="h1"
        gutterBottom
        sx={{ textAlign: "left", marginBottom: 4 }}
      >
        Network Configuration
      </Typography>

      {alert.severity && (
        <Alert
          severity={alert.severity}
          onClose={() => setAlert({ message: "", severity: null })}
        >
          {alert.message}
        </Alert>
      )}

      <Grid container spacing={4} justifyContent="flex-start">
        <Grid size={4}>
          <Box
            sx={{
              border: "1px solid #ccc",
              borderRadius: 4,
              padding: 4,
              width: "100%",
              margin: "0 auto",
              textAlign: "center",
            }}
          >
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
        </Grid>
      </Grid>
    </Box>
  );
};

export default NetworkConfiguration;
