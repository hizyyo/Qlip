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

type SearchResult struct {
	Item        ClipboardItem
	Score       float64
	MatchRanges [][2]int
	MatchType   string
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
		if substringMatch(item.Content, q) {
			results = append(results, item)
		}
	}
	return results
}

func (s *Storage) SearchFuzzy(query string, limit int) []SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []SearchResult

	for _, item := range s.Items {
		score, positions := fuzzyScore(query, item.Content)
		if score > 0 {
			matchType := "fuzzy"
			if allWordsMatch(query, item.Content) {
				score += 100
				matchType = "word"
			}
			if exactMatch(query, item.Content) {
				score += 200
				matchType = "exact"
			}
			results = append(results, SearchResult{
				Item:        item,
				Score:       score,
				MatchRanges: toRanges(positions),
				MatchType:   matchType,
			})
		}
	}

	sortByScore(results, limit)
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func fuzzyScore(query, text string) (float64, []int) {
	if query == "" || text == "" {
		return 0, nil
	}

	q := toLower(query)
	t := toLower(text)

	var positions []int
	qi := 0
	prevMatch := -5
	score := 0.0
	consecutive := 0

	for i := 0; i < len(t) && qi < len(q); i++ {
		if t[i] == q[qi] {
			positions = append(positions, i)
			score += 10.0

			if prevMatch == i-1 {
				consecutive++
				score += float64(consecutive) * 5
			} else {
				consecutive = 1
				score -= float64(i - prevMatch - 1) * 0.5
			}
			prevMatch = i
			qi++
		}
	}

	if qi < len(q) {
		return 0, nil
	}

	firstPos := float64(positions[0])
	score += (1.0 - firstPos/float64(len(t)+1)) * 50

	coverage := float64(positions[len(positions)-1]-positions[0]+1) / float64(len(t)+1)
	score += (1.0 - coverage) * 30

	return score, positions
}

func allWordsMatch(query, text string) bool {
	q := toLower(query)
	t := toLower(text)
	words := splitWords(q)
	for _, word := range words {
		if !substringMatch(t, word) {
			return false
		}
	}
	return len(words) > 0
}

func exactMatch(query, text string) bool {
	return toLower(text) == toLower(query)
}

func substringMatch(s, substr string) bool {
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

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func splitWords(s string) []string {
	var words []string
	start := -1
	for i := 0; i <= len(s); i++ {
		if i < len(s) && isAlpha(s[i]) {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 {
				words = append(words, s[start:i])
				start = -1
			}
		}
	}
	return words
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func toRanges(positions []int) [][2]int {
	if len(positions) == 0 {
		return nil
	}
	var ranges [][2]int
	start := positions[0]
	end := positions[0] + 1

	for i := 1; i < len(positions); i++ {
		if positions[i] == positions[i-1]+1 {
			end = positions[i] + 1
		} else {
			ranges = append(ranges, [2]int{start, end})
			start = positions[i]
			end = positions[i] + 1
		}
	}
	ranges = append(ranges, [2]int{start, end})
	return ranges
}

func sortByScore(results []SearchResult, limit int) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
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
