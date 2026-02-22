package buildmgr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/glennswest/rosecicd/internal/config"
)

// mkube pod spec structures
type podSpec struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Metadata   podMetadata  `json:"metadata"`
	Spec       podInnerSpec `json:"spec"`
}

type podMetadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

type podInnerSpec struct {
	RestartPolicy string      `json:"restartPolicy"`
	Containers    []container `json:"containers"`
	Volumes       []volume    `json:"volumes,omitempty"`
}

type container struct {
	Name         string        `json:"name"`
	Image        string        `json:"image"`
	Command      []string      `json:"command,omitempty"`
	Args         []string      `json:"args,omitempty"`
	VolumeMounts []volumeMount `json:"volumeMounts,omitempty"`
}

type volume struct {
	Name     string            `json:"name"`
	HostPath *hostPathVolume   `json:"hostPath,omitempty"`
}

type hostPathVolume struct {
	Path string `json:"path"`
}

type volumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

func CreateBuildPod(cfg *config.Config, podName, specJSON string) error {
	// Write build spec to cache dir so pod can read it
	specPath := fmt.Sprintf("%s/%s.json", cfg.Build.CacheDir, podName)
	if err := writeSpecFile(cfg, specPath, specJSON); err != nil {
		return err
	}

	pod := podSpec{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: podMetadata{
			Name:      podName,
			Namespace: "default",
			Labels: map[string]string{
				"app":                  "rosecicd-builder",
				"rosecicd.dev/builder": "true",
			},
		},
		Spec: podInnerSpec{
			RestartPolicy: "Never",
			Containers: []container{
				{
					Name:  "builder",
					Image: cfg.Build.BuilderImage,
					Args:  []string{"/etc/rosecicd/build-spec.json"},
					VolumeMounts: []volumeMount{
						{Name: "build-spec", MountPath: "/etc/rosecicd"},
					},
				},
			},
			Volumes: []volume{
				{
					Name: "build-spec",
					HostPath: &hostPathVolume{
						Path: cfg.Build.CacheDir,
					},
				},
			},
		},
	}

	body, err := json.Marshal(pod)
	if err != nil {
		return fmt.Errorf("marshal pod: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/namespaces/default/pods", cfg.Mkube.APIURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create pod: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create pod: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func writeSpecFile(_ *config.Config, path, specJSON string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create spec dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(specJSON), 0644); err != nil {
		return fmt.Errorf("write spec file: %w", err)
	}
	return nil
}

func WaitForPod(cfg *config.Config, podName string, stopCh <-chan struct{}) (string, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces/default/pods/%s", cfg.Mkube.APIURL, podName)
	client := &http.Client{Timeout: 10 * time.Second}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute)

	for {
		select {
		case <-stopCh:
			return "", fmt.Errorf("stopped")
		case <-timeout:
			return "", fmt.Errorf("build timed out after 30m")
		case <-ticker.C:
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()

			status, ok := result["status"].(map[string]interface{})
			if !ok {
				continue
			}
			phase, _ := status["phase"].(string)
			if phase == "Succeeded" || phase == "Failed" {
				return phase, nil
			}
		}
	}
}

func DeletePod(cfg *config.Config, podName string) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/default/pods/%s", cfg.Mkube.APIURL, podName)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
