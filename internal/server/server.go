package server

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/example/todo/internal/store"
	"github.com/example/todo/internal/todo"
)

type apiResponse struct {
	Message string `json:"message"`
	Body    any    `json:"body"`
}

func responseBody(w http.ResponseWriter, status int, message string, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(apiResponse{Message: message, Body: body}); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	slog.Warn("handler error",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"status", status,
		"message", msg,
	)
	responseBody(w, status, msg, nil)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	if rec.status == 0 {
		rec.status = code
	}
	rec.ResponseWriter.WriteHeader(code)
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		slog.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"remote", r.RemoteAddr,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func parseRFC3339(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

type handler struct {
	store *store.Memory
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req todo.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Task = strings.TrimSpace(req.Task)
	if req.Task == "" {
		writeError(w, r, http.StatusBadRequest, "task is required")
		return
	}
	if req.DueDate == "" {
		writeError(w, r, http.StatusBadRequest, "due_date is required (RFC3339)")
		return
	}
	due, err := parseRFC3339(req.DueDate)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "due_date must be RFC3339, e.g. 2026-05-20T15:04:05Z")
		return
	}
	t := h.store.Create(req.Task, due)
	slog.Info("todo created", "id", t.ID, "due_date", t.DueDate.UTC().Format(time.RFC3339), "task_len", len(t.Task))
	responseBody(w, http.StatusCreated, "Todo created successfully", t)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, ok := h.store.Get(id)
	if !ok {
		writeError(w, r, http.StatusNotFound, "todo not found")
		return
	}
	slog.Info("todo read", "id", id, "completed", t.Completed)
	responseBody(w, http.StatusOK, "Todo retrieved successfully", t)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_completed")))
	include := q == "1" || q == "true" || q == "yes"
	items := h.store.List(include)
	if items == nil {
		items = []todo.Todo{}
	}
	slog.Info("todos listed",
		"returned", len(items),
		"include_completed", include,
	)
	responseBody(w, http.StatusOK, "Todos listed successfully", map[string]any{"todos": items})
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req todo.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Task == nil && req.DueDate == nil && req.Completed == nil {
		writeError(w, r, http.StatusBadRequest, "provide at least one of: task, due_date, completed")
		return
	}
	var duePtr *time.Time
	if req.DueDate != nil && *req.DueDate != "" {
		d, err := parseRFC3339(*req.DueDate)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "due_date must be RFC3339")
			return
		}
		duePtr = &d
	}
	var taskPtr *string
	if req.Task != nil {
		trim := strings.TrimSpace(*req.Task)
		if trim == "" {
			writeError(w, r, http.StatusBadRequest, "task cannot be empty")
			return
		}
		taskPtr = &trim
	}
	t, ok := h.store.Update(id, taskPtr, duePtr, req.Completed)
	if !ok {
		writeError(w, r, http.StatusNotFound, "todo not found")
		return
	}
	slog.Info("todo updated",
		"id", id,
		"changed_task", taskPtr != nil,
		"changed_due", duePtr != nil,
		"changed_completed", req.Completed != nil,
		"completed_now", t.Completed,
	)
	responseBody(w, http.StatusOK, "Todo updated successfully", t)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.store.Delete(id) {
		writeError(w, r, http.StatusNotFound, "todo not found")
		return
	}
	slog.Info("todo deleted", "id", id)
	responseBody(w, http.StatusOK, "Todo deleted successfully", map[string]string{"id": id})
}

func newMux(st *store.Memory) http.Handler {
	h := &handler{store: st}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /todos", h.create)
	mux.HandleFunc("GET /todos", h.list)
	mux.HandleFunc("GET /todos/{id}", h.get)
	mux.HandleFunc("PUT /todos/{id}", h.update)
	mux.HandleFunc("DELETE /todos/{id}", h.delete)
	return withRequestLogging(mux)
}

func Run(addr string) error {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: "time", Value: slog.StringValue(a.Value.Time().UTC().Format(time.RFC3339Nano))}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(h))

	st := store.NewMemory()
	slog.Info("todo API starting", "listen_addr", addr, "url", "http://localhost"+addr)
	return http.ListenAndServe(addr, newMux(st))
}

func Main() {
	const addr = ":8080"
	if err := Run(addr); err != nil {
		log.Fatal(err)
	}
}
