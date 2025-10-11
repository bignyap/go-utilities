package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/jwk"
)

func TestExtractRealmFromPath(t *testing.T) {
	tests := []struct {
		path    string
		want    string
		wantErr bool
	}{
		{"/realms/dev", "dev", false},
		{"/auth/realms/dev", "dev", false},
		{"/prefix/auth/realms/staging/extra", "staging", false},
		{"/no/realm/here", "", true},
	}

	for _, tt := range tests {
		got, err := extractRealmFromPath(tt.path)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for path %q, got none (realm=%q)", tt.path, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for path %q: %v", tt.path, err)
		}
		if got != tt.want {
			t.Fatalf("realm mismatch for %q: got %q want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer abc.def.ghi")

	tok, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "abc.def.ghi" {
		t.Fatalf("token mismatch: got %q", tok)
	}

	// bad formats
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := ExtractToken(req2); err == nil {
		t.Fatalf("expected error for missing header")
	}
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Header.Set("Authorization", "Bearer")
	if _, err := ExtractToken(req3); err == nil {
		t.Fatalf("expected error for malformed header")
	}
}

func TestParseAndVerifyJWT_Success(t *testing.T) {
	// fresh cache per test
	if certCache != nil {
		certCache.Flush()
	}

	// RSA keypair
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Build JWKS with public key
	pubJWK, err := jwk.New(&priv.PublicKey)
	if err != nil {
		t.Fatalf("failed to build JWK: %v", err)
	}
	if err := pubJWK.Set(jwk.KeyIDKey, "kid1"); err != nil {
		t.Fatalf("failed to set kid: %v", err)
	}
	set := jwk.NewSet()
	set.Add(pubJWK)
	jwksJSON, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("failed to marshal jwks: %v", err)
	}

	// JWKS server
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/dev/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON)
	})
	// Anything else -> 404
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	issuer := srv.URL + "/realms/dev"

	// ENV used to validate host
	t.Setenv("AUTH_URL", srv.URL)

	claims := jwtlib.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	token.Header["kid"] = "kid1"
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	got, err := ParseAndVerifyJWT(signed)
	if err != nil {
		t.Fatalf("unexpected error verifying token: %v", err)
	}
	if got["realm"] != "dev" {
		t.Fatalf("expected realm=dev, got %v", got["realm"])
	}
	if got["iss"] != issuer {
		t.Fatalf("issuer mismatch: got %v want %v", got["iss"], issuer)
	}
}

func TestParseAndVerifyJWT_UnknownKID(t *testing.T) {
	if certCache != nil {
		certCache.Flush()
	}

	// RSA keypair
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// JWKS with different kid
	pubJWK, _ := jwk.New(&priv.PublicKey)
	_ = pubJWK.Set(jwk.KeyIDKey, "otherkid")
	set := jwk.NewSet()
	set.Add(pubJWK)
	jwksJSON, _ := json.Marshal(set)

	// Server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/certs") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(jwksJSON)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	issuer := srv.URL + "/realms/dev"
	t.Setenv("AUTH_URL", srv.URL)

	claims := jwtlib.MapClaims{"iss": issuer, "exp": time.Now().Add(time.Hour).Unix()}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tok.Header["kid"] = "kid1" // kid not present in JWKS
	signed, _ := tok.SignedString(priv)

	_, err = ParseAndVerifyJWT(signed)
	if err == nil || !strings.Contains(err.Error(), "key ID not found") {
		t.Fatalf("expected key ID error, got: %v", err)
	}
}

func TestParseAndVerifyJWT_InvalidIssuerHost(t *testing.T) {
	if certCache != nil {
		certCache.Flush()
	}

	// Minimal server that returns empty JWKS (won't be reached if host check fails)
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	issuer := srv.URL + "/realms/dev"
	// Set a different host to force mismatch
	t.Setenv("AUTH_URL", "http://example.com")

	// Build a token (signature won't matter if host check fails first)
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	claims := jwtlib.MapClaims{"iss": issuer, "exp": time.Now().Add(time.Hour).Unix()}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tok.Header["kid"] = "kid1"
	signed, _ := tok.SignedString(priv)

	_, err := ParseAndVerifyJWT(signed)
	if err == nil || !strings.Contains(err.Error(), "token not issued by") {
		t.Fatalf("expected issuer host error, got: %v", err)
	}
}

func TestParseAndVerifyJWT_AudienceOK(t *testing.T) {
	if certCache != nil {
		certCache.Flush()
	}

	// RSA keypair
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Build JWKS with public key
	pubJWK, err := jwk.New(&priv.PublicKey)
	if err != nil {
		t.Fatalf("failed to build JWK: %v", err)
	}
	if err := pubJWK.Set(jwk.KeyIDKey, "kid1"); err != nil {
		t.Fatalf("failed to set kid: %v", err)
	}
	set := jwk.NewSet()
	set.Add(pubJWK)
	jwksJSON, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("failed to marshal jwks: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/realms/dev/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	issuer := srv.URL + "/realms/dev"
	t.Setenv("AUTH_URL", srv.URL)
	t.Setenv("AUTH_AUDIENCE", "myaud")

	claims := jwtlib.MapClaims{
		"iss": issuer,
		"aud": "myaud",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tok.Header["kid"] = "kid1"
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	if _, err := ParseAndVerifyJWT(signed); err != nil {
		t.Fatalf("unexpected error verifying token with matching audience: %v", err)
	}
}

func TestParseAndVerifyJWT_AudienceMismatch(t *testing.T) {
	if certCache != nil {
		certCache.Flush()
	}

	// RSA keypair
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Build JWKS with public key
	pubJWK, err := jwk.New(&priv.PublicKey)
	if err != nil {
		t.Fatalf("failed to build JWK: %v", err)
	}
	if err := pubJWK.Set(jwk.KeyIDKey, "kid1"); err != nil {
		t.Fatalf("failed to set kid: %v", err)
	}
	set := jwk.NewSet()
	set.Add(pubJWK)
	jwksJSON, _ := json.Marshal(set)

	mux := http.NewServeMux()
	mux.HandleFunc("/realms/dev/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	issuer := srv.URL + "/realms/dev"
	t.Setenv("AUTH_URL", srv.URL)
	t.Setenv("AUTH_AUDIENCE", "myaud")

	claims := jwtlib.MapClaims{
		"iss": issuer,
		"aud": "otheraud",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tok.Header["kid"] = "kid1"
	signed, _ := tok.SignedString(priv)

	if _, err := ParseAndVerifyJWT(signed); err == nil {
		t.Fatalf("expected audience validation error, got none")
	}
}
