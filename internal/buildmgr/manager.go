package buildmgr

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/glennswest/rosecicd/internal/builder"
	"github.com/glennswest/rosecicd/internal/config"
)

type BuildStatus string

const (
	StatusPending  BuildStatus = "pending"
	StatusRunning  BuildStatus = "running"
	StatusSuccess  BuildStatus = "success"
	StatusFailed   BuildStatus = "failed"
	StatusUnknown  BuildStatus = "unknown"
)

type Build struct {
	ID        string      `json:"id"`
	RepoName  string      `json:"repoName"`
	Status    BuildStatus `json:"status"`
	PodName   string      `json:"podName"`
	StartTime time.Time   `json:"startTime"`
	EndTime   time.Time   `json:"endTime,omitempty"`
	Commit    string      `json:"commit,omitempty"`
	Logs      string      `json:"logs,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type Manager struct {
	cfg    *config.Config
	mu     sync.RWMutex
	builds map[string]*Build
	nextID int
	stopCh chan struct{}
}

func New(cfg *config.Config) *Manager {
	return &Manager{
		cfg:    cfg,
		builds: make(map[string]*Build),
		nextID: 1,
		stopCh: make(chan struct{}),
	}
}

func (m *Manager) TriggerBuild(repo config.RepoConfig, commit string) (*Build, error) {
	m.mu.Lock()
	id := fmt.Sprintf("build-%d", m.nextID)
	m.nextID++
	m.mu.Unlock()

	podName := fmt.Sprintf("rosecicd-%s-%s", repo.Name, id)

	b := &Build{
		ID:        id,
		RepoName:  repo.Name,
		Status:    StatusPending,
		PodName:   podName,
		StartTime: time.Now(),
		Commit:    commit,
	}

	m.mu.Lock()
	m.builds[id] = b
	m.mu.Unlock()

	go m.runBuild(b, repo)
	return b, nil
}

func (m *Manager) runBuild(b *Build, repo config.RepoConfig) {
	m.setBuildStatus(b.ID, StatusRunning)

	spec := m.buildSpec(repo)
	specJSON, err := json.Marshal(spec)
	if err != nil {
		m.failBuild(b.ID, fmt.Sprintf("marshal spec: %v", err))
		return
	}

	if err := CreateBuildPod(m.cfg, b.PodName, string(specJSON)); err != nil {
		m.failBuild(b.ID, fmt.Sprintf("create pod: %v", err))
		return
	}

	log.Printf("[build %s] pod %s created, waiting for completion", b.ID, b.PodName)

	status, err := WaitForPod(m.cfg, b.PodName, m.stopCh)
	if err != nil {
		m.failBuild(b.ID, fmt.Sprintf("wait for pod: %v", err))
		return
	}

	logs, _ := GetPodLogs(m.cfg, b.PodName)

	m.mu.Lock()
	b.Logs = logs
	b.EndTime = time.Now()
	m.mu.Unlock()

	if status == "Succeeded" {
		m.setBuildStatus(b.ID, StatusSuccess)
		log.Printf("[build %s] succeeded", b.ID)
	} else {
		m.failBuild(b.ID, fmt.Sprintf("pod status: %s", status))
	}

	// Cleanup pod
	if err := DeletePod(m.cfg, b.PodName); err != nil {
		log.Printf("[build %s] cleanup error: %v", b.ID, err)
	}
}

func (m *Manager) buildSpec(repo config.RepoConfig) builder.BuildSpec {
	return builder.BuildSpec{
		Repo:       repo.URL,
		Branch:     repo.Branch,
		Dockerfile: repo.Dockerfile,
		Tags:       repo.Tags,
		ImageName:  repo.Name,
		Registries: map[string]builder.Registry{
			"local": {
				URL:      m.cfg.Registry.Local,
				Insecure: true,
			},
			"ghcr": {
				URL:   m.cfg.Registry.GHCR,
				User:  m.cfg.GitHub.User,
				Token: m.cfg.GitHub.Token,
			},
		},
	}
}

func (m *Manager) setBuildStatus(id string, status BuildStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.builds[id]; ok {
		b.Status = status
	}
}

func (m *Manager) failBuild(id string, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.builds[id]; ok {
		b.Status = StatusFailed
		b.Error = errMsg
		b.EndTime = time.Now()
		log.Printf("[build %s] failed: %s", id, errMsg)
	}
}

func (m *Manager) GetBuild(id string) *Build {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if b, ok := m.builds[id]; ok {
		cp := *b
		return &cp
	}
	return nil
}

func (m *Manager) ListBuilds() []*Build {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Build, 0, len(m.builds))
	for _, b := range m.builds {
		cp := *b
		result = append(result, &cp)
	}
	return result
}

func (m *Manager) GetRepoBuilds(repoName string) []*Build {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Build
	for _, b := range m.builds {
		if b.RepoName == repoName {
			cp := *b
			result = append(result, &cp)
		}
	}
	return result
}

func (m *Manager) FindRepo(name string) (config.RepoConfig, bool) {
	for _, r := range m.cfg.Repos {
		if r.Name == name {
			return r, true
		}
	}
	return config.RepoConfig{}, false
}

func (m *Manager) Stop() {
	close(m.stopCh)
}
