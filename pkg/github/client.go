package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// apiBase is the GitHub API root. Tests point it at a local server.
var apiBase = "https://api.github.com"

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
// Otherwise it asks the GitHub CLI (`gh auth token`), so anyone already
// logged in to gh gets the authenticated rate limit with no setup.
// The gh lookup runs at most once per process.
func Token() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}
	ghTokenOnce.Do(func() { ghToken = ghAuthToken() })
	return ghToken
}

var (
	ghTokenOnce sync.Once
	ghToken     string
	// ghAuthToken is replaceable in tests.
	ghAuthToken = func() string {
		path, err := exec.LookPath("gh")
		if err != nil {
			return ""
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, "auth", "token", "--hostname", "github.com")
		cmd.Stdin = nil
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
)

// RateLimitError means GitHub refused the request because the API rate
// limit is used up.
type RateLimitError struct {
	Reset         time.Time // zero if GitHub didn't say
	Authenticated bool
}

func (e *RateLimitError) Error() string {
	msg := "GitHub API rate limit reached"
	if !e.Reset.IsZero() {
		msg += ", resets at " + e.Reset.Local().Format("15:04")
	}
	if !e.Authenticated {
		msg += "; set GH_TOKEN or run `gh auth login` to raise the limit from 60 to 5000 requests per hour"
	}
	return msg
}

// rateLimitError returns a *RateLimitError if resp is a rate-limit refusal.
// GitHub uses 403 or 429, with X-RateLimit-Remaining: 0 for the primary
// limit or Retry-After for the secondary one.
func rateLimitError(resp *http.Response, authenticated bool) error {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return nil
	}
	e := &RateLimitError{Authenticated: authenticated}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if sec, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			e.Reset = time.Unix(sec, 0)
		}
		return e
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if sec, err := strconv.Atoi(ra); err == nil {
			e.Reset = time.Now().Add(time.Duration(sec) * time.Second)
		}
		return e
	}
	return nil
}

// GetLatestRelease fetches the latest release info for a github repo url
// url format: https://github.com/owner/repo
func GetLatestRelease(repoURL string) (*Release, error) {
	parts := strings.Split(repoURL, "github.com/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid github url")
	}
	repoPath := strings.TrimSuffix(parts[1], "/")

	apiURL := fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, repoPath)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	// Authenticated requests get 5000/hour instead of 60/hour
	token := Token()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := rateLimitError(resp, token != ""); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	return &rel, nil
}
