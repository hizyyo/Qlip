package backend

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	storage *Storage
	imgDir  string
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
	mux.HandleFunc("/api/detect", s.handleDetect)
	mux.HandleFunc("/api/eval", s.handleEval)
	mux.HandleFunc("/api/format", s.handleFormat)
	mux.HandleFunc("/api/paste", s.handlePaste)
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
	Category    string `json:"category,omitempty"`
	EvalResult  string `json:"eval_result,omitempty"`
}

var urlRe = regexp.MustCompile(`(?i)^https?://\S+$`)
var emailRe = regexp.MustCompile(`(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
var phoneRe = regexp.MustCompile(`^[\+\d][\d\s\-\(\)]{6,20}$`)
var jsonRe = regexp.MustCompile(`^\s*[\{\[].*[\}\]]\s*$`)
var mathRe = regexp.MustCompile(`(?i)^=(.+)$`)

func detectCategory(text string) string {
	if jsonRe.MatchString(text) {
		var js interface{}
		if json.Unmarshal([]byte(text), &js) == nil {
			return "json"
		}
	}
	if urlRe.MatchString(strings.TrimSpace(text)) {
		return "link"
	}
	if emailRe.MatchString(strings.TrimSpace(text)) {
		return "email"
	}
	if phoneRe.MatchString(strings.TrimSpace(text)) {
		return "phone"
	}
	if strings.Contains(text, "func ") || strings.Contains(text, "function ") ||
		strings.Contains(text, "class ") || strings.Contains(text, "def ") ||
		strings.Contains(text, "```") || strings.Contains(text, "\t") {
		return "code"
	}
	if strings.Contains(text, "{{") && strings.Contains(text, "}}") {
		return "template"
	}
	return ""
}

func evalMath(expr string) string {
	m := mathRe.FindStringSubmatch(expr)
	if m == nil {
		return ""
	}
	expr = strings.TrimSpace(m[1])
	expr = strings.ReplaceAll(expr, "x", "*")
	expr = strings.ReplaceAll(expr, "÷", "/")
	result, err := simpleEval(expr)
	if err != nil {
		return ""
	}
	return result
}

func simpleEval(expr string) (string, error) {
	tokens := tokenize(expr)
	if len(tokens) == 0 {
		return "", nil
	}
	result, err := parseExpr(&tokens)
	if err != nil {
		return "", err
	}
	return formatNum(result), nil
}

func formatNum(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

func tokenize(s string) []string {
	var tokens []string
	i := 0
	for i < len(s) {
		c := s[i]
		if c == ' ' {
			i++
			continue
		}
		if c >= '0' && c <= '9' || c == '.' {
			j := i
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
				j++
			}
			tokens = append(tokens, s[i:j])
			i = j
			continue
		}
		if c == '+' || c == '-' || c == '*' || c == '/' || c == '(' || c == ')' {
			tokens = append(tokens, string(c))
			i++
			continue
		}
		return nil
	}
	return tokens
}

func parseExpr(tokens *[]string) (float64, error) {
	result, err := parseTerm(tokens)
	if err != nil {
		return 0, err
	}
	for len(*tokens) > 0 {
		op := (*tokens)[0]
		if op != "+" && op != "-" {
			break
		}
		*tokens = (*tokens)[1:]
		right, err := parseTerm(tokens)
		if err != nil {
			return 0, err
		}
		if op == "+" {
			result += right
		} else {
			result -= right
		}
	}
	return result, nil
}

func parseTerm(tokens *[]string) (float64, error) {
	result, err := parseFactor(tokens)
	if err != nil {
		return 0, err
	}
	for len(*tokens) > 0 {
		op := (*tokens)[0]
		if op != "*" && op != "/" {
			break
		}
		*tokens = (*tokens)[1:]
		right, err := parseFactor(tokens)
		if err != nil {
			return 0, err
		}
		if op == "*" {
			result *= right
		} else {
			if right == 0 {
				return 0, nil
			}
			result /= right
		}
	}
	return result, nil
}

func parseFactor(tokens *[]string) (float64, error) {
	if len(*tokens) == 0 {
		return 0, nil
	}
	t := (*tokens)[0]
	if t == "(" {
		*tokens = (*tokens)[1:]
		result, err := parseExpr(tokens)
		if err != nil {
			return 0, err
		}
		if len(*tokens) > 0 && (*tokens)[0] == ")" {
			*tokens = (*tokens)[1:]
		}
		return result, nil
	}
	*tokens = (*tokens)[1:]
	return strconv.ParseFloat(t, 64)
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
	if item.ContentType == "text" {
		h.Category = detectCategory(item.Content)
		if h.Category == "" {
			if r := evalMath(item.Content); r != "" {
				h.Category = "math"
				h.EvalResult = r
			}
		}
	}
	return h
}

