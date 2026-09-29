// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { logoAlt, logoHeight } from "@/utils/product";

export default function Logo() {
  return (
    <img
      src={BRANDING.logoUrl}
      alt={logoAlt()}
      height={logoHeight()}
      style={{ width: "auto", maxWidth: 220, objectFit: "contain" }}
    />
  );
}
