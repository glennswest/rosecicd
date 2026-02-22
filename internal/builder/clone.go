package builder

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func Clone(spec *BuildSpec, workDir string) error {
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}

	args := []string{"clone", "--depth=1", "--branch", spec.Branch, spec.Repo, workDir}
	log.Printf("[clone] git %v", args)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	log.Printf("[clone] done")
	return nil
}
