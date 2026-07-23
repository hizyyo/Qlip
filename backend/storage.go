package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ClipboardItem struct {
	ID          int64  `json:"id"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
	IsFavorite  bool   `json:"is_favorite"`
	CreatedAt   string `json:"created_at"`
}

type Snippet struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type Storage struct {
	mu       sync.RWMutex
	path     string
	Items    []ClipboardItem `json:"items"`
	Snippets []Snippet       `json:"snippets"`
	NextID   int64           `json:"next_id"`
}

func NewStorage() *Storage {
	dir, _ := os.UserConfigDir()
	path := filepath.Join(dir, "clipflow", "data.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	s := &Storage{path: path}
	s.load()
	if s.NextID == 0 {
		s.NextID = 1
	}
	return s
}

func (s *Storage) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	json.Unmarshal(data, s)
}

func (s *Storage) save() {
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(s.path, data, 0644)
}

func (s *Storage) AddItem(contentType, content string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.Items {
		if item.Content == content && item.ContentType == contentType {
			if i == 0 {
				return item.ID
			}
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			s.Items = append([]ClipboardItem{item}, s.Items...)
			s.save()
			return item.ID
		}
	}

	id := s.NextID
	s.NextID++
	item := ClipboardItem{
		ID:          id,
		ContentType: contentType,
		Content:     content,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	s.Items = append([]ClipboardItem{item}, s.Items...)
	if len(s.Items) > 500 {
		s.Items = s.Items[:500]
	}
	s.save()
	return id
}

func (s *Storage) GetHistory(limit, offset int, contentType string) []ClipboardItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []ClipboardItem
	for _, item := range s.Items {
		if contentType != "" && contentType != "all" {
			if contentType == "favorites" && !item.IsFavorite {
				continue
			}
			if contentType != "favorites" && item.ContentType != contentType {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	if offset >= len(filtered) {
		return nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end]
}

func (s *Storage) Search(query string, limit int) []ClipboardItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := query
	var results []ClipboardItem
	for _, item := range s.Items {
		if len(results) >= limit {
			break
		}
		if contains(item.Content, q) {
			results = append(results, item)
		}
	}
	return results
}

func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (s *Storage) ToggleFavorite(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Items {
		if s.Items[i].ID == id {
			s.Items[i].IsFavorite = !s.Items[i].IsFavorite
			s.save()
			return s.Items[i].IsFavorite
		}
	}
	return false
}

func (s *Storage) GetByID(id int64) *ClipboardItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.Items {
		if s.Items[i].ID == id {
			return &s.Items[i]
		}
	}
	return nil
}

func (s *Storage) DeleteItem(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Items {
		if s.Items[i].ID == id {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			s.save()
			return
		}
	}
}

func (s *Storage) AddSnippet(title, content string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.NextID
	s.NextID++
	s.Snippets = append(s.Snippets, Snippet{
		ID:        id,
		Title:     title,
		Content:   content,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	s.save()
	return id
}

func (s *Storage) GetSnippets() []Snippet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Snippet, len(s.Snippets))
	copy(result, s.Snippets)
	return result
}

func (s *Storage) DeleteSnippet(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Snippets {
		if s.Snippets[i].ID == id {
			s.Snippets = append(s.Snippets[:i], s.Snippets[i+1:]...)
			s.save()
			return
		}
	}
}
