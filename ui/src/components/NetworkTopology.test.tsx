// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import NetworkTopology, {
  formatDatapath,
  type InterfaceSegment,
} from "./NetworkTopology";

const interfaces = {
  n2: { addresses: ["10.10.0.5"], port: 38412, interface: "enp1s0" },
  n3: {
    name: "enp3s0",
    addresses: ["10.20.0.5"],
    external_address: "10.20.0.5",
  },
  n6: { name: "enp4s0", addresses: ["192.168.250.5"] },
  api: { addresses: ["0.0.0.0"], port: 5002 },
};

const renderTopology = (
  over: {
    active?: InterfaceSegment | null;
    datapathAttachMode?: string;
    onActiveChange?: (segment: InterfaceSegment | null) => void;
  } = {},
) =>
  renderWithProviders(
    <NetworkTopology
      interfaces={interfaces}
      datapathAttachMode={over.datapathAttachMode}
      active={over.active ?? null}
      onActiveChange={over.onActiveChange ?? (() => {})}
    />,
  );

describe("formatDatapath", () => {
  it.each([
    ["xdp-native", "eBPF · XDP native"],
    ["xdp-generic", "eBPF · XDP generic"],
    ["tcx", "eBPF · TCX"],
    [undefined, "eBPF"],
  ])("renders %s as %s", (mode, expected) => {
    expect(formatDatapath(mode)).toBe(expected);
  });
});

describe("NetworkTopology", () => {
  it("summarises the whole path for screen readers", () => {
    renderTopology();

    expect(screen.getByRole("img").getAttribute("aria-label")).toContain(
      "Radios signal Ella Core over N2 at 10.10.0.5 · port 38412",
    );
  });

  it("names the NIC behind every interface", () => {
    renderTopology();

    expect(screen.getByText("enp1s0")).toBeInTheDocument();
    expect(screen.getByText("enp3s0")).toBeInTheDocument();
    expect(screen.getByText("enp4s0")).toBeInTheDocument();
  });

  it("labels the user plane with the mechanism the UPF attached with", () => {
    renderTopology({ datapathAttachMode: "tcx" });

    expect(screen.getByText("eBPF · TCX")).toBeInTheDocument();
  });

  it("keeps addresses out of the drawing and in the summary", () => {
    renderTopology();

    expect(screen.queryByText("192.168.250.5")).not.toBeInTheDocument();
    expect(screen.getByRole("img").getAttribute("aria-label")).toContain(
      "192.168.250.5",
    );
  });

  it("dims every segment except the active one", () => {
    const { container } = renderTopology({ active: "n6" });
    const n6 = screen.getByText("N6").closest("g");
    const n2 = screen.getByText("N2 · NGAP / S1AP").closest("g");

    expect(n6).toHaveAttribute("opacity", "1");
    expect(n2).toHaveAttribute("opacity", "0.2");
    expect(container.querySelector("mask#ella-n6-mask")).not.toBeNull();
  });

  it("completes the uplink arrow at the socket when N3 is active", () => {
    const idleHeads =
      renderTopology().container.querySelectorAll("path[d^='M']");
    const activeHeads = renderTopology({
      active: "n3",
    }).container.querySelectorAll("path[d^='M']");

    expect(activeHeads.length).toBe(idleHeads.length + 1);
  });

  it("splits the data plane tracks at the sockets, not at the user plane", () => {
    const { container } = renderTopology();
    const n3 = screen.getByText("N3 · GTP-U").closest("g")!;
    const n6 = screen.getByText("N6").closest("g")!;

    expect(n3.querySelector("line")).toHaveAttribute("x2", "380");
    expect(n6.querySelector("line")).toHaveAttribute("x1", "620");
    expect(container).toBeTruthy();
  });

  it("reports the segment under the pointer", async () => {
    const onActiveChange = vi.fn();
    renderTopology({ onActiveChange });

    await userEvent.hover(screen.getByText("N3 · GTP-U"));

    expect(onActiveChange).toHaveBeenCalledWith("n3");
  });
});
