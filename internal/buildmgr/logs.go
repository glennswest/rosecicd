package buildmgr

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/glennswest/rosecicd/internal/config"
)

func GetPodLogs(cfg *config.Config, podName string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces/default/pods/%s/log", cfg.Mkube.APIURL, podName)
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read logs: %w", err)
	}
	return string(body), nil
}
