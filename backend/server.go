package backend

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Server struct {
	storage  *Storage
	imgDir   string
}

func NewServer(storage *Storage, imgDir string) *Server {
	return &Server{storage: storage, imgDir: imgDir}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/favorite", s.handleFavorite)
	mux.HandleFunc("/api/delete", s.handleDelete)
	mux.HandleFunc("/api/snippets", s.handleSnippets)
	mux.HandleFunc("/api/copy", s.handleCopy)
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(s.imgDir))))
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type HistoryItem struct {
	ID          int64  `json:"id"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
	IsFavorite  bool   `json:"is_favorite"`
	CreatedAt   string `json:"created_at"`
	Thumbnail   string `json:"thumbnail,omitempty"`
}

func toHistoryItem(item ClipboardItem, imgDir string) HistoryItem {
	h := HistoryItem{
		ID:          item.ID,
		ContentType: item.ContentType,
		Content:     item.Content,
		IsFavorite:  item.IsFavorite,
		CreatedAt:   item.CreatedAt,
	}
	if item.ContentType == "image" && item.Content != "" {
		filename := filepath.Base(item.Content)
		h.Thumbnail = "/images/" + filename
	}
	return h
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	ctype := r.URL.Query().Get("type")
	if limit == 0 {
		limit = 50
	}
	items := s.storage.GetHistory(limit, offset, ctype)
	result := make([]HistoryItem, len(items))
	for i, item := range items {
		result[i] = toHistoryItem(item, s.imgDir)
	}
	writeJSON(w, result)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	items := s.storage.Search(q, limit)
	result := make([]HistoryItem, len(items))
	for i, item := range items {
		result[i] = toHistoryItem(item, s.imgDir)
	}
	writeJSON(w, result)
}

func (s *Server) handleFavorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	result := s.storage.ToggleFavorite(id)
	writeJSON(w, map[string]bool{"favorite": result})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)

	item := s.storage.GetByID(id)
	if item != nil && item.ContentType == "image" && item.Content != "" {
		os.Remove(item.Content)
	}

	s.storage.DeleteItem(id)
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleSnippets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		writeJSON(w, s.storage.GetSnippets())
	case "POST":
		var req struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		id := s.storage.AddSnippet(req.Title, req.Content)
		writeJSON(w, map[string]int64{"id": id})
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		s.storage.DeleteSnippet(id)
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	var req struct {
		Text      string `json:"text"`
		ImagePath string `json:"image_path"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Text != "" {
		WriteClipboardText(req.Text)
	} else if req.ImagePath != "" {
		data, err := os.ReadFile(req.ImagePath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := WriteClipboardPNG(data); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) addImageItem(path string) {
	s.storage.AddItem("image", path)
}

func (s *Server) findImagePathByID(id int64) string {
	item := s.storage.GetByID(id)
	if item == nil {
		return ""
	}

	filepath.Clean(item.Content)
	if strings.HasPrefix(item.Content, s.imgDir) || len(item.Content) > 0 {
		return item.Content
	}
	return ""
}
