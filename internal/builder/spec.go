package builder

import (
	"encoding/json"
	"fmt"
	"os"
)

type BuildSpec struct {
	Repo       string              `json:"repo"`
	Branch     string              `json:"branch"`
	Dockerfile string              `json:"dockerfile"`
	Tags       []string            `json:"tags"`
	ImageName  string              `json:"imageName"`
	Registries map[string]Registry `json:"registries"`
}

type Registry struct {
	URL      string `json:"url"`
	User     string `json:"user,omitempty"`
	Token    string `json:"token,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
}

func LoadSpec(path string) (*BuildSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read build spec: %w", err)
	}
	var spec BuildSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse build spec: %w", err)
	}
	if spec.Branch == "" {
		spec.Branch = "main"
	}
	if spec.Dockerfile == "" {
		spec.Dockerfile = "Dockerfile"
	}
	return &spec, nil
}
