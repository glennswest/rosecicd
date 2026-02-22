package builder

import (
	"log"
	"os"
)

const (
	DefaultSpecPath = "/etc/rosecicd/build-spec.json"
	DefaultWorkDir  = "/tmp/build"
)

func Run(specPath string) int {
	if specPath == "" {
		specPath = DefaultSpecPath
	}

	log.Printf("=== rosecicd-builder starting ===")
	log.Printf("spec: %s", specPath)

	spec, err := LoadSpec(specPath)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return 1
	}
	log.Printf("repo: %s branch: %s image: %s", spec.Repo, spec.Branch, spec.ImageName)

	workDir := DefaultWorkDir
	defer os.RemoveAll(workDir)

	// Clone
	if err := Clone(spec, workDir); err != nil {
		log.Printf("ERROR: %v", err)
		return 1
	}

	// Build
	if err := Build(spec, workDir); err != nil {
		log.Printf("ERROR: %v", err)
		return 1
	}

	// Push
	if err := Push(spec); err != nil {
		log.Printf("ERROR: %v", err)
		return 1
	}

	log.Printf("=== build complete ===")
	return 0
}
