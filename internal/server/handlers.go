package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"wallpaper-api/internal/errs"
	"wallpaper-api/internal/filter"
	"wallpaper-api/internal/manifest"
	"wallpaper-api/internal/selection"
	"wallpaper-api/internal/session"
	"wallpaper-api/internal/store"
)

const (
	defaultPageSize = 24
	maxPageSize     = 100
)

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	version, count := 0, 0
	if m := s.catalog.Snapshot(); m != nil {
		version, count = m.Version, m.Count
	}
	s.writeJSON(w, http.StatusOK, indexResponse{
		Service: "wallpaper-api",
		Version: version,
		Images:  count,
		Endpoints: []string{
			"/v1/random",
			"/v1/images",
			"/v1/images/{id}",
			"/v1/images/{id}/file",
			"/v1/tags",
			"/v1/categories",
			"/healthz",
		},
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	m := s.catalog.Snapshot()
	if m == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
		return
	}
	s.writeJSON(w, http.StatusOK, healthResponse{
		Status:              "ok",
		Images:              m.Count,
		ManifestGeneratedAt: m.GeneratedAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleRandom(w http.ResponseWriter, r *http.Request) {
	m, ok := s.manifestOrFail(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	criteria, err := filter.Parse(q)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	mode := selection.ParseMode(strings.ToLower(strings.TrimSpace(q.Get("mode"))))
	key, err := s.selectionKey(w, r, mode, q)
	if err != nil {
		s.writeErr(w, err)
		return
	}

	candidates := filter.Apply(m.Images, criteria)
	if len(candidates) == 0 {
		s.writeErr(w, errs.NotFound("no_match", "no image matches the requested filters"))
		return
	}
	img := candidates[selection.Index(mode, key, len(candidates))]

	if isTruthy(q.Get("redirect")) {
		http.Redirect(w, r, s.fileURL(r, img.ID), http.StatusFound)
		return
	}
	s.writeJSON(w, http.StatusOK, s.randomView(r, img, mode, m.Version))
}

// selectionKey resolves the mode-specific key, validating the daily date and
// issuing a session cookie when needed.
func (s *Server) selectionKey(w http.ResponseWriter, r *http.Request, mode selection.Mode, q url.Values) (string, error) {
	switch mode {
	case selection.ModeSeed:
		return selection.Key(mode, q.Get("seed"), "", ""), nil
	case selection.ModeDaily:
		raw := strings.TrimSpace(q.Get("date"))
		if raw == "" {
			return s.today(), nil
		}
		date, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return "", errs.BadRequest("invalid_date", "date must be formatted as YYYY-MM-DD")
		}
		return date.Format("2006-01-02"), nil
	case selection.ModeSession:
		id, set := session.Derive(r, q.Get("session"))
		if set {
			http.SetCookie(w, &http.Cookie{
				Name:     session.CookieName,
				Value:    id,
				Path:     "/",
				MaxAge:   86400,
				SameSite: http.SameSiteLaxMode,
			})
		}
		return id, nil
	default:
		return "", nil
	}
}

func (s *Server) today() string {
	return s.now().In(s.cfg.Location).Format("2006-01-02")
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	m, ok := s.manifestOrFail(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	criteria, err := filter.Parse(q)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	page, err := parsePage(q.Get("page"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	size, err := parsePageSize(q.Get("page_size"))
	if err != nil {
		s.writeErr(w, err)
		return
	}

	matched := filter.Apply(m.Images, criteria)
	items := make([]imageView, 0, size)
	if start := (page - 1) * size; start < len(matched) {
		end := min(start+size, len(matched))
		for _, img := range matched[start:end] {
			items = append(items, s.metaView(r, img))
		}
	}
	s.writeJSON(w, http.StatusOK, listResponse{
		Total:    len(matched),
		Page:     page,
		PageSize: size,
		Items:    items,
	})
}

func (s *Server) handleImageMeta(w http.ResponseWriter, r *http.Request) {
	m, ok := s.manifestOrFail(w)
	if !ok {
		return
	}
	img, found := m.Find(r.PathValue("id"))
	if !found {
		s.writeErr(w, errs.NotFound("not_found", "no image with that id"))
		return
	}
	s.writeJSON(w, http.StatusOK, s.metaView(r, img))
}

func (s *Server) handleImageFile(w http.ResponseWriter, r *http.Request) {
	if !s.hotlinkAllowed(r) {
		s.writeErr(w, errs.Forbidden("forbidden_origin", "the request origin is not allowed"))
		return
	}
	m, ok := s.manifestOrFail(w)
	if !ok {
		return
	}
	img, found := m.Find(r.PathValue("id"))
	if !found {
		s.writeErr(w, errs.NotFound("not_found", "no image with that id"))
		return
	}

	opened, err := s.store.Open(r.Context(), img.Path)
	if errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, errs.NotFound("not_found", "the image file is missing"))
		return
	}
	if err != nil {
		s.logger.Error("open image", "id", img.ID, "error", err)
		s.writeErr(w, errs.Unavailable("image_unavailable", "the image could not be loaded"))
		return
	}
	defer opened.Close()

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", contentType(img.Format))
	w.Header().Set("ETag", quoteETag(img.Hash))
	w.Header().Set("X-Cache", cacheLabel(opened.CacheHit))
	http.ServeContent(w, r, path.Base(img.Path), opened.ModTime, opened.File)
}

func (s *Server) handleTags(w http.ResponseWriter, _ *http.Request) {
	m, ok := s.manifestOrFail(w)
	if !ok {
		return
	}
	counts := make(map[string]int)
	for _, img := range m.Images {
		for _, tag := range img.Tags {
			if tag != "" {
				counts[tag]++
			}
		}
	}
	s.writeJSON(w, http.StatusOK, countResponse{Items: sortedCounts(counts)})
}

func (s *Server) handleCategories(w http.ResponseWriter, _ *http.Request) {
	m, ok := s.manifestOrFail(w)
	if !ok {
		return
	}
	counts := make(map[string]int)
	for _, img := range m.Images {
		if img.Category != "" {
			counts[img.Category]++
		}
	}
	s.writeJSON(w, http.StatusOK, countResponse{Items: sortedCounts(counts)})
}

func sortedCounts(counts map[string]int) []countItem {
	items := make([]countItem, 0, len(counts))
	for name, count := range counts {
		items = append(items, countItem{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func parsePage(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, errs.BadRequest("invalid_page", "page must be a positive integer")
	}
	return v, nil
}

func parsePageSize(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultPageSize, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 || v > maxPageSize {
		return 0, errs.BadRequest("invalid_page_size", "page_size must be between 1 and "+strconv.Itoa(maxPageSize))
	}
	return v, nil
}

func (s *Server) randomView(r *http.Request, img manifest.Image, mode selection.Mode, version int) imageView {
	v := s.metaView(r, img)
	v.Mode = string(mode)
	v.ManifestVersion = version
	return v
}

func (s *Server) metaView(r *http.Request, img manifest.Image) imageView {
	return imageView{
		ID:          img.ID,
		Title:       img.Title,
		Category:    img.Category,
		Tags:        ensureTags(img.Tags),
		Orientation: img.Orientation,
		Width:       img.Width,
		Height:      img.Height,
		Format:      img.Format,
		Bytes:       img.Bytes,
		Hash:        img.Hash,
		URL:         s.fileURL(r, img.ID),
		PageURL:     s.pageURL(r, img.ID),
	}
}

func (s *Server) fileURL(r *http.Request, id string) string {
	return s.baseURL(r) + "/v1/images/" + url.PathEscape(id) + "/file"
}

func (s *Server) pageURL(r *http.Request, id string) string {
	return s.baseURL(r) + "/v1/images/" + url.PathEscape(id)
}

func (s *Server) baseURL(r *http.Request) string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL
	}
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return scheme + "://" + host
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		s.logger.Error("encode response", "error", err)
	}
}

func (s *Server) writeErr(w http.ResponseWriter, err error) {
	var apiErr *errs.Error
	if errors.As(err, &apiErr) {
		s.writeJSON(w, apiErr.Status, errorResponse{Error: errorBody{Code: apiErr.Code, Message: apiErr.Message}})
		return
	}
	s.logger.Error("unhandled error", "error", err)
	s.writeJSON(w, http.StatusInternalServerError, errorResponse{Error: errorBody{Code: "internal", Message: "internal error"}})
}

func contentType(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func quoteETag(hash string) string { return `"` + hash + `"` }

func cacheLabel(hit bool) string {
	if hit {
		return "HIT"
	}
	return "MISS"
}

func ensureTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func isTruthy(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
