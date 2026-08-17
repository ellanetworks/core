// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box } from "@mui/material";
import { formatTacDecimal } from "@/utils/tac";

const TacValue: React.FC<{ tac: string }> = ({ tac }) => {
  const decimal = formatTacDecimal(tac);

  return (
    <>
      {tac}
      {decimal !== null && (
        <Box component="span" sx={{ opacity: 0.6, ml: 0.5 }}>
          ({decimal})
        </Box>
      )}
    </>
  );
};

export default TacValue;
