package store

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/example/todo/internal/todo"
)


type Memory struct {
	mu   sync.RWMutex
	data map[string]todo.Todo
}


func NewMemory() *Memory {
	return &Memory{data: make(map[string]todo.Todo)}
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func (s *Memory) Create(task string, due time.Time) todo.Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	t := todo.Todo{
		ID:        newID(),
		Task:      task,
		DueDate:   due.UTC(),
		Completed: false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.data[t.ID] = t
	return t
}

func (s *Memory) Get(id string) (todo.Todo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data[id]
	return t, ok
}

func (s *Memory) List(includeCompleted bool) []todo.Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]todo.Todo, 0, len(s.data))
	for _, t := range s.data {
		if !includeCompleted && t.Completed {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DueDate.Before(out[j].DueDate)
	})
	return out
}

func (s *Memory) Update(id string, task *string, due *time.Time, completed *bool) (todo.Todo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data[id]
	if !ok {
		return todo.Todo{}, false
	}
	now := time.Now().UTC()
	if task != nil {
		t.Task = *task
	}
	if due != nil {
		t.DueDate = due.UTC()
	}
	if completed != nil {
		t.Completed = *completed
	}
	t.UpdatedAt = now
	s.data[id] = t
	return t, true
}

func (s *Memory) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return false
	}
	delete(s.data, id)
	return true
}
