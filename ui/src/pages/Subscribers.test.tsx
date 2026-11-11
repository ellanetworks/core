// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  LOCATION_TEST_ID,
  renderWithProviders,
} from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import Subscribers from "./Subscribers";

const api = setupApiServer();

const SUBSCRIBERS_PATH = "/api/v1/subscribers";

const IMSIS = ["001010100007487", "001010100007488", "001010100009999"];

const DESCRIPTIONS: Record<string, string> = {
  "001010100007487": "Warehouse gate reader",
};

const MSISDNS: Record<string, string> = {
  "001010100007487": "+15551230001",
};

const subscriber = (imsi: string) => ({
  imsi,
  profile_name: "default",
  description: DESCRIPTIONS[imsi],
  msisdn: MSISDNS[imsi],
  status: { registered: false, num_sessions: 0 },
});

const seedApi = (imsis = IMSIS) => {
  api.get(SUBSCRIBERS_PATH, ({ params }) => {
    const search = params.get("search") ?? "";
    const page = Number(params.get("page") ?? 1);
    const perPage = Number(params.get("per_page") ?? 25);

    const matched = imsis.filter(
      (imsi) =>
        imsi.includes(search) || (DESCRIPTIONS[imsi] ?? "").includes(search),
    );
    const start = (page - 1) * perPage;

    return {
      items: matched.slice(start, start + perPage).map(subscriber),
      page,
      per_page: perPage,
      total_count: matched.length,
    };
  });
};

const renderSubscribers = async () => {
  const result = renderWithProviders(<Subscribers />, { auth: {} });
  await screen.findByRole("heading", { level: 1, name: "Subscribers" });
  return result;
};

const searchBox = () => screen.getByLabelText("Search");

const lastParams = () => {
  const request = api.lastRequest(SUBSCRIBERS_PATH);
  if (!request) throw new Error("no subscribers request was made");
  return request.params;
};

const waitForRequests = async (n: number) =>
  waitFor(() => expect(api.requests(SUBSCRIBERS_PATH).length).toBe(n));

