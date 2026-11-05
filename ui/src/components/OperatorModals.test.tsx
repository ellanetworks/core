// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import EditOperatorIdModal from "./EditOperatorIdModal";
import EditOperatorCodeModal from "./EditOperatorCodeModal";
import EditOperatorSPNModal from "./EditOperatorSPNModal";
import EditOperatorSMSModal from "./EditOperatorSMSModal";
import SMSCModal from "./SMSCModal";
import EditOperatorTrackingModal from "./EditOperatorTrackingModal";
import IMSApplicationServerModal from "./IMSApplicationServerModal";

const api = setupApiServer();

const dialog = () => screen.getByRole("dialog");
const updateButton = () =>
  within(dialog()).getByRole("button", { name: /^Update$/ });
const field = (label: RegExp) => screen.getByLabelText(label);

const retype = async (
  user: ReturnType<typeof userEvent.setup>,
  label: RegExp,
  value: string,
) => {
  await user.clear(field(label));
  if (value) await user.type(field(label), value);
  await user.tab();
};

describe("EditOperatorIdModal", () => {
  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorIdModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ mcc: "001", mnc: "01" }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("seeds both fields and resolves its description", () => {
    render();
    expect(field(/MCC/)).toHaveValue("001");
    expect(field(/MNC/)).toHaveValue("01");
    expect(dialog()).toHaveAccessibleName("Edit Operator ID");

    const describedBy = dialog().getAttribute("aria-describedby");
    expect(document.getElementById(describedBy!)).toHaveTextContent(
      /Mobile Country Code \(MCC\)/,
    );
  });

  it("warns that the Diameter identities change", () => {
    render();
    expect(within(dialog()).getByRole("alert")).toHaveTextContent(
      /renames the Diameter identity and realm of every node/,
    );
  });

  it("rejects an MCC that is not three digits", async () => {
    const user = userEvent.setup();
    render();

    await retype(user, /MCC/, "12");
    await screen.findByText("MCC must be a 3 decimal digit");
    expect(updateButton()).toBeDisabled();
  });

  it("accepts a three digit MNC", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/operator/id", () => ({}));
    const { onClose } = render();

    await retype(user, /MNC/, "010");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/operator/id")?.body).toEqual({
      mcc: "001",
      mnc: "010",
    });
  });
});

describe("EditOperatorCodeModal", () => {
  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorCodeModal open onClose={onClose} onSuccess={vi.fn()} />,
      { auth: {} },
    );
    return { onClose };
  };

  it("warns that the operation cannot be undone", () => {
    render();
    expect(
      screen.getByText(/This operation cannot be undone/),
    ).toBeInTheDocument();
    expect(updateButton()).toBeDisabled();
  });

  it("requires a 32 character hex code", async () => {
    const user = userEvent.setup();
    render();

    await retype(user, /^Operator Code$/, "nothex");
    await screen.findByText(
      "Operator Code must be a 32-character hexadecimal string.",
    );
    expect(updateButton()).toBeDisabled();

    await retype(user, /^Operator Code$/, "0".repeat(32));
    await waitFor(() => expect(updateButton()).toBeEnabled());
  });

  it("submits the code", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/operator/code", () => ({}));
    const { onClose } = render();

    const code = "0123456789abcdef0123456789ABCDEF";
    await retype(user, /^Operator Code$/, code);
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/operator/code")?.body).toEqual({
      operatorCode: code,
    });
  });
});

describe("EditOperatorSPNModal", () => {
  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorSPNModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ fullName: "Ella", shortName: "Ella" }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("shows both length hints until a field is invalid", async () => {
    const user = userEvent.setup();
    render();

    expect(
      screen.getByText(/The full network name shown on UE displays/),
    ).toBeInTheDocument();

    await retype(user, /Full Name/, "");
    await screen.findByText("Full name is required");
    expect(updateButton()).toBeDisabled();
  });

  it("caps input at the maximum length", () => {
    render();
    expect(field(/Full Name/)).toHaveAttribute("maxlength", "50");
    expect(field(/Short Name/)).toHaveAttribute("maxlength", "50");
  });

  it("submits both names with the operator SPN error prefix on failure", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/operator/spn", () => httpError(500, "spn rejected"));
    const { onClose } = render();

    await retype(user, /Short Name/, "EN");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await screen.findByText(/Failed to update operator SPN: .*spn rejected/);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("EditOperatorTrackingModal", () => {
  const TRACKING_PATH = "/api/v1/operator/tracking";

  const render = (supportedTacs = ["000001"]) => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorTrackingModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ supportedTacs }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("renders existing TACs as chips with the default hint", () => {
    render(["000001", "0000FF"]);
    expect(
      screen.getByText(/Enter each TAC as a 3 bytes hex string/),
    ).toBeInTheDocument();
    expect(dialog()).toHaveAccessibleName("Edit Operator Tracking Information");
  });

  it("shows a decimal hint while typing a valid TAC", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Supported TACs/), "000010");
    await screen.findByText("000010 is 16 in decimal");
  });

  it("commits a pending TAC that was never turned into a chip", async () => {
    const user = userEvent.setup();
    api.put(TRACKING_PATH, () => ({}));
    const { onClose } = render(["000001"]);

    await user.type(field(/Supported TACs/), "000002");
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(TRACKING_PATH)?.body).toEqual({
      supportedTacs: ["000001", "000002"],
    });
  });

  it("rejects a pending TAC that is not valid hex and stays open", async () => {
    const user = userEvent.setup();
    const { onClose } = render(["000001"]);

    await user.type(field(/Supported TACs/), "zzzzzz");
    await user.click(updateButton());

    await screen.findByText("Invalid TACs: zzzzzz");
    expect(onClose).not.toHaveBeenCalled();
    expect(api.requests(TRACKING_PATH)).toHaveLength(0);
  });

  it("submits chips added with Enter", async () => {
    const user = userEvent.setup();
    api.put(TRACKING_PATH, () => ({}));
    const { onClose } = render([]);

    await user.type(field(/Supported TACs/), "00000A{Enter}");
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(TRACKING_PATH)?.body).toEqual({
      supportedTacs: ["00000A"],
    });
  });
});

