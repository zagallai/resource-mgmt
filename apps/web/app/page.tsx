"use client";
import { useEffect, useState, FormEvent } from "react";
type Row = Record<string, string | number | boolean | null>;
type Org = { id: string; name: string; role: string };
const collections = [
  "resources",
  "projects",
  "tasks",
  "sprints",
  "allocations",
  "capacity",
  "daily-capacity",
  "worklogs",
];
export default function Home() {
  const [orgs, setOrgs] = useState<Org[]>([]),
    [org, setOrg] = useState(""),
    [role, setRole] = useState(""),
    [loaded, setLoaded] = useState(false),
    [data, setData] = useState<Record<string, Row[]>>({}),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false),
    [tab, setTab] = useState("Overview");
  useEffect(() => {
    fetch("/api/me")
      .then(async (r) => {
        if (r.ok) {
          const o: Org[] = await r.json();
          setOrgs(o);
          if (o.length) {
            setOrg(o[0].id);
            setRole(o[0].role);
          }
        }
      })
      .catch(() => setMessage("Unable to connect to the API."))
      .finally(() => setLoaded(true));
  }, []);
  async function refresh(id: string, access: string) {
    const names = [
      ...collections,
      ...(access !== "viewer" ? ["dashboard"] : []),
      ...(access === "admin" ? ["history", "rates"] : []),
    ];
    const results = await Promise.all(
      names.map(async (n) => {
        const r = await fetch(`/api/orgs/${id}/${n}`);
        if (!r.ok)
          throw new Error(
            `Unable to load ${n}. Please sign in again if your session expired.`,
          );
        return [n, await r.json()] as const;
      }),
    );
    setData(Object.fromEntries(results));
  }
  useEffect(() => {
    if (org) refresh(org, role).catch((e) => setMessage(e.message));
  }, [org, role]);
  async function submit(
    e: FormEvent<HTMLFormElement>,
    path: string,
    numbers: string[] = [],
  ) {
    e.preventDefault();
    const form = e.currentTarget;
    const fields = Object.fromEntries(new FormData(form).entries());
    const payload: Record<string, unknown> = { ...fields };
    for (const key of numbers) payload[key] = Number(fields[key]);
    if (payload.sprint_id === "") delete payload.sprint_id;
    await mutate(path, payload, () => form.reset());
  }
  async function mutate(path: string, payload: unknown, done?: () => void) {
    setBusy(true);
    setMessage("");
    try {
      const r = await fetch(`/api/orgs/${org}/${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      const result = await r.json();
      if (!r.ok) throw new Error(result.error || "Unable to save");
      done?.();
      await refresh(org, role);
      setMessage("Saved. Capacity and costs updated.");
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function login(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    try {
      const email = new FormData(e.currentTarget).get("email");
      const r = await fetch("/auth/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      const body = await r.json();
      setMessage(body.message || body.error);
    } catch {
      setMessage("Unable to connect. Please try again.");
    }
    setBusy(false);
  }
  const rows = (n: string) => data[n] || [];
  const name = (n: string, id: unknown) =>
    String(rows(n).find((x) => x.id === id)?.name || "—");
  const task = (id: unknown) =>
    String(rows("tasks").find((x) => x.id === id)?.jira_key || "—");
  const editable = role === "admin" || role === "manager";
  const money = (c: unknown) =>
    new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(Number(c || 0) / 100);
  function select(
    field: string,
    label: string,
    collection: string,
    optional = false,
  ) {
    return (
      <label>
        {label}
        <select aria-label={label} name={field} required={!optional}>
          <option value="">{optional ? "No sprint" : "Select…"}</option>
          {rows(collection).map((x) => (
            <option key={String(x.id)} value={String(x.id)}>
              {String(x.name || x.jira_key)}
            </option>
          ))}
        </select>
      </label>
    );
  }
  function input(field: string, label: string, type = "text", value?: string) {
    return (
      <label>
        {label}
        <input
          name={field}
          type={type}
          required
          defaultValue={value}
          min={type === "number" ? 0 : undefined}
          step={type === "number" ? "0.01" : undefined}
        />
      </label>
    );
  }
  function save() {
    return <button disabled={busy}>Save</button>;
  }
  if (!loaded)
    return (
      <main className="login">
        <p>Loading your workspace…</p>
      </main>
    );
  if (!org)
    return (
      <main className="login">
        <div className="brand">◈ Resource Compass</div>
        <div className="eyebrow">ENGINEERING OPERATIONS</div>
        <h1>
          Clarity for
          <br />
          every commitment.
        </h1>
        <p>
          Plan your team’s work, protect capacity, and keep delivery costs in
          view.
        </p>
        <form onSubmit={login}>
          {input("email", "Work email", "email")}
          <button disabled={busy}>Email me a sign-in link</button>
        </form>
        <p className="muted">Invite-only access · No password required</p>
        <p role="status">{message}</p>
      </main>
    );
  return (
    <div className="shell">
      <aside>
        <div className="brand">
          ◈ Resource
          <br />
          Compass
        </div>
        <p className="eyebrow">WORKSPACE</p>
        <select
          aria-label="Organization"
          value={org}
          onChange={(e) => {
            setData({});
            setOrg(e.target.value);
            setRole(
              orgs.find((o) => o.id === e.target.value)?.role || "viewer",
            );
          }}
        >
          {orgs.map((o) => (
            <option key={o.id} value={o.id}>
              {o.name}
            </option>
          ))}
        </select>
        <nav>
          {[
            "Overview",
            "Tasks",
            "People & projects",
            "Planning",
            "Activity",
            ...(role === "admin" ? ["Administration"] : []),
          ].map((t) => (
            <button
              className={t === tab ? "selected" : ""}
              key={t}
              onClick={() => setTab(t)}
            >
              {t}
            </button>
          ))}
        </nav>
        <div className="aside-bottom">
          <p>{role} access</p>
          <button
            onClick={async () => {
              await fetch("/auth/logout", { method: "POST" });
              location.reload();
            }}
          >
            Sign out
          </button>
        </div>
      </aside>
      <main>
        <header>
          <div>
            <p className="eyebrow">
              ENGINEERING / {orgs.find((o) => o.id === org)?.name}
            </p>
            <h1>{tab}</h1>
            <p className="muted">
              Every task. Every contributor. One shared plan.
            </p>
          </div>
          <span className="badge">Live workspace</span>
        </header>
        {message && (
          <div className="notice" role="status">
            {message}
          </div>
        )}
        {tab === "Overview" && (
          <>
            <div className="metrics">
              <article>
                <span>Team members</span>
                <strong>{rows("resources").length}</strong>
              </article>
              <article>
                <span>Active tasks</span>
                <strong>
                  {rows("tasks").filter((t) => t.status === "active").length}
                </strong>
              </article>
              <article>
                <span>On hold</span>
                <strong>
                  {rows("tasks").filter((t) => t.status === "hold").length}
                </strong>
              </article>
              <article>
                <span>Overloaded days</span>
                <strong>
                  {rows("daily-capacity").filter((t) => t.overloaded).length}
                </strong>
              </article>
            </div>
            <section>
              <h2>Project costs</h2>
              <p className="muted">
                USD · Actual work plus remaining scheduled spend. Unscheduled
                work is excluded.
              </p>
              {role === "viewer" ? (
                <p>Financial reporting requires manager access.</p>
              ) : (
                <table>
                  <thead>
                    <tr>
                      <th>Project</th>
                      <th>Budget</th>
                      <th>Actual</th>
                      <th>Scheduled forecast</th>
                      <th>Unpriced entries</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows("dashboard").map((p) => (
                      <tr key={String(p.id)}>
                        <td>{p.name}</td>
                        <td>{money(p.budget_cents)}</td>
                        <td>{money(p.actual_cents)}</td>
                        <td>{money(p.forecast_cents)}</td>
                        <td>{p.unpriced_allocations}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              {!rows("projects").length && (
                <p className="empty">
                  Start in People & projects: add your team, then a project.
                </p>
              )}
            </section>
            <section>
              <h2>Sprint capacity</h2>
              <p className="muted">
                Weekdays × daily hours. Leave and holidays are not yet deducted.
              </p>
              <table>
                <thead>
                  <tr>
                    <th>Person</th>
                    <th>Sprint</th>
                    <th>Capacity</th>
                    <th>Allocated</th>
                    <th>Available</th>
                  </tr>
                </thead>
                <tbody>
                  {rows("capacity").map((c) => (
                    <tr key={`${c.resource_id}-${c.sprint_id}`}>
                      <td>{c.name}</td>
                      <td>{c.sprint_name}</td>
                      <td>{c.capacity_hours}h</td>
                      <td>{c.allocated_hours}h</td>
                      <td
                        className={
                          Number(c.allocated_hours) > Number(c.capacity_hours)
                            ? "risk"
                            : ""
                        }
                      >
                        {Number(c.capacity_hours) - Number(c.allocated_hours)}h
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          </>
        )}
        {tab === "Tasks" && (
          <>
            <section>
              <h2>Task board</h2>
              <div className="task-grid">
                {rows("tasks").map((t) => (
                  <article className="task" key={String(t.id)}>
                    <div className="task-top">
                      <b>{t.jira_key}</b>
                      <span
                        className={`badge ${t.status === "hold" ? "hold" : ""}`}
                      >
                        {t.status}
                      </span>
                    </div>
                    <h3>{t.title}</h3>
                    <p>
                      {name("projects", t.project_id)} ·{" "}
                      {name("resources", t.primary_owner_id)}
                    </p>
                    <p className="muted">
                      {t.start_date} → {t.target_date}
                    </p>
                    <div className="task-numbers">
                      <span>{t.estimate_hours}h estimated</span>
                      <span>{t.actual_hours}h actual</span>
                      <span>{t.remaining_hours}h remaining</span>
                    </div>
                    {editable && (
                      <>
                        <form
                          onSubmit={(e) =>
                            submit(e, `tasks/${t.id}/status`, ["version"])
                          }
                        >
                          <input
                            type="hidden"
                            name="version"
                            value={String(t.version)}
                          />
                          <label>
                            Status
                            <select
                              aria-label="Status"
                              name="status"
                              defaultValue={String(t.status)}
                            >
                              {["planned", "active", "hold", "done"].map(
                                (s) => (
                                  <option key={s}>{s}</option>
                                ),
                              )}
                            </select>
                          </label>
                          {input("reason", "Reason for change")}
                          {save()}
                        </form>
                        <details>
                          <summary>Change primary owner</summary>
                          <p className="muted">
                            Releases all remaining allocations. Schedule
                            contributors again under Planning.
                          </p>
                          <form
                            onSubmit={(e) =>
                              submit(e, `tasks/${t.id}/owner`, ["version"])
                            }
                          >
                            <input
                              type="hidden"
                              name="version"
                              value={String(t.version)}
                            />
                            {select(
                              "primary_owner_id",
                              "New owner",
                              "resources",
                            )}
                            {input("reason", "Reassignment reason")}
                            {save()}
                          </form>
                        </details>
                      </>
                    )}
                  </article>
                ))}
              </div>
              {!rows("tasks").length && (
                <p className="empty">Create your first Jira task below.</p>
              )}
            </section>
            {editable && (
              <section>
                <h2>Create task</h2>
                <form
                  className="form-grid"
                  onSubmit={(e) => submit(e, "tasks", ["estimate_hours"])}
                >
                  {select("project_id", "Project", "projects")}
                  {input("jira_key", "Jira key")}
                  {input("title", "Task title")}
                  {select("primary_owner_id", "Primary owner", "resources")}
                  {input("estimate_hours", "Estimated hours", "number")}
                  {input("start_date", "Start date", "date")}
                  {input("target_date", "Target date", "date")}
                  {select("sprint_id", "Sprint", "sprints", true)}
                  {save()}
                </form>
              </section>
            )}
          </>
        )}
        {tab === "People & projects" && (
          <div className="two-col">
            <section>
              <h2>People</h2>
              {rows("resources").map((r) => (
                <p key={String(r.id)}>
                  <b>{r.name}</b>
                  <span className="float">{r.daily_hours}h / day</span>
                </p>
              ))}
              {editable && (
                <form onSubmit={(e) => submit(e, "resources", ["daily_hours"])}>
                  {input("name", "Name")}
                  {input(
                    "daily_hours",
                    "Daily capacity (hours)",
                    "number",
                    "8",
                  )}
                  {save()}
                </form>
              )}
            </section>
            <section>
              <h2>Projects</h2>
              {rows("projects").map((p) => (
                <p key={String(p.id)}>{p.name}</p>
              ))}
              {editable && (
                <form onSubmit={(e) => submit(e, "projects", ["budget_cents"])}>
                  {input("name", "Project name")}
                  {input("budget_cents", "Budget in USD cents", "number", "0")}
                  {save()}
                </form>
              )}
            </section>
          </div>
        )}
        {tab === "Planning" && (
          <>
            <div className="two-col">
              {editable && (
                <>
                  <section>
                    <h2>Schedule a contributor</h2>
                    <form onSubmit={(e) => submit(e, "allocations", ["hours"])}>
                      {select("task_id", "Task", "tasks")}
                      {select("resource_id", "Contributor", "resources")}
                      {input("day", "Work date", "date")}
                      {input("hours", "Hours", "number")}
                      {save()}
                    </form>
                  </section>
                  <section>
                    <h2>Record actual work</h2>
                    <p className="muted">
                      Requires an effective rate. Recorded costs are preserved.
                    </p>
                    <form onSubmit={(e) => submit(e, "worklogs", ["hours"])}>
                      {select("task_id", "Task", "tasks")}
                      {select("resource_id", "Contributor", "resources")}
                      {input("day", "Work date", "date")}
                      {input("hours", "Hours", "number")}
                      {save()}
                    </form>
                  </section>
                  <section>
                    <h2>Create sprint</h2>
                    <form onSubmit={(e) => submit(e, "sprints")}>
                      {input("name", "Sprint name")}
                      {input("start_date", "Start date", "date")}
                      {input("end_date", "End date", "date")}
                      {save()}
                    </form>
                  </section>
                </>
              )}
            </div>
            <section>
              <h2>Daily workload</h2>
              <table>
                <thead>
                  <tr>
                    <th>Person</th>
                    <th>Date</th>
                    <th>Scheduled</th>
                    <th>Capacity</th>
                    <th>Risk</th>
                  </tr>
                </thead>
                <tbody>
                  {rows("daily-capacity").map((c) => (
                    <tr key={`${c.resource_id}-${c.day}`}>
                      <td>{c.name}</td>
                      <td>{c.day}</td>
                      <td>{c.allocated_hours}h</td>
                      <td>{c.daily_hours}h</td>
                      <td className={c.overloaded ? "risk" : ""}>
                        {c.overloaded ? "Over capacity" : "Within capacity"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          </>
        )}
        {tab === "Activity" && (
          <>
            <section>
              <h2>Allocation history</h2>
              <table>
                <thead>
                  <tr>
                    <th>Task</th>
                    <th>Contributor</th>
                    <th>Date</th>
                    <th>Hours</th>
                    <th>State</th>
                    <th>Reason</th>
                  </tr>
                </thead>
                <tbody>
                  {rows("allocations").map((a) => (
                    <tr key={String(a.id)}>
                      <td>{task(a.task_id)}</td>
                      <td>{name("resources", a.resource_id)}</td>
                      <td>{a.day}</td>
                      <td>{a.hours}</td>
                      <td>{a.released_at ? "Released" : "Scheduled"}</td>
                      <td>{a.release_reason || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
            <section>
              <h2>Actual work</h2>
              <table>
                <thead>
                  <tr>
                    <th>Task</th>
                    <th>Contributor</th>
                    <th>Date</th>
                    <th>Hours</th>
                  </tr>
                </thead>
                <tbody>
                  {rows("worklogs").map((x) => (
                    <tr key={String(x.id)}>
                      <td>{task(x.task_id)}</td>
                      <td>{name("resources", x.resource_id)}</td>
                      <td>{x.day}</td>
                      <td>{x.hours}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          </>
        )}
        {tab === "Administration" && role === "admin" && (
          <div className="two-col">
            <section>
              <h2>Invite a colleague</h2>
              <p className="muted">
                After saving, ask them to request a sign-in link with this
                email.
              </p>
              <form onSubmit={(e) => submit(e, "invitations")}>
                {input("email", "Email", "email")}
                <label>
                  Role
                  <select aria-label="Role" name="role">
                    <option>viewer</option>
                    <option>manager</option>
                    <option>admin</option>
                  </select>
                </label>
                {save()}
              </form>
            </section>
            <section>
              <h2>Add historical rate</h2>
              <p className="muted">
                Rates are append-only. Existing actual costs stay fixed.
              </p>
              <form onSubmit={(e) => submit(e, "rates", ["hourly_cents"])}>
                {select("resource_id", "Resource", "resources")}
                {input("effective_from", "Effective from", "date")}
                {input("hourly_cents", "USD cents per hour", "number")}
                {save()}
              </form>
              {rows("rates").map((r) => (
                <p key={String(r.id)}>
                  {name("resources", r.resource_id)} · {money(r.hourly_cents)}/h
                  · {r.effective_from}
                </p>
              ))}
            </section>
          </div>
        )}
        <footer>
          Resource Compass · Planning is a forecast; task completion is always
          explicit.
        </footer>
      </main>
    </div>
  );
}
