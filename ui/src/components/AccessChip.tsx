// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Box, Chip } from "@mui/material";
import { visuallyHidden } from "@mui/utils";

/**
 * Dimming is the only visual cue, so an inactive chip also carries the state as
 * text for assistive technology: without it the label reads the same either way
 * and a permitted RAT is indistinguishable from a forbidden one.
 */
const AccessChip: React.FC<{ label: string; active?: boolean }> = ({
  label,
  active = true,
}) => (
  <Chip
    size="small"
    label={
      active ? (
        label
      ) : (
        <>
          {label}
          <Box component="span" sx={visuallyHidden}>
            {" "}
            (not permitted)
          </Box>
        </>
      )
    }
    color="default"
    variant={active ? "filled" : "outlined"}
    sx={active ? undefined : { opacity: 0.5 }}
  />
);

export default AccessChip;
