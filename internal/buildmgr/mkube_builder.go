package buildmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/glennswest/rosecicd/internal/builder"
	"github.com/glennswest/rosecicd/internal/config"
)

// MkubeBuilder runs builds as mkube pods (existing behavior).
type MkubeBuilder struct {
	cfg      *config.Config
	name     string
	arch     string
	stopCh   chan struct{}
}

func NewMkubeBuilder(cfg *config.Config, bc config.BuilderConfig) *MkubeBuilder {
	return &MkubeBuilder{
		cfg:    cfg,
		name:   bc.Name,
		arch:   bc.Arch,
		stopCh: make(chan struct{}),
	}
}

func (m *MkubeBuilder) Name() string { return m.name }
func (m *MkubeBuilder) Arch() string { return m.arch }

func (m *MkubeBuilder) Healthy() bool {
	// TODO: could ping mkube API
	return true
}

func (m *MkubeBuilder) Run(ctx context.Context, spec builder.BuildSpec, buildID string) (string, error) {
	podName := fmt.Sprintf("rosecicd-%s-%s", spec.ImageName, buildID)

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal spec: %w", err)
	}

	if err := CreateBuildPod(m.cfg, podName, string(specJSON)); err != nil {
		return "", fmt.Errorf("create pod: %w", err)
	}
	log.Printf("[mkube/%s] pod %s created", m.name, podName)

	// Use context-aware stop channel
	stopCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			close(stopCh)
		case <-stopCh:
		}
	}()

	status, err := WaitForPod(m.cfg, podName, stopCh)
	if err != nil {
		return "", fmt.Errorf("wait for pod: %w", err)
	}

	logs, _ := GetPodLogs(m.cfg, podName)

	// Cleanup
	if err := DeletePod(m.cfg, podName); err != nil {
		log.Printf("[mkube/%s] cleanup error: %v", m.name, err)
	}

	if status != "Succeeded" {
		return logs, fmt.Errorf("pod status: %s", status)
	}
	return logs, nil
}
