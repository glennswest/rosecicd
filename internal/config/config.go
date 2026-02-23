package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	GitHub   GitHubConfig   `yaml:"github"`
	Registry RegistryConfig `yaml:"registry"`
	Mkube    MkubeConfig    `yaml:"mkube"`
	Build    BuildConfig    `yaml:"build"`
	Builders []BuilderConfig `yaml:"builders"`
	Repos    []RepoConfig   `yaml:"repos"`
}

type BuilderConfig struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`     // "ssh" or "mkube"
	Arch     string `yaml:"arch"`     // "amd64" or "arm64"
	Host     string `yaml:"host"`     // SSH host (ssh type only)
	User     string `yaml:"user"`     // SSH user, default "root"
	Port     int    `yaml:"port"`     // SSH port, default 22
	KeyPath  string `yaml:"keyPath"`  // Path to SSH private key
	BuildDir string `yaml:"buildDir"` // "/build" — wiped between builds
	Capacity int    `yaml:"capacity"` // Max concurrent (1 for now)
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type GitHubConfig struct {
	User  string `yaml:"user"`
	Token string `yaml:"token"`
}

type MkubeConfig struct {
	APIURL string `yaml:"apiURL"`
}

type RegistryConfig struct {
	Local string `yaml:"local"`
	GHCR  string `yaml:"ghcr"`
}

type BuildConfig struct {
	BuilderImage string `yaml:"builderImage"`
	Network      string `yaml:"network"`
	CacheDir     string `yaml:"cacheDir"`
}

type RepoConfig struct {
	Name       string        `yaml:"name"`
	URL        string        `yaml:"url"`
	Branch     string        `yaml:"branch"`
	Dockerfile string        `yaml:"dockerfile"`
	Tags       []string      `yaml:"tags"`
	Poll       time.Duration `yaml:"poll"`
	Arch       string        `yaml:"arch"` // "amd64", "arm64", or "" for any
}

var envRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expandEnv(s string) string {
	return envRe.ReplaceAllStringFunc(s, func(m string) string {
		name := envRe.FindStringSubmatch(m)[1]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return m
	})
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	expanded := expandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8090"
	}
	// Default builder config values
	for i := range cfg.Builders {
		if cfg.Builders[i].User == "" {
			cfg.Builders[i].User = "root"
		}
		if cfg.Builders[i].Port == 0 {
			cfg.Builders[i].Port = 22
		}
		if cfg.Builders[i].Capacity == 0 {
			cfg.Builders[i].Capacity = 1
		}
		if cfg.Builders[i].BuildDir == "" {
			cfg.Builders[i].BuildDir = "/build"
		}
	}

	for i := range cfg.Repos {
		if cfg.Repos[i].Branch == "" {
			cfg.Repos[i].Branch = "main"
		}
		if cfg.Repos[i].Dockerfile == "" {
			cfg.Repos[i].Dockerfile = "Dockerfile"
		}
		if len(cfg.Repos[i].Tags) == 0 {
			cfg.Repos[i].Tags = []string{"edge"}
		}
	}
	return &cfg, nil
}
