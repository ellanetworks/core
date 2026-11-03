// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { RoamerRegistration, RoamerRAT } from "@/queries/roaming";

export type RoamerState = {
  label: string;
  color: "success" | "default" | "warning";
  rank: number;
};

export const registrationState = (reg: RoamerRegistration): RoamerState => {
  if (!reg.reachable) return { label: "Unknown", color: "warning", rank: 0 };
  if (reg.registered === false) {
    return { label: "Registering", color: "default", rank: 1 };
  }
  if (reg.connectionState === "connected") {
    return { label: "Connected", color: "success", rank: 3 };
  }
  if (reg.connectionState === "idle") {
    return { label: "Idle", color: "default", rank: 2 };
  }
  return { label: "Registered", color: "default", rank: 2 };
};

export const roamerState = (regs: RoamerRegistration[]): RoamerState =>
  regs
    .map(registrationState)
    .reduce<RoamerState>(
      (best, state) => (state.rank > best.rank ? state : best),
      { label: "Unknown", color: "warning", rank: -1 },
    );

export const roamerRATs = (regs: RoamerRegistration[]): RoamerRAT[] =>
  [...new Set(regs.map((reg) => reg.rat))].sort();
