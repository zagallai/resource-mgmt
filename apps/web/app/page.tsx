"use client";
import { useEffect, useState, FormEvent } from "react";
import { ResourcesScreen, ProjectsScreen } from "./management";
import { syncDateRange } from "./date-range";
type Row = Record<string, string | number | boolean | null>;
type Org = { id: string; name: string; role: string };
const collections = [
  "resources",
  "archived-resources",
  "projects",
  "project-activities",
  "tasks",
  "sprints",
  "allocations",
  "capacity",
  "daily-capacity",
  "worklogs",
];
export default function Home() {
  const [taskProject, setTaskProject] = useState("");
  const [workloadDate, setWorkloadDate] = useState(new Date().toISOString().slice(0,10));
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
    if (path === "tasks" && fields.sprint_id) {
      const sprint = rows("sprints").find(s => s.id === fields.sprint_id);
      if (sprint && (String(fields.start_date) < String(sprint.start_date) || String(fields.target_date) > String(sprint.end_date))) {
        setMessage(`Task dates must stay within the selected sprint (${sprint.start_date} to ${sprint.end_date}).`);
        return;
      }
    }
    const payload: Record<string, unknown> = { ...fields };
    for (const key of numbers)
      payload[key] = Number(fields[key] || (key === "daily_hours" ? 8 : 0));
    for (const key of ["hourly_rate", "budget"]) {
      if (key in fields) {
        payload[key === "hourly_rate" ? "hourly_cents" : "budget_cents"] =
          Math.round(Number(fields[key]) * 100);
        delete payload[key];
      }
    }
    if ("allocation_days" in fields) {
      payload.hours = Number(fields.allocation_days) * 8;
      delete payload.allocation_days;
    }
    if ("estimate_days" in fields) {
      payload.estimate_hours = Number(fields.estimate_days) * 8;
      delete payload.estimate_days;
    }
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
      return result.id as string;
    } catch (e) {
      setMessage((e as Error).message);
      return null;
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
    String(
      (n === "resources"
        ? [...rows("resources"), ...rows("archived-resources")]
        : rows(n)
      ).find((x) => x.id === id)?.name || "—",
    );
  const task = (id: unknown) =>
    String(rows("tasks").find((x) => x.id === id)?.jira_key || rows("tasks").find((x) => x.id === id)?.title || "—");
  const editable = role === "admin" || role === "manager";
  const money = (c: unknown) =>
    new Intl.NumberFormat("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
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
          {(collection === "all-resources"
            ? [...rows("resources"), ...rows("archived-resources")]
            : rows(collection)
          ).map((x) => (
            <option key={String(x.id)} value={String(x.id)}>
              {String(x.name || x.jira_key || x.title)}
              {x.archived_at ? " (Archived)" : ""}
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
          required={field !== "jira_key"}
          defaultValue={value}
          min={
            field === "daily_hours" ? 0.01 : type === "number" ? 0 : undefined
          }
          max={field === "daily_hours" ? 24 : undefined}
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
        <div className="brand">◈ Resource Management</div>
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
          Management
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
            "Projects",
            "Resources",
            "Planning",
            "Tasks",
            "Activity",
            ...(role === "admin" ? ["Admin"] : []),
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
              {tab === "Resources"
                ? "Manage people, companies, rates and working hours."
                : tab === "Projects"
                  ? "Manage project details and planned activities."
                  : "Every task. Every contributor. One shared plan."}
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
              <h2>Who’s working on what</h2>
              <p className="muted">Current and upcoming work, grouped by resource. Planned and on-hold work is labelled; overdue active work stays visible. This summary is independent of the workload date below.</p>
              <table>
                <thead><tr><th>Resource</th><th>Current and upcoming tasks</th></tr></thead>
                <tbody>{rows("resources").map(r => {
                  const today = new Date().toISOString().slice(0,10);
                  const bookings = rows("allocations").filter(a => a.resource_id === r.id && !a.released_at && String(a.day) >= today);
                  const assigned = rows("tasks").filter(t => t.status !== "done" &&
                    (t.primary_owner_id === r.id || bookings.some(a => a.task_id === t.id)) &&
                    (String(t.target_date) >= today || t.status === "active" || t.status === "hold" || bookings.some(a => a.task_id === t.id))
                  ).sort((a,b) => String(a.start_date).localeCompare(String(b.start_date)) || String(a.title).localeCompare(String(b.title)));
                  return <tr key={String(r.id)}><td>{r.name}</td><td>{assigned.length ? <ul>{assigned.map(t => <li key={String(t.id)}>
                    <strong>{t.title}</strong>{t.jira_key ? ` (${t.jira_key})` : ""} · {name("projects",t.project_id)} · <span className={`badge ${t.status === "hold" ? "hold" : ""}`}>{t.status}</span>
                    <div className="muted">{t.start_date} → {t.target_date}{String(t.target_date) < today ? " · Overdue" : ""}{t.primary_owner_id !== r.id ? " · Contributor" : ""}</div>
                  </li>)}</ul> : <span className="muted">No current or upcoming tasks</span>}</td></tr>;
                })}</tbody>
              </table>
              {!rows("resources").length && <p className="empty">Add resources and assign tasks to see the team’s work here.</p>}
            </section>
            <section>
              <h2>Team availability</h2>
              <label>Workload date<input aria-label="Workload date" type="date" value={workloadDate} onChange={e => setWorkloadDate(e.target.value)} /></label>
              <p className="muted">Booked time and remaining capacity for this date. Unbooked tasks still need time allocated; availability does not predict early completion. Standard effort days use 8 hours.</p>
              <table><thead><tr><th>Resource</th><th>Booked days</th><th>Available days</th><th>Status</th><th>Tasks on this date</th><th>Assigned task target dates</th></tr></thead>
                <tbody>{rows("resources").map(r => {
                  const bookings = rows("allocations").filter(a => a.resource_id === r.id && a.day === workloadDate && !a.released_at);
                  const hours = bookings.reduce((sum,a) => sum + Number(a.hours),0);
                  const capacity = Number(r.daily_hours);
                  const assigned = rows("tasks").filter(t => t.primary_owner_id === r.id && t.status !== "done");
                  return <tr key={String(r.id)}>
                    <td>{r.name}</td><td>{hours / 8}</td><td>{Math.max(0,capacity-hours)/8}</td>
                    <td className={hours>capacity ? "risk" : ""}>{hours>capacity ? "Over capacity" : hours===capacity ? "Fully booked" : "Capacity available"}</td>
                    <td>{bookings.map(a => String(a.jira_key || a.task_title || task(a.task_id))).filter((v,i,all) => all.indexOf(v)===i).join(", ") || "No time booked"}</td>
                    <td>{assigned.map(t => `${t.title}: ${t.target_date} (${t.status})`).join("; ") || "No open tasks"}</td>
                  </tr>;
                })}</tbody>
              </table>
              {!rows("resources").length && <p className="empty">Add resources to see team availability.</p>}
            </section>
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
            {editable && (
              <section>
                <h2>Create task</h2>
                <form
                  className="form-grid"
                  onSubmit={(e) => submit(e, "tasks")}
                  onChange={e => syncDateRange(e, "target_date")}
                  onFocusCapture={e => syncDateRange(e, "target_date")}
                >
                  <label>Project<select aria-label="Project" name="project_id" required value={taskProject} onChange={e => setTaskProject(e.target.value)}><option value="">Select…</option>{rows("projects").map(p => <option key={String(p.id)} value={String(p.id)}>{p.name}</option>)}</select></label>
                  {input("jira_key", "Jira key (optional)")}
                  {input("title", "Task title")}
                  {select("primary_owner_id", "Primary owner", "resources")}
                  <label>Estimated days<input name="estimate_days" type="number" required min="0" step="0.25" aria-describedby="estimate-days-help" /></label>
                  <p id="estimate-days-help" className="muted">Effort in days: 0.25, 0.5, 1, 3, etc. One estimated day equals 8 working hours. Resource availability still uses each person’s configured daily hours.</p>
                  {input("start_date", "Start date", "date")}
                  {input("target_date", "End date", "date")}
                  <label>Sprint<select key={taskProject} name="sprint_id" aria-label="Sprint"><option value="">No sprint</option>{rows("sprints").filter(s => s.project_id === taskProject).map(s => <option key={String(s.id)} value={String(s.id)}>{s.name}</option>)}</select></label>
                  <p className="muted">If a sprint is selected, task dates must fall within its start and end dates.</p>
                  {save()}
                </form>
              </section>
            )}
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
                      <span>{Number(t.estimate_hours) / 8} days estimated</span>
                      <span>{Number(t.actual_hours) / 8} days actual</span>
                      <span>{Number(t.remaining_hours) / 8} days remaining</span>
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
                          <summary>Allocate resource time</summary>
                          <section aria-label="Allocate resource time">
                            <p className="muted">Reserve time on this task. The assigned owner is selected by default. One day equals 8 hours.</p>
                            <form onSubmit={e => submit(e,"allocations")}>
                              <input type="hidden" name="task_id" value={String(t.id)} />
                              <label>Resource<select name="resource_id" aria-label="Resource" required defaultValue={String(t.primary_owner_id)}>{rows("resources").map(r => <option key={String(r.id)} value={String(r.id)}>{r.name}</option>)}</select></label>
                              {input("day","Work date","date")}
                              <label>Days<input name="allocation_days" required type="number" min="0.25" step="0.25" /></label>
                              <button disabled={busy || t.status === "hold" || t.status === "done"}>Allocate time</button>
                            </form>
                            {(t.status === "hold" || t.status === "done") && <p className="muted">Resume this task before booking time.</p>}
                          </section>
                        </details>
                        <details>
                          <summary>Record actual work</summary>
                          <section aria-label="Record actual work">
                            <p className="muted">Optional: record hours already worked. Requires a rate effective on the work date.</p>
                            <form onSubmit={e => submit(e,"worklogs",["hours"])}>
                              <input type="hidden" name="task_id" value={String(t.id)} />
                              <label>Contributor<select name="resource_id" required defaultValue={String(t.primary_owner_id)}>{[...rows("resources"),...rows("archived-resources")].map(r => <option key={String(r.id)} value={String(r.id)}>{r.name}</option>)}</select></label>
                              {input("day","Work date","date")}
                              {input("hours","Hours","number")}
                              {save()}
                            </form>
                          </section>
                        </details>
                        <details>
                          <summary>Change primary owner</summary>
                          <p className="muted">
                            Releases remaining future bookings. Allocate time again here for the new owner.
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
                <p className="empty">Create your first task above.</p>
              )}
            </section>

          </>
        )}
        {tab === "Resources" && (
          <ResourcesScreen
            key={org}
            resources={rows("resources")}
            archivedResources={rows("archived-resources")}
            rates={rows("rates")}
            role={role}
            busy={busy}
            onSave={mutate}
          />
        )}
        {tab === "Projects" && (
          <ProjectsScreen
            key={org}
            projects={rows("projects")}
            tasks={rows("tasks")}
            resources={rows("resources")}
            archivedResources={rows("archived-resources")}
            activities={rows("project-activities")}
            role={role}
            busy={busy}
            onSave={mutate}
          />
        )}
        {tab === "Planning" && (
          <>
            <div className="two-col">
              {editable && (
                <>
                  <section>
                    <h2>Create sprint</h2>
                    <form onSubmit={(e) => submit(e, "sprints")} onChange={syncDateRange} onFocusCapture={syncDateRange}>
                      {select("project_id","Project","projects")}
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
              <h2>Sprints</h2>
              {rows("sprints").length ? <div className="management-list">{rows("sprints").map(s => <details className="management-card" key={String(s.id)}>
                <summary><strong>{s.name}</strong><span>{s.project_id ? name("projects",s.project_id) : "Choose project"} · {s.start_date} → {s.end_date}</span></summary>
                {editable && <form className="form-grid" onSubmit={e => submit(e,`sprints/${s.id}/edit`)} onChange={syncDateRange} onFocusCapture={syncDateRange}>
                  <input type="hidden" name="previous_name" value={String(s.name)} />
                  <input type="hidden" name="previous_start" value={String(s.start_date)} />
                  <input type="hidden" name="previous_end" value={String(s.end_date)} />
                  <input type="hidden" name="previous_project" value={String(s.project_id || "")} />
                  <label>Project<select aria-label="Project" name="project_id" required defaultValue={String(s.project_id || "")}><option value="">Select…</option>{rows("projects").map(p => <option key={String(p.id)} value={String(p.id)}>{p.name}</option>)}</select></label>
                  {input("name","Sprint name","text",String(s.name))}
                  {input("start_date","Start date","date",String(s.start_date))}
                  {input("end_date","End date","date",String(s.end_date))}
                  <button disabled={busy}>Save sprint</button>
                  <p className="muted">Sprint dates must contain all linked tasks. Changing dates recalculates capacity; existing bookings remain as entered.</p>
                </form>}
              </details>)}</div> : <p className="empty">No sprints yet. Create a sprint above.</p>}

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
                      <td>{a.jira_key || a.task_title || task(a.task_id)}</td>
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
                      <td>{x.jira_key || x.task_title || task(x.task_id)}</td>
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
        {tab === "Admin" && role === "admin" && (
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
          </div>
        )}
        <footer>
          Resource Management · Planning is a forecast; task completion is always
          explicit.
        </footer>
      </main>
    </div>
  );
}
