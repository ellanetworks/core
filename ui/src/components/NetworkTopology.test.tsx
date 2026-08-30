// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import NetworkTopology, { detailsFor, formatDatapath } from "./NetworkTopology";

const defaultInterfaces = {
  n2: {
    addresses: ["10.10.0.5", "2001:db8::5"],
    port: 38412,
    interface: "enp1s0",
  },
  n3: {
    name: "enp3s0",
    addresses: ["10.20.0.5"],
    external_address: "10.20.0.5",
    vlan: { vlan_id: 42, master_interface: "enp3s0" },
  },
  n6: { name: "enp4s0", addresses: ["192.168.250.5"] },
  api: { addresses: ["0.0.0.0"], port: 5002 },
};

const renderTopology = (
  over: {
    datapathAttachMode?: string;
    canEdit?: boolean;
    onEditN3?: () => void;
    interfaces?: typeof defaultInterfaces;
  } = {},
) =>
  renderWithProviders(
    <NetworkTopology
      interfaces={over.interfaces ?? defaultInterfaces}
      datapathAttachMode={over.datapathAttachMode}
      canEdit={over.canEdit ?? true}
      onEditN3={over.onEditN3 ?? (() => {})}
    />,
  );

const segment = (name: RegExp) => screen.getByRole("button", { name });

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

describe("detailsFor", () => {
  it("labels the port once instead of repeating it on every address", () => {
    expect(detailsFor("n2", defaultInterfaces)).toEqual([
      { label: "interface", text: "enp1s0" },
      { label: "port", text: "38412" },
      { label: "address", text: "10.10.0.5" },
      { label: undefined, text: "2001:db8::5" },
    ]);
  });

  it("marks the N3 external address as the one editable line", () => {
    const editable = detailsFor("n3", defaultInterfaces).filter(
      (detail) => detail.editable,
    );

    expect(editable).toEqual([
      { label: "external", text: "10.20.0.5", editable: true },
    ]);
  });

  it("drops absent values rather than emitting an em dash", () => {
    expect(detailsFor("n6", { n6: { name: "lo", addresses: [] } })).toEqual([
      { label: "interface", text: "lo" },
    ]);
  });

  it("says so when the external address is unset", () => {
    expect(detailsFor("n3", { n3: { name: "lo" } })).toEqual([
      { label: "interface", text: "lo" },
      { label: "external", text: "not set", editable: true },
    ]);
  });
});

