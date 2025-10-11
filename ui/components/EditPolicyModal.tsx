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
import { updatePolicy, APIPolicy } from "@/queries/policies";
import {
  listDataNetworks,
  type ListDataNetworksResponse,
} from "@/queries/data_networks";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIPolicy;
}

type FormState = Omit<APIPolicy, "bitrate_uplink" | "bitrate_downlink"> & {
  bitrateUpValue: number;
  bitrateUpUnit: "Mbps" | "Gbps";
  bitrateDownValue: number;
  bitrateDownUnit: "Mbps" | "Gbps";
};

const PER_PAGE = 12;

const NON_GBR_5QI_OPTIONS = [5, 6, 7, 8, 9, 69, 70, 79, 80];

const EditPolicyModal: React.FC<EditPolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (open && authReady && !accessToken) {
      router.push("/login");
    }
  }, [open, authReady, accessToken, router]);

  const [formValues, setFormValues] = useState<FormState>({
    name: "",
    bitrateUpValue: 0,
    bitrateUpUnit: "Mbps",
    bitrateDownValue: 0,
    bitrateDownUnit: "Mbps",
    var5qi: 0,
    arp: 0,
    data_network_name: "",
  });

  const [dataNetworks, setDataNetworks] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (!open) return;
    const [bitrateUpValueStr, bitrateUpUnit] =
      initialData.bitrate_uplink.split(" ");
    const [bitrateDownValueStr, bitrateDownUnit] =
      initialData.bitrate_downlink.split(" ");

    setFormValues({
      name: initialData.name,
      bitrateUpValue: parseInt(bitrateUpValueStr, 10),
      bitrateUpUnit: (bitrateUpUnit as "Mbps" | "Gbps") ?? "Mbps",
      bitrateDownValue: parseInt(bitrateDownValueStr, 10),
      bitrateDownUnit: (bitrateDownUnit as "Mbps" | "Gbps") ?? "Mbps",
      var5qi: initialData.var5qi,
      arp: initialData.arp,
      data_network_name: initialData.data_network_name,
    });
    setErrors({});
  }, [open, initialData]);

  useEffect(() => {
    const fetchDataNetworks = async () => {
      if (!open || !accessToken) return;
      try {
        const res: ListDataNetworksResponse = await listDataNetworks(
          accessToken,
          1,
          PER_PAGE,
        );
        setDataNetworks((res.items ?? []).map((dn) => dn.name));
      } catch (error) {
        console.error("Failed to fetch data networks:", error);
      }
    };
    fetchDataNetworks();
  }, [open, accessToken]);

  const handleChange = (field: keyof FormState, value: string | number) => {
    setFormValues((prev) => ({ ...prev, [field]: value as never }));
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    const bitrateUp = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
    const bitrateDown = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;

    try {
      await updatePolicy(
        accessToken,
        formValues.name,
        bitrateUp,
        bitrateDown,
        formValues.var5qi,
        formValues.arp,
        formValues.data_network_name,
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

  const fiveQiIsAllowed = NON_GBR_5QI_OPTIONS.includes(formValues.var5qi);

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
          <InputLabel id="data-network-select-label">
            Data Network Name
          </InputLabel>
          <Select
            labelId="data-network-select-label"
            label="Data Network Name"
            value={formValues.data_network_name}
            onChange={(e) => handleChange("data_network_name", e.target.value)}
            error={!!errors.data_network_name}
          >
            {dataNetworks.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
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

        <FormControl fullWidth margin="normal">
          <InputLabel id="fiveqi-edit-select-label">5QI (non-GBR)</InputLabel>
          <Select
            labelId="fiveqi-edit-select-label"
            label="5QI (non-GBR)"
            value={formValues.var5qi}
            onChange={(e) => handleChange("var5qi", Number(e.target.value))}
            error={!!errors.var5qi}
          >
            {!fiveQiIsAllowed && (
              <MenuItem value={formValues.var5qi} disabled>
                {formValues.var5qi} (current, unsupported)
              </MenuItem>
            )}
            {NON_GBR_5QI_OPTIONS.map((val) => (
              <MenuItem key={val} value={val}>
                {val}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <TextField
          fullWidth
          label="Allocation and Retention Priority (ARP)"
          type="number"
          value={formValues.arp}
          onChange={(e) => handleChange("arp", Number(e.target.value))}
          error={!!errors.arp}
          helperText={errors.arp}
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
