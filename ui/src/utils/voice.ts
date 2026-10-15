// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const IMS_DATA_NETWORK = "ims";

export const IMS_SIGNALLING_5QI = 5;

export const isVoiceDataNetwork = (name: string | undefined) =>
  name === IMS_DATA_NETWORK;