describe("NetworkTopology", () => {
  it("shows only the topology until an interface is pointed at", () => {
    renderTopology();

    expect(screen.getByText("N3 · GTP-U")).toBeInTheDocument();
    expect(screen.queryByText("enp3s0")).not.toBeInTheDocument();
    expect(screen.queryByText("10.20.0.5")).not.toBeInTheDocument();
  });

  it("names each kind of value beside it", async () => {
    renderTopology();

    await userEvent.hover(segment(/^N3/));

    expect(screen.getByText("interface")).toBeInTheDocument();
    expect(screen.getByText("address")).toBeInTheDocument();
    expect(screen.getByText("external")).toBeInTheDocument();
    expect(screen.getByText("vlan")).toBeInTheDocument();
  });

  it("reveals every value of the interface under the pointer", async () => {
    renderTopology();

    await userEvent.hover(segment(/^N2/));

    expect(screen.getByText("enp1s0")).toBeInTheDocument();
    expect(screen.getByText("38412")).toBeInTheDocument();
    expect(screen.getByText("10.10.0.5")).toBeInTheDocument();
    expect(screen.getByText("2001:db8::5")).toBeInTheDocument();
  });

  it("reveals one interface at a time", async () => {
    renderTopology();

    await userEvent.hover(segment(/^N6/));

    expect(screen.getByText("enp4s0")).toBeInTheDocument();
    expect(screen.queryByText("enp1s0")).not.toBeInTheDocument();
  });

  it("dims the interfaces that are not revealed", async () => {
    const { container } = renderTopology();

    await userEvent.hover(segment(/^N6/));

    const drawing = (id: string) =>
      container.querySelector(`[data-segment="${id}"]`);

    expect(drawing("n6")).toHaveAttribute("opacity", "1");
    expect(drawing("n2")).toHaveAttribute("opacity", "0.2");
  });

  it("labels the user plane with the mechanism the UPF attached with", () => {
    renderTopology({ datapathAttachMode: "tcx" });

    expect(screen.getByText("eBPF · TCX")).toBeInTheDocument();
  });

  it("offers the N3 external address edit only to editors", async () => {
    renderTopology({ canEdit: false });

    await userEvent.hover(segment(/^N3/));

    expect(screen.getByText("external")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /edit n3 external address/i }),
    ).not.toBeInTheDocument();
  });

  it("opens the N3 editor from the pencil", async () => {
    const onEditN3 = vi.fn();
    renderTopology({ onEditN3 });

    await userEvent.hover(segment(/^N3/));
    await userEvent.click(
      screen.getByRole("button", { name: /edit n3 external address/i }),
    );

    expect(onEditN3).toHaveBeenCalledOnce();
  });

  it("reveals an interface on keyboard focus and reaches its pencil", () => {
    const onEditN3 = vi.fn();
    renderTopology({ onEditN3 });

    // jsdom has no focus() on SVGElement, so the handler is driven directly.
    fireEvent.focus(segment(/^N3/));
    expect(screen.getByText("enp3s0")).toBeInTheDocument();

    fireEvent.keyDown(
      screen.getByRole("button", { name: /edit n3 external address/i }),
      { key: "Enter" },
    );

    expect(onEditN3).toHaveBeenCalledOnce();
  });

  it("keeps the interfaces in place when one is revealed", () => {
    renderTopology();

    const order = () =>
      screen
        .getAllByRole("button")
        .map((element) => element.getAttribute("aria-label"));
    const before = order();

    fireEvent.focus(segment(/^N3/));

    expect(order().filter((label) => before.includes(label))).toEqual(before);
    expect(order()).toEqual([
      "N2, NGAP and S1AP signalling",
      "N3, GTP-U user plane",
      "Edit N3 external address",
      "N6, external network",
      "API, management",
    ]);
  });

  it("draws the revealed panel over every interface drawing", async () => {
    const { container } = renderTopology();

    await userEvent.hover(segment(/^N2/));
    const drawing = container.querySelector('[data-segment="n3"]')!;
    const panelValue = screen.getByText("2001:db8::5");

    expect(
      drawing.compareDocumentPosition(panelValue) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("keeps the pencil out of the interface button so both are reachable", () => {
    renderTopology();

    fireEvent.focus(segment(/^N3/));
    const pencil = screen.getByRole("button", {
      name: /edit n3 external address/i,
    });

    expect(segment(/^N3/).contains(pencil)).toBe(false);
    expect(segment(/^N3/).parentElement?.contains(pencil)).toBe(true);
  });

  it("reveals an interface on activation where there is no pointer to hover", () => {
    renderTopology();

    fireEvent.click(segment(/^N6/));

    expect(screen.getByText("enp4s0")).toBeInTheDocument();
    expect(segment(/^N6/)).toHaveAttribute("aria-expanded", "true");
  });

  it("leaves the revealed interface up when it is activated again", async () => {
    renderTopology();

    await userEvent.hover(segment(/^N6/));
    fireEvent.keyDown(segment(/^N6/), { key: "Enter" });
    fireEvent.click(segment(/^N6/));

    expect(screen.getByText("enp4s0")).toBeInTheDocument();
    expect(segment(/^N6/)).toHaveAttribute("aria-expanded", "true");
  });

  it("keeps the panel up while the pointer is on it", async () => {
    renderTopology();

    await userEvent.hover(segment(/^N2/));
    await userEvent.hover(screen.getByText("2001:db8::5"));

    expect(screen.getByText("2001:db8::5")).toBeInTheDocument();
  });
});
