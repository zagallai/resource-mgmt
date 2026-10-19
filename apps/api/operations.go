package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"
)

func (a *App) write(r *http.Request, tx *Tx, org, user, role, path string, w http.ResponseWriter) (string, error) {
	ctx := r.Context()
	if matched, id, err := a.managementWrite(r, tx, org, user, role, path, w); matched {
		return id, err
	}
	var id string
	var e error
	switch path {
	case "invitations":
		if role != "admin" {
			return "", &validation{403, "Administrator required"}
		}
		var b struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if !decode(w, r, &b) {
			return "", &validation{400, "Invalid invitation"}
		}
		b.Email = strings.ToLower(strings.TrimSpace(b.Email))
		if !strings.Contains(b.Email, "@") || strings.ContainsAny(b.Email, "\r\n") {
			return "", invalid("Valid email required")
		}
		e = tx.QueryRow(ctx, "INSERT INTO invitations(org_id,email,role) VALUES(?1,?2,?3) RETURNING id", org, b.Email, b.Role).Scan(&id)
	case "sprints":
		var b struct {
			Name    string `json:"name"`
			Start   string `json:"start_date"`
			End     string `json:"end_date"`
			Project string `json:"project_id"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid sprint")
		}
		if start, _, err := dates(b.Start, b.End); err != nil || start == nil {
			return "", invalid("Valid sprint start and end dates required")
		}
		if b.Project == "" {
			return "", invalid("Project required")
		}
		if e = lockActiveProject(ctx, tx, b.Project); e != nil {
			return "", e
		}
		if strings.TrimSpace(b.Name) == "" {
			return "", invalid("Sprint name required")
		}
		e = tx.QueryRow(ctx, "INSERT INTO sprints(org_id,name,start_date,end_date,project_id) VALUES(?1,?2,?3,?4,?5) RETURNING id", org, strings.TrimSpace(b.Name), b.Start, b.End, b.Project).Scan(&id)
	case "rates":
		if role != "admin" {
			return "", &validation{403, "Administrator required"}
		}
		var b struct {
			Resource string `json:"resource_id"`
			From     string `json:"effective_from"`
			Cents    int64  `json:"hourly_cents"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid rate")
		}
		if _, err := time.Parse("2006-01-02", b.From); err != nil {
			return "", invalid("Valid rate effective date required")
		}
		if e = lockResourceRecord(ctx, tx, b.Resource, true); e != nil {
			return "", e
		}
		e = tx.QueryRow(ctx, "INSERT INTO rates(org_id,resource_id,effective_from,hourly_cents) VALUES(?1,?2,?3,?4) RETURNING id", org, b.Resource, b.From, b.Cents).Scan(&id)
	case "tasks":
		var b struct {
			Project  string  `json:"project_id"`
			Key      string  `json:"jira_key"`
			Title    string  `json:"title"`
			Estimate float64 `json:"estimate_hours"`
			Start    string  `json:"start_date"`
			Target   string  `json:"target_date"`
			Owner    string  `json:"primary_owner_id"`
			Sprint   *string `json:"sprint_id"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid task")
		}
		if start, _, err := dates(b.Start, b.Target); err != nil || start == nil {
			return "", invalid("Valid task start and target dates required")
		}
		if strings.TrimSpace(b.Title) == "" {
			return "", invalid("Task title required")
		}
		if e = requireActiveResource(ctx, tx, b.Owner); e != nil {
			return "", e
		}
		if e = lockActiveProject(ctx, tx, b.Project); e != nil {
			return "", e
		}
		if b.Sprint != nil {
			var start, end, sprintProject string
			e = tx.QueryRow(ctx, "SELECT start_date,end_date,coalesce(project_id,'') FROM sprints WHERE id=?1", *b.Sprint).Scan(&start, &end, &sprintProject)
			if e == sql.ErrNoRows {
				return "", &validation{404, "Sprint not found"}
			} else if e != nil {
				return "", e
			}
			if sprintProject != b.Project {
				return "", invalid("Sprint must belong to the selected project")
			}
			if b.Start < start || b.Target > end {
				return "", invalid("Task dates must stay within the selected sprint (" + start + " to " + end + ")")
			}
		}
		e = tx.QueryRow(ctx, "INSERT INTO tasks(org_id,project_id,jira_key,title,estimate_hours,start_date,target_date,primary_owner_id,sprint_id) VALUES(?1,?2,NULLIF(trim(?3),''),?4,?5,?6,?7,?8,?9) RETURNING id", org, b.Project, b.Key, b.Title, b.Estimate, b.Start, b.Target, b.Owner, b.Sprint).Scan(&id)
	case "allocations":
		var b struct {
			Task     string  `json:"task_id"`
			Resource string  `json:"resource_id"`
			Day      string  `json:"day"`
			Hours    float64 `json:"hours"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid allocation")
		}
		if e = requireActiveResource(ctx, tx, b.Resource); e != nil {
			return "", e
		}
		if e = lockTaskProject(ctx, tx, b.Task); e != nil {
			return "", e
		}
		var status string
		e = tx.QueryRow(ctx, "SELECT status FROM tasks WHERE id=?1", b.Task).Scan(&status)
		if e != nil {
			return "", e
		}
		if status == "hold" || status == "done" {
			return "", invalid("Resume task before scheduling")
		}
		if _, e = time.Parse("2006-01-02", b.Day); e != nil {
			return "", invalid("Invalid date")
		}
		if b.Day < time.Now().UTC().Format("2006-01-02") {
			return "", invalid("Schedule today or later; record past work as a worklog")
		}
		e = tx.QueryRow(ctx, "INSERT INTO allocations(org_id,task_id,resource_id,day,hours) VALUES(?1,?2,?3,?4,?5) RETURNING id", org, b.Task, b.Resource, b.Day, b.Hours).Scan(&id)
	case "worklogs":
		var b struct {
			Task     string  `json:"task_id"`
			Resource string  `json:"resource_id"`
			Day      string  `json:"day"`
			Hours    float64 `json:"hours"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid worklog")
		}
		if e = lockResourceRecord(ctx, tx, b.Resource, true); e != nil {
			return "", e
		}
		if e = lockTaskProject(ctx, tx, b.Task); e != nil {
			return "", e
		}
		if _, e = time.Parse("2006-01-02", b.Day); e != nil || b.Day > time.Now().UTC().Format("2006-01-02") {
			return "", invalid("Work date must be today or earlier")
		}
		e = tx.QueryRow(ctx, `INSERT INTO worklogs(org_id,task_id,resource_id,day,hours,hourly_cents) SELECT ?1,?2,?3,?4,?5,hourly_cents FROM rates WHERE resource_id=?3 AND effective_from<=?4 ORDER BY effective_from DESC LIMIT 1 RETURNING id`, org, b.Task, b.Resource, b.Day, b.Hours).Scan(&id)
		if e == sql.ErrNoRows {
			return "", invalid("Add a rate effective on the work date first")
		}
	default:
		parts := strings.Split(path, "/")
		if len(parts) == 3 && parts[0] == "resources" && parts[2] == "capacity" {
			var b struct {
				Hours *float64 `json:"daily_hours"`
			}
			if !decode(w, r, &b) || b.Hours == nil || *b.Hours <= 0 || *b.Hours > 24 {
				return "", invalid("Working hours per day must be greater than 0 and at most 24")
			}
			id = parts[1]
			var previous float64
			e = tx.QueryRow(ctx, "SELECT daily_hours FROM resources WHERE id=?1 AND deleted_at IS NULL", id).Scan(&previous)
			if e == sql.ErrNoRows {
				return "", &validation{404, "Resource not found"}
			}
			if e != nil {
				return "", e
			}
			_, e = tx.Exec(ctx, "UPDATE resources SET daily_hours=?2,version=version+1 WHERE id=?1", id, *b.Hours)
			if e == nil {
				e = audit(ctx, tx, org, user, id, path, map[string]any{"previous_daily_hours": previous, "daily_hours": *b.Hours})
			}
			return id, e
		}
		if len(parts) != 3 || parts[0] != "tasks" {
			return "", &validation{404, "Unknown operation"}
		}
		id = parts[1]
		var b struct {
			Version int    `json:"version"`
			Status  string `json:"status"`
			Owner   string `json:"primary_owner_id"`
			Reason  string `json:"reason"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid change")
		}
		if strings.TrimSpace(b.Reason) == "" {
			return "", invalid("Reason required for historical changes")
		}
		if e = lockTaskProject(ctx, tx, id); e != nil {
			return "", e
		}
		var version int
		var status, previousOwner string
		e = tx.QueryRow(ctx, "SELECT version,status,primary_owner_id FROM tasks WHERE id=?1", id).Scan(&version, &status, &previousOwner)
		if e != nil {
			return "", e
		}
		if version != b.Version {
			return "", &validation{409, "Task changed; refresh before saving"}
		}
		switch parts[2] {
		case "status":
			if b.Status != "planned" && b.Status != "active" && b.Status != "hold" && b.Status != "done" {
				return "", invalid("Invalid status")
			}
			_, e = tx.Exec(ctx, "UPDATE tasks SET status=?2,version=version+1 WHERE id=?1", id, b.Status)
			if e == nil && (b.Status == "hold" || b.Status == "done") {
				_, e = tx.Exec(ctx, "UPDATE allocations SET released_at=(strftime('%Y-%m-%dT%H:%M:%f','now') || '000Z'),release_reason=?2 WHERE task_id=?1 AND released_at IS NULL AND day>=CURRENT_DATE", id, b.Reason)
			}
		case "owner":
			if e = requireActiveResource(ctx, tx, b.Owner); e != nil {
				return "", e
			}
			_, e = tx.Exec(ctx, "UPDATE tasks SET primary_owner_id=?2,version=version+1 WHERE id=?1", id, b.Owner)
			if e == nil {
				_, e = tx.Exec(ctx, "UPDATE allocations SET released_at=(strftime('%Y-%m-%dT%H:%M:%f','now') || '000Z'),release_reason=?2 WHERE task_id=?1 AND released_at IS NULL AND day>=CURRENT_DATE", id, b.Reason)
			}
		default:
			return "", &validation{404, "Unknown task operation"}
		}
		if e == nil {
			e = audit(ctx, tx, org, user, id, path, map[string]any{"previous_status": status, "previous_owner_id": previousOwner, "change": b})
		}
		return id, e
	}
	if e == nil {
		e = audit(ctx, tx, org, user, id, path, map[string]string{"operation": "created"})
	}
	return id, e
}
func (a *App) read(w http.ResponseWriter, r *http.Request, tx *Tx, path, role string) {
	queries := map[string]string{
		"resources":          "SELECT * FROM resources WHERE archived_at IS NULL AND deleted_at IS NULL ORDER BY created_at DESC,id DESC",
		"archived-resources": "SELECT * FROM resources WHERE archived_at IS NOT NULL AND deleted_at IS NULL ORDER BY created_at DESC,id DESC",
		"project-activities": "SELECT a.* FROM project_activities a JOIN projects p ON p.id=a.project_id AND p.org_id=a.org_id WHERE p.deleted_at IS NULL ORDER BY a.project_id,CASE upper(trim(a.name)) WHEN 'LLD' THEN 1 WHEN 'SIT' THEN 2 WHEN 'UAT' THEN 3 WHEN 'RB' THEN 4 WHEN 'RF' THEN 5 WHEN 'PTE' THEN 6 ELSE 7 END,a.name",
		"projects":           "SELECT * FROM projects WHERE deleted_at IS NULL ORDER BY name",
		"sprints":            "SELECT s.* FROM sprints s LEFT JOIN projects p ON p.id=s.project_id AND p.org_id=s.org_id WHERE s.project_id IS NULL OR p.deleted_at IS NULL ORDER BY s.created_at DESC,s.id DESC",
		"tasks":              `SELECT t.*,coalesce((SELECT sum(hours) FROM worklogs w WHERE w.task_id=t.id),0) actual_hours,max(0,estimate_hours-coalesce((SELECT sum(hours) FROM worklogs w WHERE w.task_id=t.id),0)) remaining_hours FROM tasks t JOIN projects p ON p.id=t.project_id WHERE p.deleted_at IS NULL ORDER BY jira_key`,
		"allocations":        "SELECT a.*,t.jira_key,t.title task_title FROM allocations a JOIN tasks t ON t.id=a.task_id ORDER BY a.day,a.created_at",
		"history":            "SELECT * FROM audit_events ORDER BY created_at DESC LIMIT 200",
		"rates":              "SELECT * FROM rates ORDER BY effective_from DESC",
		"worklogs":           "SELECT w.org_id,w.id,w.task_id,w.resource_id,w.day,w.hours,t.jira_key,t.title task_title FROM worklogs w JOIN tasks t ON t.id=w.task_id ORDER BY w.day DESC",
		"capacity":           `SELECT r.id resource_id,r.name,s.id sprint_id,s.name sprint_name,r.daily_hours*(WITH RECURSIVE dates(d) AS (SELECT s.start_date UNION ALL SELECT date(d,'+1 day') FROM dates WHERE d<s.end_date) SELECT count(*) FROM dates WHERE strftime('%w',d) NOT IN ('0','6')) capacity_hours,coalesce((SELECT sum(a.hours) FROM allocations a WHERE a.resource_id=r.id AND a.released_at IS NULL AND a.day BETWEEN s.start_date AND s.end_date),0) allocated_hours FROM resources r CROSS JOIN sprints s WHERE r.deleted_at IS NULL ORDER BY s.start_date,r.name`,
		"dashboard":          `SELECT p.id,p.name,p.budget_cents,coalesce((SELECT round(sum(w.hours*w.hourly_cents)) FROM worklogs w JOIN tasks t ON t.id=w.task_id WHERE t.project_id=p.id),0) actual_cents,coalesce((SELECT round(sum(a.hours*(SELECT hourly_cents FROM rates r WHERE r.resource_id=a.resource_id AND r.effective_from<=a.day ORDER BY r.effective_from DESC LIMIT 1))) FROM allocations a JOIN tasks t ON t.id=a.task_id WHERE t.project_id=p.id AND a.released_at IS NULL AND a.day>=CURRENT_DATE),0) forecast_cents,(SELECT count(*) FROM allocations a JOIN tasks t ON t.id=a.task_id WHERE t.project_id=p.id AND a.released_at IS NULL AND a.day>=CURRENT_DATE AND NOT EXISTS(SELECT 1 FROM rates r WHERE r.resource_id=a.resource_id AND r.effective_from<=a.day)) unpriced_allocations FROM projects p WHERE p.deleted_at IS NULL ORDER BY p.name`,
		"daily-capacity":     `SELECT r.id resource_id,r.name,a.day,r.daily_hours,sum(a.hours) allocated_hours,sum(a.hours)>r.daily_hours overloaded FROM allocations a JOIN resources r ON r.id=a.resource_id WHERE a.released_at IS NULL GROUP BY r.id,r.name,r.daily_hours,a.day ORDER BY a.day,r.name`,
	}
	if path == "projects" && role == "viewer" {
		queries[path] = "SELECT org_id,id,name,start_date,end_date,closed_date,pm_name,prime_resource_id,version FROM projects WHERE deleted_at IS NULL ORDER BY name"
	}
	if path == "resources" && role == "admin" {
		queries[path] = `SELECT r.*,(SELECT hourly_cents FROM rates x WHERE x.resource_id=r.id AND x.effective_from<=CURRENT_DATE ORDER BY effective_from DESC LIMIT 1) current_hourly_cents FROM resources r WHERE r.archived_at IS NULL AND r.deleted_at IS NULL ORDER BY r.created_at DESC,r.id DESC`
	}
	if path == "archived-resources" && role == "admin" {
		queries[path] = `SELECT r.*,(SELECT hourly_cents FROM rates x WHERE x.resource_id=r.id AND x.effective_from<=CURRENT_DATE ORDER BY effective_from DESC LIMIT 1) current_hourly_cents FROM resources r WHERE r.archived_at IS NOT NULL AND r.deleted_at IS NULL ORDER BY r.created_at DESC,r.id DESC`
	}
	q, ok := queries[path]
	if !ok {
		respond(w, 404, nil)
		return
	}
	if (path == "rates" || path == "history") && role != "admin" {
		respond(w, 403, map[string]string{"error": "Administrator required"})
		return
	}
	if path == "dashboard" && role == "viewer" {
		respond(w, 403, map[string]string{"error": "Financial access required"})
		return
	}
	rows, e := tx.QueryMaps(r.Context(), q)
	if e != nil {
		log.Printf("Read %s: %v", path, e)
		respond(w, 500, map[string]string{"error": "Unable to read data"})
		return
	}
	respond(w, 200, rows)
}
