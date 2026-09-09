// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { mergeRegistrations, type Registration } from "@/queries/subscribers";

const older = "2026-08-17T10:00:00Z";
const newer = "2026-08-17T10:05:00Z";

const on5GS: Registration = {
  system: "5GS",
  access_type: "3GPP",
  registered: true,
  connection_state: "connected",
  radio: "gnb-1",
  last_seen_at: older,
  pei: "imeisv-4901542032375181",
  imei: "490154203237518",
  ciphering_algorithm: "128-NEA2",
  integrity_algorithm: "128-NIA2",
  connection: { amf_ue_ngap_id: 12, ran_ue_ngap_id: 39 },
};

const onEPS: Registration = {
  system: "EPS",
  access_type: "3GPP",
  registered: true,
  connection_state: "connected",
  radio: "enb-1",
  last_seen_at: older,
  imei: "490154203237518",
  ciphering_algorithm: "128-EEA2",
  integrity_algorithm: "128-EIA2",
  connection: { mme_ue_s1ap_id: 3, enb_ue_s1ap_id: 42 },
};

const at = (r: Registration, seen: string): Registration => ({
  ...r,
  last_seen_at: seen,
});

const idle = (r: Registration): Registration => ({
  ...r,
  connection_state: "idle",
  connection: null,
});

const deregistered = (r: Registration): Registration => ({
  system: r.system,
  access_type: r.access_type,
  registered: false,
  connection_state: null,
  radio: r.radio,
  last_seen_at: r.last_seen_at,
  connection: null,
});

describe("mergeRegistrations", () => {
  it("reports nothing for a subscriber the core has never served", () => {
    const merged = mergeRegistrations([]);

    expect(merged.registered).toBe(false);
    expect(merged.connection_state).toBeUndefined();
    expect(merged.radio_access_types).toEqual([]);
    expect(merged.imei).toBe("");
    expect(merged.last_seen_radio).toBeUndefined();
  });

  it("labels a 5GS registration 5G and an EPS one 4G", () => {
    expect(mergeRegistrations([on5GS]).radio_access_types).toEqual(["5G"]);
    expect(mergeRegistrations([onEPS]).radio_access_types).toEqual(["4G"]);
  });

  it("pairs the serving radio with that registration's algorithms", () => {
    const merged = mergeRegistrations([at(on5GS, older), at(onEPS, newer)]);

    expect(merged.last_seen_radio).toBe("enb-1");
    expect(merged.ciphering_algorithm).toBe("128-EEA2");
    expect(merged.integrity_algorithm).toBe("128-EIA2");
  });

  it("never pairs a radio with another registration's algorithms", () => {
    const stamps = [older, newer, "2026-08-17T10:10:00Z"];

    for (const a of stamps) {
      for (const b of stamps) {
        const merged = mergeRegistrations([at(on5GS, a), at(onEPS, b)]);

        if (merged.last_seen_radio === "gnb-1") {
          expect(merged.ciphering_algorithm).toBe("128-NEA2");
        } else {
          expect(merged.ciphering_algorithm).toBe("128-EEA2");
        }
      }
    }
  });

  it("is connected when any registration is, and idle when none is", () => {
    expect(mergeRegistrations([idle(on5GS), onEPS]).connection_state).toBe(
      "connected",
    );
    expect(
      mergeRegistrations([idle(on5GS), idle(onEPS)]).connection_state,
    ).toBe("idle");
  });

  it("keeps the last serving radio once the context is released", () => {
    const merged = mergeRegistrations([deregistered(at(onEPS, newer))]);

    expect(merged.registered).toBe(false);
    expect(merged.connection_state).toBeUndefined();
    expect(merged.radio_access_types).toEqual([]);
    expect(merged.last_seen_radio).toBe("enb-1");
    expect(merged.last_seen_at).toBe(newer);
  });

  it("prefers a live registration over a more recent released one", () => {
    const merged = mergeRegistrations([
      at(on5GS, older),
      deregistered(at(onEPS, newer)),
    ]);

    expect(merged.registered).toBe(true);
    expect(merged.radio_access_types).toEqual(["5G"]);
    expect(merged.last_seen_radio).toBe("gnb-1");
    expect(merged.last_seen_at).toBe(older);
  });

  it("reports the IMEI rather than the prefixed PEI", () => {
    expect(mergeRegistrations([on5GS]).imei).toBe("490154203237518");
  });
});
