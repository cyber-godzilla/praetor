// Package update checks GitHub Releases for a newer praetor version. The
// check is a single anonymous GET with a caller-supplied timeout; any failure
// is returned as an error for the caller to log and otherwise ignore — an
// update check must never break startup.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// DefaultEndpoint is the GitHub "latest release" API for the praetor repo.
// This endpoint never returns drafts or prereleases.
const DefaultEndpoint = "https://api.github.com/repos/cyber-godzilla/praetor/releases/latest"

// Info is the outcome of a check, shaped for direct JSON delivery to the GUI.
type Info struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	URL       string `json:"url"`
}

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

var semverRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

// parseSemver extracts major.minor.patch from "0.2.7" or "v0.2.7"; trailing
// suffixes ("-rc1") are ignored. ok is false for anything else (e.g. "dev").
func parseSemver(s string) (v [3]int, ok bool) {
	m := semverRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return v, false
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func newer(latest, current [3]int) bool {
	for i := 0; i < 3; i++ {
		if latest[i] != current[i] {
			return latest[i] > current[i]
		}
	}
	return false
}

// Check fetches the latest release from endpoint and compares it against
// current. A current version that is not semver (e.g. a "dev" build) skips
// the network entirely and reports no update available.
func Check(ctx context.Context, client *http.Client, endpoint, current string) (Info, error) {
	info := Info{Current: current}
	cur, ok := parseSemver(current)
	if !ok {
		return info, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("update check: unexpected status %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return info, fmt.Errorf("update check: decoding response: %w", err)
	}
	latest, ok := parseSemver(rel.TagName)
	if !ok {
		return info, fmt.Errorf("update check: unparseable release tag %q", rel.TagName)
	}

	info.Latest = strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info.URL = rel.HTMLURL
	info.Available = newer(latest, cur)
	return info, nil
}
