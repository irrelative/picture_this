import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("home page exposes the primary game actions", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Picture This/);
  await expect(page.getByRole("button", { name: "Join game" })).toBeVisible();
});

test("signup and lobby forms expose their defaults and constraints", async ({ page }) => {
  await page.goto("/");
  const password = page.locator('#registerForm input[name="password"]');
  await expect(password).toHaveAttribute("minlength", "8");
  await expect(password).toHaveAttribute("maxlength", "128");
  await expect(page.getByText("Use 8–128 characters.")).toBeVisible();
  await expect(page.locator('#createGameForm input[name="min_players"]')).toHaveValue("3");
  await expect(page.locator('#createGameForm input[name="max_players"]')).toHaveValue("8");
});

test("home page has no automatically detectable accessibility violations", async ({
  page,
}) => {
  await page.goto("/");
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});
