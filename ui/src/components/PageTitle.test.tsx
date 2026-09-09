// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import PageTitle from "./PageTitle";

const renderTitle = (ui: React.ReactNode) =>
  render(<MemoryRouter>{ui}</MemoryRouter>);

describe("PageTitle", () => {
  beforeEach(() => {
    document.title = "";
  });

  it("names the document after the heading it renders", () => {
    renderTitle(<PageTitle title="Audit Logs" />);

    expect(
      screen.getByRole("heading", { level: 1, name: "Audit Logs" }),
    ).toBeInTheDocument();
    expect(document.title).toBe("Audit Logs · Ella Core");
  });

  it("qualifies a detail page with its parent", () => {
    renderTitle(
      <PageTitle parent={{ label: "Radios", to: "/radios" }} title="gnb-001" />,
    );

    expect(screen.getByRole("link", { name: "Radios" })).toHaveAttribute(
      "href",
      "/radios",
    );
    expect(document.title).toBe("Radios / gnb-001 · Ella Core");
  });

  it("keeps a live count out of the document title", () => {
    renderTitle(<PageTitle title="Radios" count={12} />);

    expect(
      screen.getByRole("heading", { level: 1, name: "Radios (12)" }),
    ).toBeInTheDocument();
    expect(document.title).toBe("Radios · Ella Core");
  });

  it("lets a page whose heading is not its name override the title", () => {
    renderTitle(<PageTitle title="Ella Core" documentTitle="Dashboard" />);

    expect(document.title).toBe("Dashboard · Ella Core");
  });

  it("does not repeat the app name when the page is the app name", () => {
    renderTitle(<PageTitle title="Ella Core" />);

    expect(document.title).toBe("Ella Core");
  });

  it("retitles when the name changes", () => {
    const { rerender } = renderTitle(<PageTitle title="Profiles" />);
    expect(document.title).toBe("Profiles · Ella Core");

    rerender(
      <MemoryRouter>
        <PageTitle title="Users" />
      </MemoryRouter>,
    );
    expect(document.title).toBe("Users · Ella Core");
  });
});
