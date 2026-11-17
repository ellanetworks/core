// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useId } from "react";
import { Box, Checkbox, FormControlLabel, FormHelperText } from "@mui/material";

interface PinRoleFieldProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  required?: boolean;
}

const PinRoleField: React.FC<PinRoleFieldProps> = ({
  checked,
  onChange,
  required = false,
}) => {
  const helpId = useId();
  return (
    <Box sx={{ mt: 1 }}>
      <FormControlLabel
        control={
          <Checkbox
            checked={checked}
            disabled={required}
            onChange={(e) => onChange(e.target.checked)}
            slotProps={{ input: { "aria-describedby": helpId } }}
          />
        }
        label="Pin role"
      />
      <FormHelperText id={helpId} sx={{ mt: 0 }}>
        {required
          ? "Pinned, or groups would replace it at the next login."
          : "A pinned role is not updated from the user's groups."}
      </FormHelperText>
    </Box>
  );
};

export default PinRoleField;
