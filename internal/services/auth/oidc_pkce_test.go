package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc7636CodeVerifierRe matches the unreserved character set allowed in a
// PKCE code_verifier: [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~".
var rfc7636CodeVerifierRe = regexp.MustCompile(`^[A-Za-z0-9\-._~]+$`)

// TestGenerateCodeVerifier_RFC7636 guards the PKCE code_verifier contract.
//
// A padded base64 verifier (base64.URLEncoding) ends with "=", which is
// outside the RFC 7636 unreserved set. Strict providers such as Pocket ID
// reject it at the token endpoint with "The PKCE code verifier must only
// contain [a-Z], [0-9], '-', '.', '_', '~'.", which breaks every OIDC login.
func TestGenerateCodeVerifier_RFC7636(t *testing.T) {
	for i := 0; i < 50; i++ {
		verifier := generateCodeVerifier()

		assert.Regexp(t, rfc7636CodeVerifierRe, verifier,
			"code_verifier must only use the RFC 7636 unreserved set")
		assert.NotContains(t, verifier, "=",
			"code_verifier must not be base64 padded")
		assert.GreaterOrEqual(t, len(verifier), 43,
			"code_verifier must be at least 43 characters")
		assert.LessOrEqual(t, len(verifier), 128,
			"code_verifier must be at most 128 characters")
	}
}

// TestGenerateCodeVerifier_Unique ensures the verifier is random enough that
// two consecutive logins never reuse the same one.
func TestGenerateCodeVerifier_Unique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		v := generateCodeVerifier()
		_, dup := seen[v]
		require.False(t, dup, "code_verifier repeated: %s", v)
		seen[v] = struct{}{}
	}
}

// TestGenerateCodeChallenge_MatchesRFC7636 checks the S256 challenge is the
// unpadded base64url of SHA-256(verifier).
func TestGenerateCodeChallenge_MatchesRFC7636(t *testing.T) {
	verifier := generateCodeVerifier()
	challenge := generateCodeChallenge(verifier)

	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])

	assert.Equal(t, expected, challenge)
	assert.NotContains(t, challenge, "=", "code_challenge must not be base64 padded")
	assert.Regexp(t, rfc7636CodeVerifierRe, challenge)
	assert.NotEqual(t, verifier, challenge)
}

// TestGenerateCodeChallenge_Deterministic ensures the challenge is stable for
// a given verifier, otherwise the token exchange can never match.
func TestGenerateCodeChallenge_Deterministic(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	assert.Equal(t, generateCodeChallenge(verifier), generateCodeChallenge(verifier))
}

// TestGenerateCodeVerifier_ExportedMatchesInternal makes sure the exported
// helper used by the handlers produces the same RFC 7636 compliant output.
func TestGenerateCodeVerifier_ExportedMatchesInternal(t *testing.T) {
	v := GenerateCodeVerifier()
	assert.Regexp(t, rfc7636CodeVerifierRe, v)
	assert.NotContains(t, v, "=")
	assert.GreaterOrEqual(t, len(v), 43)
}
