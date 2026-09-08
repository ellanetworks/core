// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { logoAlt } from "@/utils/product";

export default function Logo({
  width = 50,
  height = 50,
}: {
  width?: number;
  height?: number;
}) {
  return <img src="/logo.svg" alt={logoAlt()} width={width} height={height} />;
}
