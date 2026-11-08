package jwt

import (
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bignyap/go-utilities/httpclient"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/patrickmn/go-cache"
)

var certCache *cache.Cache

// ============================================================================
// Configuration & Startup Parsing
// ============================================================================

type allowedTarget struct {
	raw      string
	hostname string // lowercase
	port     string // explicit port, or default port for scheme if URL ("80" / "443"), or "" if host-only
}

func (t allowedTarget) matches(issuerHostname, issuerPort string) bool {
	// Loopback equivalence (localhost == 127.0.0.1 == ::1 == host.docker.internal)
	// Requires matching port to prevent SSRF against arbitrary local services.
	if isLoopback(issuerHostname) && isLoopback(t.hostname) {
		return issuerPort == t.port
	}

	// Exact hostname match
	if strings.EqualFold(issuerHostname, t.hostname) {
		if t.port != "" {
			return issuerPort == t.port
		}
		// If target has no port, allow standard HTTP/HTTPS ports only
		return issuerPort == "80" || issuerPort == "443"
	}

	return false
}

func parseAllowedTarget(entry string) (allowedTarget, bool) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return allowedTarget{}, false
	}

	// If entry has a scheme (http:// or https://), parse as URL
	if strings.HasPrefix(entry, "http://") || strings.HasPrefix(entry, "https://") {
		u, err := url.Parse(entry)
		if err != nil || u.Host == "" {
			return allowedTarget{}, false
		}
		port := u.Port()
		if port == "" {
			if strings.EqualFold(u.Scheme, "https") {
				port = "443"
			} else {
				port = "80"
			}
		}
		return allowedTarget{
			raw:      entry,
			hostname: strings.ToLower(u.Hostname()),
			port:     port,
		}, true
	}

	// If entry contains a port (host:port)
	if strings.Contains(entry, ":") {
		h, p, err := net.SplitHostPort(entry)
		if err == nil && h != "" && p != "" {
			return allowedTarget{
				raw:      entry,
				hostname: strings.ToLower(h),
				port:     p,
			}, true
		}
	}

	// Hostname only (no port)
	return allowedTarget{
		raw:      entry,
		hostname: strings.ToLower(entry),
		port:     "",
	}, true
}

type authConfig struct {
	authURL         string
	allowedHosts    string
	jwksURL         string
	rawSkipTLS      string
	skipTLSVerify   bool
	keycloakBaseURL string
	audience        string

	allowedTargets         []allowedTarget
	internalAuthCandidates []string
	internalKeycloakBase   string
}

var (
	currentConfig atomic.Pointer[authConfig]
	configMu      sync.Mutex

	sharedHTTPClientMu sync.RWMutex
	sharedHTTPClient   *http.Client
	lastSkipTLS        bool
)

func init() {
	// Initialize the cache with a default expiration time of 1 hour, and cleanup interval of 30 minutes
	certCache = cache.New(1*time.Hour, 30*time.Minute)
	ReloadConfig()
}

// ReloadConfig forces a reload of the JWT authentication configuration from environment variables.
func ReloadConfig() {
	configMu.Lock()
	defer configMu.Unlock()

	authURL := os.Getenv("AUTH_URL")
	allowedHosts := os.Getenv("AUTH_ALLOWED_HOSTS")
	jwksURL := os.Getenv("AUTH_JWKS_URL")
	skipTLS := os.Getenv("AUTH_SKIP_TLS_VERIFY")
	kcURL := os.Getenv("KEYCLOAK_BASE_URL")
	if kcURL == "" {
		kcURL = os.Getenv("KEYCLOAK_URL")
	}
	aud := os.Getenv("AUTH_AUDIENCE")

	reloadConfigLocked(authURL, allowedHosts, jwksURL, skipTLS, kcURL, aud)
}

