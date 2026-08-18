// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// Date formatting is timezone-dependent, so a fixed zone is the only way these
// assertions mean the same thing on a developer machine and in CI.
process.env.TZ = "UTC";

// Testing Library only auto-cleans when Vitest runs with `globals: true`, which
// this project does not enable. Without this, a mounted component from one test
// stays in the document and the next test's queries match it too.
afterEach(cleanup);
