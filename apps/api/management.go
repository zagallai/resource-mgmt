package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type resourceInput struct {
	Name      string   `json:"name"`
	Type      *string  `json:"location_type"`
	Company   string   `json:"company_name"`
	Hours     *float64 `json:"daily_hours"`
	Rate      *int64   `json:"hourly_cents"`
	Effective string   `json:"effective_from"`
	Version   int      `json:"version"`
}
type initialActivity struct {
	Name  string `json:"name"`
	Start string `json:"start_date"`
	End   string `json:"end_date"`
}
type projectInput struct {
	Activities []initialActivity `json:"activities"`
	Closed     *string           `json:"closed_date"`
	Prime      *string           `json:"prime_resource_id"`
	Name       string            `json:"name"`
	Start      string            `json:"start_date"`
	End        string            `json:"end_date"`
	PM         string            `json:"pm_name"`
	Budget     *int64            `json:"budget_cents"`
	Version    int               `json:"version"`
}
type activityInput struct {
	Project string `json:"project_id"`
	Name    string `json:"name"`
	Start   string `json:"start_date"`
	End     string `json:"end_date"`
	Version int    `json:"version"`
}

func dates(start, end string) (*string, *string, error) {
	if start == "" && end == "" {
		return nil, nil, nil
	}
	if _, e := time.Parse("2006-01-02", start); e != nil {
		return nil, nil, invalid("Choose both start and end dates")
	}
	if _, e := time.Parse("2006-01-02", end); e != nil || end < start {
		return nil, nil, invalid("End date must be on or after start date")
	}
	return &start, &end, nil
}
func lockResourceRecord(ctx context.Context, tx *Tx, id string, allowArchived bool) error {
	var found string
	e := tx.QueryRow(ctx, "SELECT id FROM resources WHERE id=?1 AND deleted_at IS NULL AND (?2 OR archived_at IS NULL)", id, allowArchived).Scan(&found)
	if e == sql.ErrNoRows {
		return invalid("Choose an available resource from this organization")
	}
	return e
}
func requireActiveResource(ctx context.Context, tx *Tx, id string) error {
	return lockResourceRecord(ctx, tx, id, false)
}

