package scriptmanager

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	transportclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// GoGitBackend keeps managed repositories cross-platform without requiring a
// separately installed git executable.
type GoGitBackend struct{}

var installHTTPTransportOnce sync.Once

func installSecureHTTPTransport() {
	installHTTPTransportOnce.Do(func() {
		transportclient.InstallProtocol("https", githttp.NewClient(&http.Client{
			Transport:     http.DefaultTransport,
			CheckRedirect: secureRedirectPolicy,
		}))
	})
}

func secureRedirectPolicy(req *http.Request, via []*http.Request) error {
	if !strings.EqualFold(req.URL.Scheme, "https") {
		return fmt.Errorf("repository redirect downgraded from HTTPS")
	}
	if req.URL.User != nil {
		return fmt.Errorf("repository redirect contains credentials")
	}
	if len(via) >= 5 {
		return fmt.Errorf("repository exceeded the redirect limit")
	}
	return nil
}

func (GoGitBackend) Inspect(ctx context.Context, rawURL string) (RemoteInfo, error) {
	installSecureHTTPTransport()
	remote := git.NewRemote(memory.NewStorage(), &gitconfig.RemoteConfig{
		Name: "origin", URLs: []string{rawURL},
	})
	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return RemoteInfo{}, err
	}
	return remoteInfoFromRefs(refs), nil
}

func remoteInfoFromRefs(refs []*plumbing.Reference) RemoteInfo {
	info := RemoteInfo{Tags: map[string]string{}, Branches: map[string]string{}}
	peeledTags := make(map[string]string)
	for _, ref := range refs {
		name := ref.Name().String()
		switch {
		case ref.Name() == plumbing.HEAD && ref.Type() == plumbing.SymbolicReference:
			info.DefaultBranch = strings.TrimPrefix(ref.Target().String(), "refs/heads/")
		case ref.Name().IsBranch():
			info.Branches[ref.Name().Short()] = ref.Hash().String()
		case strings.HasPrefix(name, "refs/tags/") && strings.HasSuffix(name, "^{}"):
			peeledTags[strings.TrimSuffix(strings.TrimPrefix(name, "refs/tags/"), "^{}")] = ref.Hash().String()
		case ref.Name().IsTag():
			info.Tags[ref.Name().Short()] = ref.Hash().String()
		}
	}
	for name, commit := range peeledTags {
		info.Tags[name] = commit
	}
	return info
}

func (GoGitBackend) Checkout(ctx context.Context, req CheckoutRequest) (string, error) {
	installSecureHTTPTransport()
	reference := plumbing.NewBranchReferenceName(req.Ref)
	if req.Tracking == "tag" {
		reference = plumbing.NewTagReferenceName(req.Ref)
	}
	repo, err := git.PlainCloneContext(ctx, req.Destination, false, &git.CloneOptions{
		URL: req.URL, Depth: 1, SingleBranch: true, ReferenceName: reference,
		Tags: git.AllTags,
	})
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("reading installed revision: %w", err)
	}
	return head.Hash().String(), nil
}