describe("EditOperatorSMSModal", () => {
  const SMS_PATH = "/api/v1/operator/sms";

  const render = (initialData = { smsNumber: "" }) => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditOperatorSMSModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={initialData}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("submits the SMS number", async () => {
    const user = userEvent.setup();
    api.put(SMS_PATH, () => ({}));
    const { onClose } = render();

    await retype(user, /E.164 Number/, "+15550001111");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(SMS_PATH)?.body).toEqual({
      smsNumber: "+15550001111",
    });
  });

  it("requires the SMS number", async () => {
    const user = userEvent.setup();
    render({ smsNumber: "+15550001111" });

    await retype(user, /E.164 Number/, "");
    await screen.findByText("SMS number is required");
    expect(updateButton()).toBeDisabled();
  });

  it("rejects an SMS number without the + prefix", async () => {
    const user = userEvent.setup();
    render();

    await retype(user, /E.164 Number/, "15550001111");

    await screen.findByText(/Must be an E.164 number/);
    expect(updateButton()).toBeDisabled();
  });
});

describe("SMSCModal", () => {
  const SMSCS_PATH = "/api/v1/operator/sms/smscs";

  const smsc = {
    name: "main",
    serviceCentres: ["+15550000000"],
    nodes: [
      {
        diameterIdentity: "smsc1.example.org",
        address: "192.0.2.10",
        port: 3868,
      },
      { diameterIdentity: "", address: "192.0.2.11", port: 3868 },
    ],
  };

  const render = (existing?: typeof smsc) => {
    const onClose = vi.fn();
    renderWithProviders(
      <SMSCModal open onClose={onClose} onSuccess={vi.fn()} smsc={existing} />,
      { auth: {} },
    );
    return { onClose };
  };

  const addButton = () =>
    within(dialog()).getByRole("button", { name: /^Add$/ });

  it("adds an SMSC with two nodes", async () => {
    const user = userEvent.setup();
    api.post(SMSCS_PATH, () => ({}));
    const { onClose } = render();

    await retype(user, /^Name/, "main");
    await retype(user, /^Numbers/, "+15550000000, +15550000001");
    await retype(user, /^Node 1 address/, "2001:db8::10");
    await retype(user, /^Node 1 Diameter identity/, "smsc1.example.org");
    await user.click(screen.getByRole("button", { name: "Add Node" }));
    await retype(user, /^Node 2 address/, "192.0.2.11");
    await retype(user, /^Node 2 port/, "3869");
    await waitFor(() => expect(addButton()).toBeEnabled());
    await user.click(addButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(SMSCS_PATH)?.body).toEqual({
      name: "main",
      serviceCentres: ["+15550000000", "+15550000001"],
      nodes: [
        {
          diameterIdentity: "smsc1.example.org",
          address: "2001:db8::10",
          port: 3868,
        },
        { diameterIdentity: "", address: "192.0.2.11", port: 3869 },
      ],
    });
  });

  it("updates an SMSC without changing its name", async () => {
    const user = userEvent.setup();
    api.put(`${SMSCS_PATH}/main`, () => ({}));
    const { onClose } = render(smsc);

    expect(field(/^Name/)).toBeDisabled();
    expect(field(/^Numbers/)).toHaveValue("+15550000000");
    expect(field(/^Node 2 address/)).toHaveValue("192.0.2.11");
    await user.click(screen.getByRole("button", { name: "Remove node 1" }));
    await retype(user, /^Node 1 port/, "3869");
    await waitFor(() => expect(updateButton()).toBeEnabled());
    await user.click(updateButton());

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(`${SMSCS_PATH}/main`)?.body).toEqual({
      serviceCentres: ["+15550000000"],
      nodes: [{ diameterIdentity: "", address: "192.0.2.11", port: 3869 }],
    });
  });

  it("keeps at least one node", async () => {
    const user = userEvent.setup();
    render(smsc);

    await user.click(screen.getByRole("button", { name: "Remove node 2" }));

    expect(
      screen.getByRole("button", { name: "Remove node 1" }),
    ).toBeDisabled();
  });

  it("allows at most 8 nodes", async () => {
    const user = userEvent.setup();
    render(smsc);

    for (let i = 0; i < 6; i++) {
      await user.click(screen.getByRole("button", { name: "Add Node" }));
    }

    expect(screen.getByLabelText(/^Node 8 address/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add Node" })).toBeDisabled();
  });

  it("rejects an address that is not an IP address", async () => {
    const user = userEvent.setup();
    render(smsc);

    await retype(user, /^Node 1 address/, "smsc.example.org");

    await screen.findByText("Must be an IPv4 or IPv6 address");
    expect(updateButton()).toBeDisabled();
  });

  it("rejects a Diameter identity that is not a domain name", async () => {
    const user = userEvent.setup();
    render(smsc);

    await retype(user, /^Node 1 Diameter identity/, "smsc");

    await screen.findByText("Must be a fully qualified domain name");
    expect(updateButton()).toBeDisabled();
  });

  it("rejects two nodes with the same address and port", async () => {
    const user = userEvent.setup();
    render(smsc);

    await retype(user, /^Node 2 address/, "192.0.2.10");

    await screen.findByText("Two nodes have the same address and port");
    expect(updateButton()).toBeDisabled();
  });

  it("requires valid, distinct service centre numbers", async () => {
    const user = userEvent.setup();
    render(smsc);

    await retype(user, /^Numbers/, "");
    await screen.findByText("At least one number is required");

    await retype(user, /^Numbers/, "15550000000");
    await screen.findByText(/Numbers must be E.164 numbers/);

    await retype(user, /^Numbers/, "+15550000000 +15550000000");
    await screen.findByText("A number is listed twice");
    expect(updateButton()).toBeDisabled();
  });

  it("shows the server's error", async () => {
    const user = userEvent.setup();
    api.post(SMSCS_PATH, () =>
      httpError(
        409,
        "another SMSC serves the service centre number +15550000000",
      ),
    );
    render();

    await retype(user, /^Name/, "backup");
    await retype(user, /^Numbers/, "+15550000000");
    await retype(user, /^Node 1 address/, "192.0.2.12");
    await waitFor(() => expect(addButton()).toBeEnabled());
    await user.click(addButton());

    expect(
      await screen.findByText(/another SMSC serves the service centre/),
    ).toBeInTheDocument();
  });
});

describe("IMSApplicationServerModal", () => {
  const AS_PATH = "/api/v1/operator/ims/application-servers";

  const render = (
    props: Partial<React.ComponentProps<typeof IMSApplicationServerModal>>,
  ) => {
    const onClose = vi.fn();
    renderWithProviders(
      <IMSApplicationServerModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        {...props}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  const fillButton = () =>
    within(dialog()).queryByRole("button", {
      name: "Fill for Ella IMS MCPTT",
    });

  it("fills in the Ella IMS MCPTT without a PSI", async () => {
    const user = userEvent.setup();
    api.post(AS_PATH, () => ({}));
    const { onClose } = render({
      homeDomain: "ims.mnc001.mcc001.3gppnetwork.org",
    });

    await retype(user, /^Name/, "other");
    await user.click(
      within(dialog()).getByRole("button", { name: "Add Identity" }),
    );
    await user.type(field(/Identity 1/), "sip:!.*!@other.example.org");
    await user.click(fillButton()!);

    expect(field(/^Name/)).toHaveValue("mcptt");
    expect(field(/Server Name/)).toHaveValue(
      "sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org",
    );
    expect(screen.queryByLabelText(/Identity 1/)).not.toBeInTheDocument();
    const addButton = within(dialog()).getByRole("button", { name: /^Add$/ });
    await waitFor(() => expect(addButton).toBeEnabled());
    await user.click(addButton);

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(AS_PATH)?.body).toEqual({
      name: "mcptt",
      server_name: "sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org",
      public_service_identities: [],
    });
  });

  it("offers the fill only when adding, with a home domain", () => {
    render({
      homeDomain: "ims.mnc001.mcc001.3gppnetwork.org",
      applicationServer: {
        name: "mcptt",
        server_name: "sip:mcptt.example.org",
        public_service_identities: [],
      },
    });
    expect(fillButton()).not.toBeInTheDocument();
  });

  it("hides the fill without a home domain", () => {
    render({});
    expect(fillButton()).not.toBeInTheDocument();
  });

  it("rejects an identity listed twice", async () => {
    const user = userEvent.setup();
    api.post(AS_PATH, () => ({}));
    const { onClose } = render({});

    await retype(user, /^Name/, "conf");
    await retype(user, /Server Name/, "sip:conf.example.org");
    for (const n of [1, 2]) {
      await user.click(
        within(dialog()).getByRole("button", { name: "Add Identity" }),
      );
      await user.type(field(new RegExp(`Identity ${n}`)), "sip:a@example.org");
    }
    await user.tab();

    expect(
      within(dialog()).getByRole("button", { name: /^Add$/ }),
    ).toBeDisabled();
    expect(
      await within(dialog()).findByText("An identity is listed twice"),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(api.lastRequest(AS_PATH)).toBeUndefined();
  });
});