func lockActiveProject(ctx context.Context, tx *Tx, id string) error {
	var found string
	e := tx.QueryRow(ctx, "SELECT id FROM projects WHERE id=?1 AND deleted_at IS NULL", id).Scan(&found)
	if e == sql.ErrNoRows {
		return &validation{404, "Project not found"}
	}
	return e
}
func activityDates(ctx context.Context, tx *Tx, project string, start, end *string) error {
	var ps, pe *string
	if e := tx.QueryRow(ctx, "SELECT start_date,end_date FROM projects WHERE id=?1 AND deleted_at IS NULL", project).Scan(&ps, &pe); e == sql.ErrNoRows {
		return &validation{404, "Project not found"}
	} else if e != nil {
		return e
	}
	if start != nil && ps != nil && (*start < *ps || *end > *pe) {
		return invalid("Activity dates must fall within the project dates")
	}
	return nil
}
func (a *App) managementWrite(r *http.Request, tx *Tx, org, user, role, path string, w http.ResponseWriter) (bool, string, error) {
	ctx := r.Context()
	parts := strings.Split(path, "/")
	if len(parts) == 3 && parts[0] == "sprints" && parts[2] == "edit" {
		var b struct {
			Name            string `json:"name"`
			Start           string `json:"start_date"`
			End             string `json:"end_date"`
			Project         string `json:"project_id"`
			PreviousProject string `json:"previous_project"`
			PreviousName    string `json:"previous_name"`
			PreviousStart   string `json:"previous_start"`
			PreviousEnd     string `json:"previous_end"`
		}
		if !decode(w, r, &b) {
			return true, "", invalid("Invalid sprint")
		}
		if strings.TrimSpace(b.Name) == "" {
			return true, "", invalid("Sprint name required")
		}
		if _, e := time.Parse("2006-01-02", b.Start); e != nil {
			return true, "", invalid("Valid start date required")
		}
		if _, e := time.Parse("2006-01-02", b.End); e != nil || b.End < b.Start {
			return true, "", invalid("End date must be on or after start date")
		}
		if b.Project == "" {
			return true, "", invalid("Project required")
		}
		if e := lockActiveProject(ctx, tx, b.Project); e != nil {
			return true, "", e
		}
		var name, start, end, previousProject string
		e := tx.QueryRow(ctx, "SELECT name,start_date,end_date,coalesce(project_id,'') FROM sprints WHERE id=?1", parts[1]).Scan(&name, &start, &end, &previousProject)
		if e == sql.ErrNoRows {
			return true, "", &validation{404, "Sprint not found"}
		} else if e != nil {
			return true, "", e
		}
		if name != b.PreviousName || start != b.PreviousStart || end != b.PreviousEnd || previousProject != b.PreviousProject {
			return true, "", &validation{409, "Sprint changed; refresh before saving"}
		}
		var incompatible bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE sprint_id=?1 AND project_id<>?2)", parts[1], b.Project).Scan(&incompatible); e != nil {
			return true, "", e
		}
		if incompatible {
			return true, "", invalid("Existing sprint tasks belong to a different project")
		}
		var outside bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tasks t JOIN projects p ON p.id=t.project_id AND p.org_id=t.org_id WHERE t.sprint_id=?1 AND p.deleted_at IS NULL AND (t.start_date<?2 OR t.target_date>?3))", parts[1], b.Start, b.End).Scan(&outside); e != nil {
			return true, "", e
		}
		if outside {
			return true, "", &validation{409, "Sprint dates would exclude existing tasks. Keep dates that contain all linked tasks."}
		}
		if _, e = tx.Exec(ctx, "UPDATE sprints SET name=?2,start_date=?3,end_date=?4,project_id=?5 WHERE id=?1", parts[1], strings.TrimSpace(b.Name), b.Start, b.End, b.Project); e != nil {
			return true, "", e
		}
		e = audit(ctx, tx, org, user, parts[1], path, map[string]any{"previous_name": name, "previous_start": start, "previous_end": end, "change": b})
		return true, parts[1], e
	}
	kind := parts[0]
	edit := len(parts) == 3 && parts[2] == "edit"
	remove := (kind == "project-activities" || kind == "projects") && len(parts) == 3 && parts[2] == "delete"
	lifecycle := kind == "resources" && len(parts) == 3 && (parts[2] == "archive" || parts[2] == "restore" || parts[2] == "delete")
	if kind != "resources" && kind != "projects" && kind != "project-activities" {
		return false, "", nil
	}
	if len(parts) != 1 && !edit && !remove && !lifecycle {
		return false, "", nil
	}
	id := ""
	if edit || remove || lifecycle {
		id = parts[1]
	}
	var e error
	var detail any
	if lifecycle {
		var b struct {
			Version int `json:"version"`
		}
		if !decode(w, r, &b) {
			return true, "", invalid("Invalid resource action")
		}
		var version int
		var archived *string
		var before []byte
		e = tx.QueryRow(ctx, "SELECT version,archived_at,row_to_json(x) FROM resources x WHERE id=?1 AND deleted_at IS NULL", id).Scan(&version, &archived, &before)
		if e == sql.ErrNoRows {
			return true, "", &validation{404, "Resource not found"}
		}
		if e != nil {
			return true, "", e
		}
		if b.Version != version {
			return true, "", &validation{409, "Resource changed; refresh and try again"}
		}
		if parts[2] == "delete" {
			var used bool
			e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE primary_owner_id=?1) OR EXISTS(SELECT 1 FROM allocations WHERE resource_id=?1) OR EXISTS(SELECT 1 FROM worklogs WHERE resource_id=?1) OR EXISTS(SELECT 1 FROM projects WHERE prime_resource_id=?1) OR EXISTS(SELECT 1 FROM audit_events WHERE json_extract(detail,'$.previous_owner_id')=?1 OR json_extract(detail,'$.change.primary_owner_id')=?1)`, id).Scan(&used)
			if e != nil {
				return true, "", e
			}
			if used {
				return true, "", &validation{409, "This resource is linked to tasks, allocations, recorded work or a project. Archive it instead."}
			}
			if e = audit(ctx, tx, org, user, id, path, map[string]any{"deleted_resource": json.RawMessage(before)}); e != nil {
				return true, "", e
			}
			_, e = tx.Exec(ctx, "UPDATE resources SET deleted_at=(strftime('%Y-%m-%dT%H:%M:%f','now') || '000Z'),version=version+1 WHERE id=?1", id)
			return true, id, e
		}
		if (parts[2] == "archive" && archived != nil) || (parts[2] == "restore" && archived == nil) {
			return true, "", &validation{409, "Resource is already in this state; refresh"}
		}
		if e = audit(ctx, tx, org, user, id, path, map[string]any{"previous": json.RawMessage(before)}); e != nil {
			return true, "", e
		}
		if parts[2] == "archive" {
			_, e = tx.Exec(ctx, "UPDATE resources SET archived_at=(strftime('%Y-%m-%dT%H:%M:%f','now') || '000Z'),version=version+1 WHERE id=?1", id)
		} else {
			_, e = tx.Exec(ctx, "UPDATE resources SET archived_at=NULL,version=version+1 WHERE id=?1", id)
		}
		return true, id, e
	}
	if remove {
		var b struct {
			Version            int    `json:"version"`
			ConfirmName        string `json:"confirm_name"`
			IncludeLinkedTasks bool   `json:"include_linked_tasks"`
		}
		if !decode(w, r, &b) {
			return true, "", invalid("Invalid delete request")
		}
		if kind == "projects" {
			var version int
			var before []byte
			e = tx.QueryRow(ctx, "SELECT version,row_to_json(p) FROM projects p WHERE id=?1 AND deleted_at IS NULL", id).Scan(&version, &before)
			if e == sql.ErrNoRows {
				return true, "", &validation{404, "Project not found"}
			}
			if e != nil {
				return true, "", e
			}
			if b.Version != version {
				return true, "", &validation{409, "Project changed; refresh before deleting"}
			}
			var linked bool
			if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE project_id=?1)", id).Scan(&linked); e != nil {
				return true, "", e
			}
			if linked && !b.IncludeLinkedTasks {
				return true, "", &validation{409, "This project has linked Jira tasks and cannot be deleted. Its tasks and allocations have not been changed."}
			}
			if b.IncludeLinkedTasks {
				var name string
				if e = tx.QueryRow(ctx, "SELECT name FROM projects WHERE id=?1", id).Scan(&name); e != nil {
					return true, "", e
				}
				if b.ConfirmName != name {
					return true, "", invalid("Enter the exact project name to confirm deletion of linked tasks")
				}
				if _, e = tx.Exec(ctx, "UPDATE allocations SET released_at=(strftime('%Y-%m-%dT%H:%M:%f','now') || '000Z'),release_reason='Project deleted with linked tasks' WHERE task_id IN (SELECT id FROM tasks WHERE project_id=?1) AND released_at IS NULL AND day>=CURRENT_DATE", id); e != nil {
					return true, "", e
				}
			}
			if e = audit(ctx, tx, org, user, id, path, map[string]any{"deleted_project": json.RawMessage(before)}); e != nil {
				return true, "", e
			}
			_, e = tx.Exec(ctx, "UPDATE projects SET deleted_at=(strftime('%Y-%m-%dT%H:%M:%f','now') || '000Z'),version=version+1 WHERE id=?1", id)
			return true, id, e
		}
		var project string
		if e = tx.QueryRow(ctx, "SELECT project_id FROM project_activities WHERE id=?1", id).Scan(&project); e == sql.ErrNoRows {
			return true, "", &validation{404, "Activity not found"}
		} else if e != nil {
			return true, "", e
		}
		if e = lockActiveProject(ctx, tx, project); e != nil {
			return true, "", e
		}
		var version int
		var before []byte
		if e = tx.QueryRow(ctx, "SELECT version,row_to_json(x) FROM project_activities x WHERE id=?1", id).Scan(&version, &before); e == sql.ErrNoRows {
			return true, "", &validation{404, "Activity not found"}
		} else if e != nil {
			return true, "", e
		}
		if b.Version != version {
			return true, "", &validation{409, "Activity changed; refresh before deleting"}
		}
		if e = audit(ctx, tx, org, user, id, path, map[string]any{"deleted_activity": json.RawMessage(before)}); e != nil {
			return true, "", e
		}
		_, e = tx.Exec(ctx, "DELETE FROM project_activities WHERE id=?1", id)
		return true, id, e
	}

	switch kind {
	case "resources":
		var b resourceInput
		if !decode(w, r, &b) {
			return true, "", invalid("Invalid resource")
		}
		b.Name = strings.TrimSpace(b.Name)
		if b.Name == "" {
			return true, "", invalid("Resource name required")
		}
		hours := 8.0
		if b.Hours != nil {
			hours = *b.Hours
		}
		if hours <= 0 || hours > 24 {
			return true, "", invalid("Working hours must be greater than 0 and at most 24")
		}
		if b.Type != nil && *b.Type != "onshore" && *b.Type != "offshore" {
			return true, "", invalid("Choose onshore or offshore")
		}
		if b.Rate != nil && role != "admin" {
			return true, "", &validation{403, "Administrator required to manage rates"}
		}
		if edit {
			if b.Rate != nil {
				return true, "", invalid("Add a dated rate using the resource rate history")
			}
			var version int
			var before []byte
			e = tx.QueryRow(ctx, "SELECT version,row_to_json(x) FROM resources x WHERE id=?1 AND deleted_at IS NULL", id).Scan(&version, &before)
			if e != nil {
				return true, "", e
			}
			if version != b.Version {
				return true, "", &validation{409, "Resource changed; refresh and try again"}
			}
			_, e = tx.Exec(ctx, "UPDATE resources SET name=?2,location_type=?3,company_name=?4,daily_hours=?5,version=version+1 WHERE id=?1", id, b.Name, b.Type, strings.TrimSpace(b.Company), hours)
			detail = map[string]any{"previous": string(before), "change": b}
		} else {
			e = tx.QueryRow(ctx, "INSERT INTO resources(org_id,name,location_type,company_name,daily_hours) VALUES(?1,?2,?3,?4,?5) RETURNING id", org, b.Name, b.Type, strings.TrimSpace(b.Company), hours).Scan(&id)
			if e == nil && b.Rate != nil {
				if b.Effective == "" {
					b.Effective = time.Now().UTC().Format("2006-01-02")
				}
				if _, err := time.Parse("2006-01-02", b.Effective); err != nil {
					return true, "", invalid("Valid rate effective date required")
				}
				_, e = tx.Exec(ctx, "INSERT INTO rates(org_id,resource_id,effective_from,hourly_cents) VALUES(?1,?2,?3,?4)", org, id, b.Effective, *b.Rate)
			}
			detail = map[string]any{"name": b.Name, "location_type": b.Type, "company_name": b.Company, "daily_hours": hours}
		}
	case "projects":
		var b projectInput
		if !decode(w, r, &b) {
			return true, "", invalid("Invalid project")
		}
		b.Name = strings.TrimSpace(b.Name)
		if b.Name == "" {
			return true, "", invalid("Project name required")
		}
		if !edit && (b.Prime == nil || strings.TrimSpace(*b.Prime) == "") {
			return true, "", invalid("Choose a prime resource for the project")
		}
		if b.Prime != nil {
			var exists bool
			if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM resources r WHERE r.id=?1 AND r.deleted_at IS NULL AND (r.archived_at IS NULL OR (?2 AND EXISTS(SELECT 1 FROM projects p WHERE p.id=?3 AND p.prime_resource_id=r.id AND p.deleted_at IS NULL))))", *b.Prime, edit, id).Scan(&exists); e != nil {
				return true, "", e
			}
			if !exists {
				return true, "", invalid("Choose a prime resource from this organization")
			}
			if e = lockResourceRecord(ctx, tx, *b.Prime, true); e != nil {
				return true, "", e
			}
		}
		start, end, err := dates(b.Start, b.End)
		if err != nil {
			return true, "", err
		}
		if edit && b.Activities != nil {
			return true, "", invalid("Manage existing project activities using the activity editor")
		}
		prepared := make([]initialActivity, 0, len(b.Activities))
		seen := map[string]bool{}
		for _, item := range b.Activities {
			item.Name = strings.TrimSpace(item.Name)
			if item.Name == "" && item.Start == "" && item.End == "" {
				continue
			}
			if item.Name == "" {
				return true, "", invalid("Each activity with dates needs a name")
			}
			key := strings.ToLower(item.Name)
			if seen[key] {
				return true, "", invalid("Activity names must be unique within the project")
			}
			seen[key] = true
			as, ae, err := dates(item.Start, item.End)
			if err != nil {
				return true, "", err
			}
			if as != nil && start != nil && (*as < *start || *ae > *end) {
				return true, "", invalid("Activity dates must fall within the project dates")
			}
			prepared = append(prepared, item)
		}
		b.Activities = prepared
		var closed *string
		if b.Closed != nil && *b.Closed != "" {
			if _, err := time.Parse("2006-01-02", *b.Closed); err != nil {
				return true, "", invalid("Invalid closed date")
			}
			if start != nil && *b.Closed < *start {
				return true, "", invalid("Closed date must be on or after the project start date")
			}
			closed = b.Closed
		}
		if edit {
			var version int
			var before []byte
			e = tx.QueryRow(ctx, "SELECT version,row_to_json(p) FROM projects p WHERE id=?1 AND deleted_at IS NULL", id).Scan(&version, &before)
			if e != nil {
				return true, "", e
			}
			if version != b.Version {
				return true, "", &validation{409, "Project changed; refresh and try again"}
			}
			if start != nil {
				var conflict bool
				e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM project_activities WHERE project_id=?1 AND (start_date<?2 OR end_date>?3))", id, *start, *end).Scan(&conflict)
				if e != nil {
					return true, "", e
				}
				if conflict {
					return true, "", invalid("Activities fall outside these project dates; adjust or delete them first")
				}
			}
			_, e = tx.Exec(ctx, "UPDATE projects SET name=?2,start_date=?3,end_date=?4,pm_name=?5,budget_cents=coalesce(?6,budget_cents),prime_resource_id=coalesce(?7,prime_resource_id),closed_date=CASE WHEN ?8 THEN ?9 ELSE closed_date END,version=version+1 WHERE id=?1", id, b.Name, start, end, strings.TrimSpace(b.PM), b.Budget, b.Prime, b.Closed != nil, closed)
			detail = map[string]any{"previous": string(before), "change": b}
		} else {
			budget := int64(0)
			if b.Budget != nil {
				budget = *b.Budget
			}
			e = tx.QueryRow(ctx, "INSERT INTO projects(org_id,name,start_date,end_date,pm_name,budget_cents,prime_resource_id,closed_date) VALUES(?1,?2,?3,?4,?5,?6,?7,?8) RETURNING id", org, b.Name, start, end, strings.TrimSpace(b.PM), budget, b.Prime, closed).Scan(&id)
			if e == nil {
				for _, item := range prepared {
					as, ae, _ := dates(item.Start, item.End)
					var activityID string
					if e = tx.QueryRow(ctx, "INSERT INTO project_activities(org_id,project_id,name,start_date,end_date) VALUES(?1,?2,?3,?4,?5) RETURNING id", org, id, item.Name, as, ae).Scan(&activityID); e != nil {
						return true, "", e
					}
					if e = audit(ctx, tx, org, user, activityID, "project-activities", item); e != nil {
						return true, "", e
					}
				}
			}
			detail = b
		}
	case "project-activities":
		var b activityInput
		if !decode(w, r, &b) {
			return true, "", invalid("Invalid project activity")
		}
		b.Name = strings.TrimSpace(b.Name)
		if b.Name == "" {
			return true, "", invalid("Activity name required")
		}
		start, end, err := dates(b.Start, b.End)
		if err != nil {
			return true, "", err
		}
		if !edit {
			if e = activityDates(ctx, tx, b.Project, start, end); e != nil {
				return true, "", e
			}
			e = tx.QueryRow(ctx, "INSERT INTO project_activities(org_id,project_id,name,start_date,end_date) VALUES(?1,?2,?3,?4,?5) RETURNING id", org, b.Project, b.Name, start, end).Scan(&id)
			detail = b
		} else {
			var project string
			if e = tx.QueryRow(ctx, "SELECT project_id FROM project_activities WHERE id=?1", id).Scan(&project); e != nil {
				return true, "", e
			}
			if b.Project != "" && b.Project != project {
				return true, "", invalid("An activity cannot move to a different project")
			}
			if e = activityDates(ctx, tx, project, start, end); e != nil {
				return true, "", e
			}
			var version int
			var before []byte
			e = tx.QueryRow(ctx, "SELECT version,row_to_json(x) FROM project_activities x WHERE id=?1", id).Scan(&version, &before)
			if e != nil {
				return true, "", e
			}
			if version != b.Version {
				return true, "", &validation{409, "Activity changed; refresh and try again"}
			}
			_, e = tx.Exec(ctx, "UPDATE project_activities SET name=?2,start_date=?3,end_date=?4,version=version+1 WHERE id=?1", id, b.Name, start, end)
			detail = map[string]any{"previous": string(before), "change": b}
		}
	}
	if e == sql.ErrNoRows {
		return true, "", &validation{404, "Record not found"}
	}
	if e == nil {
		e = audit(ctx, tx, org, user, id, path, detail)
	}
	return true, id, e
}

func lockTaskProject(ctx context.Context, tx *Tx, id string) error {
	var project string
	if e := tx.QueryRow(ctx, "SELECT project_id FROM tasks WHERE id=?1", id).Scan(&project); e == sql.ErrNoRows {
		return &validation{404, "Task not found"}
	} else if e != nil {
		return e
	}
	return lockActiveProject(ctx, tx, project)
}
