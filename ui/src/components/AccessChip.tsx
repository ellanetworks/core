// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Box, Chip } from "@mui/material";
import { visuallyHidden } from "@mui/utils";

/**
 * AccessChip renders a neutral radio-access-technology tag (e.g. "4G" / "5G").
 * Technologies aren't a success/error state, so the chip is colour-neutral:
 * `active` (the default) renders it solid; an inactive RAT — e.g. one a profile
 * does not permit — is shown outlined and dimmed. Used everywhere a RAT is shown
 * (profile access, subscriber access type) so the styling stays consistent.
 *
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