describe("Subscribers search", () => {
  it("sends no search parameter before the operator types", async () => {
    seedApi();
    await renderSubscribers();
    await waitForRequests(1);

    expect(lastParams().has("search")).toBe(false);
    expect(await screen.findByText(IMSIS[0])).toBeInTheDocument();
  });

  it("applies the search once typing settles", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.type(searchBox(), "0748");

    await waitFor(() => expect(lastParams().get("search")).toBe("0748"), {
      timeout: 2000,
    });
  });

  it("issues one request for a burst of keystrokes, not one per character", async () => {
    seedApi();
    await renderSubscribers();
    await waitForRequests(1);

    const before = api.requests(SUBSCRIBERS_PATH).length;
    const box = searchBox();
    for (const value of ["0", "07", "074", "0748"]) {
      fireEvent.change(box, { target: { value } });
    }

    await waitFor(() => expect(lastParams().get("search")).toBe("0748"), {
      timeout: 2000,
    });

    const searched = api
      .requests(SUBSCRIBERS_PATH)
      .slice(before)
      .filter((r) => r.params.get("search"));

    expect(searched).toHaveLength(1);
  });

  it("shows only the matching subscribers", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await screen.findByText(IMSIS[2]);

    await user.type(searchBox(), "0748");

    await waitFor(
      () => expect(screen.queryByText(IMSIS[2])).not.toBeInTheDocument(),
      { timeout: 2000 },
    );
    expect(screen.getByText(IMSIS[0])).toBeInTheDocument();
    expect(screen.getByText(IMSIS[1])).toBeInTheDocument();
  });

  it("drops the parameter when the search is cleared", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.type(searchBox(), "0748");
    await waitFor(() => expect(lastParams().get("search")).toBe("0748"), {
      timeout: 2000,
    });

    await user.click(screen.getByRole("button", { name: "Clear search" }));

    await waitFor(() => expect(lastParams().has("search")).toBe(false), {
      timeout: 2000,
    });
  });

  it("treats a whitespace-only search as unset", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.type(searchBox(), "   ");

    await new Promise((resolve) => setTimeout(resolve, 800));

    expect(
      api
        .requests(SUBSCRIBERS_PATH)
        .every((r) => !r.params.has("search") || r.params.get("search") === ""),
    ).toBe(true);
  });

  it("returns to the first page when the search changes", async () => {
    seedApi(Array.from({ length: 60 }, (_v, i) => `00101010000${1000 + i}`));
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() => expect(lastParams().get("page")).toBe("2"));

    await user.type(searchBox(), "1005");

    await waitFor(
      () => {
        const params = lastParams();
        expect(params.get("search")).toBe("1005");
        expect(params.get("page")).toBe("1");
      },
      { timeout: 2000 },
    );
  });

  it("offers no-results guidance rather than the create-one empty state", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.type(searchBox(), "12345");

    expect(
      await screen.findByText(
        "No subscribers match your search",
        {},
        { timeout: 2000 },
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("No subscribers yet")).not.toBeInTheDocument();
  });

  it("keeps the empty state when there are no subscribers at all", async () => {
    seedApi([]);
    await renderSubscribers();

    expect(await screen.findByText("No subscribers yet")).toBeInTheDocument();
  });

  it("offers no search box on a network with no subscribers", async () => {
    seedApi([]);
    await renderSubscribers();
    await screen.findByText("No subscribers yet");

    expect(screen.queryByLabelText("Search")).not.toBeInTheDocument();
  });

  it("offers the search box once there are subscribers", async () => {
    seedApi();
    await renderSubscribers();
    await screen.findByText(IMSIS[0]);

    expect(screen.getByLabelText("Search")).toBeInTheDocument();
  });

  it("keeps the search box mounted while a cleared zero-result search refetches", async () => {
    let release: (() => void) | undefined;
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.type(searchBox(), "12345");
    await screen.findByText(
      "No subscribers match your search",
      {},
      { timeout: 2000 },
    );

    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    api.get(SUBSCRIBERS_PATH, async () => {
      await gate;
      return {
        items: IMSIS.map(subscriber),
        page: 1,
        per_page: 25,
        total_count: IMSIS.length,
      };
    });

    await user.click(screen.getByRole("button", { name: "Clear search" }));

    await waitFor(
      () =>
        expect(api.lastRequest(SUBSCRIBERS_PATH)?.params.has("search")).toBe(
          false,
        ),
      { timeout: 2000 },
    );
    expect(screen.getByLabelText("Search")).toBeInTheDocument();

    release?.();
    await screen.findByText(IMSIS[0]);
    expect(screen.getByLabelText("Search")).toBeInTheDocument();
  });

  it("searches descriptions as well as IMSIs", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.type(searchBox(), "gate");

    await waitFor(() => expect(lastParams().get("search")).toBe("gate"), {
      timeout: 2000,
    });
    await waitFor(
      () => expect(screen.queryByText(IMSIS[1])).not.toBeInTheDocument(),
      { timeout: 2000 },
    );
    expect(screen.getByText(IMSIS[0])).toBeInTheDocument();
  });

  it("caps the search input at the length the API accepts", async () => {
    seedApi();
    await renderSubscribers();
    await waitForRequests(1);

    expect(searchBox()).toHaveAttribute("maxlength", "254");
  });

  it("keeps the search box on screen when the search matches nothing", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();
    await waitForRequests(1);

    await user.type(searchBox(), "12345");
    await screen.findByText(
      "No subscribers match your search",
      {},
      { timeout: 2000 },
    );

    expect(screen.getByLabelText("Search")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Clear search" }),
    ).toBeInTheDocument();
  });
});

describe("Subscribers description column", () => {
  it("renders the description, and an em dash when there is none", async () => {
    seedApi();
    await renderSubscribers();

    expect(await screen.findByText(DESCRIPTIONS[IMSIS[0]])).toBeInTheDocument();

    const row = screen.getByText(IMSIS[1]).closest(".MuiDataGrid-row");
    const cell = row?.querySelector('[data-field="description"]');
    expect(cell).toHaveTextContent("—");
  });

  it("offers the full description as a tooltip", async () => {
    seedApi();
    const user = userEvent.setup();
    await renderSubscribers();

    await user.hover(await screen.findByText(DESCRIPTIONS[IMSIS[0]]));

    expect(
      await screen.findByRole("tooltip", {}, { timeout: 2000 }),
    ).toHaveTextContent(DESCRIPTIONS[IMSIS[0]]);
  });
});

