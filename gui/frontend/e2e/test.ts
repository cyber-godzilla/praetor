import { test as base, expect, type Locator, type Page } from "@playwright/test";
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
      const text = msg.text();
      if (text.includes("fake-backend: unhandled") || text.includes("fake-backend: unhandledrejection")) {
        problems.push(text);
      }
    });
    const backend = await installFakeBackend(page, init);
    await use(backend);
    expect(problems, "page errors / unhandled GuiApp calls").toEqual([]);
  },
});

export { expect };

// Playwright's pointer-driven dragTo can drop the HTML drag session when it
// scrolls Layout's nested modal body to reveal a sidebar. Dispatch the browser
// drag events with one DataTransfer instead: this exercises the same production
// handlers and remains stable whether the source and destination share a fold.
export async function dragAndDrop(page: Page, source: Locator, destination: Locator) {
  const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
  try {
    await source.dispatchEvent("dragstart", { dataTransfer });
    await destination.dispatchEvent("dragenter", { dataTransfer });
    await destination.dispatchEvent("dragover", { dataTransfer });
    await destination.dispatchEvent("drop", { dataTransfer });
  } finally {
    await dataTransfer.dispose();
  }
}