func reloadConfigLocked(authURL, allowedHosts, jwksURL, skipTLS, kcURL, aud string) *authConfig {
	skipTLSBool := false
	if skipTLS != "" {
		if b, err := strconv.ParseBool(skipTLS); err == nil {
			skipTLSBool = b
		}
	}

	var targets []allowedTarget
	var internalAuthCandidates []string

	if authURL != "" {
		for _, u := range strings.Split(authURL, ",") {
			if target, ok := parseAllowedTarget(u); ok {
				targets = append(targets, target)
			}
			trimmed := strings.TrimSpace(u)
			if trimmed != "" {
				if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
					if !isLoopback(parsed.Hostname()) {
						internalAuthCandidates = append(internalAuthCandidates, trimmed)
					}
				}
			}
		}
	}

	if allowedHosts != "" {
		for _, h := range strings.Split(allowedHosts, ",") {
			if target, ok := parseAllowedTarget(h); ok {
				targets = append(targets, target)
			}
		}
	}

	var internalKeycloakBase string
	if kcURL != "" {
		if parsed, err := url.Parse(kcURL); err == nil && !isLoopback(parsed.Hostname()) {
			internalKeycloakBase = kcURL
		}
	}

	cfg := &authConfig{
		authURL:                authURL,
		allowedHosts:           allowedHosts,
		jwksURL:                jwksURL,
		rawSkipTLS:             skipTLS,
		skipTLSVerify:          skipTLSBool,
		keycloakBaseURL:        kcURL,
		audience:               aud,
		allowedTargets:         targets,
		internalAuthCandidates: internalAuthCandidates,
		internalKeycloakBase:   internalKeycloakBase,
	}

	currentConfig.Store(cfg)
	return cfg
}

func getConfig() *authConfig {
	cfg := currentConfig.Load()

	// In test environments where t.Setenv is used without calling ReloadConfig,
	// detect changes dynamically and reload. In production, this fast-paths with string comparison.
	authURL := os.Getenv("AUTH_URL")
	allowedHosts := os.Getenv("AUTH_ALLOWED_HOSTS")
	jwksURL := os.Getenv("AUTH_JWKS_URL")
	skipTLS := os.Getenv("AUTH_SKIP_TLS_VERIFY")
	kcURL := os.Getenv("KEYCLOAK_BASE_URL")
	if kcURL == "" {
		kcURL = os.Getenv("KEYCLOAK_URL")
	}
	aud := os.Getenv("AUTH_AUDIENCE")

	if cfg != nil &&
		cfg.authURL == authURL &&
		cfg.allowedHosts == allowedHosts &&
		cfg.jwksURL == jwksURL &&
		cfg.rawSkipTLS == skipTLS &&
		cfg.keycloakBaseURL == kcURL &&
		cfg.audience == aud {
		return cfg
	}

	configMu.Lock()
	defer configMu.Unlock()
	return reloadConfigLocked(authURL, allowedHosts, jwksURL, skipTLS, kcURL, aud)
}

// ============================================================================
// Shared HTTP Client
// ============================================================================

func getJWKSHTTPClient(skipTLS bool) *http.Client {
	sharedHTTPClientMu.RLock()
	client := sharedHTTPClient
	currentSkip := lastSkipTLS
	sharedHTTPClientMu.RUnlock()

	if client != nil && currentSkip == skipTLS {
		return client
	}

	sharedHTTPClientMu.Lock()
	defer sharedHTTPClientMu.Unlock()

	if sharedHTTPClient != nil && lastSkipTLS == skipTLS {
		return sharedHTTPClient
	}

	config := httpclient.DefaultConfig()
	config.RetryCount = 1
	config.Timeout = 10 * time.Second
	config.CircuitBreakerCommand = "jwks-client"
	config.TLSClientConfig.SkipTLSVerify = skipTLS

	sharedHTTPClient = httpclient.NewHTTPClient("", config, func(err error) error {
		return fmt.Errorf("service temporarily unavailable")
	})
	lastSkipTLS = skipTLS
	return sharedHTTPClient
}

