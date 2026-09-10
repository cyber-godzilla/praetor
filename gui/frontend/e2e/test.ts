import { test as base, expect } from "@playwright/test";
import type { InitState } from "../src/lib/types";
import { installFakeBackend, type FakeBackend } from "./fake-backend";
import { baseInit } from "./fixtures";

type Fixtures = {
  init: InitState;
  backend: FakeBackend;
};

// `init` is an option: a spec overrides it with test.use({ init: withAccounts }).
// `backend` installs the fake before navigation and, on teardown, fails the
// test if the page threw or the fake saw a GuiApp call it does not know.
export const test = base.extend<Fixtures>({
  init: [baseInit, { option: true }],
  backend: async ({ page, init }, use) => {
    const problems: string[] = [];
    page.on("pageerror", (err) => problems.push(`pageerror: ${err.message}`));
    page.on("console", (msg) => {
      if (msg.text().includes("fake-backend: unhandled")) problems.push(msg.text());
    });
    const backend = await installFakeBackend(page, init);
    await use(backend);
    expect(problems, "page errors / unhandled GuiApp calls").toEqual([]);
  },
});

export { expect };
