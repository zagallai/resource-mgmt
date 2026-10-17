package main

import (
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

func (a *App) write(r *http.Request, tx pgx.Tx, org, user, role, path string, w http.ResponseWriter) (string, error) {
	ctx := r.Context()
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
		e = tx.QueryRow(ctx, "INSERT INTO invitations(org_id,email,role) VALUES($1,$2,$3) RETURNING id", org, b.Email, b.Role).Scan(&id)
	case "resources":
		var b struct {
			Name       string  `json:"name"`
			DailyHours float64 `json:"daily_hours"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid resource")
		}
		e = tx.QueryRow(ctx, "INSERT INTO resources(org_id,name,daily_hours) VALUES($1,$2,$3) RETURNING id", org, b.Name, b.DailyHours).Scan(&id)
	case "projects":
		var b struct {
			Name   string `json:"name"`
			Budget int64  `json:"budget_cents"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid project")
		}
		e = tx.QueryRow(ctx, "INSERT INTO projects(org_id,name,budget_cents) VALUES($1,$2,$3) RETURNING id", org, b.Name, b.Budget).Scan(&id)
	case "sprints":
		var b struct {
			Name  string `json:"name"`
			Start string `json:"start_date"`
			End   string `json:"end_date"`
		}
		if !decode(w, r, &b) {
			return "", invalid("Invalid sprint")
		}
		e = tx.QueryRow(ctx, "INSERT INTO sprints(org_id,name,start_date,end_date) VALUES($1,$2,$3,$4) RETURNING id", org, b.Name, b.Start, b.End).Scan(&id)
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
		e = tx.QueryRow(ctx, "INSERT INTO rates(org_id,resource_id,effective_from,hourly_cents) VALUES($1,$2,$3,$4) RETURNING id", org, b.Resource, b.From, b.Cents).Scan(&id)
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
		if strings.TrimSpace(b.Key) == "" || strings.TrimSpace(b.Title) == "" {
			return "", invalid("Jira key and title required")
		}
		e = tx.QueryRow(ctx, "INSERT INTO tasks(org_id,project_id,jira_key,title,estimate_hours,start_date,target_date,primary_owner_id,sprint_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id", org, b.Project, b.Key, b.Title, b.Estimate, b.Start, b.Target, b.Owner, b.Sprint).Scan(&id)
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
		var status string
		e = tx.QueryRow(ctx, "SELECT status FROM tasks WHERE id=$1 FOR UPDATE", b.Task).Scan(&status)
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
		e = tx.QueryRow(ctx, "INSERT INTO allocations(org_id,task_id,resource_id,day,hours) VALUES($1,$2,$3,$4,$5) RETURNING id", org, b.Task, b.Resource, b.Day, b.Hours).Scan(&id)
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
		if _, e = time.Parse("2006-01-02", b.Day); e != nil || b.Day > time.Now().UTC().Format("2006-01-02") {
			return "", invalid("Work date must be today or earlier")
		}
		e = tx.QueryRow(ctx, `INSERT INTO worklogs(org_id,task_id,resource_id,day,hours,hourly_cents) SELECT $1,$2,$3,$4,$5,hourly_cents FROM rates WHERE resource_id=$3 AND effective_from<=$4::date ORDER BY effective_from DESC LIMIT 1 RETURNING id`, org, b.Task, b.Resource, b.Day, b.Hours).Scan(&id)
		if e == pgx.ErrNoRows {
			return "", invalid("Add a rate effective on the work date first")
		}
	default:
		parts := strings.Split(path, "/")
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
		var version int
		var status, previousOwner string
		e = tx.QueryRow(ctx, "SELECT version,status,primary_owner_id FROM tasks WHERE id=$1 FOR UPDATE", id).Scan(&version, &status, &previousOwner)
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
			_, e = tx.Exec(ctx, "UPDATE tasks SET status=$2,version=version+1 WHERE id=$1", id, b.Status)
			if e == nil && (b.Status == "hold" || b.Status == "done") {
				_, e = tx.Exec(ctx, "UPDATE allocations SET released_at=now(),release_reason=$2 WHERE task_id=$1 AND released_at IS NULL AND day>=CURRENT_DATE", id, b.Reason)
			}
		case "owner":
			_, e = tx.Exec(ctx, "UPDATE tasks SET primary_owner_id=$2,version=version+1 WHERE id=$1", id, b.Owner)
			if e == nil {
				_, e = tx.Exec(ctx, "UPDATE allocations SET released_at=now(),release_reason=$2 WHERE task_id=$1 AND released_at IS NULL AND day>=CURRENT_DATE", id, b.Reason)
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
func (a *App) read(w http.ResponseWriter, r *http.Request, tx pgx.Tx, path, role string) {
	queries := map[string]string{
		"resources":      "SELECT * FROM resources ORDER BY name",
		"projects":       "SELECT * FROM projects ORDER BY name",
		"sprints":        "SELECT * FROM sprints ORDER BY start_date",
		"tasks":          `SELECT t.*,coalesce((SELECT sum(hours) FROM worklogs w WHERE w.task_id=t.id),0) actual_hours,greatest(0,estimate_hours-coalesce((SELECT sum(hours) FROM worklogs w WHERE w.task_id=t.id),0)) remaining_hours FROM tasks t ORDER BY jira_key`,
		"allocations":    "SELECT * FROM allocations ORDER BY day,created_at",
		"history":        "SELECT * FROM audit_events ORDER BY created_at DESC LIMIT 200",
		"rates":          "SELECT * FROM rates ORDER BY effective_from DESC",
		"worklogs":       "SELECT org_id,id,task_id,resource_id,day,hours FROM worklogs ORDER BY day DESC",
		"capacity":       `SELECT r.id resource_id,r.name,s.id sprint_id,s.name sprint_name,r.daily_hours*(SELECT count(*) FROM generate_series(s.start_date::timestamp,s.end_date::timestamp,interval '1 day') d WHERE extract(isodow FROM d)<6) capacity_hours,coalesce((SELECT sum(a.hours) FROM allocations a WHERE a.resource_id=r.id AND a.released_at IS NULL AND a.day BETWEEN s.start_date AND s.end_date),0) allocated_hours FROM resources r CROSS JOIN sprints s ORDER BY s.start_date,r.name`,
		"dashboard":      `SELECT p.id,p.name,p.budget_cents,coalesce((SELECT round(sum(w.hours*w.hourly_cents)) FROM worklogs w JOIN tasks t ON t.id=w.task_id WHERE t.project_id=p.id),0) actual_cents,coalesce((SELECT round(sum(a.hours*(SELECT hourly_cents FROM rates r WHERE r.resource_id=a.resource_id AND r.effective_from<=a.day ORDER BY r.effective_from DESC LIMIT 1))) FROM allocations a JOIN tasks t ON t.id=a.task_id WHERE t.project_id=p.id AND a.released_at IS NULL AND a.day>=CURRENT_DATE),0) forecast_cents,(SELECT count(*) FROM allocations a JOIN tasks t ON t.id=a.task_id WHERE t.project_id=p.id AND a.released_at IS NULL AND a.day>=CURRENT_DATE AND NOT EXISTS(SELECT 1 FROM rates r WHERE r.resource_id=a.resource_id AND r.effective_from<=a.day)) unpriced_allocations FROM projects p ORDER BY p.name`,
		"daily-capacity": `SELECT r.id resource_id,r.name,a.day,r.daily_hours,sum(a.hours) allocated_hours,sum(a.hours)>r.daily_hours overloaded FROM allocations a JOIN resources r ON r.id=a.resource_id WHERE a.released_at IS NULL GROUP BY r.id,r.name,r.daily_hours,a.day ORDER BY a.day,r.name`,
	}
	if path == "projects" && role == "viewer" {
		queries[path] = "SELECT org_id,id,name FROM projects ORDER BY name"
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
	var raw []byte
	e := tx.QueryRow(r.Context(), "SELECT coalesce(json_agg(row_to_json(x)),'[]') FROM ("+q+") x").Scan(&raw)
	if e != nil {
		respond(w, 500, map[string]string{"error": "Unable to read data"})
		return
	}
	respond(w, 200, json.RawMessage(raw))
}
