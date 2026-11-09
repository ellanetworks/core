// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import type { IMSInitialFilterCriterion } from "@/queries/ims";
import IMSServiceProfileModal from "./IMSServiceProfileModal";

const api = setupApiServer();

const SP_PATH = "/api/v1/integrations/ims/service-profiles";
const SP_ITEM_PATH = `${SP_PATH}/sp`;
const AS_PATH = "/api/v1/integrations/ims/application-servers";

const registerIFC = {
  applicationServerName: "voicemail",
  trigger: {
    conditionsMatch: "all",
    groups: [{ conditions: [{ method: "REGISTER" }] }],
  },
  defaultHandling: "SESSION_CONTINUED",
  includeRegisterRequest: false,
  includeRegisterResponse: false,
};

const terminatingIFC = {
  applicationServerName: "conf",
  trigger: {
    conditionsMatch: "all",
    groups: [{ conditions: [{ sessionCase: "TERMINATING_UNREGISTERED" }] }],
  },
  defaultHandling: "SESSION_TERMINATED",
  includeRegisterRequest: false,
  includeRegisterResponse: false,
};

const mcpttPresetIFCs = [
  {
    applicationServerName: "mcptt",
    trigger: {
      conditionsMatch: "all",
      groups: [{ conditions: [{ method: "REGISTER" }] }],
    },
    defaultHandling: "SESSION_CONTINUED",
    includeRegisterRequest: true,
    includeRegisterResponse: true,
  },
  {
    applicationServerName: "mcptt",
    trigger: {
      conditionsMatch: "all",
      groups: [
        {
          conditions: [
            { sessionCase: "ORIGINATING_REGISTERED" },
            {
              sipHeader: {
                header: "^P-Asserted-Service$",
                content: "^urn:urn-7:3gpp-service\\.ims\\.icsi\\.mcptt$",
              },
            },
          ],
        },
      ],
    },
    defaultHandling: "SESSION_TERMINATED",
  },
];

const onSuccess = vi.fn();

beforeEach(() => {
  onSuccess.mockReset();
  api.get("/api/v1/integrations/ims", () => ({
    pcscfAddresses: [],
    voiceStatus: "no_pcscf",
    homeDomain: "",
    derivedHomeDomain: "ims.example.org",
    restrictPeers: true,
  }));
  api.get(AS_PATH, () => ({
    items: [
      {
        name: "mcptt",
        serverName: "sip:mcptt.ims.example.org",
        publicServiceIdentities: [],
      },
      {
        name: "voicemail",
        serverName: "sip:vm.ims.example.org",
        publicServiceIdentities: [],
      },
    ],
  }));
});

const openEditor = async (criteria: object[]) => {
  renderWithProviders(
    <IMSServiceProfileModal
      open
      onClose={() => {}}
      onSuccess={onSuccess}
      serviceProfile={{
        name: "sp",
        initialFilterCriteria: criteria as IMSInitialFilterCriterion[],
      }}
    />,
    { auth: { role: "Admin" } },
  );
  return within(
    await screen.findByRole("dialog", { name: "Edit Service Profile" }),
  );
};

const criterionForm = async () =>
  within(await screen.findByRole("dialog", { name: /Filter Criterion$/ }));

const choose = async (
  user: ReturnType<typeof userEvent.setup>,
  scope: ReturnType<typeof within>,
  name: string | RegExp,
  option: string | RegExp,
) => {
  await user.click(scope.getByRole("combobox", { name }));
  await user.click(await screen.findByRole("option", { name: option }));
};

const putBody = () =>
  api.requests(SP_ITEM_PATH).find((r) => r.method === "PUT")?.body;

const rows = (dialog: ReturnType<typeof within>) =>
  dialog.queryAllByRole("listitem").map((r: HTMLElement) => r.textContent);

