package ui

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/glennswest/rosecicd/internal/buildmgr"
	"github.com/glennswest/rosecicd/internal/config"
)

type handlers struct {
	cfg  *config.Config
	mgr  *buildmgr.Manager
	tmpl *template.Template
}

func Register(mux *http.ServeMux, cfg *config.Config, mgr *buildmgr.Manager) {
	funcMap := template.FuncMap{
		"timeAgo":    timeAgo,
		"upper":      strings.ToUpper,
		"duration":   formatDuration,
		"badgeClass": badgeClass,
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html"))

	h := &handlers{cfg: cfg, mgr: mgr, tmpl: tmpl}

	staticFS, _ := fs.Sub(content, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("/", h.dashboard)
	mux.HandleFunc("/repos", h.repos)
	mux.HandleFunc("/repos/", h.repoDetail)
	mux.HandleFunc("/builds", h.builds)
	mux.HandleFunc("/builds/", h.buildDetail)
	mux.HandleFunc("/api/build", h.triggerBuild)
	mux.HandleFunc("/api/quickbuild", h.quickBuild)
}

type dashboardData struct {
	Repos        []repoCard
	ActiveBuilds int
}

type repoCard struct {
	Name      string
	URL       string
	LastBuild *buildmgr.Build
}

func (h *handlers) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	var cards []repoCard
	active := 0
	for _, repo := range h.cfg.Repos {
		builds := h.mgr.GetRepoBuilds(repo.Name)
		var last *buildmgr.Build
		for _, b := range builds {
			if last == nil || b.StartTime.After(last.StartTime) {
				last = b
			}
			if b.Status == buildmgr.StatusRunning || b.Status == buildmgr.StatusPending {
				active++
			}
		}
		cards = append(cards, repoCard{Name: repo.Name, URL: repo.URL, LastBuild: last})
	}

	h.render(w, "dashboard", dashboardData{Repos: cards, ActiveBuilds: active})
}

func (h *handlers) repos(w http.ResponseWriter, r *http.Request) {
	h.render(w, "repos", h.cfg.Repos)
}

type repoDetailData struct {
	Repo   config.RepoConfig
	Builds []*buildmgr.Build
}

func (h *handlers) repoDetail(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/repos/")
	repo, ok := h.mgr.FindRepo(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	builds := h.mgr.GetRepoBuilds(name)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].StartTime.After(builds[j].StartTime)
	})
	h.render(w, "repo_detail", repoDetailData{Repo: repo, Builds: builds})
}

func (h *handlers) builds(w http.ResponseWriter, r *http.Request) {
	all := h.mgr.ListBuilds()
	sort.Slice(all, func(i, j int) bool {
		return all[i].StartTime.After(all[j].StartTime)
	})
	h.render(w, "builds", all)
}

func (h *handlers) buildDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/builds/")
	b := h.mgr.GetBuild(id)
	if b == nil {
		http.NotFound(w, r)
		return
	}
	h.render(w, "build_detail", b)
}

func (h *handlers) triggerBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	name := r.FormValue("repo")
	repo, ok := h.mgr.FindRepo(name)
	if !ok {
		http.Error(w, "unknown repo", http.StatusNotFound)
		return
	}

	b, err := h.mgr.TriggerBuild(repo, "manual")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "build_row", b)
		return
	}
	http.Redirect(w, r, "/builds/"+b.ID, http.StatusSeeOther)
}

func (h *handlers) quickBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	repoURL := r.FormValue("url")
	if repoURL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	// Extract name from URL
	parts := strings.Split(strings.TrimSuffix(repoURL, "/"), "/")
	name := parts[len(parts)-1]

	repo := config.RepoConfig{
		Name:       name,
		URL:        repoURL,
		Branch:     "main",
		Dockerfile: "Dockerfile",
		Tags:       []string{"edge"},
	}

	b, err := h.mgr.TriggerBuild(repo, "quickbuild")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "build_row", b)
		return
	}
	http.Redirect(w, r, "/builds/"+b.ID, http.StatusSeeOther)
}

func (h *handlers) render(w http.ResponseWriter, name string, data interface{}) {
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strings.TrimSuffix(d.Truncate(time.Minute).String(), "0s") + " ago"
	case d < 24*time.Hour:
		return strings.TrimSuffix(d.Truncate(time.Hour).String(), "0m0s") + " ago"
	default:
		return t.Format("Jan 2 15:04")
	}
}

func badgeClass(s buildmgr.BuildStatus) string {
	switch s {
	case buildmgr.StatusSuccess:
		return "success"
	case buildmgr.StatusFailed:
		return "failed"
	case buildmgr.StatusRunning:
		return "running"
	default:
		return "pending"
	}
}

func formatDuration(start, end time.Time) string {
	if end.IsZero() {
		return "running..."
	}
	d := end.Sub(start)
	if d < time.Minute {
		return d.Truncate(time.Second).String()
	}
	return d.Truncate(time.Second).String()
}
