// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

import { useContext } from "react";
import { AuthContext } from "@/contexts/AuthContext";

export const useAuth = () => useContext(AuthContext);
