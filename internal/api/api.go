package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/relentlessworks/taskpilot/internal/auth"
	"github.com/relentlessworks/taskpilot/internal/models"
	"github.com/relentlessworks/taskpilot/internal/store"
)

// Server is the API server.
type Server struct {
	store *store.Store
	auth  *auth.AuthService
}

// NewServer creates a new API server.
func NewServer(s *store.Store, a *auth.AuthService) *Server {
	return &Server{store: s, auth: a}
}

// Router is the main HTTP router.
func (s *Server) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// --- Public routes ---

	// Help / agent manual
	if path == "/help" || path == "/.well-known/agent.md" {
		s.handleHelp(w, r)
		return
	}

	// Health check
	if path == "/health" {
		s.handleHealth(w, r)
		return
	}

	// Auth: request OTP
	if path == "/auth/request" && r.Method == "POST" {
		s.handleRequestOTP(w, r)
		return
	}

	// Auth: verify OTP
	if path == "/auth/verify" && r.Method == "POST" {
		s.handleVerifyOTP(w, r)
		return
	}

	// --- Authenticated routes ---
	// All /api/* routes require a bearer token

	if strings.HasPrefix(path, "/api/") {
		s.handleAPI(w, r)
		return
	}

	// Root: if nothing matched, show help
	if path == "/" {
		s.handleHelp(w, r)
		return
	}

	s.errorResponse(w, r, http.StatusNotFound, "not found", "GET /help to see available endpoints")
}

// --- Helpers ---

func (s *Server) wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if q := r.URL.Query().Get("format"); q == "json" {
		return true
	}
	return false
}

func (s *Server) writeResponse(w http.ResponseWriter, r *http.Request, status int, text string, data interface{}) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(data)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, text)
}

func (s *Server) errorResponse(w http.ResponseWriter, r *http.Request, status int, msg, hint string) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": msg, "hint": hint})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, "error: %s\nhint: %s\n", msg, hint)
}

func (s *Server) getBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func (s *Server) authenticate(r *http.Request) (string, error) {
	token := s.getBearerToken(r)
	if token == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	t, err := s.store.GetToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid or expired token")
	}
	if time.Now().After(t.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return t.Workspace, nil
}

// formatTask returns a plain text line for a task.
func formatTask(t *models.Task) string {
	desc := t.Description
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	if desc == "" {
		return fmt.Sprintf("handle=%s title=%s status=%s priority=%s", t.Handle, t.Title, t.Status, t.Priority)
	}
	return fmt.Sprintf("handle=%s title=%s status=%s priority=%s description=%s", t.Handle, t.Title, t.Status, t.Priority, desc)
}

// --- Handlers ---

func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	help := `TaskPilot — Agentic-First Task Manager
======================================

TaskPilot is a task management service designed for AI agents. The API is the product.
No UI, no SDK. Plain text by default, JSON on demand.

AUTHENTICATION
--------------
1. POST /auth/request   body: email=<email>&workspace=<handle>
   → Sends a 6-digit OTP code (returned in plain text for local dev).
2. POST /auth/verify     body: email=<email>&code=<code>
   → Returns a long-lived bearer token. Use it in Authorization: Bearer <token>.

CREATE A TASK
-------------
POST /api/tasks         body: title=<summary>&priority=<low|medium|high>&description=<optional>
   → Returns: handle=task_a1b2c title=... status=todo priority=medium

LIST TASKS
----------
GET /api/tasks                  → All tasks, one per line.
GET /api/tasks?status=todo      → Filter by status (todo, in_progress, done).
GET /api/tasks?status=done      → Only completed tasks.

GET A TASK
----------
GET /api/tasks/<handle>  → handle=task_a1b2c title=... status=todo priority=medium description=...

UPDATE A TASK
-------------
PATCH /api/tasks/<handle>  body: status=<todo|in_progress|done>&priority=<low|medium|high>&title=<new title>&description=<new desc>
   → Any field is optional. Only provided fields are updated.

DELETE A TASK
-------------
DELETE /api/tasks/<handle>

WORKSPACE INFO
--------------
GET /api/workspace        → name=My Workspace plan=free tasks=42

STATUS VALUES
-------------
todo, in_progress, done
(Aliases accepted on create/update: open/new/pending → todo, doing/started/active → in_progress, completed/closed/finished → done)

PRIORITY VALUES
---------------
low, medium, high
(Aliases accepted: p3 → low, p2/normal → medium, p1/urgent/critical → high)

FORMATS
-------
- Plain text (default): one labeled, grepable line per record.
- JSON: add Accept: application/json or ?format=json to any request.

ERRORS
------
4xx responses include an "error" and a "hint" field to guide you.

EXAMPLES
--------
  curl -X POST http://localhost:8080/auth/request -d 'email=me@example.com&workspace=ws_demo'
  curl -X POST http://localhost:8080/auth/verify -d 'email=me@example.com&code=123456'
  curl -X POST http://localhost:8080/api/tasks -H 'Authorization: Bearer tp_xxx' -d 'title=Fix bug&priority=high'
  curl http://localhost:8080/api/tasks -H 'Authorization: Bearer tp_xxx'
  curl http://localhost:8080/api/tasks?status=todo -H 'Authorization: Bearer tp_xxx'
  curl -X PATCH http://localhost:8080/api/tasks/task_a1b2c -H 'Authorization: Bearer tp_xxx' -d 'status=done'
  curl -X DELETE http://localhost:8080/api/tasks/task_a1b2c -H 'Authorization: Bearer tp_xxx'

STORAGE
-------
Data is persisted to a JSON file (default: taskpilot.json). Zero external dependencies.
`

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, help)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeResponse(w, r, http.StatusOK, "ok", map[string]string{"status": "ok"})
}

