package appmeta

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UpdateCheckResult struct {
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	ReleaseURL     string    `json:"release_url"`
	HasUpdate      bool      `json:"has_update"`
	CheckedAt      time.Time `json:"checked_at"`
	Source         string    `json:"source"`
}

type UpdateChecker struct {
	client *http.Client

	mu          sync.RWMutex
	cached      *UpdateCheckResult
	lastRequest time.Time
	nextAllowed time.Time
}

func NewUpdateChecker() *UpdateChecker {
	return &UpdateChecker{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *UpdateChecker) LastResult() *UpdateCheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cached == nil {
		return nil
	}
	cp := *c.cached
	return &cp
}

func (c *UpdateChecker) Check(ctx context.Context, currentVersion string) (*UpdateCheckResult, error) {
	now := time.Now().UTC()

	c.mu.RLock()
	nextAllowed := c.nextAllowed
	lastRequest := c.lastRequest
	cached := c.cached
	c.mu.RUnlock()

	if cached != nil && now.Sub(lastRequest) < 30*time.Minute {
		cp := *cached
		return &cp, nil
	}
	if now.Before(nextAllowed) && cached != nil {
		cp := *cached
		return &cp, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPIReleases, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "berkut-scc/"+currentVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return c.LastResult(), err
	}
	defer resp.Body.Close()

	type releasePayload struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
	}
	var payload releasePayload
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return c.LastResult(), err
		}
	} else {
		c.setNextAllowedFromHeaders(resp.Header, now)
		last := c.LastResult()
		if last != nil {
			return last, nil
		}
		fallback := &UpdateCheckResult{
			CurrentVersion: currentVersion,
			LatestVersion:  currentVersion,
			ReleaseURL:     RepositoryURL,
			HasUpdate:      false,
			CheckedAt:      now,
			Source:         "github_unavailable",
		}
		c.mu.Lock()
		c.cached = fallback
		c.lastRequest = now
		c.mu.Unlock()
		return c.LastResult(), nil
	}

	latest := normalizeVersion(payload.TagName)
	current := normalizeVersion(currentVersion)
	result := &UpdateCheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  payload.TagName,
		ReleaseURL:     payload.HTMLURL,
		HasUpdate:      compareSemver(latest, current) > 0,
		CheckedAt:      now,
		Source:         "github",
	}
	c.mu.Lock()
	c.cached = result
	c.lastRequest = now
	c.nextAllowed = now
	c.mu.Unlock()
	return c.LastResult(), nil
}

func (c *UpdateChecker) setNextAllowedFromHeaders(headers http.Header, now time.Time) {
	retryAfter := strings.TrimSpace(headers.Get("Retry-After"))
	if retryAfter != "" {
		if sec, err := strconv.Atoi(retryAfter); err == nil && sec > 0 {
			c.mu.Lock()
			c.nextAllowed = now.Add(time.Duration(sec) * time.Second)
			c.lastRequest = now
			c.mu.Unlock()
			return
		}
	}
	reset := strings.TrimSpace(headers.Get("X-RateLimit-Reset"))
	if reset != "" {
		if unixSec, err := strconv.ParseInt(reset, 10, 64); err == nil && unixSec > 0 {
			next := time.Unix(unixSec, 0).UTC()
			c.mu.Lock()
			c.nextAllowed = next
			c.lastRequest = now
			c.mu.Unlock()
			return
		}
	}
	c.mu.Lock()
	c.nextAllowed = now.Add(15 * time.Minute)
	c.lastRequest = now
	c.mu.Unlock()
}

func normalizeVersion(v string) string {
	val := strings.TrimSpace(strings.ToLower(v))
	val = strings.TrimPrefix(val, "v")
	return val
}

func compareSemver(a, b string) int {
	parse := func(v string) [3]int {
		var out [3]int
		parts := strings.Split(v, ".")
		for i := 0; i < len(parts) && i < 3; i++ {
			n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
			out[i] = n
		}
		return out
	}
	av := parse(a)
	bv := parse(b)
	for i := 0; i < 3; i++ {
		if av[i] > bv[i] {
			return 1
		}
		if av[i] < bv[i] {
			return -1
		}
	}
	return 0
}
