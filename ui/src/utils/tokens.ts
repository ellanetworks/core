// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export interface Tokens {
  primary: string;
  success: string;
  error: string;
  warning: string;
  link: string;
  backgroundSubtle: string;
  chart: {
    uplink: string;
    downlink: string;
    protocolText: string;
    series: string[];
    protocols: Record<number, string>;
  };
}

export const light: Tokens = {
  primary: "#26374A",
  success: "#1B6C1C",
  error: "#C62828",
  warning: "#ED6C02",
  link: "#2B3FD4",
  backgroundSubtle: "#F5F5F5",
  chart: {
    uplink: "#FF9800",
    downlink: "#4254FB",
    protocolText: "#FFFFFF",
    series: [
      "#2196F3",
      "#4CAF50",
      "#FF9800",
      "#C2185B",
      "#9C27B0",
      "#00BCD4",
      "#FF5722",
      "#795548",
      "#546E7A",
      "#8BC34A",
      "#3F51B5",
      "#CDDC39",
    ],
    protocols: {
      1: "#FF9800",
      6: "#2196F3",
      17: "#4CAF50",
      47: "#9C27B0",
      58: "#C2185B",
      132: "#00BCD4",
    },
  },
};

export const dark: Tokens = {
  primary: "#89ABD2",
  success: "#4ABF4B",
  error: "#EC9393",
  warning: "#F09142",
  link: "#97A3F7",
  backgroundSubtle: "#1E1E1E",
  chart: {
    uplink: "#D99126",
    downlink: "#6C77E5",
    protocolText: "#0B0B0B",
    series: [
      "#3893DC",
      "#4CAE50",
      "#D99126",
      "#DE4581",
      "#C34BD7",
      "#1FA1B2",
      "#DD6640",
      "#A27362",
      "#648391",
      "#89C247",
      "#6B7ACC",
      "#CDDC38",
    ],
    protocols: {
      1: "#D99126",
      6: "#3893DC",
      17: "#4CAE50",
      47: "#C34BD7",
      58: "#DE4581",
      132: "#1FA1B2",
    },
  },
};