describe("Subscribers connection state", () => {
  const seedStatuses = (statuses: Record<string, Record<string, unknown>>) => {
    api.get(SUBSCRIBERS_PATH, () => ({
      items: Object.entries(statuses).map(([imsi, status]) => ({
        imsi,
        profile_name: "default",
        status,
      })),
      page: 1,
      per_page: 25,
      total_count: Object.keys(statuses).length,
    }));
  };

  const cellOf = (imsi: string, field: string) =>
    screen
      .getByText(imsi)
      .closest(".MuiDataGrid-row")
      ?.querySelector(`[data-field="${field}"]`);

  it("merges registration, connection and system into one status", async () => {
    seedStatuses({
      [IMSIS[0]]: {
        registered: true,
        connection_state: "connected",
        systems: ["5G"],
        last_seen_radio: "gnb-01",
      },
      [IMSIS[1]]: {
        registered: true,
        connection_state: "idle",
        systems: ["4G"],
        last_seen_radio: "gnb-01",
      },
      [IMSIS[2]]: { registered: false },
    });
    await renderSubscribers();
    await screen.findByText(IMSIS[0]);

    expect(cellOf(IMSIS[0], "status")).toHaveTextContent("Connected");
    expect(cellOf(IMSIS[0], "status")).toHaveTextContent("5G");

    expect(cellOf(IMSIS[1], "status")).toHaveTextContent("Idle");
    expect(cellOf(IMSIS[1], "status")).toHaveTextContent("4G");

    expect(cellOf(IMSIS[2], "status")).toHaveTextContent("Deregistered");
    expect(cellOf(IMSIS[2], "status")).not.toHaveTextContent(/4G|5G/);
  });

  it("shows a registration in progress", async () => {
    seedStatuses({
      [IMSIS[0]]: {
        registered: false,
        connection_state: "connected",
        systems: ["5G"],
      },
    });
    await renderSubscribers();
    await screen.findByText(IMSIS[0]);

    expect(cellOf(IMSIS[0], "status")).toHaveTextContent("Registering");
    expect(cellOf(IMSIS[0], "status")).toHaveTextContent("5G");
  });

  it("keeps the last radio on an idle subscriber", async () => {
    seedStatuses({
      [IMSIS[0]]: {
        registered: true,
        connection_state: "idle",
        last_seen_radio: "gnb-01",
      },
      [IMSIS[1]]: { registered: false },
    });
    await renderSubscribers();
    await screen.findByText(IMSIS[0]);

    expect(cellOf(IMSIS[0], "last_seen_radio")).toHaveTextContent("gnb-01");
    expect(cellOf(IMSIS[1], "last_seen_radio")).toHaveTextContent("—");
  });
});

describe("Subscribers MSISDN column", () => {
  const msisdnCell = async (imsi: string) => {
    const row = (await screen.findByText(imsi)).closest(
      '[role="row"]',
    ) as HTMLElement;
    return row.querySelector('[data-field="msisdn"]') as HTMLElement;
  };

  it("shows the MSISDN, or a dash without one", async () => {
    seedApi();
    await renderSubscribers();

    expect(await msisdnCell(IMSIS[0])).toHaveTextContent("+15551230001");
    expect(await msisdnCell(IMSIS[1])).toHaveTextContent("—");
  });
});

const ROAMERS_PATH = "/api/v1/roamers";

const ROAMER = "208930000000001";

const roamer = (imsi: string, overrides: Record<string, unknown> = {}) => ({
  imsi,
  partner: "Partner A",
  since: "2026-11-01T10:00:00Z",
  status: {
    registered: true,
    connection_state: "connected",
    systems: ["5G"],
    num_sessions: 2,
  },
  ...overrides,
});

const seedRoamers = (imsis = [ROAMER, "208940000000002"]) => {
  api.get(ROAMERS_PATH, ({ params }) => {
    const search = params.get("search") ?? "";
    const matched = imsis.filter((imsi) => imsi.includes(search));
    return {
      items: matched.map((imsi) => roamer(imsi)),
      page: 1,
      per_page: 25,
      total_count: matched.length,
    };
  });
};

const renderRoamers = async (role = "Admin") => {
  const result = renderWithProviders(<Subscribers />, {
    initialEntries: ["/subscribers/roamers"],
    auth: { role },
  });
  await screen.findByRole("heading", { level: 2, name: /^Roamers/ });
  return result;
};

const roamerRow = async (imsi: string) =>
  within(
    (await screen.findByText(imsi)).closest('[role="row"]') as HTMLElement,
  );