func (s *Server) handleRequestOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	workspace := r.FormValue("workspace")

	if email == "" || workspace == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or workspace",
			"POST with email=<your-email>&workspace=<handle> (e.g. ws_demo)")
		return
	}

	// Auto-create workspace if it doesn't exist
	exists, err := s.store.WorkspaceExists(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}
	if !exists {
		if err := s.store.CreateWorkspace(workspace, workspace); err != nil {
			s.errorResponse(w, r, http.StatusInternalServerError, "failed to create workspace", "try a different workspace handle")
			return
		}
	}

	code := s.auth.GenerateOTP()
	if err := s.store.SaveOTP(email, code, workspace, auth.OTPExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to save OTP", "try again")
		return
	}

	// In production, email this. In dev, return it directly.
	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("otp_sent=true email=%s code=%s", email, code),
		map[string]string{"status": "otp_sent", "email": email, "code": code, "hint": "use POST /auth/verify with this code to get a token"},
	)
}

func (s *Server) handleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	code := r.FormValue("code")

	if email == "" || code == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or code",
			"POST with email=<your-email>&code=<6-digit-code>")
		return
	}

	workspace, expiresAt, err := s.store.GetOTP(email, code)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, "invalid OTP code",
			"request a new OTP via POST /auth/request")
		return
	}

	if time.Now().After(expiresAt) {
		s.store.DeleteOTP(email, code)
		s.errorResponse(w, r, http.StatusUnauthorized, "OTP expired",
			"request a new OTP via POST /auth/request")
		return
	}

	// Delete used OTP
	s.store.DeleteOTP(email, code)

	// Generate token
	token := s.auth.GenerateToken(workspace)
	if err := s.store.CreateToken(token, workspace, auth.TokenExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create token", "try again")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("token=%s workspace=%s", token, workspace),
		map[string]string{"token": token, "workspace": workspace, "hint": "use this token in Authorization: Bearer header"},
	)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	workspace, err := s.authenticate(r)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, err.Error(),
			"POST /auth/request with email and workspace, then POST /auth/verify with the code")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api")

	switch {
	case path == "/tasks" && r.Method == "POST":
		s.handleCreateTask(w, r, workspace)
	case path == "/tasks" && r.Method == "GET":
		s.handleListTasks(w, r, workspace)
	case strings.HasPrefix(path, "/tasks/") && r.Method == "GET":
		s.handleGetTask(w, r, workspace)
	case strings.HasPrefix(path, "/tasks/") && r.Method == "PATCH":
		s.handleUpdateTask(w, r, workspace)
	case strings.HasPrefix(path, "/tasks/") && r.Method == "DELETE":
		s.handleDeleteTask(w, r, workspace)
	case path == "/workspace" && r.Method == "GET":
		s.handleGetWorkspace(w, r, workspace)
	default:
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"GET /help to see available endpoints")
	}
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request, workspace string) {
	title := r.FormValue("title")
	if title == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing title",
			"POST with title=<task summary> (e.g. title=Fix the login bug)")
		return
	}

	description := r.FormValue("description")
	priority := store.NormalizePriority(r.FormValue("priority"))
	if priority == "" {
		priority = "medium"
	}
	if !store.ValidPriority(priority) {
		s.errorResponse(w, r, http.StatusBadRequest, "invalid priority",
			"use one of: low, medium, high")
		return
	}

	status := store.NormalizeStatus(r.FormValue("status"))
	if status == "" {
		status = "todo"
	}
	if !store.ValidStatus(status) {
		s.errorResponse(w, r, http.StatusBadRequest, "invalid status",
			"use one of: todo, in_progress, done")
		return
	}

	handle := auth.GenerateHandle("task")
	if err := s.store.CreateTask(handle, title, description, status, priority, workspace); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create task", "try again")
		return
	}

	task := &models.Task{
		Handle:      handle,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		Workspace:   workspace,
	}
	s.writeResponse(w, r, http.StatusCreated,
		fmt.Sprintf("handle=%s title=%s status=%s priority=%s", handle, title, status, priority),
		task,
	)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request, workspace string) {
	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" {
		statusFilter = store.NormalizeStatus(statusFilter)
		if !store.ValidStatus(statusFilter) {
			s.errorResponse(w, r, http.StatusBadRequest, "invalid status filter",
				"use one of: todo, in_progress, done")
			return
		}
	}

	tasks, err := s.store.ListTasks(workspace, statusFilter, 50)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}

	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if tasks == nil {
			tasks = []*models.Task{}
		}
		json.NewEncoder(w).Encode(tasks)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if len(tasks) == 0 {
		fmt.Fprintln(w, "no tasks found. POST /api/tasks with title=<summary> to create one.")
		return
	}
	for _, t := range tasks {
		fmt.Fprintln(w, formatTask(t))
	}
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "GET /api/tasks/<handle>")
		return
	}

	task, err := s.store.GetTask(handle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "task not found",
			"GET /api/tasks to list all tasks")
		return
	}

	// Verify ownership
	if task.Workspace != workspace {
		s.errorResponse(w, r, http.StatusNotFound, "task not found",
			"this task belongs to a different workspace")
		return
	}

	s.writeResponse(w, r, http.StatusOK, formatTask(task), task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "PATCH /api/tasks/<handle>")
		return
	}

	updates := make(map[string]string)

	if v := r.FormValue("title"); v != "" {
		updates["title"] = v
	}
	if v := r.FormValue("description"); v != "" {
		updates["description"] = v
	}
	if v := r.FormValue("status"); v != "" {
		status := store.NormalizeStatus(v)
		if !store.ValidStatus(status) {
			s.errorResponse(w, r, http.StatusBadRequest, "invalid status",
				"use one of: todo, in_progress, done (aliases: open, doing, completed, etc.)")
			return
		}
		updates["status"] = status
	}
	if v := r.FormValue("priority"); v != "" {
		priority := store.NormalizePriority(v)
		if !store.ValidPriority(priority) {
			s.errorResponse(w, r, http.StatusBadRequest, "invalid priority",
				"use one of: low, medium, high (aliases: p3, p2, p1, urgent, etc.)")
			return
		}
		updates["priority"] = priority
	}

	if len(updates) == 0 {
		s.errorResponse(w, r, http.StatusBadRequest, "no fields to update",
			"PATCH with status=<todo|in_progress|done> or priority=<low|medium|high> or title=<new title> or description=<new desc>")
		return
	}

	task, err := s.store.UpdateTask(handle, workspace, updates)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "task not found",
			"GET /api/tasks to list all tasks")
		return
	}

	s.writeResponse(w, r, http.StatusOK, formatTask(task), task)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "DELETE /api/tasks/<handle>")
		return
	}

	if err := s.store.DeleteTask(handle, workspace); err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "task not found",
			"GET /api/tasks to list all tasks")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("deleted handle=%s", handle),
		map[string]string{"status": "deleted", "handle": handle},
	)
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request, workspace string) {
	ws, err := s.store.GetWorkspace(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "workspace not found", "create it via POST /auth/request")
		return
	}

	tasks, _ := s.store.ListTasks(workspace, "", 10000)
	taskCount := len(tasks)

	doneCount := 0
	for _, t := range tasks {
		if t.Status == "done" {
			doneCount++
		}
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s name=%s plan=%s tasks=%d done=%d", ws.Handle, ws.Name, ws.Plan, taskCount, doneCount),
		map[string]string{
			"handle": ws.Handle,
			"name":   ws.Name,
			"plan":   ws.Plan,
			"tasks":  fmt.Sprintf("%d", taskCount),
			"done":   fmt.Sprintf("%d", doneCount),
		},
	)
}
