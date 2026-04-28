package transfer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"filehub/config"
)

type sinceResponse struct {
	Changes []config.Change `json:"changes"`
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// PullFile downloads a single file from url and writes it to destPath atomically.
func (c *Client) PullFile(url, destPath string) error {
	resp, err := c.http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull %s: status %d", url, resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(destPath), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, destPath)
}

// SincePull calls /since on baseURL, then pulls each changed file into destDir.
// Returns the list of changes that were successfully pulled.
func (c *Client) SincePull(baseURL, folderID, since, destDir string) ([]config.Change, error) {
	url := fmt.Sprintf("%s/since?folder=%s&t=%s", baseURL, folderID, since)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("since %s: status %d", url, resp.StatusCode)
	}

	var sr sinceResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	var pulled []config.Change
	for _, ch := range sr.Changes {
		if ch.Op == config.OpDelete {
			os.Remove(filepath.Join(destDir, filepath.FromSlash(ch.Path)))
			pulled = append(pulled, ch)
			continue
		}
		fileURL := fmt.Sprintf("%s/files/%s/%s", baseURL, folderID, ch.Path)
		dest := filepath.Join(destDir, filepath.FromSlash(ch.Path))
		if err := c.PullFile(fileURL, dest); err != nil {
			continue
		}
		pulled = append(pulled, ch)
	}
	return pulled, nil
}