describe("IMS service profile filter criteria", () => {
  it("lists the filter criteria in priority order", async () => {
    const dialog = await openEditor([
      { ...terminatingIFC, priority: 1 },
      { ...registerIFC, priority: 0 },
    ]);

    expect(rows(dialog)).toEqual([
      "1voicemailmethod is REGISTERContinue",
      "2confTerminating (unregistered)Terminate",
    ]);
  });

  it("shows when the service profile has no filter criteria", async () => {
    const dialog = await openEditor([]);

    expect(dialog.getByText("No IMS filter criteria.")).toBeInTheDocument();
  });

  it("creates a service profile with a name", async () => {
    const user = userEvent.setup();
    api.post(SP_PATH, () => ({}));
    renderWithProviders(
      <IMSServiceProfileModal open onClose={() => {}} onSuccess={onSuccess} />,
      { auth: { role: "Admin" } },
    );

    const dialog = within(
      await screen.findByRole("dialog", { name: "Add Service Profile" }),
    );
    await user.click(dialog.getByRole("textbox", { name: "Name" }));
    await user.tab();
    expect(await dialog.findByText("Name is required")).toBeInTheDocument();

    await user.type(dialog.getByRole("textbox", { name: "Name" }), "mcptt");
    await choose(user, dialog, /Preset/, /^MCPTT/);
    await user.click(dialog.getByRole("button", { name: "Add preset" }));
    await user.click(dialog.getByRole("button", { name: "Add" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(
      api.requests(SP_PATH).find((r) => r.method === "POST")?.body,
    ).toEqual({ name: "mcptt", initialFilterCriteria: mcpttPresetIFCs });
  });

  it("saves the criteria unchanged without priorities", async () => {
    const user = userEvent.setup();
    const criteria = [
      { ...registerIFC, priority: 0 },
      { ...terminatingIFC, priority: 1 },
    ];
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor(criteria);
    expect(rows(dialog)).toEqual([
      "1voicemailmethod is REGISTERContinue",
      "2confTerminating (unregistered)Terminate",
    ]);

    await user.click(dialog.getByRole("button", { name: "Edit criterion 1" }));
    await user.click(
      (await criterionForm()).getByRole("button", { name: "Update" }),
    );
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({
      initialFilterCriteria: [registerIFC, terminatingIFC],
    });
  });

  it("edits a criterion's trigger and handling", async () => {
    const user = userEvent.setup();
    const criteria = [{ ...registerIFC, priority: 0 }];
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor(criteria);
    await user.click(dialog.getByRole("button", { name: "Edit criterion 1" }));
    const form = await criterionForm();

    const condition = within(form.getByRole("group", { name: "Condition 1" }));
    expect(condition.getByRole("combobox", { name: "Method" })).toHaveValue(
      "REGISTER",
    );
    await user.click(
      condition.getByRole("checkbox", { name: "Re-registration" }),
    );
    await user.click(
      form.getByRole("radio", { name: "Terminate the session" }),
    );
    await user.click(form.getByRole("button", { name: "Update" }));

    expect(rows(dialog)).toEqual([
      "1voicemailmethod is REGISTER (initial, de-registration)Terminate",
    ]);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({
      initialFilterCriteria: [
        {
          ...registerIFC,
          trigger: {
            conditionsMatch: "all",
            groups: [
              {
                conditions: [
                  {
                    method: "REGISTER",
                    registrationTypes: [
                      "INITIAL_REGISTRATION",
                      "DE-REGISTRATION",
                    ],
                  },
                ],
              },
            ],
          },
          defaultHandling: "SESSION_TERMINATED",
        },
      ],
    });
  });

  it("adds a criterion with condition groups", async () => {
    const user = userEvent.setup();
    const criteria: object[] = [];
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor(criteria);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();

    await choose(user, form, "Application Server", /^voicemail/);
    await user.type(
      within(form.getByRole("group", { name: "Condition 1" })).getByRole(
        "combobox",
        { name: "Method" },
      ),
      "INVITE",
    );
    await user.click(form.getByRole("button", { name: "Add Condition" }));
    const second = within(form.getByRole("group", { name: "Condition 2" }));
    await choose(user, second, "Field", "SIP method");
    await user.type(
      second.getByRole("combobox", { name: "Method" }),
      "MESSAGE",
    );
    await choose(user, form, "Conditions match", "any");

    await user.click(form.getByRole("button", { name: "Add Group" }));
    expect(
      form.getByText(/The criterion matches when every group matches/),
    ).toBeInTheDocument();
    const third = within(
      form.getByRole("group", { name: "Condition 1 of group 2" }),
    );
    await choose(user, third, "Field", "SIP header");
    await choose(user, third, "Operator", "is absent");
    await user.type(
      third.getByRole("textbox", { name: "Header name (regex)" }),
      "^Privacy$",
    );
    await user.click(form.getByRole("button", { name: "Add" }));

    expect(rows(dialog)).toEqual([
      "1voicemail(method is INVITE or method is MESSAGE) and header ^Privacy$ absentContinue",
    ]);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({
      initialFilterCriteria: [
        {
          applicationServerName: "voicemail",
          trigger: {
            conditionsMatch: "any",
            groups: [
              { conditions: [{ method: "INVITE" }, { method: "MESSAGE" }] },
              {
                conditions: [
                  { negated: true, sipHeader: { header: "^Privacy$" } },
                ],
              },
            ],
          },
          defaultHandling: "SESSION_CONTINUED",
          includeRegisterRequest: false,
          includeRegisterResponse: false,
        },
      ],
    });
  });

  it("adds an unconditional criterion with advanced settings", async () => {
    const user = userEvent.setup();
    const criteria: object[] = [];
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor(criteria);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();

    await user.click(form.getByRole("radio", { name: "Always" }));
    await user.click(form.getByRole("button", { name: "Advanced" }));
    await choose(user, form, "Applies To", "Unregistered subscribers");
    await user.type(form.getByRole("textbox", { name: "Service Info" }), "vm");
    await user.click(form.getByRole("button", { name: "Add" }));

    expect(rows(dialog)).toEqual(["1mcpttAlwaysContinue"]);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({
      initialFilterCriteria: [
        {
          applicationServerName: "mcptt",
          defaultHandling: "SESSION_CONTINUED",
          profilePart: "UNREGISTERED",
          serviceInfo: "vm",
          includeRegisterRequest: false,
          includeRegisterResponse: false,
        },
      ],
    });
  });

  it("refuses an incomplete condition", async () => {
    const user = userEvent.setup();
    const criteria: object[] = [];

    const dialog = await openEditor(criteria);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();
    const condition = within(form.getByRole("group", { name: "Condition 1" }));

    await user.click(condition.getByRole("combobox", { name: "Method" }));
    await user.tab();
    expect(await form.findByText("Enter a SIP method")).toBeInTheDocument();
    expect(form.getByRole("button", { name: "Add" })).toBeDisabled();

    await choose(user, condition, "Field", "Request-URI");
    await user.type(
      condition.getByRole("textbox", { name: "Pattern (regex)" }),
      "([[",
    );
    await user.tab();
    expect(
      await form.findByText("Must be a regular expression"),
    ).toBeInTheDocument();
    expect(form.getByRole("button", { name: "Add" })).toBeDisabled();

    await choose(user, condition, "Field", "SIP method");
    await user.type(
      condition.getByRole("combobox", { name: "Method" }),
      "REGISTER",
    );
    for (const name of [
      "Initial registration",
      "Re-registration",
      "De-registration",
    ])
      await user.click(condition.getByRole("checkbox", { name }));
    expect(await form.findByText("Select at least one")).toBeInTheDocument();
    expect(form.getByRole("button", { name: "Add" })).toBeDisabled();
  });

  it("warns about a negated session case", async () => {
    const user = userEvent.setup();
    const criteria: object[] = [];

    const dialog = await openEditor(criteria);
    await user.click(dialog.getByRole("button", { name: "Add Criterion" }));
    const form = await criterionForm();
    const condition = within(form.getByRole("group", { name: "Condition 1" }));

    await choose(user, condition, "Field", "Session case");
    await choose(user, condition, "Operator", "is not");
    expect(
      form.getByText(/also matches session cases that later 3GPP releases add/),
    ).toBeInTheDocument();
  });

  it("deletes a criterion", async () => {
    const user = userEvent.setup();
    const criteria = [
      { ...registerIFC, priority: 0 },
      { ...terminatingIFC, priority: 1 },
    ];
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor(criteria);
    await user.click(
      dialog.getByRole("button", { name: "Delete criterion 1" }),
    );
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({ initialFilterCriteria: [terminatingIFC] });
  });

  it("reorders criteria by dragging", async () => {
    const user = userEvent.setup();
    const criteria = [
      { ...registerIFC, priority: 0 },
      { ...terminatingIFC, priority: 1 },
    ];
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor(criteria);
    const [first, second] = dialog.getAllByRole("listitem");
    const dataTransfer = {
      setDragImage: () => {},
      effectAllowed: "",
      dropEffect: "",
    };
    fireEvent.dragStart(second, { dataTransfer });
    fireEvent.dragOver(first, { dataTransfer });
    fireEvent.drop(first, { dataTransfer });
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({
      initialFilterCriteria: [terminatingIFC, registerIFC],
    });
  });

  it("puts the MCPTT preset first, once, for the suggested application server", async () => {
    const user = userEvent.setup();
    const criteria = [{ ...registerIFC, priority: 0 }];
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor(criteria);
    expect(dialog.getByRole("button", { name: "Add preset" })).toBeDisabled();
    await choose(user, dialog, /Preset/, /^MCPTT/);
    expect(
      dialog.getByRole("combobox", { name: /Application Server/ }),
    ).toHaveTextContent("mcptt (sip:mcptt.ims.example.org)");
    await user.click(dialog.getByRole("button", { name: "Add preset" }));

    expect(
      dialog.queryByText("This preset is already added."),
    ).not.toBeInTheDocument();
    expect(dialog.getByRole("button", { name: "Add preset" })).toBeDisabled();
    await choose(user, dialog, /Preset/, /^MCPTT/);
    await choose(user, dialog, /Application Server/, /^mcptt \(sip:/);
    expect(
      dialog.getByText("This preset is already added."),
    ).toBeInTheDocument();
    expect(dialog.getByRole("button", { name: "Add preset" })).toBeDisabled();
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({
      initialFilterCriteria: [...mcpttPresetIFCs, registerIFC],
    });
  });

  it("suggests the Ella IMS MCPTT by its server name for the MCPTT preset", async () => {
    const user = userEvent.setup();
    const criteria: object[] = [];
    api.get(AS_PATH, () => ({
      items: [
        {
          name: "mcptt",
          serverName: "sip:ptt.example.org",
          publicServiceIdentities: [],
        },
        {
          name: "ella",
          serverName: "sip:mcptt.ims.example.org",
          publicServiceIdentities: [],
        },
      ],
    }));

    const dialog = await openEditor(criteria);
    await choose(user, dialog, /Preset/, /^MCPTT/);
    expect(
      dialog.getByRole("combobox", { name: /Application Server/ }),
    ).toHaveTextContent("ella (sip:mcptt.ims.example.org)");
  });

  it("adds the SMS over IP preset for the IP-SM-GW", async () => {
    const user = userEvent.setup();
    api.get(AS_PATH, () => ({
      items: [
        {
          name: "ip-sm-gw",
          serverName: "sip:ipsmgw.ims.example.org",
          publicServiceIdentities: [],
        },
      ],
    }));
    api.put(SP_ITEM_PATH, () => ({}));

    const dialog = await openEditor([]);
    await choose(user, dialog, /Preset/, /^SMS over IP/);
    await user.click(dialog.getByRole("button", { name: "Add preset" }));
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(putBody()).toEqual({
      initialFilterCriteria: [
        {
          applicationServerName: "ip-sm-gw",
          trigger: {
            conditionsMatch: "all",
            groups: [{ conditions: [{ method: "REGISTER" }] }],
          },
          defaultHandling: "SESSION_CONTINUED",
          includeRegisterRequest: true,
          includeRegisterResponse: true,
        },
        {
          applicationServerName: "ip-sm-gw",
          trigger: {
            conditionsMatch: "all",
            groups: [
              {
                conditions: [
                  { method: "MESSAGE" },
                  { sessionCase: "ORIGINATING_REGISTERED" },
                  {
                    sipHeader: {
                      header: "^Content-Type$",
                      content: "^application/vnd\\.3gpp\\.sms$",
                    },
                  },
                ],
              },
            ],
          },
          defaultHandling: "SESSION_CONTINUED",
        },
      ],
    });
  });

  it("points to the Integrations IMS tab when there is no application server", async () => {
    const criteria: object[] = [];
    api.get(AS_PATH, () => ({ items: [] }));

    const dialog = await openEditor(criteria);
    expect(
      await dialog.findByRole("link", { name: "Integrations › IMS tab" }),
    ).toBeInTheDocument();
    expect(
      dialog.queryByRole("button", { name: "Add Criterion" }),
    ).not.toBeInTheDocument();
  });

  it("shows the error when the server rejects the filter criteria", async () => {
    const user = userEvent.setup();
    const criteria = [{ ...registerIFC, priority: 0 }];
    api.put(SP_ITEM_PATH, () =>
      httpError(400, "Application server not found: voicemail"),
    );

    const dialog = await openEditor(criteria);
    await user.click(dialog.getByRole("button", { name: "Save" }));

    expect(
      await dialog.findByText(
        "Failed to update service profile: Application server not found: voicemail",
      ),
    ).toBeInTheDocument();
  });
});
