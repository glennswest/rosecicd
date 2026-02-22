package builder

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func Build(spec *BuildSpec, workDir string) error {
	localReg := spec.Registries["local"]
	fullTag := fmt.Sprintf("%s/%s:%s", localReg.URL, spec.ImageName, spec.Tags[0])

	args := []string{
		"bud",
		"--isolation", "chroot",
		"--storage-driver", "vfs",
		"-f", spec.Dockerfile,
		"-t", fullTag,
		".",
	}
	log.Printf("[build] buildah %v", args)

	cmd := exec.Command("buildah", args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buildah bud: %w", err)
	}

	log.Printf("[build] image built: %s", fullTag)
	return nil
}

func Push(spec *BuildSpec) error {
	localReg := spec.Registries["local"]
	localTag := fmt.Sprintf("%s/%s:%s", localReg.URL, spec.ImageName, spec.Tags[0])

	// Push to local registry
	if err := pushImage(localTag, localTag, localReg.Insecure, "", ""); err != nil {
		return fmt.Errorf("push to local: %w", err)
	}

	// Push to GHCR
	if ghcr, ok := spec.Registries["ghcr"]; ok {
		ghcrTag := fmt.Sprintf("%s/%s:%s", ghcr.URL, spec.ImageName, spec.Tags[0])
		if err := pushImage(localTag, ghcrTag, false, ghcr.User, ghcr.Token); err != nil {
			return fmt.Errorf("push to ghcr: %w", err)
		}
	}

	return nil
}

func pushImage(srcTag, destTag string, insecure bool, user, token string) error {
	args := []string{"push", "--storage-driver", "vfs"}
	if insecure {
		args = append(args, "--tls-verify=false")
	}
	if user != "" && token != "" {
		args = append(args, "--creds", user+":"+token)
	}
	args = append(args, srcTag, "docker://"+destTag)

	log.Printf("[push] buildah %v", args)

	cmd := exec.Command("buildah", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buildah push %s: %w", destTag, err)
	}
	log.Printf("[push] pushed: %s", destTag)
	return nil
}
