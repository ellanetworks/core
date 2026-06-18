// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useContext } from "react";
import { AuthContext } from "@/contexts/AuthContext";

export const useAuth = () => useContext(AuthContext);
