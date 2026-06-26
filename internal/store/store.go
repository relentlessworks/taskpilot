package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/relentlessworks/taskpilot/internal/models"
)

// Store is a file-backed persistent store using JSON.
// It uses a simple but robust approach: load all data into memory at startup,
// persist changes atomically (write to temp file, rename).
type Store struct {
	mu       sync.RWMutex
	filePath string
	data     *storeData
}

type storeData struct {
	Workspaces map[string]*models.Workspace `json:"workspaces"`
	Tasks      map[string]*models.Task      `json:"tasks"`
	Tokens     map[string]*models.Token     `json:"tokens"`
	OTPs       map[string]*otpEntry         `json:"otps"`
}

type otpEntry struct {
	Email     string    `json:"email"`
	Code      string    `json:"code"`
	Workspace string    `json:"workspace"`
	ExpiresAt time.Time `json:"expires_at"`
}

// New opens (or creates) the store file.
func New(path string) (*Store, error) {
	s := &Store{
		filePath: path,
		data: &storeData{
			Workspaces: make(map[string]*models.Workspace),
			Tasks:      make(map[string]*models.Task),
			Tokens:     make(map[string]*models.Token),
			OTPs:       make(map[string]*otpEntry),
		},
	}

	// Load existing data if file exists
	if _, err := os.Stat(path); err == nil {
		if err := s.load(); err != nil {
			return nil, fmt.Errorf("load store: %w", err)
		}
	}

	return s, nil
}

// Close persists any pending changes.
func (s *Store) Close() error {
	return s.save()
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	// Empty file = fresh store
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, s.data)
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename
	dir := filepath.Dir(s.filePath)
	if dir == "" {
		dir = "."
	}
	tmpFile, err := os.CreateTemp(dir, ".taskpilot-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(b); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.filePath)
}

// --- Workspace operations ---

func (s *Store) CreateWorkspace(handle, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Workspaces[handle] = &models.Workspace{
		Handle: handle,
		Name:   name,
		Plan:   "free",
	}
	return s.save()
}

func (s *Store) GetWorkspace(handle string) (*models.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.data.Workspaces[handle]
	if !ok {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

func (s *Store) WorkspaceExists(handle string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data.Workspaces[handle]
	return ok, nil
}

// --- Task operations ---

func (s *Store) CreateTask(handle, title, description, status, priority, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.data.Tasks[handle] = &models.Task{
		Handle:      handle,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		Workspace:   workspace,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.save()
}

func (s *Store) GetTask(handle string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.data.Tasks[handle]
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	return task, nil
}

func (s *Store) ListTasks(workspace string, statusFilter string, limit int) ([]*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var tasks []*models.Task
	for _, t := range s.data.Tasks {
		if t.Workspace != workspace {
			continue
		}
		if statusFilter != "" && t.Status != statusFilter {
			continue
		}
		tasks = append(tasks, t)
	}

	// Sort by created_at descending
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}

func (s *Store) UpdateTask(handle, workspace string, updates map[string]string) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.data.Tasks[handle]
	if !ok || task.Workspace != workspace {
		return nil, fmt.Errorf("task not found")
	}

	if v, ok := updates["title"]; ok {
		task.Title = v
	}
	if v, ok := updates["description"]; ok {
		task.Description = v
	}
	if v, ok := updates["status"]; ok {
		task.Status = v
	}
	if v, ok := updates["priority"]; ok {
		task.Priority = v
	}
	task.UpdatedAt = time.Now()

	if err := s.save(); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Store) DeleteTask(handle, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.data.Tasks[handle]
	if !ok || task.Workspace != workspace {
		return fmt.Errorf("task not found")
	}
	delete(s.data.Tasks, handle)
	return s.save()
}

// --- Token operations ---

func (s *Store) CreateToken(value, workspace string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Tokens[value] = &models.Token{
		Value:     value,
		Workspace: workspace,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	return s.save()
}

func (s *Store) GetToken(value string) (*models.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.data.Tokens[value]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}
	return token, nil
}

func (s *Store) DeleteToken(value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Tokens, value)
	return s.save()
}

// --- OTP operations ---

func (s *Store) SaveOTP(email, code, workspace string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := email + ":" + code
	s.data.OTPs[key] = &otpEntry{
		Email:     email,
		Code:      code,
		Workspace: workspace,
		ExpiresAt: expiresAt,
	}
	return s.save()
}

func (s *Store) GetOTP(email, code string) (string, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := email + ":" + code
	entry, ok := s.data.OTPs[key]
	if !ok {
		return "", time.Time{}, fmt.Errorf("OTP not found")
	}
	return entry.Workspace, entry.ExpiresAt, nil
}

func (s *Store) DeleteOTP(email, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := email + ":" + code
	delete(s.data.OTPs, key)
	return s.save()
}

// --- Helpers ---

// ValidStatus returns true if the status is a recognized value.
func ValidStatus(status string) bool {
	return status == "todo" || status == "in_progress" || status == "done"
}

// ValidPriority returns true if the priority is a recognized value.
func ValidPriority(priority string) bool {
	return priority == "low" || priority == "medium" || priority == "high"
}

// NormalizeStatus converts common status aliases to canonical values.
func NormalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "todo", "open", "new", "pending":
		return "todo"
	case "in_progress", "inprogress", "doing", "started", "active":
		return "in_progress"
	case "done", "completed", "closed", "finished", "complete":
		return "done"
	default:
		return status
	}
}

// NormalizePriority converts common priority aliases to canonical values.
func NormalizePriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "low", "l", "p3":
		return "low"
	case "medium", "med", "normal", "m", "p2":
		return "medium"
	case "high", "h", "urgent", "critical", "p1":
		return "high"
	default:
		return priority
	}
}
