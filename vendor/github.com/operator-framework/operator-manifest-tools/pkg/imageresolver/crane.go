package imageresolver

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

var _ ImageResolver = CraneResolver{}

// DefaultResolver uses the containers series of libraries to resolve image digests
type CraneResolver struct {
	authenticator authn.Authenticator
	useDefault    bool
}

// CraneOption is a function that configures the `CraneResolver`
type CraneOption func(*CraneResolver)

// WithUserPassAuth returns a CraneOption that adds an authenticator
func WithUserPassAuth(username, password string) CraneOption {
	return func(res *CraneResolver) {
		res.authenticator = &authn.Basic{Username: username, Password: password}
	}
}

// WithDefaultAuth returns a CraneOption that sets the auth to the default keychain
func WithDefaultKeychain() CraneOption {
	return func(res *CraneResolver) {
		res.useDefault = true
	}
}

// NewCraneResolver returns a CraneResolver with the applied options.
func NewCraneResolver(opts ...CraneOption) CraneResolver {
	res := CraneResolver{authenticator: authn.Anonymous}
	for _, opt := range opts {
		opt(&res)
	}

	return res
}

func (res CraneResolver) ResolveImageReference(imageReference string) (string, error) {
	var digest string
	var err error
	if res.useDefault {
		digest, err = crane.Digest(imageReference)
	} else {
		digest, err = crane.Digest(imageReference, crane.WithAuth(res.authenticator))
	}
	if err != nil {
		return "", err
	}

	name, err := getName(imageReference)
	if err != nil {
		return "", err
	}

	return name + "@" + digest, nil
}
