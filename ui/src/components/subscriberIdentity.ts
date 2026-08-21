// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const MCC_DIGITS = 3;
export const IMSI_MIN_DIGITS = 6;
export const IMSI_MAX_DIGITS = 15;

export const getMSINBounds = (mncLength: number) => {
  const prefixLength = MCC_DIGITS + mncLength;
  return {
    min: Math.max(1, IMSI_MIN_DIGITS - prefixLength),
    max: Math.max(1, IMSI_MAX_DIGITS - prefixLength),
  };
};

const sanitizeDigits = (value: string) => value.replace(/\D/g, "");

export const parseIMSIorMSIN = (
  raw: string,
  operatorMcc: string,
  operatorMnc: string,
): { msin: string | null; mismatchMsg: string | null } => {
  const digits = sanitizeDigits(raw);
  const prefix = `${operatorMcc}${operatorMnc}`;
  const { max } = getMSINBounds(operatorMnc.length);

  if (digits.length <= max) {
    return { msin: digits, mismatchMsg: null };
  }

  if (digits.startsWith(prefix)) {
    return {
      msin: digits.slice(prefix.length, prefix.length + max),
      mismatchMsg: null,
    };
  }

  return {
    msin: null,
    mismatchMsg: `IMSI prefix does not match MCC ${operatorMcc} / MNC ${operatorMnc}.`,
  };
};

export const randomMSIN = (mncLength: number) => {
  const { max } = getMSINBounds(mncLength);
  return Array.from({ length: max }, () => Math.floor(Math.random() * 10)).join(
    "",
  );
};

export const randomKey = () =>
  [...crypto.getRandomValues(new Uint8Array(16))]
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
