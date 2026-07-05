package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"snippetsync/backend/internal/models"
	"snippetsync/backend/internal/seed"
	"snippetsync/backend/internal/synckv"
)

type Server struct {
	cluster *synckv.Cluster
	modules map[string]models.Module
	mux     *http.ServeMux
}

func NewServer() *Server {
	s := &Server{
		cluster: synckv.NewCluster(),
		modules: map[string]models.Module{},
		mux:     http.NewServeMux(),
	}
	for _, module := range seed.Modules() {
		s.modules[module.ID] = module
		payload, _ := json.Marshal(module)
		_, _ = s.cluster.Put(synckv.KVOperation{
			Type:      synckv.OpPut,
			Key:       moduleKey(module.ID),
			Value:     string(payload),
			RequestID: "seed-" + module.ID,
			ClientID:  "seed",
		})
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return cors(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("POST /api/auth/demo-login", s.demoLogin)
	s.mux.HandleFunc("GET /api/modules", s.listModules)
	s.mux.HandleFunc("POST /api/modules", s.createModule)
	s.mux.HandleFunc("GET /api/modules/{id}", s.getModule)
	s.mux.HandleFunc("PUT /api/modules/{id}", s.updateModule)
	s.mux.HandleFunc("DELETE /api/modules/{id}", s.deleteModule)
	s.mux.HandleFunc("GET /api/search", s.searchModules)
	s.mux.HandleFunc("POST /api/generate", s.generateProject)
	s.mux.HandleFunc("POST /api/sync", s.sync)
	s.mux.HandleFunc("GET /api/cluster/status", s.clusterStatus)
	s.mux.HandleFunc("POST /api/cluster/failover", s.failover)
	s.mux.HandleFunc("POST /api/cluster/snapshot", s.snapshot)
	s.mux.HandleFunc("POST /api/cluster/reassign-shard", s.reassignShard)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) demoLogin(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"user":  "demo-user",
		"token": "demo-token",
	})
}

func (s *Server) listModules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.sortedModules())
}

func (s *Server) createModule(w http.ResponseWriter, r *http.Request) {
	var module models.Module
	if err := json.NewDecoder(r.Body).Decode(&module); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	now := time.Now().UTC()
	if module.ID == "" {
		module.ID = slug(module.Title)
	}
	module.Owner = defaultString(module.Owner, "demo-user")
	module.CreatedAt = now
	module.UpdatedAt = now
	if len(module.Versions) == 0 {
		module.Versions = []models.ModuleVersion{{Version: "v1.0.0", Message: "Created in SnippetSync", Files: module.Files, CreatedAt: now}}
	}
	if err := s.persistModule(module, requestID(r, "create-"+module.ID)); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusCreated, module)
}

func (s *Server) getModule(w http.ResponseWriter, r *http.Request) {
	module, ok := s.modules[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("module not found"))
		return
	}
	writeJSON(w, http.StatusOK, module)
}

func (s *Server) updateModule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok := s.modules[id]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("module not found"))
		return
	}
	var module models.Module
	if err := json.NewDecoder(r.Body).Decode(&module); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	module.ID = id
	module.CreatedAt = existing.CreatedAt
	module.UpdatedAt = time.Now().UTC()
	module.Owner = defaultString(module.Owner, existing.Owner)
	if len(module.Versions) == 0 {
		module.Versions = existing.Versions
	}
	if err := s.persistModule(module, requestID(r, "update-"+id)); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, module)
}