describe("Roamers tab", () => {
  it("lists roamers with their partner, status and sessions", async () => {
    seedRoamers();
    await renderRoamers();

    const row = await roamerRow(ROAMER);
    expect(row.getByText("Partner A")).toBeInTheDocument();
    expect(row.getByText("Connected")).toBeInTheDocument();
    expect(row.getByText("5G")).toBeInTheDocument();
    expect(
      screen
        .getByText(ROAMER)
        .closest('[role="row"]')
        ?.querySelector('[data-field="num_sessions"]'),
    ).toHaveTextContent("2");
    expect(row.getByRole("link", { name: ROAMER })).toHaveAttribute(
      "href",
      `/subscribers/roamers/${ROAMER}`,
    );
    expect(
      screen.getByRole("heading", { level: 2, name: "Roamers (2)" }),
    ).toBeInTheDocument();
    expect(api.requests(SUBSCRIBERS_PATH)).toHaveLength(0);
  });

  it("searches roamers", async () => {
    const user = userEvent.setup();
    seedRoamers();
    await renderRoamers();
    await screen.findByText(ROAMER);

    await user.type(screen.getByLabelText("Search"), "20894");

    await waitFor(
      () =>
        expect(api.lastRequest(ROAMERS_PATH)?.params.get("search")).toBe(
          "20894",
        ),
      { timeout: 2000 },
    );
    await waitFor(
      () => expect(screen.queryByText(ROAMER)).not.toBeInTheDocument(),
      { timeout: 2000 },
    );

    await user.clear(screen.getByLabelText("Search"));
    await user.type(screen.getByLabelText("Search"), "999");
    expect(
      await screen.findByText(
        "No roamers match your search",
        {},
        { timeout: 2000 },
      ),
    ).toBeInTheDocument();
  });

  it("says so when no roamer is registered", async () => {
    seedRoamers([]);
    await renderRoamers();

    expect(await screen.findByText("No roamers")).toBeInTheDocument();
    expect(screen.queryByLabelText("Search")).not.toBeInTheDocument();
  });

  it("deregisters a roamer after confirmation", async () => {
    const user = userEvent.setup();
    seedRoamers();
    api.delete(`${ROAMERS_PATH}/:imsi`, () => ({
      message: "Roamer deregistered successfully",
    }));
    await renderRoamers();

    await user.click(
      (await roamerRow(ROAMER)).getByRole("button", {
        name: `Deregister roamer ${ROAMER}`,
      }),
    );
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Deregister",
      }),
    );

    expect(
      await screen.findByText(`Roamer "${ROAMER}" deregistered successfully.`),
    ).toBeInTheDocument();
    expect(api.lastRequest(`${ROAMERS_PATH}/${ROAMER}`)?.method).toBe("DELETE");
  });

  it("shows why a deregistration was refused", async () => {
    const user = userEvent.setup();
    seedRoamers();
    api.delete(`${ROAMERS_PATH}/:imsi`, () =>
      httpError(409, "The roamer is still registered after one minute"),
    );
    await renderRoamers();

    await user.click(
      (await roamerRow(ROAMER)).getByRole("button", {
        name: `Deregister roamer ${ROAMER}`,
      }),
    );
    const dialog = screen.getByRole("dialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Deregister" }),
    );

    expect(
      await within(dialog).findByText(
        "The roamer is still registered after one minute",
      ),
    ).toBeInTheDocument();
  });

  it("hides the deregistration from read-only users", async () => {
    seedRoamers();
    await renderRoamers("Read Only");

    await screen.findByText(ROAMER);
    expect(
      screen.queryByRole("button", { name: /Deregister/ }),
    ).not.toBeInTheDocument();
  });
});

describe("Subscribers tabs", () => {
  it("counts the home subscribers in the Subscribers tab", async () => {
    seedApi();
    await renderSubscribers();

    expect(
      await screen.findByRole("heading", { level: 2, name: "Subscribers (3)" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Subscribers" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(lastParams().has("roaming")).toBe(false);
  });

  it("switches to the Roamers tab", async () => {
    const user = userEvent.setup();
    seedApi();
    seedRoamers();
    await renderSubscribers();

    await user.click(screen.getByRole("tab", { name: "Roamers" }));

    expect(screen.getByTestId(LOCATION_TEST_ID)).toHaveTextContent(
      "/subscribers/roamers",
    );
    expect(await screen.findByText(ROAMER)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Create" }),
    ).not.toBeInTheDocument();
  });
});
