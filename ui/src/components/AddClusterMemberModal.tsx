import React, { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  Collapse,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  MenuItem,
  TextField,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { addClusterMember } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";

const schema = yup.object().shape({
  nodeId: yup
    .number()
    .typeError("Node ID must be a number")
    .required("Node ID is required")
    .min(1, "Node ID must be between 1 and 63")
    .max(63, "Node ID must be between 1 and 63"),
  raftAddress: yup
    .string()
    .required("Cluster address is required")
    .matches(/^[^\s:]+:\d+$/, "Must be host:port"),
  apiAddress: yup
    .string()
    .required("API address is required")
    .min(1, "API address is required"),
  suffrage: yup.string().oneOf(["voter", "nonvoter"]).required(),
});

type FormValues = {
  nodeId: number;
  raftAddress: string;
  apiAddress: string;
  suffrage: "voter" | "nonvoter";
};

interface Props {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const AddClusterMemberModal: React.FC<Props> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const [values, setValues] = useState<FormValues>({
    nodeId: 2,
    raftAddress: "",
    apiAddress: "",
    suffrage: "nonvoter",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<string>("");

  const validate = useCallback(async () => {
    try {
      await schema.validate(values, { abortEarly: false });
      setErrors({});
      setIsValid(true);
    } catch (err) {
      if (err instanceof ValidationError) {
        setErrors(
          err.inner.reduce(
            (acc, cur) => {
              if (cur.path) acc[cur.path] = cur.message;
              return acc;
            },
            {} as Record<string, string>,
          ),
        );
      }
      setIsValid(false);
    }
  }, [values]);

  useEffect(() => {
    validate();
  }, [values, validate]);

  const setField = <K extends keyof FormValues>(field: K, v: FormValues[K]) => {
    setValues((prev) => ({ ...prev, [field]: v }));
  };

  const blur = (field: string) =>
    setTouched((prev) => ({ ...prev, [field]: true }));

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert("");
    try {
      await addClusterMember(accessToken, {
        nodeId: values.nodeId,
        raftAddress: values.raftAddress,
        apiAddress: values.apiAddress,
        suffrage: values.suffrage,
      });
      onClose();
      onSuccess();
    } catch (err) {
      setAlert(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Add Cluster Member</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert}>
          <Alert severity="error" onClose={() => setAlert("")} sx={{ mb: 2 }}>
            {alert}
          </Alert>
        </Collapse>

        <TextField
          fullWidth
          autoFocus
          label="Node ID"
          type="number"
          value={values.nodeId}
          onChange={(e) => setField("nodeId", Number(e.target.value))}
          onBlur={() => blur("nodeId")}
          error={!!errors.nodeId && touched.nodeId}
          helperText={
            touched.nodeId && errors.nodeId
              ? errors.nodeId
              : "Integer from 1 to 63. Must match the node's cluster certificate CN (ella-node-<n>)."
          }
          margin="normal"
        />

        <TextField
          fullWidth
          label="Cluster Address"
          value={values.raftAddress}
          onChange={(e) => setField("raftAddress", e.target.value)}
          onBlur={() => blur("raftAddress")}
          error={!!errors.raftAddress && touched.raftAddress}
          helperText={
            touched.raftAddress && errors.raftAddress
              ? errors.raftAddress
              : "host:port of the node's cluster port (same TCP socket carries Raft and cluster HTTP)."
          }
          placeholder="10.0.0.4:7000"
          margin="normal"
        />

        <TextField
          fullWidth
          label="API Address"
          value={values.apiAddress}
          onChange={(e) => setField("apiAddress", e.target.value)}
          onBlur={() => blur("apiAddress")}
          error={!!errors.apiAddress && touched.apiAddress}
          helperText={
            touched.apiAddress && errors.apiAddress
              ? errors.apiAddress
              : "HTTP URL used when proxying writes to this node (e.g. https://10.0.0.4:5000)."
          }
          placeholder="https://10.0.0.4:5000"
          margin="normal"
        />

        <TextField
          fullWidth
          select
          label="Suffrage"
          value={values.suffrage}
          onChange={(e) =>
            setField("suffrage", e.target.value as "voter" | "nonvoter")
          }
          helperText="Non-voter is recommended for new joiners during rolling upgrades; promote after it catches up."
          margin="normal"
        >
          <MenuItem value="nonvoter">nonvoter</MenuItem>
          <MenuItem value="voter">voter</MenuItem>
        </TextField>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!isValid || loading}
        >
          {loading ? "Adding…" : "Add"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default AddClusterMemberModal;
