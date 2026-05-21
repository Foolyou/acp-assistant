import { expect, test } from "@playwright/test";

const consoleURL = process.env.ACPA_CONSOLE_URL || "http://127.0.0.1:43791/";

for (const [name, viewport] of [
  ["mobile", { width: 390, height: 844 }],
  ["desktop", { width: 1280, height: 820 }],
]) {
  test(`console ${name} dashboard renders without overflow`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await page.goto(consoleURL, { waitUntil: "networkidle" });

    await expect(page.getByRole("heading", { name: "ACPA Console" })).toBeVisible();
    await expect(page.getByText("Daemon running")).toBeVisible();
    await expect(page.getByRole("button", { name: "+ New" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Assistants" })).toBeVisible();

    const metrics = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
      bodyText: document.body.innerText.trim().length,
    }));
    expect(metrics.bodyText).toBeGreaterThan(100);
    expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.clientWidth + 1);
  });
}

test("console create sheet and doctor entry are reachable", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(consoleURL, { waitUntil: "networkidle" });
  await page.getByRole("button", { name: "+ New" }).click();
  await expect(page.getByRole("dialog", { name: "Create assistant" })).toBeVisible();
  await page.getByRole("button", { name: "Close" }).click();
  await expect(page.getByRole("button", { name: "Doctor" }).first()).toBeVisible();
});

test("feishu qr setup waits for explicit completion", async ({ page }) => {
  let completeCalls = 0;
  await page.route("**/api/setup/feishu/qr/begin", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        QRURL: "https://open.feishu.cn/page/launcher?user_code=ABCD-EFGH",
        UserCode: "ABCD-EFGH",
        DeviceCode: "device-test",
        Domain: "feishu",
        Interval: 5,
        ExpireIn: 600,
      }),
    });
  });
  await page.route("**/api/setup/feishu/qr/complete", async (route) => {
    completeCalls += 1;
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: "feishu-main" }),
    });
  });
  await page.route("**/api/assistants/**/restart", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: "crawl", running: true }),
    });
  });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(consoleURL, { waitUntil: "networkidle" });
  await page.getByRole("button", { name: "Feishu" }).first().click();
  await page.getByRole("button", { name: "Start QR Setup" }).click();

  await expect(page.getByRole("link", { name: "Open Feishu setup" })).toBeVisible();
  await expect(page.locator(".code-box strong")).toHaveText("ABCD-EFGH");
  await expect(page.getByRole("button", { name: "Check and Save Channel" })).toBeVisible();
  expect(completeCalls).toBe(0);

  await page.getByRole("button", { name: "Check and Save Channel" }).click();
  await expect(page.getByText("Feishu channel saved: feishu-main. Assistant restarted.")).toBeVisible();
  expect(completeCalls).toBe(1);
});