func (s *Server) deleteModule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.modules[id]; !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("module not found"))
		return
	}
	if _, err := s.cluster.Delete(synckv.KVOperation{
		Type:      synckv.OpDelete,
		Key:       moduleKey(id),
		RequestID: requestID(r, "delete-"+id),
		ClientID:  "api",
	}); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	delete(s.modules, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) searchModules(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	language := strings.ToLower(r.URL.Query().Get("language"))
	framework := strings.ToLower(r.URL.Query().Get("framework"))
	tag := strings.ToLower(r.URL.Query().Get("tag"))
	var results []models.Module
	for _, module := range s.sortedModules() {
		if language != "" && strings.ToLower(module.Language) != language {
			continue
		}
		if framework != "" && strings.ToLower(module.Framework) != framework {
			continue
		}
		if tag != "" && !containsLower(module.Tags, tag) {
			continue
		}
		if query != "" && !matchesModule(module, query) {
			continue
		}
		results = append(results, module)
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) generateProject(w http.ResponseWriter, r *http.Request) {
	var req models.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.ProjectName == "" {
		req.ProjectName = "generated-backend"
	}
	seen := map[string]bool{}
	var files []models.GeneratedFile
	var deps []string
	for _, id := range expandDependencies(req.ModuleIDs, s.modules) {
		if seen[id] {
			continue
		}
		seen[id] = true
		module, ok := s.modules[id]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("module %q not found", id))
			return
		}
		deps = append(deps, module.Title)
		for _, f := range module.Files {
			files = append(files, models.GeneratedFile{Path: req.ProjectName + "/" + f.Path, Content: f.Content})
		}
	}
	files = append(files, models.GeneratedFile{
		Path:    req.ProjectName + "/README.md",
		Content: "# " + req.ProjectName + "\n\nGenerated by SnippetSync from trusted reusable modules.\n",
	})
	writeJSON(w, http.StatusOK, models.GenerateResponse{
		ProjectName:       req.ProjectName,
		Files:             files,
		DependencySummary: deps,
		ArchiveName:       req.ProjectName + ".zip",
	})
}

func (s *Server) sync(w http.ResponseWriter, r *http.Request) {
	s.cluster.Sync()
	writeJSON(w, http.StatusOK, s.cluster.Status())
}

func (s *Server) clusterStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cluster.Status())
}

func (s *Server) failover(w http.ResponseWriter, r *http.Request) {
	view, err := s.cluster.Failover()
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	path, err := s.cluster.Snapshot("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"snapshot": path})
}

func (s *Server) reassignShard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Shard int    `json:"shard"`
		Owner string `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.cluster.ReassignShard(req.Shard, req.Owner); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, s.cluster.Status())
}

func (s *Server) persistModule(module models.Module, reqID string) error {
	payload, err := json.Marshal(module)
	if err != nil {
		return err
	}
	_, err = s.cluster.Put(synckv.KVOperation{
		Type:      synckv.OpPut,
		Key:       moduleKey(module.ID),
		Value:     string(payload),
		RequestID: reqID,
		ClientID:  "api",
	})
	if err != nil {
		return err
	}
	s.modules[module.ID] = module
	return nil
}

func (s *Server) sortedModules() []models.Module {
	modules := make([]models.Module, 0, len(s.modules))
	for _, module := range s.modules {
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Title < modules[j].Title
	})
	return modules
}

func moduleKey(id string) string {
	return "module:" + id
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestID(r *http.Request, fallback string) string {
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return fallback + "-" + time.Now().UTC().Format("20060102150405.000000000")
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, value)
	if value == "" {
		return "module"
	}
	return value
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func containsLower(values []string, target string) bool {
	for _, value := range values {
		if strings.ToLower(value) == target {
			return true
		}
	}
	return false
}

func matchesModule(module models.Module, query string) bool {
	haystack := strings.ToLower(module.Title + " " + module.Description + " " + module.Language + " " + module.Framework + " " + strings.Join(module.Tags, " "))
	return strings.Contains(haystack, query)
}

func expandDependencies(ids []string, modules map[string]models.Module) []string {
	ordered := []string{}
	visited := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		module, ok := modules[id]
		if ok {
			for _, dep := range module.Dependencies {
				visit(dep)
			}
		}
		ordered = append(ordered, id)
	}
	for _, id := range ids {
		visit(id)
	}
	return ordered
}
