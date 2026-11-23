package auth

import (
	"regexp"
	"testing"
)

func TestGenerateCodeVerifier(t *testing.T) {
	// RFC 7636 Section 3: PKCE code verifier must match [a-zA-Z0-9-._~] and have no padding '='
	pkceRegex := regexp.MustCompile(`^[a-zA-Z0-9\-._~]+$`)

	for i := 0; i < 50; i++ {
		verifier := GenerateCodeVerifier()
		if !pkceRegex.MatchString(verifier) {
			t.Errorf("GenerateCodeVerifier() produced invalid characters: %s", verifier)
		}
		if len(verifier) < 43 || len(verifier) > 128 {
			t.Errorf("GenerateCodeVerifier() produced invalid length %d: %s", len(verifier), verifier)
		}
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	pkceRegex := regexp.MustCompile(`^[a-zA-Z0-9\-._~]+$`)

	verifier := GenerateCodeVerifier()
	challenge := generateCodeChallenge(verifier)

	if !pkceRegex.MatchString(challenge) {
		t.Errorf("generateCodeChallenge() produced invalid characters: %s", challenge)
	}
}
