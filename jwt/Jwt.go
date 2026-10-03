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
	"time"

	"github.com/bignyap/go-utilities/httpclient"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/patrickmn/go-cache"
)

var certCache *cache.Cache

func init() {
	// Initialize the cache with a default expiration time of 1 hour, and cleanup interval of 30 minutes
	certCache = cache.New(1*time.Hour, 30*time.Minute)
}

func getJWKSet(issuer string) (jwk.Set, error) {
	if jwks, found := certCache.Get(issuer); found {
		return jwks.(jwk.Set), nil
	}

	// Build the JWKS endpoint URL
	certEndpoint, err := url.JoinPath(issuer, "protocol/openid-connect/certs")
	if err != nil {
		return nil, fmt.Errorf("error getting the public certificate: %v", err)
	}

	// Configure resilient HTTP client
	config := httpclient.DefaultConfig()
	config.RetryCount = 2
	config.Timeout = 10 * time.Second
	// Gate TLS verification via env (default: verify TLS)
	skipTLS := false
	if v := os.Getenv("AUTH_SKIP_TLS_VERIFY"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			skipTLS = b
		}
	}
	config.TLSClientConfig.SkipTLSVerify = skipTLS

	hc := httpclient.NewHTTPClient(issuer, config, func(err error) error {
		return fmt.Errorf("service temporarily unavailable")
	})

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

	// Cache the jwks for future use
	certCache.Set(issuer, jwks, cache.DefaultExpiration)
	return jwks, nil
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
	if err != nil {
		return false
	}
	issuerHost := parsedIssuer.Host
	issuerHostname := parsedIssuer.Hostname()

	// Gather all allowed targets from AUTH_URL and AUTH_ALLOWED_HOSTS
	var allowedEntries []string
	if authURL := os.Getenv("AUTH_URL"); authURL != "" {
		for _, u := range strings.Split(authURL, ",") {
			if trimmed := strings.TrimSpace(u); trimmed != "" {
				allowedEntries = append(allowedEntries, trimmed)
			}
		}
	}
	if allowedHosts := os.Getenv("AUTH_ALLOWED_HOSTS"); allowedHosts != "" {
		for _, h := range strings.Split(allowedHosts, ",") {
			if trimmed := strings.TrimSpace(h); trimmed != "" {
				allowedEntries = append(allowedEntries, trimmed)
			}
		}
	}

	if len(allowedEntries) == 0 {
		return false
	}

	for _, entry := range allowedEntries {
		targetHost := entry
		targetHostname := entry

		// If entry is a full URL, parse its host & hostname
		if strings.HasPrefix(entry, "http://") || strings.HasPrefix(entry, "https://") {
			if u, err := url.Parse(entry); err == nil {
				targetHost = u.Host
				targetHostname = u.Hostname()
			}
		} else if strings.Contains(entry, ":") {
			if h, _, err := net.SplitHostPort(entry); err == nil {
				targetHostname = h
			}
		}

		// Exact host match (including port)
		if strings.EqualFold(issuerHost, targetHost) {
			return true
		}

		// Hostname-only match (ignores differing port numbers)
		if strings.EqualFold(issuerHostname, targetHostname) {
			return true
		}

		// Loopback equivalence (localhost == 127.0.0.1 == host.docker.internal)
		if isLoopback(issuerHostname) && isLoopback(targetHostname) {
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
		return jwt.MapClaims{}, fmt.Errorf("token not issued by %s", os.Getenv("AUTH_URL"))
	}

	realm, err := extractRealmFromPath(parsedUrl.Path)
	if err != nil {
		return jwt.MapClaims{}, fmt.Errorf("realm not found in the issuer")
	}
	claims["realm"] = realm

	// Validate audience format if present or if AUTH_AUDIENCE is specified
	if audVal, exists := claims["aud"]; exists {
		switch audVal.(type) {
		case string, []interface{}, []string:
			// valid audience type
		default:
			return jwt.MapClaims{}, fmt.Errorf("audience (aud) not found in the token")
		}
	} else if os.Getenv("AUTH_AUDIENCE") != "" {
		return jwt.MapClaims{}, fmt.Errorf("audience (aud) not found in the token")
	}

	// Step 4: Get or fetch the JWK set
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
		return jwt.MapClaims{}, fmt.Errorf("key ID not found in the certificate endpoint")
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
	if aud := os.Getenv("AUTH_AUDIENCE"); aud != "" {
		opts = append(opts, jwt.WithAudience(aud))
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
