package poller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/glennswest/rosecicd/internal/buildmgr"
	"github.com/glennswest/rosecicd/internal/config"
)

type Poller struct {
	cfg    *config.Config
	mgr   *buildmgr.Manager
	mu     sync.Mutex
	lastSHA map[string]string // repo name → last known commit SHA
	stopCh  chan struct{}
}

func New(cfg *config.Config, mgr *buildmgr.Manager) *Poller {
	return &Poller{
		cfg:     cfg,
		mgr:     mgr,
		lastSHA: make(map[string]string),
		stopCh:  make(chan struct{}),
	}
}

func (p *Poller) Start() {
	for _, repo := range p.cfg.Repos {
		if repo.Poll > 0 {
			go p.pollRepo(repo)
		}
	}
}

func (p *Poller) Stop() {
	close(p.stopCh)
}

func (p *Poller) pollRepo(repo config.RepoConfig) {
	ticker := time.NewTicker(repo.Poll)
	defer ticker.Stop()

	// Do an initial check to seed the SHA (don't build on startup)
	if sha, err := p.getLatestCommit(repo); err == nil {
		p.mu.Lock()
		p.lastSHA[repo.Name] = sha
		p.mu.Unlock()
		log.Printf("[poll] %s: seeded sha=%s", repo.Name, sha[:12])
	}

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			sha, err := p.getLatestCommit(repo)
			if err != nil {
				log.Printf("[poll] %s: error: %v", repo.Name, err)
				continue
			}

			p.mu.Lock()
			prev := p.lastSHA[repo.Name]
			p.mu.Unlock()

			if sha != prev && prev != "" {
				log.Printf("[poll] %s: new commit %s (was %s), triggering build", repo.Name, sha[:12], prev[:12])
				if _, err := p.mgr.TriggerBuild(repo, sha[:12]); err != nil {
					log.Printf("[poll] %s: trigger error: %v", repo.Name, err)
				}
			}

			p.mu.Lock()
			p.lastSHA[repo.Name] = sha
			p.mu.Unlock()
		}
	}
}

type ghCommit struct {
	SHA string `json:"sha"`
}

func (p *Poller) getLatestCommit(repo config.RepoConfig) (string, error) {
	// Parse owner/repo from URL: https://github.com/owner/repo
	parts := strings.Split(strings.TrimSuffix(repo.URL, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid repo URL: %s", repo.URL)
	}
	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?sha=%s&per_page=1",
		owner, repoName, repo.Branch)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	if p.cfg.GitHub.Token != "" {
		req.Header.Set("Authorization", "token "+p.cfg.GitHub.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("github api: HTTP %d", resp.StatusCode)
	}

	var commits []ghCommit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return "", fmt.Errorf("decode commits: %w", err)
	}
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found")
	}
	return commits[0].SHA, nil
}
