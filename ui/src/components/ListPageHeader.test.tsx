// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import ListPageHeader from "./ListPageHeader";

const props = { title: "Subscribers", description: "Manage subscribers." };

describe("ListPageHeader", () => {
  it("shows the bare title while the count is unknown", () => {
    render(<ListPageHeader {...props} />);

    expect(
      screen.getByRole("heading", { name: "Subscribers" }),
    ).toBeInTheDocument();
  });

  it("appends the count once it is known, including zero", () => {
    const { rerender } = render(<ListPageHeader {...props} count={7} />);
    expect(
      screen.getByRole("heading", { name: "Subscribers (7)" }),
    ).toBeInTheDocument();

    rerender(<ListPageHeader {...props} count={0} />);
    expect(
      screen.getByRole("heading", { name: "Subscribers (0)" }),
    ).toBeInTheDocument();
  });

  it("uses an h1 for a page and an h2 for a nested section", () => {
    const { rerender } = render(<ListPageHeader {...props} />);
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();

    rerender(<ListPageHeader {...props} variant="section" />);
    expect(screen.getByRole("heading", { level: 2 })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
  });

  it("renders filters and the action together", () => {
    render(
      <ListPageHeader
        {...props}
        filters={<input aria-label="Search" />}
        action={<button>Create</button>}
      />,
    );

    expect(screen.getByLabelText("Search")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create" })).toBeInTheDocument();
  });

  it("renders the action alone when there are no filters", () => {
    render(<ListPageHeader {...props} action={<button>Create</button>} />);

    expect(screen.getByRole("button", { name: "Create" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Search")).not.toBeInTheDocument();
  });

  // A read-only page with no filters must not leave an empty toolbar row.
  it("omits the toolbar row when neither slot is filled", () => {
    render(<ListPageHeader {...props} filters={false} action={false} />);

    expect(screen.queryByTestId("list-page-toolbar")).not.toBeInTheDocument();
  });

  it("renders the toolbar row when only one slot is filled", () => {
    render(<ListPageHeader {...props} action={<button>Create</button>} />);

    expect(screen.getByTestId("list-page-toolbar")).toBeInTheDocument();
  });
});
