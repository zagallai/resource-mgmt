import { test, expect } from "@playwright/test";
test("email sign-in → contributors → hold → resume → reassignment", async ({
  page,
  request,
}) => {
  const suffix = Date.now();
  const today = new Date().toISOString().slice(0, 10);
  const future = new Date(Date.now() + 86400000).toISOString().slice(0, 10);
  const email = process.env.E2E_ADMIN_EMAIL || "admin@example.com";
  await page.goto("/");
  await page.getByLabel("Work email").fill(email);
  await page.getByRole("button", { name: "Email me a sign-in link" }).click();
  await expect(page.getByRole("status")).toContainText("If invited");
  let link = "";
  await expect
    .poll(async () => {
      const inbox = await request.get(
        (process.env.MAIL_URL || "http://127.0.0.1:8025") + "/api/v1/messages",
      );
      const { messages } = await inbox.json();
      const message = messages.find((m: { To: { Address: string }[] }) =>
        m.To.some((t) => t.Address === email),
      );
      if (!message) return "";
      const detail = await request.get(
        (process.env.MAIL_URL || "http://127.0.0.1:8025") +
          "/api/v1/message/" +
          message.ID,
      );
      link =
        (await detail.json()).Text.match(
          /http[^\s]+\/verify#token=[a-f0-9]+/,
        )?.[0] || "";
      return link;
    })
    .not.toBe("");
  await page.goto(link);
  await page.getByRole("button", { name: "Confirm sign-in" }).click();
  await expect(
    page.getByRole("heading", { name: "Overview", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "People & projects", exact: true })
    .click();
  const people = page
    .locator("section")
    .filter({
      has: page.getByRole("heading", { name: "People", exact: true }),
    });
  for (const person of ["Alex", "Priya"]) {
    await people.getByLabel("Name", { exact: true }).fill(person + suffix);
    await people.getByLabel("Daily capacity (hours)").fill("8");
    await people.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Saved");
  }
  const projects = page
    .locator("section")
    .filter({
      has: page.getByRole("heading", { name: "Projects", exact: true }),
    });
  await projects.getByLabel("Project name").fill("Payments" + suffix);
  await projects.getByLabel("Budget in USD cents").fill("1000000");
  await projects.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page
    .getByRole("button", { name: "Administration", exact: true })
    .click();
  const rates = page
    .locator("section")
    .filter({
      has: page.getByRole("heading", { name: "Add historical rate" }),
    });
  for (const [person, cents] of [
    ["Alex", "10000"],
    ["Priya", "5000"],
  ]) {
    await rates
      .getByLabel("Resource", { exact: true })
      .selectOption({ label: person + suffix });
    await rates.getByLabel("Effective from").fill(today);
    await rates.getByLabel("USD cents per hour").fill(cents);
    await rates.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Saved");
  }
  await page.getByRole("button", { name: "Planning", exact: true }).click();
  const sprint = page
    .locator("section")
    .filter({
      has: page.getByRole("heading", { name: "Create sprint", exact: true }),
    });
  await sprint.getByLabel("Sprint name").fill("Sprint" + suffix);
  await sprint.getByLabel("Start date").fill(today);
  await sprint.getByLabel("End date").fill(future);
  await sprint.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByRole("button", { name: "Tasks", exact: true }).click();
  const form = page
    .locator("section")
    .filter({
      has: page.getByRole("heading", { name: "Create task", exact: true }),
    });
  await form
    .getByLabel("Project", { exact: true })
    .selectOption({ label: "Payments" + suffix });
  await form.getByLabel("Jira key").fill("PAY-" + suffix);
  await form.getByLabel("Task title").fill("Build payment validation");
  await form
    .getByLabel("Primary owner", { exact: true })
    .selectOption({ label: "Alex" + suffix });
  await form.getByLabel("Estimated hours").fill("24");
  await form.getByLabel("Start date").fill(today);
  await form.getByLabel("Target date").fill(future);
  await form
    .getByLabel("Sprint", { exact: true })
    .selectOption({ label: "Sprint" + suffix });
  await form.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByRole("button", { name: "Planning", exact: true }).click();
  const schedule = page
    .locator("section")
    .filter({
      has: page.getByRole("heading", { name: "Schedule a contributor" }),
    });
  for (const person of ["Alex", "Priya"]) {
    await schedule
      .getByLabel("Task", { exact: true })
      .selectOption({ label: "PAY-" + suffix });
    await schedule
      .getByLabel("Contributor")
      .selectOption({ label: person + suffix });
    await schedule.getByLabel("Work date").fill(future);
    await schedule.getByLabel("Hours", { exact: true }).fill("6");
    await schedule.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Saved");
  }
  const actual = page
    .locator("section")
    .filter({ has: page.getByRole("heading", { name: "Record actual work" }) });
  await actual
    .getByLabel("Task", { exact: true })
    .selectOption({ label: "PAY-" + suffix });
  await actual
    .getByLabel("Contributor")
    .selectOption({ label: "Alex" + suffix });
  await actual.getByLabel("Work date").fill(today);
  await actual.getByLabel("Hours", { exact: true }).fill("2");
  await actual.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  const costRow = page
    .getByRole("row")
    .filter({ hasText: "Payments" + suffix });
  await expect(costRow).toContainText("$200.00");
  await expect(costRow).toContainText("$900.00");
  await page.getByRole("button", { name: "Tasks", exact: true }).click();
  const card = page
    .locator("article.task")
    .filter({ hasText: "PAY-" + suffix });
  await card.getByLabel("Status", { exact: true }).selectOption("hold");
  await card.getByLabel("Reason for change").fill("Dependency blocked");
  await card.getByRole("button", { name: "Save", exact: true }).first().click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.reload();
  await page.getByRole("button", { name: "Activity", exact: true }).click();
  const historyRows = page
    .locator("section")
    .filter({
      has: page.getByRole("heading", {
        name: "Allocation history",
        exact: true,
      }),
    })
    .getByRole("row")
    .filter({ hasText: "PAY-" + suffix });
  await expect(historyRows).toHaveCount(2);
  for (const row of await historyRows.all()) {
    await expect(row).toContainText("Dependency blocked");
    await expect(row).toContainText("Released");
  }
  await page.getByRole("button", { name: "Tasks", exact: true }).click();
  await card.getByLabel("Status", { exact: true }).selectOption("active");
  await card.getByLabel("Reason for change").fill("Dependency resolved");
  await card.getByRole("button", { name: "Save", exact: true }).first().click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await card.getByText("Change primary owner", { exact: true }).click();
  await card.getByLabel("New owner").selectOption({ label: "Priya" + suffix });
  await card.getByLabel("Reassignment reason").fill("Capacity handoff");
  await card.getByRole("button", { name: "Save", exact: true }).last().click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await expect(card).toContainText("Priya" + suffix);
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "Payments" + suffix }),
  ).toContainText("$200.00");
  await page.screenshot({ path: "../../docs/dashboard.png", fullPage: true });
});
