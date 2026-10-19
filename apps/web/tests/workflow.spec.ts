import { test, expect } from "@playwright/test";
test("email sign-in → contributors → hold → resume → reassignment", async ({
  page,
  request,
}) => {
  const suffix = Date.now();
  const today = new Date().toISOString().slice(0, 10);
  const future = new Date(Date.now() + 86400000).toISOString().slice(0, 10);
  const email = process.env.E2E_ADMIN_EMAIL || "admin@resourcemgmt.com";
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
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  const createPerson = page.getByRole("form", {
    name: "Add resource",
    exact: true,
  });
  for (const [person, rate] of [
    ["Alex", "100"],
    ["Priya", "50"],
  ]) {
    await createPerson
      .getByLabel("Resource name", { exact: true })
      .fill(person + suffix);
    await createPerson
      .getByLabel("Type", { exact: true })
      .selectOption(person === "Alex" ? "onshore" : "offshore");
    await createPerson.getByLabel("Company name").fill("Engineering Partner");
    await expect(createPerson.getByLabel("Working hours per day")).toHaveValue(
      "8",
    );
    await createPerson.getByLabel("Hourly rate", { exact: true }).fill(rate);
    await createPerson.getByLabel("Rate effective from").fill(today);
    await createPerson
      .getByRole("button", { name: "Add resource", exact: true })
      .click();
    await expect(page.getByRole("status")).toContainText("Saved");
  }
  const alex = page
    .locator("details.management-card")
    .filter({ hasText: "Alex" + suffix });
  await alex.locator("summary").click();
  const editPerson = alex.getByRole("form", {
    name: "Edit resource Alex" + suffix,
  });
  await editPerson.getByLabel("Working hours per day").fill("6");
  await editPerson.getByRole("button", { name: "Save resource" }).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.reload();
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  await alex.locator("summary").click();
  await expect(editPerson.getByLabel("Working hours per day")).toHaveValue("6");
  await expect(alex).toContainText("Engineering Partner");
  await page.screenshot({ path: "../../docs/resources.png", fullPage: false });
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  const newProject = page.getByRole("form", {
    name: "Add project",
    exact: true,
  });
  await newProject.getByLabel("Project name").fill("Payments" + suffix);
  await newProject.getByLabel("Project start date").fill(today);
  await newProject.getByLabel("Project end date").fill(future);
  await newProject.getByLabel("PM name").fill("Morgan Lee");
  await newProject
    .getByLabel("Prime resource")
    .selectOption({ label: "Alex" + suffix });
  const firstDraft = newProject.getByRole("group", {
    name: "Activity row 1",
    exact: true,
  });
  await expect(
    firstDraft.getByLabel("Activity name", { exact: true }),
  ).toHaveValue("");
  await firstDraft
    .getByLabel("Activity name", { exact: true })
    .fill("Release kickoff");
  await firstDraft.getByLabel("Start date", { exact: true }).fill(today);
  await firstDraft.getByLabel("End date", { exact: true }).fill(future);
  await newProject
    .getByRole("button", { name: "Add activity row", exact: true })
    .click();
  await newProject
    .getByRole("group", { name: "Activity row 2", exact: true })
    .getByLabel("Activity name", { exact: true })
    .fill("One-off architecture review");
  await newProject
    .getByRole("button", { name: "Add activity row", exact: true })
    .click();
  await newProject
    .getByRole("group", { name: "Activity row 3", exact: true })
    .getByLabel("Activity name", { exact: true })
    .fill("Do not save");
  await newProject
    .getByRole("button", { name: "Remove activity row 3", exact: true })
    .click();
  await expect(
    newProject.getByRole("group", { name: /^Activity row / }),
  ).toHaveCount(2);
  await newProject
    .getByRole("button", { name: "Add project", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  const projectForm = page.getByRole("form", {
    name: "Edit project",
    exact: true,
  });
  const activityForm = page.getByRole("form", {
    name: "Project activity",
    exact: true,
  });
  await expect(activityForm).toHaveCount(0);
  await expect(
    page.locator("details.management-card").filter({
      has: page.locator("summary", { hasText: "Payments" + suffix }),
    }),
  ).toContainText("Morgan Lee");
  await page.getByRole("button", { name: /^Manage activities/ }).click();
  await expect(activityForm).toHaveCount(1);
  await expect(
    page.getByLabel("Choose activity").locator("option"),
  ).toHaveCount(3);
  await expect(
    page.getByRole("table", { name: "Saved project activities" }),
  ).toContainText("Release kickoff");
  await expect(
    page.getByRole("table", { name: "Saved project activities" }),
  ).toContainText("One-off architecture review");
  await expect(
    page.getByRole("table", { name: "Saved project activities" }),
  ).not.toContainText("Do not save");
  await activityForm.getByLabel("Activity name").fill("LLD");
  await activityForm.getByLabel("Start date", { exact: true }).fill(today);
  await activityForm.getByLabel("End date", { exact: true }).fill(future);
  await activityForm
    .getByRole("button", { name: "Add activity", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await expect(activityForm).toHaveCount(1);
  const savedActivities = page.getByRole("table", {
    name: "Saved project activities",
    exact: true,
  });
  await expect(
    savedActivities.getByRole("row").filter({ hasText: "LLD" }),
  ).toContainText(future);
  await page.getByLabel("Choose activity").selectOption("");
  await savedActivities
    .getByRole("button", { name: "Edit activity LLD", exact: true })
    .click();
  await expect(activityForm.getByLabel("Activity name")).toHaveValue("LLD");
  await activityForm.getByLabel("End date", { exact: true }).fill(today);
  await activityForm
    .getByRole("button", { name: "Update activity", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await activityForm.getByLabel("End date", { exact: true }).fill(future);
  await activityForm
    .getByRole("button", { name: "Update activity", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByLabel("Choose activity").selectOption("");
  await activityForm.getByLabel("Activity name").fill("PTE");
  await activityForm
    .getByRole("button", { name: "Add activity", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await expect(page.getByLabel("Include in plan")).toHaveCount(0);
  await savedActivities
    .getByRole("button", { name: "Delete activity PTE", exact: true })
    .click();
  await expect(
    savedActivities.getByRole("row").filter({ hasText: "PTE" }),
  ).toHaveCount(0);
  await expect(activityForm.getByLabel("Activity name")).toHaveValue("");
  await page.getByLabel("Choose activity").selectOption("");
  await activityForm.getByLabel("Activity name").fill("Penetration testing");
  await activityForm.getByLabel("Start date", { exact: true }).fill(today);
  await activityForm.getByLabel("End date", { exact: true }).fill(future);
  await activityForm
    .getByRole("button", { name: "Add activity", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByRole("button", { name: /^Hide activities/ }).click();
  await expect(activityForm).toHaveCount(0);
  await projectForm.getByLabel("Closed date", { exact: true }).fill(today);
  await projectForm
    .getByRole("button", { name: "Save project", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.reload();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await expect(projectForm).toHaveCount(0);
  await page
    .locator("details.management-card > summary")
    .filter({ hasText: "Payments" + suffix })
    .click();
  const projectSummary = page
    .locator("details.management-card > summary")
    .filter({ hasText: "Payments" + suffix });
  await expect(
    page
      .locator("details.management-card[open]")
      .getByRole("form", { name: "Edit project", exact: true }),
  ).toBeVisible();
  await projectSummary.click();
  await expect(projectForm).toHaveCount(0);
  await projectSummary.click();
  await expect(projectForm.getByLabel("PM name")).toHaveValue("Morgan Lee");
  await expect(
    projectForm.getByLabel("Closed date", { exact: true }),
  ).toHaveValue(today);
  await expect(
    projectForm.getByLabel("Prime resource").locator("option:checked"),
  ).toHaveText("Alex" + suffix);
  await page.getByRole("button", { name: /^Manage activities/ }).click();
  await page.getByLabel("Choose activity").selectOption({ label: "LLD" });
  await expect(
    activityForm.getByLabel("Start date", { exact: true }),
  ).toHaveValue(today);
  await expect(
    activityForm.getByLabel("End date", { exact: true }),
  ).toHaveValue(future);
  await projectForm
    .getByLabel("Prime resource")
    .selectOption({ label: "Priya" + suffix });
  await projectForm
    .getByRole("button", { name: "Save project", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.reload();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page
    .locator("details.management-card > summary")
    .filter({ hasText: "Payments" + suffix })
    .click();
  await expect(
    projectForm.getByLabel("Prime resource").locator("option:checked"),
  ).toHaveText("Priya" + suffix);
  await page.getByRole("button", { name: /^Manage activities/ }).click();
  await page
    .getByLabel("Choose activity")
    .selectOption({ label: "Penetration testing" });
  await expect(
    activityForm.getByLabel("Start date", { exact: true }),
  ).toHaveValue(today);
  await savedActivities
    .getByRole("button", {
      name: "Delete activity Penetration testing",
      exact: true,
    })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await savedActivities
    .getByRole("button", { name: "Delete activity LLD", exact: true })
    .click();
  await expect(savedActivities).toContainText("Release kickoff");
  await expect(savedActivities).toContainText("One-off architecture review");
  await savedActivities
    .getByRole("button", {
      name: "Delete activity Release kickoff",
      exact: true,
    })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await savedActivities
    .getByRole("button", {
      name: "Delete activity One-off architecture review",
      exact: true,
    })
    .click();
  await expect(savedActivities).toHaveCount(0);
  await expect(
    page.getByText("No activities yet. Add one when you are ready."),
  ).toBeVisible();
  await expect(
    page.getByLabel("Choose activity").locator("option"),
  ).toHaveCount(1);
  await expect(activityForm).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: /^Hide activities/ }),
  ).toHaveText("Hide activities (0)");
  await page.getByRole("button", { name: /^Hide activities/ }).click();
  await page.getByLabel("Search projects").fill("Payments" + suffix);
  await page.screenshot({ path: "../../docs/projects.png", fullPage: true });
  await page.getByRole("button", { name: "Planning", exact: true }).click();
  const sprint = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Create sprint", exact: true }),
  });
  await sprint.getByLabel("Project",{exact:true}).selectOption({label:"Payments"+suffix});
  await sprint.getByLabel("Sprint name").fill("Sprint" + suffix);
  await sprint.getByLabel("Start date").fill(today);
  await sprint.getByLabel("End date").fill(future);
  await sprint.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  const sprintCard = page.locator("details.management-card").filter({hasText:"Sprint" + suffix});
  await expect(sprintCard).toBeVisible();
  await sprintCard.locator("summary").click();
  await sprintCard.getByLabel("Sprint name").fill("Sprint" + suffix + " updated");
  await sprintCard.getByRole("button",{name:"Save sprint"}).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByRole("button", { name: "Tasks", exact: true }).click();
  const form = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Create task", exact: true }),
  });
  await form
    .getByLabel("Project", { exact: true })
    .selectOption({ label: "Payments" + suffix });
  await form.getByLabel("Jira key (optional)").fill("PAY-" + suffix);
  await form.getByLabel("Task title").fill("Build payment validation");
  await form
    .getByLabel("Primary owner", { exact: true })
    .selectOption({ label: "Alex" + suffix });
  await form.getByLabel("Estimated days").fill("3");
  await form.getByLabel("Start date").fill(today);
  await form.getByLabel("Target date").fill(future);
  await form
    .getByLabel("Sprint", { exact: true })
    .selectOption({ label: "Sprint" + suffix + " updated" });
  await form.getByLabel("Target date").fill("2099-01-01");
  await form.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Task dates must stay within");
  await expect(form.getByLabel("Task title")).toHaveValue("Build payment validation");
  await form.getByLabel("Target date").fill(future);
  await form.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  const taskCard = page.locator("article.task").filter({hasText:"PAY-" + suffix});
  await taskCard.getByText("Allocate resource time", {exact:true}).click();
  const schedule = taskCard.getByRole("region",{name:"Allocate resource time"});
  await expect(schedule.getByLabel("Resource",{exact:true})).toHaveValue((await form.getByLabel("Primary owner").locator("option").filter({hasText:"Alex"+suffix}).getAttribute("value"))!);
  for (const person of ["Alex", "Priya"]) {
    await schedule.getByLabel("Resource",{exact:true}).selectOption({label:person+suffix});
    await schedule.getByLabel("Work date").fill(future);
    await schedule.getByLabel("Days",{exact:true}).fill("0.75");
    await schedule.getByRole("button",{name:"Allocate time",exact:true}).click();
    await expect(page.getByRole("status")).toContainText("Saved");
  }
  await taskCard.getByText("Record actual work",{exact:true}).click();
  const actual = taskCard.getByRole("region",{name:"Record actual work"});
  await actual.getByLabel("Contributor").selectOption({label:"Alex"+suffix});
  await actual.getByLabel("Work date").fill(today);
  await actual.getByLabel("Hours",{exact:true}).fill("2");
  await actual.getByRole("button",{name:"Save",exact:true}).click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  await page.getByLabel("Workload date").fill(future);
  const availability = page.locator("section").filter({has:page.getByRole("heading",{name:"Team availability",exact:true})});
  await expect(availability.getByRole("row").filter({hasText:"Alex"+suffix})).toContainText("Fully booked");
  await expect(availability.getByRole("row").filter({hasText:"Alex"+suffix})).toContainText("PAY-"+suffix);
  await expect(page.getByRole("heading",{name:"Project costs",exact:true})).toHaveCount(0);
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
  await expect(page.getByRole("heading",{name:"Project costs",exact:true})).toHaveCount(0);
  await expect(page.locator("body")).not.toContainText(/USD|cents|\$/);
  await page.screenshot({ path: "../../docs/dashboard.png", fullPage: true });
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  const archiveCard = page
    .locator("details.management-card")
    .filter({ has: page.locator("summary", { hasText: "Alex" + suffix }) });
  await archiveCard.locator("summary").click();
  await archiveCard
    .getByRole("button", { name: "Archive resource", exact: true })
    .click();
  await expect(archiveCard).toHaveCount(0);
  await page.getByLabel("Show archived resources").check();
  await archiveCard.locator("summary").click();
  await expect(archiveCard).toContainText("Archived");
  await page.reload();
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  await expect(archiveCard).toHaveCount(0);
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await expect(
    newProject
      .getByLabel("Prime resource")
      .getByRole("option", { name: "Alex" + suffix, exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Activity", exact: true }).click();
  await expect(
    page.getByRole("cell", { name: "Alex" + suffix, exact: true }).first(),
  ).toBeVisible();
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  await page.getByLabel("Show archived resources").check();
  await archiveCard.locator("summary").click();
  await archiveCard
    .getByRole("button", { name: "Restore resource", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await page.getByLabel("Show archived resources").uncheck();
  await expect(archiveCard).toBeVisible();

  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page
    .locator("details.management-card > summary")
    .filter({ hasText: "Payments" + suffix })
    .click();
  await projectForm
    .getByRole("button", { name: "Delete project", exact: true })
    .click();
  const deletionDialog = page.getByRole("dialog", {name:"Delete project"});
  await expect(deletionDialog).toContainText("linked Jira tasks");
  await expect(deletionDialog.getByRole("button", {name:"Confirm deletion"})).toBeDisabled();
  await deletionDialog.getByRole("button", {name:"Cancel", exact:true}).click();
  await expect(projectForm).toBeVisible();
  await newProject.getByLabel("Project name").fill("Disposable" + suffix);
  await newProject.getByLabel("Project start date").fill(today);
  await newProject.getByLabel("Project end date").fill(future);
  await newProject.getByLabel("PM name").fill("Test PM");
  await newProject
    .getByLabel("Prime resource")
    .selectOption({ label: "Alex" + suffix });
  await newProject
    .getByRole("group", { name: "Activity row 1", exact: true })
    .getByLabel("Activity name", { exact: true })
    .fill("Project-only activity");
  await newProject
    .getByRole("button", { name: "Add project", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  await expect(projectForm.getByLabel("Project name")).toHaveValue(
    "Disposable" + suffix,
  );
  await projectForm
    .getByRole("button", { name: "Delete project", exact: true })
    .click();
  await deletionDialog.getByLabel("Type the project name to confirm").fill("Disposable" + suffix);
  await deletionDialog.getByRole("button", {name:"Confirm deletion"}).click();
  await expect(projectForm).toHaveCount(0);
  await page.reload();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await expect(
    page
      .locator("details.management-card > summary")
      .filter({ hasText: "Disposable" + suffix }),
  ).toHaveCount(0);
  await expect(
    page
      .locator("details.management-card > summary")
      .filter({ hasText: "Payments" + suffix }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  const newResource = page.getByRole("form", {
    name: "Add resource",
    exact: true,
  });
  await newResource
    .getByLabel("Resource name", { exact: true })
    .fill("Unused" + suffix);
  await newResource.getByLabel("Type", { exact: true }).selectOption("onshore");
  await newResource.getByLabel("Company name").fill("Test company");
  await newResource.getByLabel("Hourly rate", { exact: true }).fill("75");
  await newResource
    .getByRole("button", { name: "Add resource", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Saved");
  const unusedCard = page
    .locator("details.management-card")
    .filter({ has: page.locator("summary", { hasText: "Unused" + suffix }) });
  await unusedCard.locator("summary").click();
  await unusedCard
    .getByRole("button", { name: "Delete resource", exact: true })
    .click();
  await expect(unusedCard).toHaveCount(0);
  await page.getByLabel("Show archived resources").check();
  await expect(unusedCard).toHaveCount(0);
  await page.reload();
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  await expect(unusedCard).toHaveCount(0);
  const protectedCard = page
    .locator("details.management-card")
    .filter({ has: page.locator("summary", { hasText: "Alex" + suffix }) });
  await protectedCard.locator("summary").click();
  await protectedCard
    .getByRole("button", { name: "Delete resource", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Archive it instead");
  await expect(protectedCard).toBeVisible();
});
