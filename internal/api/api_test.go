package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/relentlessworks/taskpilot/internal/auth"
	"github.com/relentlessworks/taskpilot/internal/store"
)

func setupTestServer(t *testing.T) *Server {
	tmpFile, err := os.CreateTemp("", "taskpilot-test-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	s, err := store.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	a := auth.New("test-secret")
	return NewServer(s, a)
}

func TestHelp(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "TaskPilot") {
		t.Error("help text should contain 'TaskPilot'")
	}
}

func TestHealth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthFlow(t *testing.T) {
	srv := setupTestServer(t)

	// Request OTP
	form := url.Values{"email": {"agent@test.com"}, "workspace": {"ws_test"}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "code=") {
		t.Fatalf("expected code in response, got: %s", w.Body.String())
	}

	// Extract code
	codeStr := w.Body.String()
	idx := strings.Index(codeStr, "code=")
	if idx == -1 {
		t.Fatal("could not find code in response")
	}
	code := strings.TrimSpace(codeStr[idx+5:])

	// Verify OTP
	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "token=tp_") {
		t.Fatalf("expected token in response, got: %s", w2.Body.String())
	}
}

func TestCreateAndGetTask(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create task
	form := url.Values{"title": {"Fix the login bug"}, "priority": {"high"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "handle=task_") {
		t.Fatalf("expected handle in response, got: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "priority=high") {
		t.Fatalf("expected priority=high in response, got: %s", w.Body.String())
	}

	// Extract handle
	handle := extractField(w.Body.String(), "handle=")

	// Get task
	req2 := httptest.NewRequest("GET", "/api/tasks/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "Fix the login bug") {
		t.Errorf("expected title in response, got: %s", w2.Body.String())
	}
}

func TestListTasks(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create a task
	form := url.Values{"title": {"Write tests"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	// List tasks
	req2 := httptest.NewRequest("GET", "/api/tasks", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "handle=task_") {
		t.Errorf("expected task in list, got: %s", w2.Body.String())
	}
}

func TestListTasksWithStatusFilter(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create two tasks
	form1 := url.Values{"title": {"Task 1"}}
	req1 := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req1.Header.Set("Authorization", "Bearer "+token)
	w1 := httptest.NewRecorder()
	srv.Router(w1, req1)

	form2 := url.Values{"title": {"Task 2"}}
	req2 := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	handle2 := extractField(w2.Body.String(), "handle=")

	// Mark task 2 as done
	form3 := url.Values{"status": {"done"}}
	req3 := httptest.NewRequest("PATCH", "/api/tasks/"+handle2, strings.NewReader(form3.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	// List only todo tasks
	req4 := httptest.NewRequest("GET", "/api/tasks?status=todo", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	srv.Router(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w4.Code, w4.Body.String())
	}
	body := w4.Body.String()
	if !strings.Contains(body, "Task 1") {
		t.Errorf("expected Task 1 in todo list, got: %s", body)
	}
	if strings.Contains(body, "Task 2") {
		t.Errorf("Task 2 should not be in todo list, got: %s", body)
	}

	// List only done tasks
	req5 := httptest.NewRequest("GET", "/api/tasks?status=done", nil)
	req5.Header.Set("Authorization", "Bearer "+token)
	w5 := httptest.NewRecorder()
	srv.Router(w5, req5)

	if w5.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w5.Code, w5.Body.String())
	}
	body = w5.Body.String()
	if !strings.Contains(body, "Task 2") {
		t.Errorf("expected Task 2 in done list, got: %s", body)
	}
	if strings.Contains(body, "Task 1") {
		t.Errorf("Task 1 should not be in done list, got: %s", body)
	}
}

func TestUpdateTask(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create task
	form := url.Values{"title": {"Original title"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	handle := extractField(w.Body.String(), "handle=")

	// Update task
	form2 := url.Values{"status": {"in_progress"}, "priority": {"high"}}
	req2 := httptest.NewRequest("PATCH", "/api/tasks/"+handle, strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "status=in_progress") {
		t.Errorf("expected status=in_progress, got: %s", w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "priority=high") {
		t.Errorf("expected priority=high, got: %s", w2.Body.String())
	}
}

func TestUpdateTaskStatusAlias(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create task
	form := url.Values{"title": {"Test alias"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	handle := extractField(w.Body.String(), "handle=")

	// Update with alias "completed"
	form2 := url.Values{"status": {"completed"}}
	req2 := httptest.NewRequest("PATCH", "/api/tasks/"+handle, strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "status=done") {
		t.Errorf("expected status=done (normalized from 'completed'), got: %s", w2.Body.String())
	}
}

func TestDeleteTask(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create task
	form := url.Values{"title": {"To be deleted"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	handle := extractField(w.Body.String(), "handle=")

	// Delete task
	req2 := httptest.NewRequest("DELETE", "/api/tasks/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "deleted") {
		t.Errorf("expected 'deleted' in response, got: %s", w2.Body.String())
	}

	// Verify it's gone
	req3 := httptest.NewRequest("GET", "/api/tasks/"+handle, nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	if w3.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w3.Code)
	}
}

func TestMissingTitle(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"priority": {"high"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestInvalidStatus(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"title": {"Test"}, "status": {"bogus"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNoAuth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestJSONFormat(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create task with JSON accept
	form := url.Values{"title": {"JSON test"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type, got: %s", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), "\"handle\"") {
		t.Errorf("expected JSON with handle field, got: %s", w.Body.String())
	}
}

func TestWorkspaceInfo(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create a task
	form := url.Values{"title": {"Test task"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	// Get workspace info
	req2 := httptest.NewRequest("GET", "/api/workspace", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "tasks=1") {
		t.Errorf("expected tasks=1 in response, got: %s", w2.Body.String())
	}
}

func TestCrossWorkspaceIsolation(t *testing.T) {
	srv := setupTestServer(t)

	// Create token for ws_test
	token1 := getTestToken(t, srv)

	// Create token for ws_other
	token2 := getTestTokenForWorkspace(t, srv, "ws_other")

	// Create task in ws_test
	form := url.Values{"title": {"Private task"}}
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token1)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	handle := extractField(w.Body.String(), "handle=")

	// Try to access from ws_other
	req2 := httptest.NewRequest("GET", "/api/tasks/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token2)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 from different workspace, got %d", w2.Code)
	}

	// List tasks from ws_other should not see ws_test's task
	req3 := httptest.NewRequest("GET", "/api/tasks", nil)
	req3.Header.Set("Authorization", "Bearer "+token2)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	if strings.Contains(w3.Body.String(), "Private task") {
		t.Errorf("ws_other should not see ws_test's tasks, got: %s", w3.Body.String())
	}
}

// --- Helpers ---

func getTestToken(t *testing.T, srv *Server) string {
	return getTestTokenForWorkspace(t, srv, "ws_test")
}

func getTestTokenForWorkspace(t *testing.T, srv *Server, workspace string) string {
	// Request OTP
	form := url.Values{"email": {"agent@test.com"}, "workspace": {workspace}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("auth/request failed: %d %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	idx := strings.Index(body, "code=")
	if idx == -1 {
		t.Fatalf("no code in response: %s", body)
	}
	code := strings.TrimSpace(body[idx+5:])

	// Verify OTP
	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("auth/verify failed: %d %s", w2.Code, w2.Body.String())
	}

	return extractField(w2.Body.String(), "token=")
}

func extractField(body, prefix string) string {
	idx := strings.Index(body, prefix)
	if idx == -1 {
		return ""
	}
	val := body[idx+len(prefix):]
	// Value ends at space or newline
	if sp := strings.IndexAny(val, " \n"); sp != -1 {
		val = val[:sp]
	}
	return strings.TrimSpace(val)
}