func (s *Server) handleDetect(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	result := map[string]string{
		"category": detectCategory(text),
	}
	if result["category"] == "" {
		if r := evalMath(text); r != "" {
			result["category"] = "math"
			result["eval_result"] = r
		}
	}
	writeJSON(w, result)
}

func (s *Server) handleEval(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	result := evalMath(text)
	writeJSON(w, map[string]string{"result": result})
}

func (s *Server) handleFormat(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	action := r.URL.Query().Get("action")

	var out string
	switch action {
	case "json":
		var v interface{}
		if err := json.Unmarshal([]byte(text), &v); err == nil {
			b, _ := json.MarshalIndent(v, "", "  ")
			out = string(b)
		} else {
			out = text
		}
	case "json-minify":
		var v interface{}
		if err := json.Unmarshal([]byte(text), &v); err == nil {
			b, _ := json.Marshal(v)
			out = string(b)
		} else {
			out = text
		}
	case "base64-encode":
		out = base64.StdEncoding.EncodeToString([]byte(text))
	case "base64-decode":
		b, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(text)
		}
		if err == nil {
			out = string(b)
		} else {
			out = text
		}
	case "url-encode":
		out = url.QueryEscape(text)
	case "url-decode":
		if d, err := url.QueryUnescape(text); err == nil {
			out = d
		} else {
			out = text
		}
	case "upper":
		out = strings.ToUpper(text)
	case "lower":
		out = strings.ToLower(text)
	case "trim":
		out = strings.TrimSpace(text)
	case "lines-sort":
		lines := strings.Split(text, "\n")
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
		out = strings.Join(lines, "\n")
	case "lines-uniq":
		seen := map[string]bool{}
		var uniq []string
		for _, l := range strings.Split(text, "\n") {
			if !seen[l] {
				seen[l] = true
				uniq = append(uniq, l)
			}
		}
		out = strings.Join(uniq, "\n")
	default:
		out = text
	}

	writeJSON(w, map[string]string{"result": out})
}

func (s *Server) handlePaste(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	var req struct {
		Items []struct {
			Text    string `json:"text"`
			Content string `json:"content"`
		} `json:"items"`
		Delay int `json:"delay"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Delay == 0 {
		req.Delay = 200
	}
	texts := make([]string, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Text != "" {
			texts = append(texts, it.Text)
		} else if it.Content != "" {
			texts = append(texts, it.Content)
		}
	}
	go func() {
		for i, t := range texts {
			WriteClipboardText(t)
			if i < len(texts)-1 {
				time.Sleep(time.Duration(req.Delay) * time.Millisecond)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	writeJSON(w, map[string]string{"status": "ok"})
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

type SearchResultItem struct {
	HistoryItem
	Score       float64  `json:"score"`
	MatchRanges [][2]int `json:"match_ranges"`
	MatchType   string   `json:"match_type"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	mode := r.URL.Query().Get("mode")
	if limit == 0 {
		limit = 50
	}
	if mode == "fuzzy" {
		results := s.storage.SearchFuzzy(q, limit)
		out := make([]SearchResultItem, len(results))
		for i, r := range results {
			h := toHistoryItem(r.Item, s.imgDir)
			out[i] = SearchResultItem{
				HistoryItem: h,
				Score:       r.Score,
				MatchRanges: r.MatchRanges,
				MatchType:   r.MatchType,
			}
		}
		writeJSON(w, out)
		return
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
		Plain     bool   `json:"plain"`
		Vars      map[string]string `json:"vars"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	text := req.Text
	if req.Plain {
		text = stripPlain(text)
	}
	text = replaceVars(text, req.Vars)

	if text != "" {
		WriteClipboardText(text)
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

func stripPlain(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' || c == '\t' || (c >= 32 && c <= 126) || c >= 128 {
			out = append(out, c)
		}
	}
	return string(out)
}

func replaceVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	now := time.Now()
	s = strings.ReplaceAll(s, "{{date}}", now.Format("2006-01-02"))
	s = strings.ReplaceAll(s, "{{time}}", now.Format("15:04"))
	s = strings.ReplaceAll(s, "{{datetime}}", now.Format("2006-01-02 15:04"))
	return s
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
