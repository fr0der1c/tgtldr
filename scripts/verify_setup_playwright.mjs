import fs from "node:fs/promises";
import path from "node:path";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { chromium } = require("/workspace/web/node_modules/playwright");

const outputDir = "/workspace/artifacts/playwright";
const setupURL = "http://web:3000/setup";

await fs.mkdir(outputDir, { recursive: true });

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

async function screenshot(page, filename) {
  const target = path.join(outputDir, filename);
  await page.screenshot({ path: target, fullPage: true });
  console.log(`saved screenshot: ${target}`);
}

async function runDesktopChecks(browser) {
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1200 }
  });
  const page = await context.newPage();

  await page.goto(setupURL, { waitUntil: "networkidle" });
  await page.waitForSelector("h1");

  const title = await page.locator("h1").textContent();
  assert(title?.includes("首次启动向导"), "setup 页面标题不正确");

  const stepLabels = await page.locator(".setup-step strong").allTextContents();
  assert(stepLabels.length === 3, "stepper 步骤数量不为 3");
  assert(
    JSON.stringify(stepLabels) === JSON.stringify(["基础配置", "登录", "Bot 推送"]),
    `stepper 标签不符合预期: ${stepLabels.join(", ")}`
  );

  const cards = await page.locator(".setup-stack > .card").count();
  assert(cards === 2, `setup 页面当前卡片数量应为 2，实际为 ${cards}`);

  const botDisabled = await page
    .locator(".setup-step", { hasText: "Bot 推送" })
    .isDisabled();
  assert(botDisabled, "未登录状态下 Bot 推送步骤应为禁用");

  await screenshot(page, "setup-desktop-initial.png");

  await page.getByRole("button", { name: "基础配置" }).click();
  await page.waitForSelector("text=OpenAI API Key");
  assert(
    await page.getByText("基础配置").isVisible(),
    "切换到基础配置步骤后未显示对应内容"
  );
  await screenshot(page, "setup-desktop-config.png");

  await page.getByRole("button", { name: "登录" }).click();
  await page.waitForSelector('input[placeholder="+8613800138000"]');
  assert(
    await page.getByText("输入你的手机号").isVisible(),
    "登录步骤未显示手机号输入视图"
  );
  assert(
    !(await page.getByText("同步群组列表").isVisible().catch(() => false)),
    "登录步骤仍然出现旧版复杂按钮"
  );
  await screenshot(page, "setup-desktop-login-phone.png");

  await page.getByRole("button", { name: "继续" }).click();
  await page.waitForSelector('[role="alert"]', { timeout: 15000 });
  const alertText = await page.locator('[role="alert"]').textContent();
  assert(alertText && alertText.trim().length > 0, "错误提示没有以文本形式呈现");
  await screenshot(page, "setup-desktop-login-error.png");

  const progressWidth = await page
    .locator(".setup-progress-fill")
    .evaluate((node) => node.getAttribute("style") || "");
  assert(progressWidth.includes("%"), "进度条没有宽度样式");

  await context.close();
}

async function runMobileChecks(browser) {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true
  });
  const page = await context.newPage();

  await page.goto(setupURL, { waitUntil: "networkidle" });
  await page.waitForSelector("h1");
  await screenshot(page, "setup-mobile-initial.png");

  const viewportFits = await page.evaluate(() => {
    const doc = document.documentElement;
    return doc.scrollWidth <= window.innerWidth + 1;
  });
  assert(viewportFits, "移动端存在水平溢出");

  const continueVisible = await page.getByRole("button", { name: "继续" }).isVisible();
  assert(continueVisible, "移动端登录页主按钮不可见");

  await context.close();
}

const browser = await chromium.launch({ headless: true });

try {
  await runDesktopChecks(browser);
  await runMobileChecks(browser);
  console.log("playwright verification passed");
} finally {
  await browser.close();
}
