// Package scriptmanager installs and updates Git-backed Lua script sources in
// Praetor-owned storage. It deliberately has no runtime authority: callers
// decide when a validated checkout may enter the active Lua source graph.
package scriptmanager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appconfig "github.com/cyber-godzilla/praetor/internal/config"
)

const (
	SuggestedRepositoryURL = "https://github.com/cyber-godzilla/praetor-scripts.git"
	operationTimeout       = 5 * time.Minute
	maxCheckoutBytes       = int64(250 << 20)
	maxCheckoutFiles       = 10_000
)

type RemoteInfo struct {
	Tags          map[string]string
	Branches      map[string]string
	DefaultBranch string
}

type Revision struct {
	Tracking string `json:"tracking"`
	Ref      string `json:"ref"`
	Commit   string `json:"commit"`
}

type CheckoutRequest struct {
	URL         string
	Tracking    string
	Ref         string
	Destination string
}

type GitBackend interface {
	Inspect(context.Context, string) (RemoteInfo, error)
	Checkout(context.Context, CheckoutRequest) (string, error)
}

type Manager struct {
	root    string
	backend GitBackend
}

type InstallRequest struct {
	URL            string
	Name           string
	Suggested      bool
	CheckOnStartup bool
}

type UpdateStatus struct {
	Available bool   `json:"available"`
	Updated   bool   `json:"updated"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Tracking  string `json:"tracking"`
	Ref       string `json:"ref"`
	Error     string `json:"error,omitempty"`
}

func New(root string) *Manager { return NewWithBackend(root, GoGitBackend{}) }

func NewWithBackend(root string, backend GitBackend) *Manager {
	return &Manager{root: filepath.Clean(root), backend: backend}
}

func (m *Manager) Path(id string) string { return filepath.Join(m.root, id) }

func ValidateRepositoryURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("repository URL must be an absolute HTTPS URL")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("only HTTPS Git repositories are supported")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("repository URLs must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("repository URLs must not contain a query or fragment")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return "", fmt.Errorf("repository URL must include a repository path")
	}
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func SelectRevision(info RemoteInfo) (Revision, error) {
	best := ""
	for tag := range info.Tags {
		if _, ok := stableSemver(tag); !ok {
			continue
		}
		if best == "" || compareStableSemver(tag, best) > 0 {
			best = tag
		}
	}
	if best != "" {
		return Revision{Tracking: appconfig.ScriptTrackingTag, Ref: best, Commit: info.Tags[best]}, nil
	}
	branch := info.DefaultBranch
	if branch == "" {
		if _, ok := info.Branches["main"]; ok {
			branch = "main"
		} else if _, ok := info.Branches["master"]; ok {
			branch = "master"
		} else if len(info.Branches) == 1 {
			for name := range info.Branches {
				branch = name
			}
		}
	}
	commit, ok := info.Branches[branch]
	if branch == "" || !ok {
		return Revision{}, errors.New("repository has no stable semantic-version tag or discoverable default branch")
	}
	return Revision{Tracking: appconfig.ScriptTrackingBranch, Ref: branch, Commit: commit}, nil
}

func (m *Manager) Install(ctx context.Context, req InstallRequest) (appconfig.ManagedScriptSourceConfig, error) {
	canonical, err := ValidateRepositoryURL(req.URL)
	if err != nil {
		return appconfig.ManagedScriptSourceConfig{}, err
	}
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	info, err := m.backend.Inspect(ctx, canonical)
	if err != nil {
		return appconfig.ManagedScriptSourceConfig{}, fmt.Errorf("inspecting repository: %w", err)
	}
	revision, err := SelectRevision(info)
	if err != nil {
		return appconfig.ManagedScriptSourceConfig{}, err
	}
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return appconfig.ManagedScriptSourceConfig{}, fmt.Errorf("creating managed script storage: %w", err)
	}
	staging, err := os.MkdirTemp(m.root, ".install-")
	if err != nil {
		return appconfig.ManagedScriptSourceConfig{}, fmt.Errorf("creating install staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	commit, err := m.backend.Checkout(ctx, CheckoutRequest{
		URL: canonical, Tracking: revision.Tracking, Ref: revision.Ref, Destination: staging,
	})
	if err != nil {
		return appconfig.ManagedScriptSourceConfig{}, fmt.Errorf("downloading repository: %w", err)
	}
	if err := validateCheckout(staging); err != nil {
		return appconfig.ManagedScriptSourceConfig{}, err
	}
	id := appconfig.NewUUID()
	if err := os.Rename(staging, m.Path(id)); err != nil {
		return appconfig.ManagedScriptSourceConfig{}, fmt.Errorf("committing managed scripts: %w", err)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = repositoryName(canonical)
	}
	version := ""
	if revision.Tracking == appconfig.ScriptTrackingTag {
		version = revision.Ref
	}
	return appconfig.ManagedScriptSourceConfig{
		ID: id, Name: name, URL: canonical, Enabled: false,
		Trust: appconfig.ScriptTrustRestricted, CheckOnStartup: req.CheckOnStartup,
		Tracking: revision.Tracking, Ref: revision.Ref,
		InstalledCommit: commit, InstalledVersion: version,
	}, nil
}

func (m *Manager) Check(ctx context.Context, source appconfig.ManagedScriptSourceConfig) (UpdateStatus, error) {
	canonical, err := ValidateRepositoryURL(source.URL)
	if err != nil {
		return UpdateStatus{}, err
	}
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	info, err := m.backend.Inspect(ctx, canonical)
	if err != nil {
		return UpdateStatus{}, err
	}
	revision, err := SelectRevision(info)
	if err != nil {
		return UpdateStatus{}, err
	}
	status := UpdateStatus{
		Commit: revision.Commit, Tracking: revision.Tracking, Ref: revision.Ref,
		Available: revision.Commit != "" && revision.Commit != source.InstalledCommit,
	}
	if revision.Tracking == appconfig.ScriptTrackingTag {
		status.Version = revision.Ref
		status.Available = compareStableSemver(revision.Ref, source.InstalledVersion) > 0
	}
	return status, nil
}

// Update replaces a disabled managed checkout with the latest selected
// revision. The caller commits the accompanying configuration while the old
// checkout is still available for rollback.
func (m *Manager) Update(ctx context.Context, source appconfig.ManagedScriptSourceConfig, commitConfig func(appconfig.ManagedScriptSourceConfig) error) (appconfig.ManagedScriptSourceConfig, UpdateStatus, error) {
	if source.Enabled {
		return source, UpdateStatus{}, errors.New("stop automation and disable this source before updating it")
	}
	status, err := m.Check(ctx, source)
	if err != nil || !status.Available {
		return source, status, err
	}
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	staging, err := os.MkdirTemp(m.root, ".update-")
	if err != nil {
		return source, status, err
	}
	defer os.RemoveAll(staging)
	commit, err := m.backend.Checkout(ctx, CheckoutRequest{URL: source.URL, Tracking: status.Tracking, Ref: status.Ref, Destination: staging})
	if err != nil {
		return source, status, fmt.Errorf("downloading update: %w", err)
	}
	if err := validateCheckout(staging); err != nil {
		return source, status, err
	}
	target := m.Path(source.ID)
	backup := target + ".rollback"
	_ = os.RemoveAll(backup)
	if err := os.Rename(target, backup); err != nil {
		return source, status, fmt.Errorf("staging installed version for rollback: %w", err)
	}
	rolledBack := false
	rollback := func() {
		if rolledBack {
			return
		}
		rolledBack = true
		_ = os.RemoveAll(target)
		_ = os.Rename(backup, target)
	}
	if err := os.Rename(staging, target); err != nil {
		rollback()
		return source, status, fmt.Errorf("activating downloaded update: %w", err)
	}
	updated := source
	updated.Tracking, updated.Ref, updated.InstalledCommit = status.Tracking, status.Ref, commit
	updated.InstalledVersion = ""
	if status.Tracking == appconfig.ScriptTrackingTag {
		updated.InstalledVersion = status.Version
	}
	if err := commitConfig(updated); err != nil {
		rollback()
		return source, status, err
	}
	rolledBack = true
	if err := os.RemoveAll(backup); err != nil {
		return updated, status, fmt.Errorf("removing update rollback data: %w", err)
	}
	return updated, status, nil
}

func (m *Manager) Remove(source appconfig.ManagedScriptSourceConfig) error {
	if source.Enabled {
		return errors.New("disable this source before removing it")
	}
	if !appconfig.ValidUUID(source.ID) {
		return errors.New("invalid managed source ID")
	}
	return os.RemoveAll(m.Path(source.ID))
}

func boundedContext(parent context.Context) (context.Context, context.CancelFunc) {
	if _, ok := parent.Deadline(); ok {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, operationTimeout)
}

func repositoryName(raw string) string {
	parsed, _ := url.Parse(raw)
	name := strings.TrimSuffix(filepath.Base(parsed.Path), ".git")
	if name == "." || name == "" || name == "/" {
		return "Scripts"
	}
	return name
}

func validateCheckout(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("reading downloaded repository: %w", err)
	}
	hasMode := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".lua") {
			hasMode = true
			break
		}
	}
	if !hasMode {
		return errors.New("repository root contains no Lua files")
	}
	var files int
	var bytes int64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("repository contains unsupported symbolic link %q", strings.TrimPrefix(path, root+string(filepath.Separator)))
		}
		if entry.IsDir() {
			return nil
		}
		files++
		if files > maxCheckoutFiles {
			return fmt.Errorf("repository contains more than %d files", maxCheckoutFiles)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		bytes += info.Size()
		if bytes > maxCheckoutBytes {
			return fmt.Errorf("repository checkout exceeds %d MiB", maxCheckoutBytes>>20)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("validating repository: %w", err)
	}
	return nil
}

func stableSemver(raw string) ([]int, bool) {
	value := strings.TrimPrefix(raw, "v")
	if value == "" || strings.ContainsAny(value, "-+") {
		return nil, false
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, false
	}
	result := make([]int, 3)
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return nil, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		result[i] = n
	}
	return result, true
}

func compareStableSemver(a, b string) int {
	av, aok := stableSemver(a)
	bv, bok := stableSemver(b)
	if !aok {
		if bok {
			return -1
		}
		return 0
	}
	if !bok {
		return 1
	}
	for i := range av {
		if av[i] < bv[i] {
			return -1
		}
		if av[i] > bv[i] {
			return 1
		}
	}
	return 0
}
