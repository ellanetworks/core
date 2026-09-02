// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NGAPMessageView } from "./NGAPMessageRender";
import type { DecodedNGAPMessage } from "@/queries/radio_events";

const rawHex = "ab".repeat(1358);

const withRadioCapability = (decoded: unknown): DecodedNGAPMessage =>
  ({
    pdu_type: "InitiatingMessage",
    procedure_code: {
      label: "UERadioCapabilityInfoIndication",
      value: 44,
      type: "enum",
    },
    criticality: { label: "ignore", value: 1, type: "enum" },
    value: {
      ies: [
        {
          id: { label: "UERadioCapability", value: 117, type: "enum" },
          criticality: { label: "ignore", value: 1, type: "enum" },
          value: { protocol: "RRC", raw_hex: rawHex, decoded },
        },
      ],
    },
  }) as unknown as DecodedNGAPMessage;

describe("NGAPMessageView RRC container", () => {
  it("never lets the IE label shrink into its summary", () => {
    render(
      <NGAPMessageView
        decoded={withRadioCapability({
          summary: {
            nr: { access_stratum_release: "rel16", bands: [{ band: 77 }] },
          },
        })}
      />,
    );

    const label = screen.getByText(/UERadioCapability \(117\)/);
    expect(label).toHaveStyle({ flexShrink: "0", whiteSpace: "pre" });
  });

  it("caps a long band list so the header stays one glanceable summary", () => {
    render(
      <NGAPMessageView
        decoded={withRadioCapability({
          summary: {
            eutra: {
              access_stratum_release: "rel16",
              ue_category: 4,
              bands: [2, 4, 5, 7, 12, 13, 25, 29, 30, 41, 1, 3].map((band) => ({
                band,
              })),
            },
          },
        })}
      />,
    );

    expect(
      screen.getByText(/B2, B4, B5, B7, B12, B13 \+6 more/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/B41/)).not.toBeInTheDocument();
  });

  it("summarises the decoded capability in the collapsed header", () => {
    render(
      <NGAPMessageView
        decoded={withRadioCapability({
          summary: {
            nr: { access_stratum_release: "rel16", bands: [{ band: 77 }] },
            eutra: {
              access_stratum_release: "rel16",
              ue_category: 4,
              bands: [{ band: 2 }, { band: 5 }],
            },
          },
        })}
      />,
    );

    expect(screen.getByText(/NR rel16/)).toBeInTheDocument();
    expect(screen.getByText(/n77/)).toBeInTheDocument();
    expect(screen.getByText(/E-UTRA rel16 cat4/)).toBeInTheDocument();
  });

  it("reports a decode failure instead of claiming a capability", () => {
    render(
      <NGAPMessageView
        decoded={withRadioCapability({ error: "UE-NR-Capability: truncated" })}
      />,
    );

    expect(screen.getByText("decode error")).toBeInTheDocument();
  });

  it("keeps the raw hex to one line and copies the whole value", async () => {
    const user = userEvent.setup();

    render(
      <NGAPMessageView
        decoded={withRadioCapability({ summary: { nr: {} } })}
      />,
    );

    await user.click(screen.getByLabelText("Expand"));

    const hex = screen.getByText(rawHex);
    expect(hex).toHaveStyle({ whiteSpace: "nowrap", overflow: "hidden" });
    expect(screen.getByText(/1358 bytes/)).toBeInTheDocument();

    await user.click(screen.getByLabelText("Copy raw hex"));

    await waitFor(async () =>
      expect(await navigator.clipboard.readText()).toBe(rawHex),
    );
  });

  it("truncates a raw_hex nested anywhere in the tree, not just on a PDU block", () => {
    const containerHex = "2e02d4c211000901000631310101ff01".repeat(6);

    render(
      <NGAPMessageView
        decoded={
          {
            pdu_type: "InitiatingMessage",
            procedure_code: {
              label: "UplinkNASTransport",
              value: 46,
              type: "enum",
            },
            criticality: { label: "ignore", value: 1, type: "enum" },
            value: {
              ies: [
                {
                  id: { label: "NAS-PDU", value: 38, type: "enum" },
                  criticality: { label: "reject", value: 0, type: "enum" },
                  value: {
                    protocol: "NAS",
                    raw_hex: "7e00",
                    decoded: {
                      gmm_message: {
                        ul_nas_transport: {
                          payload_container: { raw_hex: containerHex },
                        },
                      },
                    },
                  },
                },
              ],
            },
          } as unknown as DecodedNGAPMessage
        }
      />,
    );

    const hex = screen.getByText(containerHex);
    expect(hex).toHaveStyle({ whiteSpace: "nowrap", overflow: "hidden" });
    expect(screen.getByText(/96 bytes/)).toBeInTheDocument();
    expect(screen.getAllByLabelText("Copy raw hex").length).toBeGreaterThan(1);
  });

  it("says byte, not bytes, for a single octet", () => {
    render(
      <NGAPMessageView
        decoded={withRadioCapability({ summary: { nr: {} } })}
      />,
    );
    expect(screen.queryByText(/ 1 bytes/)).not.toBeInTheDocument();
  });
});
