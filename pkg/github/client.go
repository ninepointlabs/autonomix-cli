package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int    `json:"size"`
}

type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
	Body    string  `json:"body"`
	HTMLURL string  `json:"html_url"`
}

// Token returns the GitHub token from GITHUB_TOKEN or GH_TOKEN, if set.
func Token() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GH_TOKEN")
}

// GetLatestRelease fetches the latest release info for a github repo url
// url format: https://github.com/owner/repo
func GetLatestRelease(repoURL string) (*Release, error) {
	parts := strings.Split(repoURL, "github.com/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid github url")
	}
	repoPath := strings.TrimSuffix(parts[1], "/")
	
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoPath)
	
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	// Authenticated requests get 5000/hour instead of 60/hour
	if token := Token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	return &rel, nil
}
