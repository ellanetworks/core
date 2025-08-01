import React, { useState, useEffect } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  FormControl,
  Select,
  InputLabel,
  Button,
  MenuItem,
  Alert,
  Collapse,
} from "@mui/material";
import { updatePolicy } from "@/queries/policies";
import { listDataNetworks } from "@/queries/data_networks";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";
import { Policy, DataNetwork } from "@/types/types";

interface EditPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: Policy;
}

type FormState = Omit<Policy, "bitrateUp" | "bitrateDown"> & {
  bitrateUpValue: number;
  bitrateUpUnit: "Mbps" | "Gbps";
  bitrateDownValue: number;
  bitrateDownUnit: "Mbps" | "Gbps";
};

const EditPolicyModal: React.FC<EditPolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const router = useRouter();
  const [cookies, ,] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }
  const [formValues, setFormValues] = useState<FormState>({
    name: "",
    bitrateUpValue: 0,
    bitrateUpUnit: "Mbps",
    bitrateDownValue: 0,
    bitrateDownUnit: "Mbps",
    fiveQi: 0,
    priorityLevel: 0,
    dataNetworkName: "",
  });

  const [dataNetworks, setDataNetworks] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    const fetchDataNetworks = async () => {
      try {
        const policyData = await listDataNetworks(cookies.user_token);
        setDataNetworks(
          policyData.map((dataNetwork: DataNetwork) => dataNetwork.name),
        );
      } catch (error) {
        console.error("Failed to fetch data networks:", error);
      }
    };

    if (open) {
      fetchDataNetworks();
      const [bitrateUpValueStr, bitrateUpUnit] =
        initialData.bitrateUp.split(" ");
      const [bitrateDownValueStr, bitrateDownUnit] =
        initialData.bitrateDown.split(" ");

      setFormValues({
        name: initialData.name,
        bitrateUpValue: parseInt(bitrateUpValueStr, 10),
        bitrateUpUnit: bitrateUpUnit as "Mbps" | "Gbps",
        bitrateDownValue: parseInt(bitrateDownValueStr, 10),
        bitrateDownUnit: bitrateDownUnit as "Mbps" | "Gbps",
        fiveQi: initialData.fiveQi,
        priorityLevel: initialData.priorityLevel,
        dataNetworkName: initialData.dataNetworkName,
      });

      setErrors({});
    }
  }, [open, initialData, cookies.user_token]);

  useEffect(() => {
    if (open) {
      const [bitrateUpValueStr, bitrateUpUnit] =
        initialData.bitrateUp.split(" ");
      const [bitrateDownValueStr, bitrateDownUnit] =
        initialData.bitrateDown.split(" ");

      setFormValues({
        name: initialData.name,
        bitrateUpValue: parseInt(bitrateUpValueStr, 10),
        bitrateUpUnit: bitrateUpUnit as "Mbps" | "Gbps",
        bitrateDownValue: parseInt(bitrateDownValueStr, 10),
        bitrateDownUnit: bitrateDownUnit as "Mbps" | "Gbps",
        fiveQi: initialData.fiveQi,
        priorityLevel: initialData.priorityLevel,
        dataNetworkName: initialData.dataNetworkName,
      });
      setErrors({});
    }
  }, [open, initialData]);

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleSubmit = async () => {
    setLoading(true);
    setAlert({ message: "" });

    const bitrateUp = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
    const bitrateDown = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;

    try {
      await updatePolicy(
        cookies.user_token,
        formValues.name,
        bitrateUp,
        bitrateDown,
        formValues.fiveQi,
        formValues.priorityLevel,
        formValues.dataNetworkName,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update policy: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-policy-modal-title"
      aria-describedby="edit-policy-modal-description"
    >
      <DialogTitle>Edit Policy</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
            severity="error"
          >
            {alert.message}
          </Alert>
        </Collapse>
        <TextField
          fullWidth
          label="Name"
          value={formValues.name}
          margin="normal"
          disabled
        />
        <FormControl fullWidth margin="normal">
          <InputLabel id="demo-simple-select-label">Policy Name</InputLabel>
          <Select
            value={formValues.dataNetworkName}
            onChange={(e) => handleChange("dataNetworkName", e.target.value)}
            error={!!errors.policyName}
            label={"Data Network Name"}
            labelId="demo-simple-select-label"
          >
            {dataNetworks.map((dataNetwork) => (
              <MenuItem key={dataNetwork} value={dataNetwork}>
                {dataNetwork}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <Box display="flex" gap={2}>
          <TextField
            label="Bitrate Up Value"
            type="number"
            value={formValues.bitrateUpValue}
            onChange={(e) =>
              handleChange("bitrateUpValue", Number(e.target.value))
            }
            error={!!errors.bitrateUpValue}
            helperText={errors.bitrateUpValue}
            margin="normal"
          />
          <TextField
            select
            label="Unit"
            value={formValues.bitrateUpUnit}
            onChange={(e) => handleChange("bitrateUpUnit", e.target.value)}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>
        <Box display="flex" gap={2}>
          <TextField
            label="Bitrate Down Value"
            type="number"
            value={formValues.bitrateDownValue}
            onChange={(e) =>
              handleChange("bitrateDownValue", Number(e.target.value))
            }
            error={!!errors.bitrateDownValue}
            helperText={errors.bitrateDownValue}
            margin="normal"
          />
          <TextField
            select
            label="Unit"
            value={formValues.bitrateDownUnit}
            onChange={(e) => handleChange("bitrateDownUnit", e.target.value)}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>
        <TextField
          fullWidth
          label="5QI"
          type="number"
          value={formValues.fiveQi}
          onChange={(e) => handleChange("fiveQi", Number(e.target.value))}
          error={!!errors.fiveQi}
          helperText={errors.fiveQi}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Priority Level"
          type="number"
          value={formValues.priorityLevel}
          onChange={(e) =>
            handleChange("priorityLevel", Number(e.target.value))
          }
          error={!!errors.priorityLevel}
          helperText={errors.priorityLevel}
          margin="normal"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={loading}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditPolicyModal;
