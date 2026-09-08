// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export interface Tokens {
  primary: string;
  success: string;
  error: string;
  warning: string;
  info: string;
  link: string;
  backgroundDefault: string;
  backgroundPaper: string;
  backgroundSubtle: string;
  textPrimary: string;
  textSecondary: string;
  borderControl: string;
  chart: {
    uplink: string;
    downlink: string;
    series: string[];
    protocols: Record<number, string>;
  };
}

export const light: Tokens = {
  primary: "#26374A",
  success: "#1B6C1C",
  error: "#C62828",
  warning: "#ED6C02",
  info: "#037EAA",
  link: "#2B3FD4",
  backgroundDefault: "#FFFFFF",
  backgroundPaper: "#FFFFFF",
  backgroundSubtle: "#F5F5F5",
  textPrimary: "rgba(0, 0, 0, 0.87)",
  textSecondary: "rgba(0, 0, 0, 0.6)",
  borderControl: "rgba(0, 0, 0, 0.42)",
  chart: {
    uplink: "#FF9800",
    downlink: "#4254FB",
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
  primary: "#5B9DFF",
  success: "#4ABF4B",
  error: "#EC9393",
  warning: "#F09142",
  info: "#19AFE6",
  link: "#97A3F7",
  backgroundDefault: "#14161B",
  backgroundPaper: "#1C2027",
  backgroundSubtle: "#2E333F",
  textPrimary: "#E3E5EA",
  textSecondary: "rgba(227, 229, 234, 0.66)",
  borderControl: "rgba(255, 255, 255, 0.36)",
  chart: {
    uplink: "#D99126",
    downlink: "#7983E7",
    series: [
      "#3893DC",
      "#4CAE50",
      "#D99126",
      "#E35F93",
      "#CA60DC",
      "#1FA1B2",
      "#DE6945",
      "#AB8172",
      "#718F9D",
      "#89C247",
      "#7B88D1",
      "#CDDC38",
    ],
    protocols: {
      1: "#D99126",
      6: "#3893DC",
      17: "#4CAE50",
      47: "#CA60DC",
      58: "#E35F93",
      132: "#1FA1B2",
    },
  },
};
