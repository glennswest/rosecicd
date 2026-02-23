package buildmgr

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/glennswest/rosecicd/internal/builder"
	"github.com/glennswest/rosecicd/internal/config"
)

type BuildStatus string

const (
	StatusPending BuildStatus = "pending"
	StatusQueued  BuildStatus = "queued"
	StatusRunning BuildStatus = "running"
	StatusSuccess BuildStatus = "success"
	StatusFailed  BuildStatus = "failed"
	StatusUnknown BuildStatus = "unknown"
)

type Build struct {
	ID          string      `json:"id"`
	RepoName    string      `json:"repoName"`
	Status      BuildStatus `json:"status"`
	PodName     string      `json:"podName"`
	BuilderName string      `json:"builderName,omitempty"`
	QueuePos    int         `json:"queuePos,omitempty"`
	StartTime   time.Time   `json:"startTime"`
	EndTime     time.Time   `json:"endTime,omitempty"`
	Commit      string      `json:"commit,omitempty"`
	Logs        string      `json:"logs,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// BuilderStatus is used by the UI to show builder state.
type BuilderStatus struct {
	Name       string
	Arch       string
	Type       string
	Online     bool
	Running    *Build
	QueueDepth int
}

type Manager struct {
	cfg      *config.Config
	mu       sync.RWMutex
	builds   map[string]*Build
	nextID   int
	builders []Builder
	queues   map[string]*BuildQueue // builder name → queue
	stopCh   chan struct{}
}

func New(cfg *config.Config) *Manager {
	m := &Manager{
		cfg:    cfg,
		builds: make(map[string]*Build),
		nextID: 1,
		queues: make(map[string]*BuildQueue),
		stopCh: make(chan struct{}),
	}

	// Initialize builders from config
	if len(cfg.Builders) == 0 {
		// Backward compat: no builders configured, create a default mkube builder
		bc := config.BuilderConfig{
			Name:     "mkube-arm64",
			Type:     "mkube",
			Arch:     "arm64",
			Capacity: 1,
		}
		b := NewMkubeBuilder(cfg, bc)
		m.builders = append(m.builders, b)
		m.queues[b.Name()] = NewBuildQueue(b)
		log.Printf("[mgr] initialized default mkube builder: %s (%s)", b.Name(), b.Arch())
	} else {
		for _, bc := range cfg.Builders {
			var b Builder
			switch bc.Type {
			case "ssh":
				b = NewSSHBuilder(cfg, bc)
			case "mkube":
				b = NewMkubeBuilder(cfg, bc)
			default:
				log.Printf("[mgr] unknown builder type %q for %s, skipping", bc.Type, bc.Name)
				continue
			}
			m.builders = append(m.builders, b)
			m.queues[b.Name()] = NewBuildQueue(b)
			log.Printf("[mgr] initialized builder: %s (type=%s, arch=%s)", b.Name(), bc.Type, b.Arch())
		}
	}

	return m
}

func (m *Manager) TriggerBuild(repo config.RepoConfig, commit string) (*Build, error) {
	m.mu.Lock()
	id := fmt.Sprintf("build-%d", m.nextID)
	m.nextID++
	m.mu.Unlock()

	// Select builder for this repo
	b := m.selectBuilder(repo)
	if b == nil {
		return nil, fmt.Errorf("no builder available for repo %s (arch=%s)", repo.Name, repo.Arch)
	}

	build := &Build{
		ID:          id,
		RepoName:    repo.Name,
		Status:      StatusQueued,
		PodName:     fmt.Sprintf("rosecicd-%s-%s", repo.Name, id),
		BuilderName: b.Name(),
		StartTime:   time.Now(),
		Commit:      commit,
	}

	m.mu.Lock()
	m.builds[id] = build
	m.mu.Unlock()

	spec := m.buildSpec(repo)
	job := &BuildJob{
		Build: build,
		Spec:  spec,
		Repo:  repo,
	}

	queue := m.queues[b.Name()]
	pos := queue.Enqueue(job)
	build.QueuePos = pos

	log.Printf("[mgr] build %s queued on %s (position %d)", id, b.Name(), pos)
	return build, nil
}

func (m *Manager) selectBuilder(repo config.RepoConfig) Builder {
	for _, b := range m.builders {
		// If repo has an arch preference, match it
		if repo.Arch != "" && b.Arch() != repo.Arch {
			continue
		}
		return b
	}
	// No arch match — return first available builder
	if repo.Arch == "" && len(m.builders) > 0 {
		return m.builders[0]
	}
	return nil
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

func (m *Manager) GetBuild(id string) *Build {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if b, ok := m.builds[id]; ok {
		cp := *b
		// Update queue position dynamically
		if cp.Status == StatusQueued {
			for _, q := range m.queues {
				if pos := q.QueuePosition(id); pos >= 0 {
					cp.QueuePos = pos
					break
				}
			}
		}
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

// ListBuilders returns status for all configured builders.
func (m *Manager) ListBuilders() []BuilderStatus {
	var result []BuilderStatus
	for _, b := range m.builders {
		q := m.queues[b.Name()]
		running, _ := q.QueueStatus()

		var runBuild *Build
		if running != nil {
			cp := *running.Build
			runBuild = &cp
		}

		bType := "mkube"
		if _, ok := b.(*SSHBuilder); ok {
			bType = "ssh"
		}

		result = append(result, BuilderStatus{
			Name:       b.Name(),
			Arch:       b.Arch(),
			Type:       bType,
			Online:     b.Healthy(),
			Running:    runBuild,
			QueueDepth: q.QueueDepth(),
		})
	}
	return result
}

func (m *Manager) Stop() {
	close(m.stopCh)
	for _, q := range m.queues {
		q.Stop()
	}
}
