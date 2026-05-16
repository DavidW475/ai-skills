package ui

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/DavidW475/ai-skills/internal/installer"
	"github.com/DavidW475/ai-skills/internal/lockfile"
	"github.com/DavidW475/ai-skills/internal/registry"
	"github.com/DavidW475/ai-skills/internal/resolver"
	"github.com/DavidW475/ai-skills/internal/sources"
)

//go:embed static
var staticFiles embed.FS

// Listen starts the HTTP server on addr and returns the URL.
// The server runs until ctx is cancelled.
func Listen(ctx context.Context, addr string, plainHTTP bool) (string, error) {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return "", err
	}

	srv := &server{plainHTTP: plainHTTP}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/installed", srv.handleInstalled)
	mux.HandleFunc("/api/update-all", srv.handleUpdateAll)
	mux.HandleFunc("/api/uninstall", srv.handleUninstall)
	mux.HandleFunc("/api/check-update", srv.handleCheckUpdate)
	mux.HandleFunc("/api/sources/list", srv.handleSourcesList)
	mux.HandleFunc("/api/sources/add", srv.handleSourcesAdd)
	mux.HandleFunc("/api/sources/remove", srv.handleSourcesRemove)
	mux.HandleFunc("/api/install", srv.handleInstall)
	mux.HandleFunc("/api/versions", srv.handleVersions)
	mux.HandleFunc("/api/available", srv.handleAvailable)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}

	httpSrv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		httpSrv.Shutdown(context.Background()) //nolint:errcheck
	}()
	go httpSrv.Serve(ln) //nolint:errcheck

	return "http://" + ln.Addr().String(), nil
}

type server struct{ plainHTTP bool }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

func (s *server) handleInstalled(w http.ResponseWriter, r *http.Request) {
	lf, err := lockfile.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	skills := lf.Skills
	if skills == nil {
		skills = []lockfile.Entry{}
	}
	writeJSON(w, skills)
}

func (s *server) handleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Name string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, "missing name", http.StatusBadRequest)
		return
	}
	lf, err := lockfile.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entry := lf.Find(req.Name)
	if entry == nil {
		writeError(w, req.Name+": not installed", http.StatusNotFound)
		return
	}
	if entry.Installed != "" {
		os.RemoveAll(entry.Installed) //nolint:errcheck
	}
	lf.Remove(req.Name)
	if err := lockfile.Save(lf); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"removed": req.Name})
}

func (s *server) handleSourcesList(w http.ResponseWriter, r *http.Request) {
	sf, err := sources.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	list := sf.Sources
	if list == nil {
		list = []string{}
	}
	writeJSON(w, list)
}

func (s *server) handleSourcesAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Source string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Source == "" {
		writeError(w, "missing source", http.StatusBadRequest)
		return
	}
	sf, err := sources.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sf.Add(req.Source)
	if err := sources.Save(sf); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	list := sf.Sources
	if list == nil {
		list = []string{}
	}
	writeJSON(w, list)
}

func (s *server) handleSourcesRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Source string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Source == "" {
		writeError(w, "missing source", http.StatusBadRequest)
		return
	}
	sf, err := sources.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sf.Remove(req.Source)
	if err := sources.Save(sf); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	list := sf.Sources
	if list == nil {
		list = []string{}
	}
	writeJSON(w, list)
}

func (s *server) handleUpdateAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	results, err := installer.Install(r.Context(), installer.Options{PlainHTTP: s.plainHTTP})
	if err != nil {
		writeError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, results)
}

func (s *server) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name    string
		Version string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, "missing name", http.StatusBadRequest)
		return
	}
	result, err := installer.InstallOne(r.Context(), req.Name, req.Version, installer.Options{
		PlainHTTP: s.plainHTTP,
	})
	if err != nil {
		writeError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, result)
}

func (s *server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, "missing name", http.StatusBadRequest)
		return
	}
	lf, err := lockfile.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entry := lf.Find(name)
	if entry == nil {
		writeError(w, name+": not installed", http.StatusNotFound)
		return
	}
	// Extract current tag from resolved ref (e.g. "registry.com/ns/skill:v1.0.0" → "v1.0.0")
	currentTag := ""
	if i := strings.LastIndex(entry.Resolved, ":"); i != -1 {
		currentTag = entry.Resolved[i+1:]
	}
	sf, err := sources.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Find the highest semver tag across all sources
	latestTag := ""
	for _, src := range sf.Sources {
		repoRef := strings.TrimRight(src, "/") + "/" + name
		tags, tagErr := registry.ListTags(r.Context(), repoRef, s.plainHTTP)
		if tagErr != nil {
			continue
		}
		if sv := resolver.LatestTag(tags); sv != "" {
			if latestTag == "" || resolver.IsNewer(sv, latestTag) {
				latestTag = sv
			}
		}
	}
	type result struct {
		Current   string `json:"current"`
		Latest    string `json:"latest"`
		HasUpdate bool   `json:"hasUpdate"`
	}
	writeJSON(w, result{
		Current:   currentTag,
		Latest:    latestTag,
		HasUpdate: latestTag != "" && resolver.IsNewer(latestTag, currentTag),
	})
}

func (s *server) handleAvailable(w http.ResponseWriter, r *http.Request) {
	sf, err := sources.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lf, err := lockfile.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type skillInfo struct {
		Name             string `json:"name"`
		Latest           string `json:"latest"`
		Source           string `json:"source"`
		Installed        bool   `json:"installed"`
		InstalledVersion string `json:"installedVersion,omitempty"`
	}
	type sourceResult struct {
		Source string      `json:"source"`
		Skills []skillInfo `json:"skills"`
		Error  string      `json:"error,omitempty"`
	}
	results := make([]sourceResult, 0, len(sf.Sources))
	for _, src := range sf.Sources {
		sr := sourceResult{Source: src}
		skills, err := registry.ListSkills(r.Context(), src, s.plainHTTP)
		if err != nil {
			sr.Error = err.Error()
			results = append(results, sr)
			continue
		}
		for _, name := range skills {
			repoRef := strings.TrimRight(src, "/") + "/" + name
			tags, _ := registry.ListTags(r.Context(), repoRef, s.plainHTTP)
			latest := resolver.LatestTag(tags)
			if latest == "" && len(tags) > 0 {
				latest = tags[len(tags)-1]
			}
			entry := lf.Find(name)
			var installedVer string
			if entry != nil {
				// Extract the tag from the resolved ref (e.g. "host/path:tag" → "tag").
				if idx := strings.LastIndex(entry.Resolved, ":"); idx >= 0 {
					installedVer = entry.Resolved[idx+1:]
				}
			}
			sr.Skills = append(sr.Skills, skillInfo{
				Name:             name,
				Latest:           latest,
				Source:           src,
				Installed:        entry != nil,
				InstalledVersion: installedVer,
			})
		}
		results = append(results, sr)
	}
	writeJSON(w, results)
}

func (s *server) handleVersions(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, "missing name", http.StatusBadRequest)
		return
	}
	sf, err := sources.Load()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type sourceResult struct {
		Source string   `json:"source"`
		Tags   []string `json:"tags"`
		Error  string   `json:"error,omitempty"`
	}
	results := make([]sourceResult, 0, len(sf.Sources))
	for _, src := range sf.Sources {
		repoRef := strings.TrimRight(src, "/") + "/" + name
		tags, err := registry.ListTags(r.Context(), repoRef, s.plainHTTP)
		sr := sourceResult{Source: src}
		if err != nil {
			sr.Error = err.Error()
		} else {
			sr.Tags = tags
		}
		results = append(results, sr)
	}
	writeJSON(w, results)
}
