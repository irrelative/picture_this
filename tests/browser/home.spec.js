import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("home page exposes the primary game actions", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Picture This/);
  await expect(page.getByRole("button", { name: "Join game" })).toBeVisible();
});

test("home page has no automatically detectable accessibility violations", async ({
  page,
}) => {
  await page.goto("/");
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});
