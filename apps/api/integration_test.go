package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	db, e := openDB(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = migrate(ctx, db); e != nil {
		t.Fatal("Migration not idempotent", e)
	}
	owner := &DB{raw: db.raw, columns: db.columns, admin: true}
	a := &App{db: db, origin: "http://localhost:3000"}
	mail := map[string]string{}
	a.send = func(email, link string) error { mail[email] = strings.Split(link, "#token=")[1]; return nil }
	handler := a.handler()
	call := func(method, path string, body any, cookie *http.Cookie) (int, []byte, *http.Cookie) {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
		r.Header.Set("Origin", a.origin)
		r.RemoteAddr = "127.0.0.1:9000"
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		var c *http.Cookie
		if cs := w.Result().Cookies(); len(cs) > 0 {
			c = cs[0]
		}
		return w.Code, w.Body.Bytes(), c
	}
	post := func(path string, b any, c *http.Cookie) string {
		t.Helper()
		code, raw, _ := call("POST", path, b, c)
		if code != 201 {
			t.Fatalf("POST %s: %d %s", path, code, raw)
		}
		var x map[string]string
		json.Unmarshal(raw, &x)
		return x["id"]
	}
	get := func(path string, c *http.Cookie) []map[string]any {
		t.Helper()
		code, raw, _ := call("GET", path, nil, c)
		if code != 200 {
			t.Fatalf("GET %s: %d %s", path, code, raw)
		}
		var x []map[string]any
		if e := json.Unmarshal(raw, &x); e != nil {
			t.Fatal(e)
		}
		return x
	}
	var org, user, other string
	owner.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('Team One') RETURNING id").Scan(&org)
	owner.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('Other') RETURNING id").Scan(&other)
	owner.QueryRow(ctx, "INSERT INTO users(email) VALUES('admin@resourcemgmt.com') RETURNING id").Scan(&user)
	if _, e = owner.Exec(ctx, "INSERT INTO memberships VALUES($1,$2,'admin')", org, user); e != nil {
		t.Fatal(e)
	}
	code, raw, _ := call("POST", "/auth/request", map[string]string{"email": "admin@resourcemgmt.com"}, nil)
	if code != 202 {
		t.Fatal(code, string(raw))
	}
	link := mail["admin@resourcemgmt.com"]
	if len(link) != 64 {
		t.Fatal("No magic link delivered")
	}
	code, _, session := call("POST", "/auth/verify", map[string]string{"token": link}, nil)
	if code != 200 || session == nil || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Fatal("Sign-in or cookie failed")
	}
	code, _, _ = call("POST", "/auth/verify", map[string]string{"token": link}, nil)
	if code != 401 {
		t.Fatal("Magic-link replay allowed")
	}
	if len(get("/api/me", session)) != 1 {
		t.Fatal("Membership listing")
	}
	base := "/api/orgs/" + org + "/"
	t.Run("tenant isolation and row security", func(t *testing.T) {
		code, _, _ := call("GET", "/api/orgs/"+other+"/resources", nil, session)
		if code != 404 {
			t.Fatal("Tenant access allowed")
		}
		var n int
		if e := db.QueryRow(ctx, "SELECT count(*) FROM resources").Scan(&n); e != nil || n != 0 {
			t.Fatal("RLS fail-closed", e, n)
		}
		_, e := db.Exec(ctx, "INSERT INTO resources(org_id,name) VALUES($1,'forbidden')", org)
		if e == nil {
			t.Fatal("RLS accepted unscoped write")
		}
	})
	alex := post(base+"resources", map[string]any{"name": "Alex", "daily_hours": 8}, session)
	priya := post(base+"resources", map[string]any{"name": "Priya", "daily_hours": 8}, session)
	project := post(base+"projects", map[string]any{"name": "Payments", "budget_cents": 1000000, "prime_resource_id": alex}, session)
	today := time.Now().UTC().Format("2006-01-02")
	future := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	sprint := post(base+"sprints", map[string]any{"project_id": project, "name": "Sprint 1", "start_date": today, "end_date": time.Now().UTC().AddDate(0, 0, 13).Format("2006-01-02")}, session)
	post(base+"rates", map[string]any{"resource_id": alex, "effective_from": "2020-01-01", "hourly_cents": 10000}, session)
	post(base+"rates", map[string]any{"resource_id": priya, "effective_from": "2020-01-01", "hourly_cents": 5000}, session)
	task := post(base+"tasks", map[string]any{"project_id": project, "jira_key": "PAY-102", "title": "Validation", "estimate_hours": 24, "start_date": today, "target_date": future, "primary_owner_id": alex, "sprint_id": sprint}, session)
	for _, person := range []string{alex, priya} {
		post(base+"allocations", map[string]any{"task_id": task, "resource_id": person, "day": future, "hours": 6}, session)
	}
	post(base+"allocations", map[string]any{"task_id": task, "resource_id": alex, "day": future, "hours": 4}, session)
	t.Run("multiple contributors and overload", func(t *testing.T) {
		if len(get(base+"allocations", session)) != 3 {
			t.Fatal("Contributor allocations lost")
		}
		daily := get(base+"daily-capacity", session)
		found := false
		for _, r := range daily {
			if r["resource_id"] == alex && r["overloaded"] == true {
				found = true
			}
		}
		if !found {
			t.Fatal("Overload not reported")
		}
		capacity := get(base+"capacity", session)
		if len(capacity) != 2 || capacity[0]["capacity_hours"] != float64(80) {
			t.Fatal("Sprint capacity", capacity)
		}
	})
	t.Run("default and configurable working hours", func(t *testing.T) {
		defaultID := post(base+"resources", map[string]any{"name": "Default hours"}, session)
		found := false
		for _, row := range get(base+"resources", session) {
			if row["id"] == defaultID && row["daily_hours"] == float64(8) {
				found = true
			}
		}
		if !found {
			t.Fatal("Missing eight-hour default")
		}
		post(base+"resources/"+alex+"/capacity", map[string]any{"daily_hours": 6}, session)
		for _, row := range get(base+"capacity", session) {
			if row["resource_id"] == alex && row["capacity_hours"] != float64(60) {
				t.Fatal("Sprint capacity not recalculated", row)
			}
		}
		for _, row := range get(base+"daily-capacity", session) {
			if row["resource_id"] == alex && (row["daily_hours"] != float64(6) || row["overloaded"] != true) {
				t.Fatal("Daily capacity not recalculated", row)
			}
		}
		for _, hours := range []float64{0, -1, 25} {
			code, _, _ := call("POST", base+"resources/"+alex+"/capacity", map[string]any{"daily_hours": hours}, session)
			if code != 400 {
				t.Fatal("Invalid capacity accepted", hours, code)
			}
			code, _, _ = call("POST", base+"resources", map[string]any{"name": "Invalid", "daily_hours": hours}, session)
			if code != 400 {
				t.Fatal("Invalid resource capacity accepted", hours, code)
			}
		}
		auditFound := false
		for _, row := range get(base+"history", session) {
			if row["action"] == "resources/"+alex+"/capacity" {
				detail := row["detail"].(map[string]any)
				if detail["previous_daily_hours"] == float64(8) && detail["daily_hours"] == float64(6) {
					auditFound = true
				}
			}
		}
		if !auditFound {
			t.Fatal("Capacity change not audited")
		}
		post(base+"resources/"+alex+"/capacity", map[string]any{"daily_hours": 8}, session)
	})
	t.Run("dedicated resource and project management", func(t *testing.T) {
		person := post(base+"resources", map[string]any{"name": "Z Contractor", "location_type": "offshore", "company_name": "Partner Ltd", "hourly_cents": 8500, "effective_from": today}, session)
		var resource map[string]any
		for _, row := range get(base+"resources", session) {
			if row["id"] == person {
				resource = row
			}
		}
		if resource["company_name"] != "Partner Ltd" || resource["daily_hours"] != float64(8) || resource["current_hourly_cents"] != float64(8500) {
			t.Fatal("Resource profile or initial rate missing", resource)
		}
		post(base+"resources/"+person+"/edit", map[string]any{"name": "Z Contractor", "location_type": "onshore", "company_name": "New Partner", "daily_hours": 7, "version": 1}, session)
		code, _, _ := call("POST", base+"resources/"+person+"/edit", map[string]any{"name": "Stale", "daily_hours": 8, "version": 1}, session)
		if code != 409 {
			t.Fatal("Stale resource edit accepted", code)
		}
		proj := post(base+"projects", map[string]any{"name": "Z Rollout", "prime_resource_id": person, "start_date": today, "end_date": future, "pm_name": "Morgan"}, session)
		for _, row := range get(base+"project-activities", session) {
			if row["project_id"] == proj {
				t.Fatal("New project unexpectedly has prefilled activities")
			}
		}
		lldID := post(base+"project-activities", map[string]any{"project_id": proj, "name": "LLD"}, session)
		pteID := post(base+"project-activities", map[string]any{"project_id": proj, "name": "PTE"}, session)
		lld, pte := map[string]any{"id": lldID}, map[string]any{"id": pteID}

		post(base+"project-activities/"+lld["id"].(string)+"/edit", map[string]any{"name": "LLD revised", "start_date": today, "end_date": future, "version": 1}, session)
		post(base+"project-activities/"+pte["id"].(string)+"/edit", map[string]any{"name": "PTE updated", "version": 1}, session)
		custom := post(base+"project-activities", map[string]any{"project_id": proj, "name": "Penetration testing", "start_date": today, "end_date": future}, session)
		second := post(base+"projects", map[string]any{"name": "Z Other project", "prime_resource_id": priya, "start_date": today, "end_date": future, "pm_name": "Taylor"}, session)
		for _, row := range get(base+"project-activities", session) {
			if row["id"] == custom && row["project_id"] != proj {
				t.Fatal("Activity moved projects")
			}
			if row["project_id"] == second && row["name"] == "Penetration testing" {
				t.Fatal("Custom activity leaked to second project")
			}
		}
		code, _, _ = call("POST", base+"project-activities", map[string]any{"project_id": proj, "name": "Outside range", "start_date": "2020-01-01", "end_date": "2020-01-02"}, session)
		if code != 400 {
			t.Fatal("Out-of-range activity accepted", code)
		}
		code, _, _ = call("POST", base+"projects/"+proj+"/edit", map[string]any{"name": "Z Rollout", "prime_resource_id": person, "start_date": today, "end_date": today, "pm_name": "Morgan", "version": 1}, session)
		if code != 400 {
			t.Fatal("Project edit invalidated activity dates", code)
		}
		post(base+"project-activities/"+pte["id"].(string)+"/edit", map[string]any{"name": "PTE", "version": 2}, session)
		code, _, _ = call("POST", base+"project-activities/"+lld["id"].(string)+"/edit", map[string]any{"name": "Stale LLD", "version": 1}, session)
		if code != 409 {
			t.Fatal("Stale activity update accepted", code)
		}
		code, _, _ = call("POST", base+"project-activities", map[string]any{"project_id": proj, "name": "LLD revised"}, session)
		if code != 400 {
			t.Fatal("Duplicate activity accepted", code)
		}
		post(base+"projects/"+proj+"/edit", map[string]any{"name": "Z Rollout updated", "start_date": today, "end_date": future, "pm_name": "Jordan", "closed_date": future, "version": 1}, session)
		code, _, _ = call("POST", base+"projects/"+proj+"/edit", map[string]any{"name": "Stale", "version": 1}, session)
		if code != 409 {
			t.Fatal("Stale project update accepted", code)
		}
		projectFound := false
		for _, row := range get(base+"projects", session) {
			if row["id"] == proj && row["pm_name"] == "Jordan" && row["closed_date"] == future {
				projectFound = true
			}
		}
		if !projectFound {
			t.Fatal("Project metadata update not persisted")
		}
		foundPrime := false
		for _, row := range get(base+"projects", session) {
			if row["id"] == proj && row["prime_resource_id"] == person {
				foundPrime = true
			}
		}
		if !foundPrime {
			t.Fatal("Project prime resource was not persisted")
		}
		post(base+"projects/"+proj+"/edit", map[string]any{"name": "Z Rollout updated", "start_date": today, "end_date": future, "pm_name": "Jordan", "prime_resource_id": priya, "version": 2}, session)
		foundPrime = false
		for _, row := range get(base+"projects", session) {
			if row["id"] == proj && row["prime_resource_id"] == priya {
				foundPrime = true
			}
		}
		if !foundPrime {
			t.Fatal("Prime resource reassignment did not persist")
		}
		for _, row := range get(base+"projects", session) {
			if row["id"] == proj && row["closed_date"] != future {
				t.Fatal("Omitting closed date changed its saved value")
			}
		}
		code, _, _ = call("POST", base+"projects/"+proj+"/edit", map[string]any{"name": "Z Rollout updated", "start_date": today, "end_date": future, "closed_date": "2020-01-01", "version": 3}, session)
		if code != 400 {
			t.Fatal("Closed date before start accepted", code)
		}
		post(base+"projects/"+proj+"/edit", map[string]any{"name": "Z Rollout updated", "start_date": today, "end_date": future, "closed_date": "", "pm_name": "Jordan", "version": 3}, session)
		for _, row := range get(base+"projects", session) {
			if row["id"] == proj && row["closed_date"] != nil {
				t.Fatal("Could not clear closed date")
			}
		}
		code, _, _ = call("POST", base+"projects", map[string]any{"name": "Missing prime"}, session)
		if code != 400 {
			t.Fatal("Missing prime accepted", code)
		}
		var outsider string
		owner.QueryRow(ctx, "INSERT INTO resources(org_id,name) VALUES($1,'Other tenant prime') RETURNING id", other).Scan(&outsider)
		code, _, _ = call("POST", base+"projects", map[string]any{"name": "Foreign prime", "prime_resource_id": outsider}, session)
		if code != 400 {
			t.Fatal("Foreign tenant prime accepted", code)
		}
		code, _, _ = call("POST", base+"project-activities/"+custom+"/delete", map[string]any{"version": 99}, session)
		if code != 409 {
			t.Fatal("Stale delete accepted", code)
		}
		post(base+"project-activities/"+custom+"/delete", map[string]any{"version": 1}, session)
		for _, row := range get(base+"project-activities", session) {
			if row["id"] == custom {
				t.Fatal("Deleted activity still listed")
			}
		}
		deletedAudit := false
		for _, row := range get(base+"history", session) {
			if row["action"] == "project-activities/"+custom+"/delete" {
				detail := row["detail"].(map[string]any)
				deleted := detail["deleted_activity"].(map[string]any)
				if deleted["name"] == "Penetration testing" {
					deletedAudit = true
				}
			}
		}
		if !deletedAudit {
			t.Fatal("Deletion audit snapshot missing")
		}
		code, _, _ = call("POST", base+"project-activities/"+custom+"/delete", map[string]any{"version": 1}, session)
		if code != 404 {
			t.Fatal("Repeated delete did not return missing", code)
		}
		post(base+"project-activities", map[string]any{"project_id": proj, "name": "Penetration testing"}, session)
		var foreignProject string
		owner.QueryRow(ctx, "INSERT INTO projects(org_id,name) VALUES($1,'Hidden project') RETURNING id", other).Scan(&foreignProject)
		code, _, _ = call("POST", base+"project-activities", map[string]any{"project_id": foreignProject, "name": "Not allowed"}, session)
		if code != 404 {
			t.Fatal("Cross-tenant project access allowed", code)
		}
		var foreignActivity string
		owner.QueryRow(ctx, "INSERT INTO project_activities(org_id,project_id,name) VALUES($1,$2,'Foreign activity') RETURNING id", other, foreignProject).Scan(&foreignActivity)
		code, _, _ = call("POST", base+"project-activities/"+foreignActivity+"/delete", map[string]any{"version": 1}, session)
		if code != 404 {
			t.Fatal("Cross-tenant deletion allowed", code)
		}

	})
	post(base+"worklogs", map[string]any{"task_id": task, "resource_id": alex, "day": today, "hours": 2}, session)
	t.Run("historical rates and forecasts", func(t *testing.T) {
		d := get(base+"dashboard", session)[0]
		if d["actual_cents"] != float64(20000) || d["forecast_cents"] != float64(130000) {
			t.Fatal(d)
		}
		post(base+"rates", map[string]any{"resource_id": alex, "effective_from": today, "hourly_cents": 12000}, session)
		d = get(base+"dashboard", session)[0]
		if d["actual_cents"] != float64(20000) || d["forecast_cents"] != float64(150000) {
			t.Fatal("Rate snapshot or effective date failed", d)
		}
	})
	t.Run("hold resume and reassignment preserve history", func(t *testing.T) {
		post(base+"tasks/"+task+"/status", map[string]any{"version": 1, "status": "hold", "reason": "Waiting for dependency"}, session)
		if get(base+"dashboard", session)[0]["forecast_cents"] != float64(0) {
			t.Fatal("Hold did not release capacity")
		}
		for _, r := range get(base+"allocations", session) {
			if r["released_at"] == nil {
				t.Fatal("Missing released record")
			}
		}
		if len(get(base+"worklogs", session)) != 1 {
			t.Fatal("Actual work lost")
		}
		code, _, _ := call("POST", base+"allocations", map[string]any{"task_id": task, "resource_id": alex, "day": future, "hours": 1}, session)
		if code != 400 {
			t.Fatal("Held task accepted scheduling")
		}
		code, _, _ = call("POST", base+"tasks/"+task+"/status", map[string]any{"version": 1, "status": "active", "reason": "stale"}, session)
		if code != 409 {
			t.Fatal("Stale version accepted")
		}
		post(base+"tasks/"+task+"/status", map[string]any{"version": 2, "status": "active", "reason": "Dependency resolved"}, session)
		post(base+"allocations", map[string]any{"task_id": task, "resource_id": alex, "day": future, "hours": 2}, session)
		post(base+"tasks/"+task+"/owner", map[string]any{"version": 3, "primary_owner_id": priya, "reason": "Handoff"}, session)
		if len(get(base+"allocations", session)) != 4 {
			t.Fatal("History was deleted")
		}
		if get(base+"tasks", session)[0]["primary_owner_id"] != priya {
			t.Fatal("Owner not updated")
		}
		if len(get(base+"history", session)) < 4 {
			t.Fatal("Audit trail missing")
		}
	})
	t.Run("invite-only and viewer permissions", func(t *testing.T) {
		code, unknown, _ := call("POST", "/auth/request", map[string]any{"email": "unknown@example.com"}, nil)
		if code != 202 || mail["unknown@example.com"] != "" {
			t.Fatal("Uninvited user received link")
		}
		_, known, _ := call("POST", "/auth/request", map[string]any{"email": "admin@resourcemgmt.com"}, nil)
		if string(known) != string(unknown) {
			t.Fatal("Enumeration response differs")
		}
		post(base+"invitations", map[string]any{"email": "viewer@example.com", "role": "viewer"}, session)
		call("POST", "/auth/request", map[string]any{"email": "viewer@example.com"}, nil)
		code, _, viewer := call("POST", "/auth/verify", map[string]any{"token": mail["viewer@example.com"]}, nil)
		if code != 200 {
			t.Fatal("Invite acceptance")
		}
		if len(get(base+"tasks", viewer)) != 1 {
			t.Fatal("Viewer read")
		}
		for _, row := range get(base+"resources", viewer) {
			if _, ok := row["current_hourly_cents"]; ok {
				t.Fatal("Hourly rates leaked to viewer")
			}
		}
		for _, p := range []string{"rates", "dashboard", "history"} {
			code, _, _ = call("GET", base+p, nil, viewer)
			if code != 403 {
				t.Fatal("Financial or audit leak", p)
			}
		}
		code, _, _ = call("POST", base+"projects", map[string]any{"name": "Forbidden"}, viewer)
		if code != 403 {
			t.Fatal("Viewer wrote data")
		}
		code, _, _ = call("POST", base+"project-activities/"+task+"/delete", map[string]any{"version": 1}, viewer)
		if code != 403 {
			t.Fatal("Viewer could delete activities", code)
		}
		code, _, _ = call("POST", base+"projects/"+project+"/delete", map[string]any{"version": 1}, viewer)
		if code != 403 {
			t.Fatal("Viewer project deletion allowed", code)
		}
		code, _, _ = call("POST", base+"resources/"+alex+"/archive", map[string]any{"version": 1}, viewer)
		if code != 403 {
			t.Fatal("Viewer archive allowed", code)
		}
		code, _, _ = call("POST", base+"resources/"+alex+"/delete", map[string]any{"version": 1}, viewer)
		if code != 403 {
			t.Fatal("Viewer resource delete allowed", code)
		}
		call("POST", "/auth/logout", nil, viewer)
		code, _, _ = call("GET", "/api/me", nil, viewer)
		if code != 401 {
			t.Fatal("Session remained valid")
		}
	})
	t.Run("cross tenant foreign keys", func(t *testing.T) {
		var foreign string
		owner.QueryRow(ctx, "INSERT INTO resources(org_id,name) VALUES($1,'Other person') RETURNING id", other).Scan(&foreign)
		code, _, _ := call("POST", base+"allocations", map[string]any{"task_id": task, "resource_id": foreign, "day": future, "hours": 1}, session)
		if code != 400 {
			t.Fatal("Cross tenant reference allowed")
		}
	})
	t.Run("origin and expiry", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/auth/request", strings.NewReader(`{"email":"admin@resourcemgmt.com"}`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("Missing origin accepted")
		}
		tok := token()
		owner.Exec(ctx, "INSERT INTO login_tokens VALUES($1,'admin@resourcemgmt.com',strftime('%Y-%m-%dT%H:%M:%fZ','now','-1 second'),NULL)", hash(tok))
		code, _, _ := call("POST", "/auth/verify", map[string]string{"token": tok}, nil)
		if code != 401 {
			t.Fatal("Expired link accepted")
		}
	})
	t.Run("immutable history and unpriced forecasts", func(t *testing.T) {
		for _, table := range []string{"rates", "worklogs", "audit_events"} {
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			func() { tx.org = org }()
			_, err = tx.Exec(ctx, "DELETE FROM "+table)
			tx.Rollback(ctx)
			if err == nil {
				t.Fatal("Historical rows could be deleted", table)
			}
		}
		noRate := post(base+"resources", map[string]any{"name": "Unpriced", "daily_hours": 8}, session)
		post(base+"allocations", map[string]any{"task_id": task, "resource_id": noRate, "day": future, "hours": 1}, session)
		if get(base+"dashboard", session)[0]["unpriced_allocations"] != float64(1) {
			t.Fatal("Missing-rate coverage not reported")
		}
		code, _, _ := call("POST", base+"worklogs", map[string]any{"task_id": task, "resource_id": noRate, "day": today, "hours": 1}, session)
		if code != 400 {
			t.Fatal("Unpriced actual work accepted")
		}
		rows := get(base+"history", session)
		found := false
		for _, r := range rows {
			if r["action"] == "tasks/"+task+"/owner" {
				detail := r["detail"].(map[string]any)
				if detail["previous_owner_id"] == alex {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("Previous owner missing from history")
		}
	})
	t.Run("optional activities at project creation", func(t *testing.T) {
		id := post(base+"projects", map[string]any{"name": "Z Project with initial activities", "prime_resource_id": alex, "start_date": today, "end_date": future, "pm_name": "PM", "activities": []map[string]any{{"name": "  Custom kickoff  ", "start_date": today, "end_date": future}, {"name": "Rare review"}, {"name": "", "start_date": "", "end_date": ""}}}, session)
		count := 0
		for _, row := range get(base+"project-activities", session) {
			if row["project_id"] == id {
				count++
				if row["name"] == "Custom kickoff" && row["start_date"] != today {
					t.Fatal("Initial activity date missing")
				}
			}
		}
		if count != 2 {
			t.Fatal("Initial activities not saved exactly", count)
		}
		blank := post(base+"projects", map[string]any{"name": "Z Blank activity row", "prime_resource_id": alex, "activities": []map[string]any{{"name": "", "start_date": "", "end_date": ""}}}, session)
		for _, row := range get(base+"project-activities", session) {
			if row["project_id"] == blank {
				t.Fatal("Blank row created a default activity")
			}
		}
		for _, bad := range [][]map[string]any{
			{{"name": "Valid first"}, {"name": "Outside", "start_date": "2020-01-01", "end_date": "2020-01-02"}},
			{{"name": "Duplicate"}, {"name": " duplicate "}},
			{{"name": "", "start_date": today, "end_date": future}},
			{{"name": "Partial date", "start_date": today}},
		} {
			projectsBefore, activitiesBefore := len(get(base+"projects", session)), len(get(base+"project-activities", session))
			code, _, _ := call("POST", base+"projects", map[string]any{"name": "Z Invalid initial rows", "prime_resource_id": alex, "start_date": today, "end_date": future, "activities": bad}, session)
			if code != 400 {
				t.Fatal("Invalid initial activity accepted", code)
			}
			if len(get(base+"projects", session)) != projectsBefore || len(get(base+"project-activities", session)) != activitiesBefore {
				t.Fatal("Failed creation left partial records")
			}
		}
	})
	t.Run("sprint date guardrails", func(t *testing.T) {
		sp := post(base+"sprints", map[string]any{"project_id": project, "name": "Guardrail sprint", "start_date": today, "end_date": future}, session)
		body := map[string]any{"project_id": project, "title": "Bounded task", "estimate_hours": 1, "start_date": today, "target_date": "2099-01-01", "primary_owner_id": alex, "sprint_id": sp}
		code, _, _ := call("POST", base+"tasks", body, session)
		if code != 400 {
			t.Fatal("Task beyond sprint allowed", code)
		}
		body["start_date"] = "2000-01-01"
		body["target_date"] = future
		code, _, _ = call("POST", base+"tasks", body, session)
		if code != 400 {
			t.Fatal("Task before sprint allowed", code)
		}
		body["start_date"] = today
		post(base+"tasks", body, session)
		edit := map[string]any{"project_id": project, "name": "Guardrail sprint", "start_date": future, "end_date": future, "previous_project": project, "previous_name": "Guardrail sprint", "previous_start": today, "previous_end": future}
		code, _, _ = call("POST", base+"sprints/"+sp+"/edit", edit, session)
		if code != 409 {
			t.Fatal("Sprint shrink excludes task", code)
		}
	})
	t.Run("project owned sprint isolation", func(t *testing.T) {
		otherProject := post(base+"projects", map[string]any{"name": "Z Sprint isolation", "prime_resource_id": alex}, session)
		sp := post(base+"sprints", map[string]any{"project_id": otherProject, "name": "Project sprint", "start_date": today, "end_date": future}, session)
		code, _, _ := call("POST", base+"sprints", map[string]any{"name": "Missing project", "start_date": today, "end_date": future}, session)
		if code != 400 {
			t.Fatal("Project-less sprint allowed", code)
		}
		body := map[string]any{"project_id": project, "title": "Wrong project sprint", "estimate_hours": 1, "start_date": today, "target_date": future, "primary_owner_id": alex, "sprint_id": sp}
		code, _, _ = call("POST", base+"tasks", body, session)
		if code != 400 {
			t.Fatal("Cross project sprint allowed", code)
		}
		body["project_id"] = otherProject
		post(base+"tasks", body, session)
		edit := map[string]any{"project_id": project, "name": "Project sprint", "start_date": today, "end_date": future, "previous_name": "Project sprint", "previous_start": today, "previous_end": future, "previous_project": otherProject}
		code, _, _ = call("POST", base+"sprints/"+sp+"/edit", edit, session)
		if code != 400 {
			t.Fatal("Sprint moved away from task project", code)
		}
	})
	t.Run("sprints newest first", func(t *testing.T) {
		first := post(base+"sprints", map[string]any{"project_id": project, "name": "Z Sprint earlier added", "start_date": future, "end_date": future}, session)
		latest := post(base+"sprints", map[string]any{"project_id": project, "name": "Z Sprint latest added", "start_date": today, "end_date": future}, session)
		list := get(base+"sprints", session)
		if len(list) < 2 || list[0]["id"] != latest || list[1]["id"] != first {
			t.Fatal("Sprint creation order not respected", list)
		}
	})
	t.Run("tasks without Jira keys", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			post(base+"tasks", map[string]any{"project_id": project, "title": "No Jira task", "estimate_hours": 2, "start_date": today, "target_date": future, "primary_owner_id": alex}, session)
		}
	})
	t.Run("explicit linked project deletion", func(t *testing.T) {
		p := post(base+"projects", map[string]any{"name": "Z Override", "prime_resource_id": alex, "start_date": today, "end_date": future}, session)
		taskID := post(base+"tasks", map[string]any{"project_id": p, "jira_key": "OVERRIDE-1", "title": "Override fixture", "estimate_hours": 8, "start_date": today, "target_date": future, "primary_owner_id": alex}, session)
		allocation := post(base+"allocations", map[string]any{"task_id": taskID, "resource_id": alex, "day": future, "hours": 2}, session)
		post(base+"worklogs", map[string]any{"task_id": taskID, "resource_id": alex, "day": today, "hours": 1}, session)
		code, _, _ := call("POST", base+"projects/"+p+"/delete", map[string]any{"version": 1, "include_linked_tasks": true, "confirm_name": "wrong"}, session)
		if code != 400 {
			t.Fatal("Wrong confirmation accepted", code)
		}
		post(base+"projects/"+p+"/delete", map[string]any{"version": 1, "include_linked_tasks": true, "confirm_name": "Z Override"}, session)
		for _, row := range get(base+"tasks", session) {
			if row["id"] == taskID {
				t.Fatal("Deleted project task visible")
			}
		}
		found := false
		for _, row := range get(base+"allocations", session) {
			if row["id"] == allocation {
				found = true
				if row["released_at"] == nil || row["jira_key"] != "OVERRIDE-1" {
					t.Fatal("Allocation history not released or labeled", row)
				}
			}
		}
		if !found {
			t.Fatal("Allocation history lost")
		}
		found = false
		for _, row := range get(base+"worklogs", session) {
			if row["task_id"] == taskID {
				found = true
			}
		}
		if !found {
			t.Fatal("Recorded work lost")
		}
		code, _, _ = call("POST", base+"allocations", map[string]any{"task_id": taskID, "resource_id": alex, "day": future, "hours": 1}, session)
		if code != 404 {
			t.Fatal("Deleted task accepts allocations", code)
		}
		code, _, _ = call("POST", base+"worklogs", map[string]any{"task_id": taskID, "resource_id": alex, "day": today, "hours": 1}, session)
		if code != 404 {
			t.Fatal("Deleted task accepts work", code)
		}
	})
	t.Run("delete projects without disrupting linked work", func(t *testing.T) {
		doomed := post(base+"projects", map[string]any{"name": "Z Removable project", "prime_resource_id": alex, "start_date": today, "end_date": future, "activities": []map[string]any{{"name": "Retained activity"}}}, session)
		code, _, _ := call("POST", base+"projects/"+doomed+"/delete", map[string]any{"version": 99}, session)
		if code != 409 {
			t.Fatal("Stale project deletion allowed", code)
		}
		post(base+"projects/"+doomed+"/delete", map[string]any{"version": 1}, session)
		for _, row := range get(base+"projects", session) {
			if row["id"] == doomed {
				t.Fatal("Deleted project still active")
			}
		}
		for _, row := range get(base+"project-activities", session) {
			if row["project_id"] == doomed {
				t.Fatal("Deleted project activities still active")
			}
		}
		var retained int
		owner.QueryRow(ctx, "SELECT count(*) FROM project_activities WHERE project_id=$1", doomed).Scan(&retained)
		if retained != 1 {
			t.Fatal("Activity data was removed")
		}
		code, _, _ = call("POST", base+"projects/"+doomed+"/delete", map[string]any{"version": 1}, session)
		if code != 404 {
			t.Fatal("Repeated delete not missing", code)
		}
		code, _, _ = call("POST", base+"tasks", map[string]any{"project_id": doomed, "jira_key": "DEL-TEST", "title": "Cannot add", "estimate_hours": 1, "start_date": today, "target_date": future, "primary_owner_id": alex}, session)
		if code != 404 {
			t.Fatal("Task added to deleted project", code)
		}
		code, _, _ = call("POST", base+"project-activities", map[string]any{"project_id": doomed, "name": "Cannot add"}, session)
		if code != 404 {
			t.Fatal("Activity added to deleted project", code)
		}
		_, before, _ := call("GET", base+"allocations", nil, session)
		logsBefore := len(get(base+"worklogs", session))
		code, _, _ = call("POST", base+"projects/"+project+"/delete", map[string]any{"version": 1}, session)
		if code != 409 {
			t.Fatal("Linked project deletion not blocked", code)
		}
		_, after, _ := call("GET", base+"allocations", nil, session)
		if string(before) != string(after) || len(get(base+"worklogs", session)) != logsBefore {
			t.Fatal("Protected planning/work history changed")
		}
		var foreignProject string
		owner.QueryRow(ctx, "INSERT INTO projects(org_id,name) VALUES($1,'Foreign delete test') RETURNING id", other).Scan(&foreignProject)
		code, _, _ = call("POST", base+"projects/"+foreignProject+"/delete", map[string]any{"version": 1}, session)
		if code != 404 {
			t.Fatal("Cross-tenant project deletion allowed", code)
		}
		found := false
		for _, row := range get(base+"history", session) {
			if row["action"] == "projects/"+doomed+"/delete" {
				found = true
			}
		}
		if !found {
			t.Fatal("Project deletion audit missing")
		}
	})
	t.Run("archive and restore resources preserves existing work", func(t *testing.T) {
		var person map[string]any
		for _, row := range get(base+"resources", session) {
			if row["id"] == alex {
				person = row
			}
		}
		version := int(person["version"].(float64))
		_, allocBefore, _ := call("GET", base+"allocations", nil, session)
		_, workBefore, _ := call("GET", base+"worklogs", nil, session)
		_, ratesBefore, _ := call("GET", base+"rates", nil, session)
		code, _, _ := call("POST", base+"resources/"+alex+"/archive", map[string]any{"version": version - 1}, session)
		if code != 409 {
			t.Fatal("Stale resource archive allowed", code)
		}
		post(base+"resources/"+alex+"/archive", map[string]any{"version": version}, session)
		for _, row := range get(base+"resources", session) {
			if row["id"] == alex {
				t.Fatal("Archived resource in active list")
			}
		}
		archived := false
		for _, row := range get(base+"archived-resources", session) {
			if row["id"] == alex && row["archived_at"] != nil {
				archived = true
			}
		}
		if !archived {
			t.Fatal("Archived resource not discoverable")
		}
		_, allocAfter, _ := call("GET", base+"allocations", nil, session)
		_, workAfter, _ := call("GET", base+"worklogs", nil, session)
		_, ratesAfter, _ := call("GET", base+"rates", nil, session)
		if string(allocBefore) != string(allocAfter) || string(workBefore) != string(workAfter) || string(ratesBefore) != string(ratesAfter) {
			t.Fatal("Archive changed existing work or rates")
		}
		code, _, _ = call("POST", base+"allocations", map[string]any{"task_id": task, "resource_id": alex, "day": future, "hours": 1}, session)
		if code != 400 {
			t.Fatal("New allocation for archived resource allowed", code)
		}
		code, _, _ = call("POST", base+"projects", map[string]any{"name": "Archived prime", "prime_resource_id": alex}, session)
		if code != 400 {
			t.Fatal("New project assigned archived prime", code)
		}
		code, _, _ = call("POST", base+"tasks", map[string]any{"project_id": project, "jira_key": "ARCH-1", "title": "Archived owner", "primary_owner_id": alex, "estimate_hours": 1, "start_date": today, "target_date": future}, session)
		if code != 400 {
			t.Fatal("New task assigned archived owner", code)
		}
		post(base+"projects/"+project+"/edit", map[string]any{"name": "Payments", "prime_resource_id": alex, "version": 1}, session)
		var foreign string
		owner.QueryRow(ctx, "INSERT INTO resources(org_id,name) VALUES($1,'Foreign archive') RETURNING id", other).Scan(&foreign)
		code, _, _ = call("POST", base+"resources/"+foreign+"/archive", map[string]any{"version": 1}, session)
		if code != 404 {
			t.Fatal("Cross tenant archive allowed", code)
		}
		post(base+"resources/"+alex+"/restore", map[string]any{"version": version + 1}, session)
		restored := false
		for _, row := range get(base+"resources", session) {
			if row["id"] == alex && row["archived_at"] == nil {
				restored = true
			}
		}
		if !restored {
			t.Fatal("Restore failed")
		}
		for _, row := range get(base+"archived-resources", session) {
			if row["id"] == alex {
				t.Fatal("Restored resource still archived")
			}
		}
	})
	t.Run("delete unused resources and protect linked work", func(t *testing.T) {
		unused := post(base+"resources", map[string]any{"name": "Unused entry", "hourly_cents": 12300, "effective_from": today}, session)
		code, _, _ := call("POST", base+"resources/"+unused+"/delete", map[string]any{"version": 99}, session)
		if code != 409 {
			t.Fatal("Stale resource delete accepted", code)
		}
		post(base+"resources/"+unused+"/delete", map[string]any{"version": 1}, session)
		for _, list := range []string{"resources", "archived-resources"} {
			for _, row := range get(base+list, session) {
				if row["id"] == unused {
					t.Fatal("Deleted resource visible", list)
				}
			}
		}
		var retained int
		owner.QueryRow(ctx, "SELECT count(*) FROM rates WHERE resource_id=$1", unused).Scan(&retained)
		if retained != 1 {
			t.Fatal("Unused profile's initial rate was erased")
		}
		code, _, _ = call("POST", base+"resources/"+unused+"/restore", map[string]any{"version": 2}, session)
		if code != 404 {
			t.Fatal("Deleted resource restored through archive endpoint", code)
		}
		code, _, _ = call("POST", base+"projects", map[string]any{"name": "Deleted prime", "prime_resource_id": unused}, session)
		if code != 400 {
			t.Fatal("Deleted resource assigned to project", code)
		}
		archivedUnused := post(base+"resources", map[string]any{"name": "Archived unused entry"}, session)
		post(base+"resources/"+archivedUnused+"/archive", map[string]any{"version": 1}, session)
		post(base+"resources/"+archivedUnused+"/delete", map[string]any{"version": 2}, session)
		var person map[string]any
		for _, row := range get(base+"resources", session) {
			if row["id"] == alex {
				person = row
			}
		}
		_, allocBefore, _ := call("GET", base+"allocations", nil, session)
		_, workBefore, _ := call("GET", base+"worklogs", nil, session)
		code, _, _ = call("POST", base+"resources/"+alex+"/delete", map[string]any{"version": person["version"]}, session)
		if code != 409 {
			t.Fatal("Used resource deleted", code)
		}
		_, allocAfter, _ := call("GET", base+"allocations", nil, session)
		_, workAfter, _ := call("GET", base+"worklogs", nil, session)
		if string(allocBefore) != string(allocAfter) || string(workBefore) != string(workAfter) {
			t.Fatal("Protected resource work changed")
		}
		var foreign string
		owner.QueryRow(ctx, "INSERT INTO resources(org_id,name) VALUES($1,'Foreign delete') RETURNING id", other).Scan(&foreign)
		code, _, _ = call("POST", base+"resources/"+foreign+"/delete", map[string]any{"version": 1}, session)
		if code != 404 {
			t.Fatal("Cross-tenant delete allowed", code)
		}
		found := false
		for _, row := range get(base+"history", session) {
			if row["action"] == "resources/"+unused+"/delete" {
				found = true
			}
		}
		if !found {
			t.Fatal("Resource delete audit missing")
		}
	})
	t.Run("resources newest first", func(t *testing.T) {
		older := post(base+"resources", map[string]any{"name": "ZZ older resource"}, session)
		newer := post(base+"resources", map[string]any{"name": "AA newer resource"}, session)
		rows := get(base+"resources", session)
		if rows[0]["id"] != newer || rows[1]["id"] != older {
			t.Fatal("Resources not newest first", rows[:2])
		}
		post(base+"resources/"+older+"/capacity", map[string]any{"daily_hours": 7}, session)
		rows = get(base+"resources", session)
		if rows[0]["id"] != newer {
			t.Fatal("Editing changed creation order")
		}
	})
	t.Run("rate limiting", func(t *testing.T) {
		var code int
		for i := 0; i < 11; i++ {
			code, _, _ = call("POST", "/auth/request", map[string]string{"email": "unknown@example.com"}, nil)
		}
		if code != 429 {
			t.Fatal("Missing throttle", code)
		}
	})
	fmt.Println("Validated SQLite persistence, scoped organization queries, invite-only auth, roles, allocation history, rates and sprint capacity")
}
