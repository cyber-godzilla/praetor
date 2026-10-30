package scriptmanager

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestRemoteInfoUsesPeeledAnnotatedTagCommit(t *testing.T) {
	tagObject := plumbing.NewHash("1111111111111111111111111111111111111111")
	commit := plumbing.NewHash("2222222222222222222222222222222222222222")
	info := remoteInfoFromRefs([]*plumbing.Reference{
		plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v1.2.3"), tagObject),
		plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v1.2.3^{}"), commit),
	})
	if got := info.Tags["v1.2.3"]; got != commit.String() {
		t.Fatalf("tag commit = %q, want peeled commit %q", got, commit)
	}
}

func TestSecureRedirectPolicyRejectsDowngradesCredentialsAndLongChains(t *testing.T) {
	request := func(raw string) *http.Request {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Request{URL: parsed}
	}
	if err := secureRedirectPolicy(request("https://example.test/repo.git"), nil); err != nil {
		t.Fatalf("safe redirect rejected: %v", err)
	}
	if err := secureRedirectPolicy(request("http://example.test/repo.git"), nil); err == nil {
		t.Fatal("HTTPS downgrade accepted")
	}
	if err := secureRedirectPolicy(request("https://user:secret@example.test/repo.git"), nil); err == nil {
		t.Fatal("credential redirect accepted")
	}
	if err := secureRedirectPolicy(request("https://example.test/repo.git"), make([]*http.Request, 5)); err == nil {
		t.Fatal("long redirect chain accepted")
	}
}
