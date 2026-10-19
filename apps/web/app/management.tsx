"use client";
import { useState, useRef, FormEvent } from "react";
import { syncDateRange } from "./date-range";
export type Row = Record<string, string | number | boolean | null>;
type Save = (
  path: string,
  payload: Record<string, unknown>,
) => Promise<string | null>;
type Props = {
  resources: Row[];
  archivedResources: Row[];
  role: string;
  busy: boolean;
  onSave: Save;
  rates: Row[];
};
const today = () => new Date().toISOString().slice(0, 10);
const amount = (v: unknown) =>
  v == null
    ? "Not set"
    : new Intl.NumberFormat("en-US", {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }).format(Number(v) / 100);
const values = (e: FormEvent<HTMLFormElement>) => {
  e.preventDefault();
  return Object.fromEntries(new FormData(e.currentTarget).entries()) as Record<
    string,
    string
  >;
};
function Field({
  name,
  label,
  value = "",
  type = "text",
  required = true,
  min,
  max,
  list,
}: {
  name: string;
  label: string;
  value?: string | number;
  type?: string;
  required?: boolean;
  min?: number;
  max?: number;
  list?: string;
}) {
  return (
    <label>
      {label}
      <input
        aria-label={label}
        name={name}
        type={type}
        defaultValue={value}
        required={required}
        min={min}
        max={max}
        step={type === "number" ? "0.01" : undefined}
        list={list}
      />
    </label>
  );
}
function Location({ value = "" }: { value?: string }) {
  return (
    <label>
      Type
      <select
        name="location_type"
        aria-label="Type"
        defaultValue={value}
        required
      >
        <option value="">Choose type</option>
        <option value="onshore">Onshore</option>
        <option value="offshore">Offshore</option>
      </select>
    </label>
  );
}
export function ResourcesScreen({
  resources,
  archivedResources,
  role,
  busy,
  onSave,
  rates,
}: Props) {
  const [search, setSearch] = useState("");
  const [showArchived, setShowArchived] = useState(false);
  const visibleResources = [
    ...resources,
    ...(showArchived ? archivedResources : []),
  ].sort(
    (a, b) =>
      String(b.created_at).localeCompare(String(a.created_at)) ||
      String(b.id).localeCompare(String(a.id)),
  );
  const editable = role !== "viewer",
    admin = role === "admin";
  async function create(e: FormEvent<HTMLFormElement>) {
    const form = e.currentTarget,
      b = values(e);
    const payload: Record<string, unknown> = {
      name: b.name,
      location_type: b.location_type,
      company_name: b.company_name,
      daily_hours: Number(b.daily_hours || 8),
    };
    if (admin) {
      payload.hourly_cents = Math.round(Number(b.hourly_rate) * 100);
      payload.effective_from = b.effective_from;
    }
    if (await onSave("resources", payload)) form.reset();
  }
  return (
    <>
      <section>
        <h2>Resource directory</h2>
        <p className="muted">
          People, companies and working hours. Select a person to manage their
          details{admin ? " and rate history" : ""}.
        </p>
        <input
          className="directory-search"
          aria-label="Filter resources"
          placeholder="Filter by name or company"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={showArchived}
            onChange={(e) => setShowArchived(e.target.checked)}
          />
          Show archived resources
        </label>
        {!visibleResources.length && (
          <p className="empty">Add your first resource below.</p>
        )}
        {visibleResources
          .filter((x) =>
            (String(x.name) + " " + String(x.company_name))
              .toLowerCase()
              .includes(search.toLowerCase()),
          )
          .map((x) => (
            <details className="management-card" key={String(x.id)}>
              <summary>
                <strong>
                  {x.name}
                  {x.archived_at && <span className="badge">Archived</span>}
                </strong>
                <span>
                  {x.location_type || "Type not set"} ·{" "}
                  {x.company_name || "Company not set"} · {x.daily_hours}h/day
                  {admin
                    ? " · Rate " + amount(x.current_hourly_cents) + "/h"
                    : ""}
                </span>
              </summary>
              {editable && (
                <form
                  aria-label={`Edit resource ${x.name}`}
                  className="form-grid"
                  key={`resource-${x.id}-${x.version}`}
                  onSubmit={async (e) => {
                    const b = values(e);
                    await onSave(`resources/${x.id}/edit`, {
                      name: b.name,
                      location_type: b.location_type,
                      company_name: b.company_name,
                      daily_hours: Number(b.daily_hours),
                      version: Number(x.version),
                    });
                  }}
                >
                  <Field
                    name="name"
                    label="Resource name"
                    value={String(x.name)}
                  />
                  <Location value={String(x.location_type || "")} />
                  <Field
                    name="company_name"
                    label="Company name"
                    value={String(x.company_name || "")}
                  />
                  <Field
                    name="daily_hours"
                    label="Working hours per day"
                    type="number"
                    value={Number(x.daily_hours)}
                    min={0.01}
                    max={24}
                  />
                  <button disabled={busy}>Save resource</button>
                  <button
                    type="button"
                    className="secondary-button"
                    disabled={busy}
                    onClick={() =>
                      onSave(
                        `resources/${x.id}/${x.archived_at ? "restore" : "archive"}`,
                        { version: Number(x.version) },
                      )
                    }
                  >
                    {x.archived_at ? "Restore resource" : "Archive resource"}
                  </button>
                  <button
                    type="button"
                    className="secondary-button"
                    disabled={busy}
                    onClick={() =>
                      onSave(`resources/${x.id}/delete`, {
                        version: Number(x.version),
                      })
                    }
                  >
                    Delete resource
                  </button>
                  <p className="muted">
                    Archiving hides this person from new assignments. Existing
                    work and allocations are retained. Delete is available only
                    for unused resources.
                  </p>
                </form>
              )}
              {admin && (
                <div className="rate-panel">
                  <h3>Rate history</h3>
                  <p className="muted">
                    Add a rate with its effective date. Recorded actual costs
                    stay unchanged.
                  </p>
                  <form
                    aria-label={`Add rate for ${x.name}`}
                    className="form-grid"
                    onSubmit={async (e) => {
                      const form = e.currentTarget,
                        b = values(e);
                      if (
                        await onSave("rates", {
                          resource_id: x.id,
                          effective_from: b.effective_from,
                          hourly_cents: Math.round(Number(b.hourly_rate) * 100),
                        })
                      )
                        form.reset();
                    }}
                  >
                    <Field
                      name="hourly_rate"
                      label="Hourly rate"
                      type="number"
                      min={0}
                    />
                    <Field
                      name="effective_from"
                      label="Effective from"
                      type="date"
                      value={today()}
                    />
                    <button disabled={busy}>Add rate</button>
                  </form>
                  <ul>
                    {rates
                      .filter((a) => a.resource_id === x.id)
                      .map((a) => (
                        <li key={String(a.id)}>
                          {a.effective_from}: {amount(a.hourly_cents)}/h
                        </li>
                      ))}
                  </ul>
                </div>
              )}
            </details>
          ))}
      </section>
      {editable && (
        <section>
          <h2>Add resource</h2>
          <form
            className="form-grid"
            aria-label="Add resource"
            onSubmit={create}
          >
            <Field name="name" label="Resource name" />
            <Location />
            <Field name="company_name" label="Company name" />
            <Field
              name="daily_hours"
              label="Working hours per day"
              type="number"
              value={8}
              min={0.01}
              max={24}
            />
            {admin && (
              <>
                <Field
                  name="hourly_rate"
                  label="Hourly rate"
                  type="number"
                  min={0}
                />
                <Field
                  name="effective_from"
                  label="Rate effective from"
                  type="date"
                  value={today()}
                />
              </>
            )}
            <button disabled={busy}>Add resource</button>
          </form>
          {!admin && (
            <p className="muted">
              An administrator can add hourly rates from this screen.
            </p>
          )}
        </section>
      )}
    </>
  );
}
const suggestions = [
  "LLD",
  "SIT",
  "UAT",
  "RB",
  "RF",
  "PTE",
  "Penetration testing",
  "Security Code Scan",
  "DAST",
  "SCA/SAST",
];
export function ProjectsScreen({
  projects,
  tasks,
  resources,
  archivedResources,
  activities,
  role,
  busy,
  onSave,
}: {
  projects: Row[];
  tasks: Row[];
  resources: Row[];
  archivedResources: Row[];
  activities: Row[];
  role: string;
  busy: boolean;
  onSave: Save;
}) {
  const [selected, setSelected] = useState(""),
    [search, setSearch] = useState(""),
    [manageActivities, setManageActivities] = useState(false),
    [activityId, setActivityId] = useState("");
  const deleteDialog = useRef<HTMLDialogElement>(null);
  const [deleting, setDeleting] = useState<Row | null>(null);
  const [confirmName, setConfirmName] = useState("");
  const [override, setOverride] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const nextDraftId = useRef(2);
  const [draftActivities, setDraftActivities] = useState([
    { id: 1, name: "", start_date: "", end_date: "" },
  ]);
  function updateDraft(
    id: number,
    field: "name" | "start_date" | "end_date",
    value: string,
  ) {
    setDraftActivities((rows) =>
      rows.map((row) => {
        if (row.id !== id) return row;
        if (field === "start_date") {
          return { ...row, start_date: value, end_date: value };
        }
        return { ...row, [field]: value };
      }),
    );
  }
  const editable = role !== "viewer";
  const projectActivities = activities.filter((x) => x.project_id === selected);
  const activity = projectActivities.find((x) => x.id === activityId);
  const resourceName = (id: unknown) =>
    [...resources, ...archivedResources].find((x) => x.id === id)?.name ||
    "Not set";
  function chooseProject(id: string) {
    setSelected(id);
    setManageActivities(false);
    setActivityId("");
  }
  function primeSelect(value = "") {
    return (
      <label>
        Prime resource
        <select
          aria-label="Prime resource"
          name="prime_resource_id"
          defaultValue={value}
          required
        >
          <option value="">
            {resources.length
              ? "Choose prime resource"
              : "Add a resource first"}
          </option>
          {[
            ...resources,
            ...archivedResources.filter((x) => x.id === value),
          ].map((x) => (
            <option key={String(x.id)} value={String(x.id)}>
              {String(x.name)}
              {x.archived_at ? " (Archived)" : ""}
            </option>
          ))}
        </select>
      </label>
    );
  }
  function projectDetails(project: Row) {
    return (
      <div className="project-inline-details" aria-label="Project details">
        <div className="section-heading">
          <h2>{project.name} — Details</h2>
          <button
            className="secondary-button"
            onClick={() => chooseProject("")}
          >
            Close details
          </button>
        </div>
        {editable ? (
          <form
            className="form-grid"
            aria-label="Edit project"
            key={`${project.id}-${project.version}`}
            onChange={syncDateRange}
            onFocusCapture={syncDateRange}
            onSubmit={async (e) => {
              const b = values(e);
              await onSave(`projects/${project.id}/edit`, {
                name: b.name,
                start_date: b.start_date,
                end_date: b.end_date,
                closed_date: b.closed_date,
                pm_name: b.pm_name,
                prime_resource_id: b.prime_resource_id,
                version: Number(project.version),
              });
            }}
          >
            <Field
              name="name"
              label="Project name"
              value={String(project.name)}
            />
            <Field
              name="start_date"
              label="Project start date"
              type="date"
              value={String(project.start_date || "")}
            />
            <Field
              name="end_date"
              label="Project end date"
              type="date"
              value={String(project.end_date || "")}
            />
            <Field
              name="closed_date"
              label="Closed date"
              type="date"
              value={String(project.closed_date || "")}
              required={false}
            />
            <Field
              name="pm_name"
              label="PM name"
              value={String(project.pm_name || "")}
            />
            {primeSelect(String(project.prime_resource_id || ""))}
            <button disabled={busy}>Save project</button>
            <button
              type="button"
              className="secondary-button"
              disabled={busy}
              onClick={() => {
                setDeleting(project); setConfirmName(""); setOverride(false); setDeleteError("");
                deleteDialog.current?.showModal();
              }}
            >
              Delete project
            </button>
            <p className="muted">
              Deletion requires confirmation. Linked Jira tasks can be included.
            </p>
          </form>
        ) : (
          <p>
            PM: {project.pm_name || "Not set"} · Prime resource:{" "}
            {resourceName(project.prime_resource_id)} · Closed date:{" "}
            {project.closed_date || "Not closed"}
          </p>
        )}
        <div className="project-activity-toggle">
          <button
            aria-expanded={manageActivities}
            onClick={() => setManageActivities((x) => !x)}
          >
            {manageActivities ? "Hide activities" : "Manage activities"} (
            {projectActivities.length})
          </button>
          <p className="muted">
            Open Manage activities to see saved activities, or add one when
            ready.
          </p>
        </div>
        {manageActivities && (
          <div className="activity-editor">
            <h3>Saved activities</h3>
            {projectActivities.length > 0 && (
              <table aria-label="Saved project activities">
                <thead>
                  <tr>
                    <th>Activity</th>
                    <th>Start date</th>
                    <th>End date</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {projectActivities.map((x) => (
                    <tr key={String(x.id)}>
                      <td>{x.name}</td>
                      <td>{x.start_date || "Not scheduled"}</td>
                      <td>{x.end_date || "Not scheduled"}</td>
                      <td>
                        <button
                          type="button"
                          className="secondary-button"
                          aria-label={`${editable ? "Edit" : "View"} activity ${x.name}`}
                          onClick={() => setActivityId(String(x.id))}
                        >
                          {editable ? "Edit" : "View"}
                        </button>
                        {editable && (
                          <button
                            type="button"
                            className="secondary-button"
                            aria-label={`Delete activity ${x.name}`}
                            disabled={busy}
                            onClick={async () => {
                              const id = await onSave(
                                `project-activities/${x.id}/delete`,
                                { version: Number(x.version) },
                              );
                              if (id && activityId === id) setActivityId("");
                            }}
                          >
                            Delete
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            <h3>{activity ? "Update activity" : "Add activity"}</h3>
            <label>
              Choose activity
              <select
                aria-label="Choose activity"
                value={activityId}
                onChange={(e) => setActivityId(e.target.value)}
              >
                <option value="">
                  {editable ? "Add new activity" : "Select an activity"}
                </option>
                {projectActivities.map((x) => (
                  <option value={String(x.id)} key={String(x.id)}>
                    {String(x.name)}
                  </option>
                ))}
              </select>
            </label>
            {!projectActivities.length && (
              <p className="muted">
                No activities yet. Add one when you are ready.
              </p>
            )}
            {editable ? (
              <form
                className="form-grid"
                aria-label="Project activity"
                key={`${project.id}-${activity?.id || "new"}-${activity?.version || 0}`}
                onChange={syncDateRange}
                onFocusCapture={syncDateRange}
                onSubmit={async (e) => {
                  const b = values(e);
                  const payload: Record<string, unknown> = {
                    name: b.name,
                    start_date: b.start_date,
                    end_date: b.end_date,
                  };
                  if (activity) payload.version = Number(activity.version);
                  else payload.project_id = project.id;
                  const id = await onSave(
                    activity
                      ? `project-activities/${activity.id}/edit`
                      : "project-activities",
                    payload,
                  );
                  if (id) setActivityId(id);
                }}
              >
                <Field
                  name="name"
                  label="Activity name"
                  value={String(activity?.name || "")}
                  list="activity-suggestions"
                />
                <datalist id="activity-suggestions">
                  {suggestions.map((name) => (
                    <option key={name} value={name} />
                  ))}
                </datalist>
                <Field
                  name="start_date"
                  label="Start date"
                  type="date"
                  value={String(activity?.start_date || "")}
                  required={false}
                />
                <Field
                  name="end_date"
                  label="End date"
                  type="date"
                  value={String(activity?.end_date || "")}
                  required={false}
                />
                <button disabled={busy}>
                  {activity ? "Update activity" : "Add activity"}
                </button>
                {activity && (
                  <button
                    type="button"
                    className="secondary-button"
                    onClick={() => setActivityId("")}
                  >
                    Add another activity
                  </button>
                )}
              </form>
            ) : activity ? (
              <p>
                {activity.name} · {activity.start_date || "Not scheduled"} →{" "}
                {activity.end_date || "Not scheduled"}
              </p>
            ) : null}
            {editable && (
              <p className="muted">
                Choose a suggested name such as LLD, SIT or UAT, or enter your
                own.
              </p>
            )}
          </div>
        )}
      </div>
    );
  }
  async function create(e: FormEvent<HTMLFormElement>) {
    const form = e.currentTarget,
      b = values(e);
    const id = await onSave("projects", {
      name: b.name,
      start_date: b.start_date,
      end_date: b.end_date,
      pm_name: b.pm_name,
      prime_resource_id: b.prime_resource_id,
      activities: draftActivities
        .filter((row) => row.name.trim() || row.start_date || row.end_date)
        .map(({ id, ...item }) => item),
    });
    if (id) {
      chooseProject(id);
      form.reset();
      setDraftActivities([
        { id: nextDraftId.current++, name: "", start_date: "", end_date: "" },
      ]);
    }
  }
  return (
    <>
      {editable && (
        <section>
          <h2>Add project</h2>
          <form
            className="form-grid"
            aria-label="Add project"
            onSubmit={create}
            onChange={syncDateRange}
            onFocusCapture={syncDateRange}
          >
            <Field name="name" label="Project name" />
            <Field name="start_date" label="Project start date" type="date" />
            <Field name="end_date" label="Project end date" type="date" />
            <Field name="pm_name" label="PM name" />
            {primeSelect()}
            <fieldset className="project-activity-drafts">
              <legend>Activities (optional)</legend>
              <p className="muted">
                Add any activities now or later. Leave the row blank to create
                the project without activities.
              </p>
              {draftActivities.map((row, index) => (
                <div
                  className="activity-draft-row"
                  role="group"
                  aria-label={`Activity row ${index + 1}`}
                  key={row.id}
                >
                  <label>
                    Activity name
                    <input
                      aria-label="Activity name"
                      value={row.name}
                      onChange={(e) =>
                        updateDraft(row.id, "name", e.target.value)
                      }
                      list="new-project-activity-suggestions"
                      required={Boolean(row.start_date || row.end_date)}
                    />
                  </label>
                  <label>
                    Start date
                    <input
                      aria-label="Start date"
                      type="date"
                      value={row.start_date}
                      onChange={(e) =>
                        updateDraft(row.id, "start_date", e.target.value)
                      }
                      required={Boolean(row.end_date)}
                    />
                  </label>
                  <label>
                    End date
                    <input
                      aria-label="End date"
                      type="date"
                      value={row.end_date}
                      min={row.start_date || undefined}
                      onChange={(e) =>
                        updateDraft(row.id, "end_date", e.target.value)
                      }
                      required={Boolean(row.start_date)}
                    />
                  </label>
                  <button
                    type="button"
                    className="secondary-button"
                    aria-label={`Remove activity row ${index + 1}`}
                    disabled={busy}
                    onClick={() =>
                      setDraftActivities((rows) =>
                        rows.filter((x) => x.id !== row.id),
                      )
                    }
                  >
                    Remove
                  </button>
                </div>
              ))}
              <datalist id="new-project-activity-suggestions">
                {suggestions.map((name) => (
                  <option key={name} value={name} />
                ))}
              </datalist>
              <button
                type="button"
                className="secondary-button"
                disabled={busy}
                onClick={() => {
                  const id = nextDraftId.current++;
                  setDraftActivities((rows) => [
                    ...rows,
                    { id, name: "", start_date: "", end_date: "" },
                  ]);
                }}
              >
                Add activity row
              </button>
            </fieldset>
            <button disabled={busy}>Add project</button>
          </form>
        </section>
      )}
      <section>
        <h2>Existing projects</h2>
        <label>
          Search projects
          <input
            aria-label="Search projects"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by project or PM"
          />
        </label>
        {!projects.length ? (
          <p className="empty">
            No projects yet. Add your first project above.
          </p>
        ) : (
          <div className="project-directory">
            {projects
              .filter((p) =>
                (String(p.name) + " " + String(p.pm_name))
                  .toLowerCase()
                  .includes(search.toLowerCase()),
              )
              .map((p) => (
                <details
                  className="management-card"
                  key={String(p.id)}
                  open={p.id === selected}
                >
                  <summary
                    aria-label={`Project ${p.name}`}
                    onClick={(e) => {
                      e.preventDefault();
                      chooseProject(p.id === selected ? "" : String(p.id));
                    }}
                  >
                    <strong>{p.name}</strong>
                    <span>
                      PM: {p.pm_name || "Not set"} · Prime:{" "}
                      {resourceName(p.prime_resource_id)} ·{" "}
                      {p.start_date || "Start not set"} →{" "}
                      {p.end_date || "End not set"}
                      {p.closed_date ? ` · Closed: ${p.closed_date}` : ""}
                    </span>
                  </summary>
                  {p.id === selected && projectDetails(p)}
                </details>
              ))}
          </div>
        )}
      </section>
      <dialog ref={deleteDialog} aria-labelledby="delete-project-title" style={{maxWidth: "540px", padding: "24px", borderRadius: "12px"}}>
        {deleting && <form onSubmit={async e => {
          e.preventDefault();
          const linked = tasks.filter(t => t.project_id === deleting.id).length;
          const result = await onSave(`projects/${deleting.id}/delete`, {version:Number(deleting.version), confirm_name:confirmName, include_linked_tasks: linked > 0 && override});
          if(result) {deleteDialog.current?.close(); setDeleting(null); chooseProject("");}
          else setDeleteError("Deletion failed. Close this dialog and refresh to review the latest project details.");
        }}>
          <h2 id="delete-project-title">Delete project</h2>
          <p>Delete {deleting.name} and its {activities.filter(a => a.project_id === deleting.id).length} activities?</p>
          <p>{tasks.filter(t => t.project_id === deleting.id).length} linked Jira tasks will be removed from active Resource Management screens. Future allocations will be released. Recorded work, rates and change history are kept.</p>
          <p>Issues in your external Jira system will not be deleted.</p>
          {tasks.some(t => t.project_id === deleting.id) && <label><input type="checkbox" checked={override} onChange={e => setOverride(e.target.checked)} /> Override restriction and include linked Jira tasks</label>}
          <label>Type the project name to confirm<input required value={confirmName} onChange={e => setConfirmName(e.target.value)} /></label>
          {deleteError && <p role="alert">{deleteError}</p>}
          <button type="button" className="secondary-button" onClick={() => deleteDialog.current?.close()}>Cancel</button>
          <button disabled={busy || confirmName !== deleting.name || (tasks.some(t => t.project_id === deleting.id) && !override)}>Confirm deletion</button>
        </form>}
      </dialog>
    </>
  );
}
