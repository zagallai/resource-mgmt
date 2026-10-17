package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

type App struct {
	db     *pgxpool.Pool
	origin string
	secure bool
	send   func(string, string) error
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func respond(w http.ResponseWriter, n int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(n)
	json.NewEncoder(w).Encode(v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return false
	}
	var extra any
	return d.Decode(&extra) == io.EOF
}

func main() {
	ctx := context.Background()
	cfg, e := pgxpool.ParseConfig(env("DATABASE_URL", "postgres://compass:compass@localhost:5432/compass?sslmode=disable"))
	if e != nil {
		log.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		log.Fatal(e)
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			var exists bool
			db.QueryRow(ctx, "SELECT to_regclass('public.schema_version') IS NOT NULL").Scan(&exists)
			if !exists {
				_, e = db.Exec(ctx, schema)
			}
		case "bootstrap":
			email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
			if !strings.Contains(email, "@") {
				log.Fatal("ADMIN_EMAIL required")
			}
			tx, err := db.Begin(ctx)
			if err != nil {
				log.Fatal(err)
			}
			defer tx.Rollback(ctx)
			var org, user string
			e = tx.QueryRow(ctx, "INSERT INTO organizations(name) VALUES($1) RETURNING id", env("ORG_NAME", "Engineering")).Scan(&org)
			if e == nil {
				e = tx.QueryRow(ctx, "INSERT INTO users(email) VALUES($1) ON CONFLICT(email) DO UPDATE SET email=excluded.email RETURNING id", email).Scan(&user)
			}
			if e == nil {
				_, e = tx.Exec(ctx, "INSERT INTO memberships VALUES($1,$2,'admin')", org, user)
			}
			if e == nil {
				e = tx.Commit(ctx)
				fmt.Println("Organization:", org)
			}
		default:
			log.Fatal("Use migrate or bootstrap")
		}
		if e != nil {
			log.Fatal(e)
		}
		return
	}
	var bypass bool
	if e = db.QueryRow(ctx, "SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname=current_user").Scan(&bypass); e != nil {
		log.Fatal(e)
	}
	if bypass {
		log.Fatal("Refusing to serve HTTP with a database role that bypasses row-level security; use the restricted application role")
	}
	a := &App{db: db, origin: env("APP_ORIGIN", "http://localhost:3000"), secure: os.Getenv("COOKIE_SECURE") != "false"}
	a.send = func(email, link string) error {
		return smtp.SendMail(env("SMTP_ADDR", "localhost:1025"), nil, env("MAIL_FROM", "compass@localhost"), []string{email}, []byte("From: "+env("MAIL_FROM", "compass@localhost")+"\r\nTo: "+email+"\r\nSubject: Resource Compass sign-in\r\n\r\nOpen this single-use link within 10 minutes:\r\n"+link))
	}
	s := &http.Server{Addr: env("LISTEN_ADDR", ":8080"), Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Resource Compass API %s", s.Addr)
	log.Fatal(s.ListenAndServe())
}
func (a *App) handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if a.db.Ping(r.Context()) != nil {
			respond(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	m.HandleFunc("POST /auth/request", a.requestLink)
	m.HandleFunc("POST /auth/verify", a.verify)
	m.HandleFunc("POST /auth/logout", a.logout)
	m.HandleFunc("GET /api/me", a.me)
	m.HandleFunc("/api/orgs/{org}/{rest...}", a.scoped)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Header.Get("Origin") != a.origin {
			respond(w, 403, map[string]string{"error": "Origin required"})
			return
		}
		m.ServeHTTP(w, r)
	})
}
func (a *App) requestLink(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &b) {
		respond(w, 400, map[string]string{"error": "Invalid request"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(b.Email))
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	var hits int
	e := a.db.QueryRow(r.Context(), `INSERT INTO auth_limits(key,hits,resets_at) VALUES($1,1,now()+interval '15 minutes') ON CONFLICT(key) DO UPDATE SET hits=CASE WHEN auth_limits.resets_at<now() THEN 1 ELSE auth_limits.hits+1 END,resets_at=CASE WHEN auth_limits.resets_at<now() THEN now()+interval '15 minutes' ELSE auth_limits.resets_at END RETURNING hits`, hash(host)).Scan(&hits)
	if e != nil {
		respond(w, 503, map[string]string{"error": "Try later"})
		return
	}
	if hits > 10 {
		respond(w, 429, map[string]string{"error": "Try again in 15 minutes"})
		return
	}
	var allowed bool
	e = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users u JOIN memberships m ON m.user_id=u.id WHERE email=$1) OR EXISTS(SELECT 1 FROM invitations WHERE email=$1 AND accepted_at IS NULL AND expires_at>now())`, email).Scan(&allowed)
	if e == nil && allowed {
		t := token()
		_, e = a.db.Exec(r.Context(), "INSERT INTO login_tokens(hash,email,expires_at) VALUES($1,$2,now()+interval '10 minutes')", hash(t), email)
		if e == nil {
			e = a.send(email, a.origin+"/verify#token="+t)
		}
		if e != nil {
			log.Print("Magic-link delivery failed")
		}
	}
	respond(w, 202, map[string]string{"message": "If invited, you will receive a sign-in email."})
}
func (a *App) verify(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &b) {
		respond(w, 400, map[string]string{"error": "Invalid request"})
		return
	}
	ctx := r.Context()
	tx, e := a.db.Begin(ctx)
	if e != nil {
		respond(w, 503, nil)
		return
	}
	defer tx.Rollback(ctx)
	var email, user string
	e = tx.QueryRow(ctx, "UPDATE login_tokens SET used_at=now() WHERE hash=$1 AND used_at IS NULL AND expires_at>now() RETURNING email", hash(b.Token)).Scan(&email)
	if e != nil {
		respond(w, 401, map[string]string{"error": "Link invalid or expired"})
		return
	}
	e = tx.QueryRow(ctx, "INSERT INTO users(email) VALUES($1) ON CONFLICT(email) DO UPDATE SET email=excluded.email RETURNING id", email).Scan(&user)
	if e == nil {
		_, e = tx.Exec(ctx, `WITH accepted AS (UPDATE invitations SET accepted_at=now() WHERE email=$1 AND accepted_at IS NULL AND expires_at>now() RETURNING org_id,role) INSERT INTO memberships SELECT org_id,$2::uuid,role FROM accepted ON CONFLICT DO NOTHING`, email, user)
	}
	var count int
	if e == nil {
		e = tx.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE user_id=$1", user).Scan(&count)
	}
	if e != nil || count == 0 {
		respond(w, 401, map[string]string{"error": "Invitation required"})
		return
	}
	s := token()
	_, e = tx.Exec(ctx, "INSERT INTO sessions VALUES($1,$2,now()+interval '24 hours')", hash(s), user)
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		respond(w, 500, nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "rc_session", Value: s, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	respond(w, 200, map[string]bool{"ok": true})
}
func (a *App) identity(r *http.Request) (string, error) {
	c, e := r.Cookie("rc_session")
	if e != nil {
		return "", e
	}
	var id string
	e = a.db.QueryRow(r.Context(), "SELECT user_id FROM sessions WHERE hash=$1 AND expires_at>now()", hash(c.Value)).Scan(&id)
	return id, e
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("rc_session"); e == nil {
		a.db.Exec(r.Context(), "DELETE FROM sessions WHERE hash=$1", hash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "rc_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode})
	respond(w, 200, map[string]bool{"ok": true})
}
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	u, e := a.identity(r)
	if e != nil {
		respond(w, 401, nil)
		return
	}
	var raw []byte
	e = a.db.QueryRow(r.Context(), `SELECT coalesce(json_agg(json_build_object('id',o.id,'name',o.name,'role',m.role)),'[]') FROM memberships m JOIN organizations o ON o.id=m.org_id WHERE user_id=$1`, u).Scan(&raw)
	if e != nil {
		respond(w, 500, nil)
		return
	}
	respond(w, 200, json.RawMessage(raw))
}
func (a *App) scoped(w http.ResponseWriter, r *http.Request) {
	u, e := a.identity(r)
	if e != nil {
		respond(w, 401, nil)
		return
	}
	ctx := r.Context()
	tx, e := a.db.Begin(ctx)
	if e != nil {
		respond(w, 503, nil)
		return
	}
	defer tx.Rollback(ctx)
	org := r.PathValue("org")
	var role string
	e = tx.QueryRow(ctx, "SELECT role FROM memberships WHERE user_id=$1 AND org_id::text=$2", u, org).Scan(&role)
	if e != nil {
		respond(w, 404, map[string]string{"error": "Organization not found"})
		return
	}
	_, e = tx.Exec(ctx, "SELECT set_config('app.tenant',$1,true)", org)
	if e != nil {
		respond(w, 500, nil)
		return
	}
	path := r.PathValue("rest")
	if r.Method == "GET" {
		a.read(w, r, tx, path, role)
		return
	}
	if role == "viewer" {
		respond(w, 403, map[string]string{"error": "Read-only membership"})
		return
	}
	if r.Method != "POST" {
		respond(w, 405, nil)
		return
	}
	id, e := a.write(r, tx, org, u, role, path, w)
	if e != nil {
		var v *validation
		if errors.As(e, &v) {
			respond(w, v.status, map[string]string{"error": v.Error()})
		} else {
			log.Printf("Operation %s failed: %v", path, e)
			respond(w, 400, map[string]string{"error": "Invalid record, reference, or conflicting update"})
		}
		return
	}
	if e = tx.Commit(ctx); e != nil {
		respond(w, 409, map[string]string{"error": "Concurrent change; refresh and retry"})
		return
	}
	respond(w, 201, map[string]string{"id": id})
}

type validation struct {
	status  int
	message string
}

func (v *validation) Error() string { return v.message }
func invalid(s string) error        { return &validation{400, s} }
func audit(ctx context.Context, tx pgx.Tx, org, user, id, action string, detail any) error {
	raw, _ := json.Marshal(detail)
	_, e := tx.Exec(ctx, "INSERT INTO audit_events(org_id,actor_id,entity_id,action,detail) VALUES($1,$2,$3,$4,$5)", org, user, id, action, raw)
	return e
}
