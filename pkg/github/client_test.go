package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// resetToken clears the cached gh lookup and stubs gh with fn.
func resetToken(t *testing.T, fn func() string) {
	t.Helper()
	old := ghAuthToken
	ghAuthToken = fn
	ghTokenOnce = sync.Once{}
	ghToken = ""
	t.Cleanup(func() {
		ghAuthToken = old
		ghTokenOnce = sync.Once{}
		ghToken = ""
	})
}

func TestTokenPrefersEnv(t *testing.T) {
	resetToken(t, func() string { t.Fatal("gh should not be called"); return "" })
	t.Setenv("GITHUB_TOKEN", "a")
	t.Setenv("GH_TOKEN", "b")
	if got := Token(); got != "a" {
		t.Fatalf("Token() = %q, want a", got)
	}
	t.Setenv("GITHUB_TOKEN", "")
	if got := Token(); got != "b" {
		t.Fatalf("Token() = %q, want b", got)
	}
}

func TestTokenFallsBackToGhOnce(t *testing.T) {
	calls := 0
	resetToken(t, func() string { calls++; return "from-gh" })
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	for i := 0; i < 3; i++ {
		if got := Token(); got != "from-gh" {
			t.Fatalf("Token() = %q, want from-gh", got)
		}
	}
	if calls != 1 {
		t.Fatalf("gh called %d times, want 1", calls)
	}
}

func serve(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	old := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = old; srv.Close() })
}

func TestGetLatestReleaseSendsGhToken(t *testing.T) {
	resetToken(t, func() string { return "tok" })
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/repos/o/r/releases/latest" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"tag_name":"v1"}`))
	})
	rel, err := GetLatestRelease("https://github.com/o/r")
	if err != nil || rel.TagName != "v1" {
		t.Fatalf("rel=%v err=%v", rel, err)
	}
}

func TestGetLatestReleaseRateLimited(t *testing.T) {
	reset := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	for _, tc := range []struct {
		name   string
		token  string
		status int
		hint   bool
	}{
		{"unauthenticated 403", "", http.StatusForbidden, true},
		{"authenticated 429", "tok", http.StatusTooManyRequests, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetToken(t, func() string { return "" })
			t.Setenv("GITHUB_TOKEN", tc.token)
			t.Setenv("GH_TOKEN", "")
			serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
				w.WriteHeader(tc.status)
			})
			_, err := GetLatestRelease("https://github.com/o/r")
			var rl *RateLimitError
			if !errors.As(err, &rl) {
				t.Fatalf("err = %v, want *RateLimitError", err)
			}
			if !rl.Reset.Equal(reset) {
				t.Errorf("Reset = %v, want %v", rl.Reset, reset)
			}
			msg := err.Error()
			if !strings.Contains(msg, "resets at "+reset.Local().Format("15:04")) {
				t.Errorf("message missing reset time: %q", msg)
			}
			if got := strings.Contains(msg, "GH_TOKEN"); got != tc.hint {
				t.Errorf("token hint present = %v, want %v: %q", got, tc.hint, msg)
			}
		})
	}
}

func TestGetLatestReleaseSecondaryLimit(t *testing.T) {
	resetToken(t, func() string { return "" })
	t.Setenv("GITHUB_TOKEN", "tok")
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := GetLatestRelease("https://github.com/o/r")
	var rl *RateLimitError
	if !errors.As(err, &rl) || rl.Reset.IsZero() {
		t.Fatalf("err = %v, want *RateLimitError with reset", err)
	}
}

func TestGetLatestReleasePlain403(t *testing.T) {
	resetToken(t, func() string { return "" })
	t.Setenv("GITHUB_TOKEN", "tok")
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "4000")
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := GetLatestRelease("https://github.com/o/r")
	var rl *RateLimitError
	if errors.As(err, &rl) {
		t.Fatalf("plain 403 reported as rate limit: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want status 403", err)
	}
}
