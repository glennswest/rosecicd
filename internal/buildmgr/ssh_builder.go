package buildmgr

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/glennswest/rosecicd/internal/builder"
	"github.com/glennswest/rosecicd/internal/config"
	"golang.org/x/crypto/ssh"
)

// SSHBuilder runs builds on a remote host via SSH.
type SSHBuilder struct {
	cfg  config.BuilderConfig
	rcfg *config.Config
}

func NewSSHBuilder(rcfg *config.Config, bc config.BuilderConfig) *SSHBuilder {
	return &SSHBuilder{cfg: bc, rcfg: rcfg}
}

func (s *SSHBuilder) Name() string { return s.cfg.Name }
func (s *SSHBuilder) Arch() string { return s.cfg.Arch }

func (s *SSHBuilder) Healthy() bool {
	client, err := s.dial()
	if err != nil {
		return false
	}
	client.Close()
	return true
}

func (s *SSHBuilder) Run(ctx context.Context, spec builder.BuildSpec, buildID string) (string, error) {
	client, err := s.dial()
	if err != nil {
		return "", fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()

	localReg := spec.Registries["local"]
	imageTag := fmt.Sprintf("%s/%s:%s", localReg.URL, spec.ImageName, spec.Tags[0])

	// Build the script to execute on the remote host.
	// No --isolation chroot or --storage-driver vfs needed — LXC with nesting=1
	// supports overlayfs natively, so builds get layer caching.
	script := fmt.Sprintf(`set -e
echo "=== rosecicd build %s ==="
echo "Cleaning build directory..."
rm -rf %s/*

echo "Cloning %s (branch %s)..."
git clone --depth 1 --branch %s %s %s/src

echo "Building image %s..."
cd %s/src
buildah bud -f %s -t %s .

echo "Pushing to local registry..."
buildah push --tls-verify=false %s docker://%s

echo "Cleaning up..."
buildah rmi %s || true
rm -rf %s/*
echo "=== build complete ==="
`,
		buildID,
		s.cfg.BuildDir,
		spec.Repo, spec.Branch,
		spec.Branch, spec.Repo, s.cfg.BuildDir,
		imageTag,
		s.cfg.BuildDir,
		spec.Dockerfile, imageTag,
		imageTag, imageTag,
		imageTag,
		s.cfg.BuildDir,
	)

	// Also push to GHCR if configured
	if ghcr, ok := spec.Registries["ghcr"]; ok && ghcr.User != "" && ghcr.Token != "" {
		ghcrTag := fmt.Sprintf("%s/%s:%s", ghcr.URL, spec.ImageName, spec.Tags[0])
		// Re-tag and push. We already removed the image, so re-build step is avoided
		// by doing GHCR push before cleanup. Let's adjust: push GHCR before cleanup.
		script = fmt.Sprintf(`set -e
echo "=== rosecicd build %s ==="
echo "Cleaning build directory..."
rm -rf %s/*

echo "Cloning %s (branch %s)..."
git clone --depth 1 --branch %s %s %s/src

echo "Building image %s..."
cd %s/src
buildah bud -f %s -t %s .

echo "Pushing to local registry..."
buildah push --tls-verify=false %s docker://%s

echo "Pushing to GHCR..."
buildah push --creds %s:%s %s docker://%s

echo "Cleaning up..."
buildah rmi %s || true
rm -rf %s/*
echo "=== build complete ==="
`,
			buildID,
			s.cfg.BuildDir,
			spec.Repo, spec.Branch,
			spec.Branch, spec.Repo, s.cfg.BuildDir,
			imageTag,
			s.cfg.BuildDir,
			spec.Dockerfile, imageTag,
			imageTag, imageTag,
			ghcr.User, ghcr.Token, imageTag, ghcrTag,
			imageTag,
			s.cfg.BuildDir,
		)
	}

	log.Printf("[ssh/%s] executing build %s on %s", s.cfg.Name, buildID, s.cfg.Host)

	output, err := s.runScript(ctx, client, script)
	if err != nil {
		return output, fmt.Errorf("build failed: %w", err)
	}
	return output, nil
}

func (s *SSHBuilder) dial() (*ssh.Client, error) {
	keyData, err := os.ReadFile(s.cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh key %s: %w", s.cfg.KeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse ssh key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: s.cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return client, nil
}

func (s *SSHBuilder) runScript(ctx context.Context, client *ssh.Client, script string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Run with context cancellation
	done := make(chan error, 1)
	go func() {
		done <- session.Run(script)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		return stdout.String() + stderr.String(), ctx.Err()
	case err := <-done:
		output := stdout.String() + stderr.String()
		if err != nil {
			return output, fmt.Errorf("ssh exec: %w\n%s", err, output)
		}
		return output, nil
	}
}
