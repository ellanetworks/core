// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import * as yup from "yup";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";

export const SST_HELPER_TEXT = "Slice/Service Type (0–255)";
export const SD_HELPER_TEXT =
  "Slice Differentiator — 6 hex digits (e.g. 010203)";

export const sliceIdentitySchema = {
  sst: yup
    .number()
    .min(0, "SST must be between 0 and 255")
    .max(255, "SST must be between 0 and 255")
    .required("SST is required"),
  sd: yup
    .string()
    .matches(/^([0-9a-fA-F]{6})?$/, "SD must be a 6-digit hex string")
    .default(""),
};

export const SliceIdentityFields = ({
  autoFocusSst = false,
}: {
  autoFocusSst?: boolean;
}) => (
  <>
    <NumberControl
      name="sst"
      label="SST"
      min={0}
      max={255}
      helperText={SST_HELPER_TEXT}
      autoFocus={autoFocusSst}
    />
    <TextControl name="sd" label="SD (optional)" helperText={SD_HELPER_TEXT} />
  </>
);