// normalizeIssuer strips query params, fragments, userinfo, and trailing slashes to prevent cache key flooding.
func normalizeIssuer(rawIssuer string) string {
	u, err := url.Parse(rawIssuer)
	if err != nil {
		return strings.TrimRight(strings.TrimSpace(rawIssuer), "/")
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)
	path := strings.TrimRight(u.Path, "/")
	if scheme == "" || host == "" {
		return strings.TrimRight(strings.TrimSpace(rawIssuer), "/")
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

// findInternalAuthURL searches pre-configured AUTH_URL and KEYCLOAK_BASE_URL for an internal (non-loopback)
// Keycloak endpoint reachable inside Docker or cluster networks.
func findInternalAuthURL(issuerPath string) string {
	realm, _ := extractRealmFromPath(issuerPath)
	cfg := getConfig()

	// 1. Check internal candidates from AUTH_URL
	var realmMatchCandidate string
	for _, candidate := range cfg.internalAuthCandidates {
		if realm != "" && strings.Contains(candidate, "/realms/"+realm) {
			realmMatchCandidate = candidate
			break
		}
	}
	if realmMatchCandidate != "" {
		return realmMatchCandidate
	} else if len(cfg.internalAuthCandidates) > 0 {
		return cfg.internalAuthCandidates[0]
	}

	// 2. Fall back to KEYCLOAK_BASE_URL or KEYCLOAK_URL
	if realm != "" && cfg.internalKeycloakBase != "" {
		if joined, err := url.JoinPath(cfg.internalKeycloakBase, "realms", realm); err == nil {
			return joined
		}
	}

	return ""
}

// fetchJWKS performs HTTP request to retrieve and parse the JWKS from certEndpoint using the shared HTTP client.
func fetchJWKS(certEndpoint string) (jwk.Set, error) {
	cfg := getConfig()
	hc := getJWKSHTTPClient(cfg.skipTLSVerify)

	req, err := http.NewRequest(http.MethodGet, certEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	jwks, err := jwk.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	return jwks, nil
}

func getJWKSet(issuer string) (jwk.Set, error) {
	cacheKey := normalizeIssuer(issuer)
	if jwks, found := certCache.Get(cacheKey); found {
		return jwks.(jwk.Set), nil
	}

	cfg := getConfig()

	// 1. Explicit override via AUTH_JWKS_URL
	if cfg.jwksURL != "" {
		jwks, err := fetchJWKS(cfg.jwksURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get JWK set from AUTH_JWKS_URL: %w", err)
		}
		certCache.Set(cacheKey, jwks, cache.DefaultExpiration)
		return jwks, nil
	}

	// 2. Try fetching directly from the token's issuer URL first.
	// When running locally on host (e.g. make run-debug), localhost:8180 works directly.
	var lastErr error
	certEndpoint, err := url.JoinPath(cacheKey, "protocol/openid-connect/certs")
	if err == nil {
		if jwks, fetchErr := fetchJWKS(certEndpoint); fetchErr == nil {
			certCache.Set(cacheKey, jwks, cache.DefaultExpiration)
			return jwks, nil
		} else {
			lastErr = fetchErr
		}
	} else {
		lastErr = err
	}

	// 3. Fallback: If direct fetch from issuer failed (e.g. localhost:8180 inside Docker container),
	// and issuer is a loopback address, fall back to internal endpoint from AUTH_URL or KEYCLOAK_BASE_URL.
	if parsed, parseErr := url.Parse(cacheKey); parseErr == nil && isLoopback(parsed.Hostname()) {
		if internalBase := findInternalAuthURL(parsed.Path); internalBase != "" {
			if internalEndpoint, joinErr := url.JoinPath(internalBase, "protocol/openid-connect/certs"); joinErr == nil {
				if jwks, internalErr := fetchJWKS(internalEndpoint); internalErr == nil {
					certCache.Set(cacheKey, jwks, cache.DefaultExpiration)
					return jwks, nil
				} else {
					lastErr = internalErr
				}
			}
		}
	}

	return nil, fmt.Errorf("failed to get JWK set: %w", lastErr)
}

func extractRealmFromPath(path string) (string, error) {
	segments := strings.Split(path, "/")
	if len(segments) < 3 {
		return "", fmt.Errorf("issuer URL path does not contain enough segments to extract realm")
	}

	// Be robust: find the segment named "realms" and return the next non-empty segment
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "realms" {
			if i+1 < len(parts) && parts[i+1] != "" {
				return parts[i+1], nil
			}
			return "", fmt.Errorf("realm not found after 'realms' in the issuer path")
		}
	}
	return "", fmt.Errorf("issuer URL path does not contain 'realms' segment")
}

func isLoopback(host string) bool {
	h := strings.ToLower(host)
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "host.docker.internal"
}

// isIssuerAllowed checks if the token's issuer matches any allowed configuration
func isIssuerAllowed(tokenIssuer string) bool {
	parsedIssuer, err := url.Parse(tokenIssuer)
	if err != nil || parsedIssuer.Host == "" {
		return false
	}
	issuerHostname := strings.ToLower(parsedIssuer.Hostname())
	issuerPort := parsedIssuer.Port()
	if issuerPort == "" {
		if strings.EqualFold(parsedIssuer.Scheme, "https") {
			issuerPort = "443"
		} else if strings.EqualFold(parsedIssuer.Scheme, "http") {
			issuerPort = "80"
		}
	}

	cfg := getConfig()
	if len(cfg.allowedTargets) == 0 {
		return false
	}

	for _, target := range cfg.allowedTargets {
		if target.matches(issuerHostname, issuerPort) {
			return true
		}
	}

	return false
}

func ParseAndVerifyJWT(tokenString string) (jwt.MapClaims, error) {
	// Step 1: Parse the JWT token without verifying the signature
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return jwt.MapClaims{}, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return jwt.MapClaims{}, fmt.Errorf("failed to parse claims")
	}

	// Step 2: Extract the issuer and then the realm from it
	issuer, ok := claims["iss"].(string)
	if !ok {
		return jwt.MapClaims{}, fmt.Errorf("issuer (iss) not found in the token")
	}

	parsedUrl, err := url.Parse(issuer)
	if err != nil {
		return jwt.MapClaims{}, fmt.Errorf("issuer (iss) not found in the token")
	}

	if !isIssuerAllowed(issuer) {
		return jwt.MapClaims{}, fmt.Errorf("token issuer is not authorized")
	}

	realm, err := extractRealmFromPath(parsedUrl.Path)
	if err != nil {
		return jwt.MapClaims{}, fmt.Errorf("realm not found in the issuer")
	}
	claims["realm"] = realm

	// Validate audience format if present or if AUTH_AUDIENCE is specified
	cfg := getConfig()
	if audVal, exists := claims["aud"]; exists {
		switch audVal.(type) {
		case string, []interface{}, []string:
			// valid audience type
		default:
			return jwt.MapClaims{}, fmt.Errorf("audience (aud) not found in the token")
		}
	} else if cfg.audience != "" {
		return jwt.MapClaims{}, fmt.Errorf("audience (aud) not found in the token")
	}

	// Step 3: Get or fetch the JWK set
	jwks, err := getJWKSet(issuer)
	if err != nil {
		return jwt.MapClaims{}, fmt.Errorf("failed to get JWK set: %w", err)
	}

	// Get the kid from the unverified header
	unverifiedHeader := token.Header
	kid, ok := unverifiedHeader["kid"].(string)
	if !ok {
		return jwt.MapClaims{}, fmt.Errorf("kid not found in the token header")
	}

	// Find the corresponding key
	key, found := jwks.LookupKeyID(kid)
	if !found {
		// Attempt cache refresh in case key rotation occurred
		cacheKey := normalizeIssuer(issuer)
		certCache.Delete(cacheKey)
		if refreshedJWKS, refErr := getJWKSet(issuer); refErr == nil {
			jwks = refreshedJWKS
			key, found = jwks.LookupKeyID(kid)
		}
		if !found {
			return jwt.MapClaims{}, fmt.Errorf("key ID not found in the certificate endpoint")
		}
	}

	// Extract the public key
	var pubKey rsa.PublicKey
	if err := key.Raw(&pubKey); err != nil {
		return jwt.MapClaims{}, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Step 4: Verify the signature
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithLeeway(1 * time.Minute),
	}
	if cfg.audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.audience))
	}

	parsedToken, err := jwt.ParseWithClaims(
		tokenString,
		jwt.MapClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return &pubKey, nil
		},
		opts...,
	)
	if err != nil {
		return jwt.MapClaims{}, fmt.Errorf("failed to verify token: %w", err)
	}

	if !parsedToken.Valid {
		return jwt.MapClaims{}, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func ExtractToken(r *http.Request) (string, error) {
	// Extract the token from header
	authHeader := strings.Trim(r.Header.Get("Authorization"), ";")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is missing")
	}

	// Split the token to get the token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("authorization header format must be Bearer {token}")
	}

	return parts[1], nil
}
