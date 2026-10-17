package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Set TEST_DATABASE_URL to a disposable PostgreSQL admin connection")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	name := "rc_test_" + token()[:10]
	role := name + "_app"
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+name); e != nil {
		t.Fatal(e)
	}
	if _, e = admin.Exec(ctx, "CREATE ROLE "+role+" LOGIN PASSWORD 'test-only'"); e != nil {
		t.Fatal(e)
	}
	defer func() { admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); admin.Exec(ctx, "DROP ROLE "+role) }()
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	owner, e := pgxpool.New(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer owner.Close()
	if _, e = owner.Exec(ctx, schema); e != nil {
		t.Fatal(e)
	}
	if _, e = owner.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+role+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+role); e != nil {
		t.Fatal(e)
	}
	u.User = url.UserPassword(role, "test-only")
	db, e := pgxpool.New(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
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
	owner.QueryRow(ctx, "INSERT INTO users(email) VALUES('admin@example.com') RETURNING id").Scan(&user)
	if _, e = owner.Exec(ctx, "INSERT INTO memberships VALUES($1,$2,'admin')", org, user); e != nil {
		t.Fatal(e)
	}
	code, raw, _ := call("POST", "/auth/request", map[string]string{"email": "admin@example.com"}, nil)
	if code != 202 {
		t.Fatal(code, string(raw))
	}
	link := mail["admin@example.com"]
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
	project := post(base+"projects", map[string]any{"name": "Payments", "budget_cents": 1000000}, session)
	today := time.Now().UTC().Format("2006-01-02")
	future := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	sprint := post(base+"sprints", map[string]any{"name": "Sprint 1", "start_date": today, "end_date": time.Now().UTC().AddDate(0, 0, 13).Format("2006-01-02")}, session)
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
		_, known, _ := call("POST", "/auth/request", map[string]any{"email": "admin@example.com"}, nil)
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
		r := httptest.NewRequest("POST", "/auth/request", strings.NewReader(`{"email":"admin@example.com"}`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("Missing origin accepted")
		}
		tok := token()
		owner.Exec(ctx, "INSERT INTO login_tokens VALUES($1,'admin@example.com',now()-interval '1 second',NULL)", hash(tok))
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
			tx.Exec(ctx, "SELECT set_config('app.tenant',$1,true)", org)
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
	t.Run("rate limiting", func(t *testing.T) {
		var code int
		for i := 0; i < 11; i++ {
			code, _, _ = call("POST", "/auth/request", map[string]string{"email": "unknown@example.com"}, nil)
		}
		if code != 429 {
			t.Fatal("Missing throttle", code)
		}
	})
	fmt.Println("Validated PostgreSQL persistence, RLS, invite-only auth, roles, allocation history, rates and sprint capacity")
}
