package backend

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.hasen.dev/vbolt"
)

const (
	appleIssuer          = "https://appleid.apple.com"
	appleAuthorizeURL    = "https://appleid.apple.com/auth/authorize"
	appleTokenURL        = "https://appleid.apple.com/auth/token"
	appleKeysURL         = "https://appleid.apple.com/auth/keys"
	appleStateCookieName = "appleOAuthState"
	appleStateLifetime   = 10 * time.Minute
	// Apple caps the client secret at six months. A shorter life costs nothing:
	// the secret is minted per exchange, never stored.
	appleClientSecretLifetime = 5 * time.Minute
)

// appleWebConfig is the Services ID credential used by the browser flow. It is
// nil until SetupAppleOAuth finds a complete set, which is what makes
// /api/login/apple answer with an error instead of redirecting to Apple with a
// half-built request.
type appleOAuthConfig struct {
	ClientID    string
	TeamID      string
	KeyID       string
	Key         *ecdsa.PrivateKey
	RedirectURL string
}

var appleWebConfig *appleOAuthConfig

// The iOS companion app authenticates with an identity token minted for its
// bundle ID, which is a different audience than the web Services ID.
var appleIOSClientID string

var appleHTTPClient = &http.Client{Timeout: 10 * time.Second}

type AppleTokenLoginRequest struct {
	IDToken string `json:"idToken"`
	// Apple releases the user's name only in the response to the very first
	// authorization, and never again. The native client forwards it here so a
	// first-time account gets a real name.
	Name string `json:"name"`
}

type AppleTokenInfo struct {
	Sub            string
	Aud            string
	Email          string
	EmailVerified  bool
	IsPrivateEmail bool
	Nonce          string
}

func SetupAppleOAuth() error {
	appleIOSClientID = os.Getenv("APPLE_IOS_CLIENT_ID")
	appleWebConfig = nil

	clientID := os.Getenv("APPLE_CLIENT_ID")
	teamID := os.Getenv("APPLE_TEAM_ID")
	keyID := os.Getenv("APPLE_KEY_ID")
	keyPath := os.Getenv("APPLE_KEY_PATH")

	if clientID == "" && teamID == "" && keyID == "" && keyPath == "" {
		return errors.New("Apple Sign In not configured. Set APPLE_CLIENT_ID, APPLE_TEAM_ID, APPLE_KEY_ID, and APPLE_KEY_PATH to enable")
	}

	var missing []string
	for _, pair := range [][2]string{
		{"APPLE_CLIENT_ID", clientID},
		{"APPLE_TEAM_ID", teamID},
		{"APPLE_KEY_ID", keyID},
		{"APPLE_KEY_PATH", keyPath},
	} {
		if pair[1] == "" {
			missing = append(missing, pair[0])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("incomplete Apple Sign In configuration, missing: %s", strings.Join(missing, ", "))
	}

	key, err := loadApplePrivateKey(keyPath)
	if err != nil {
		return err
	}

	siteRoot := os.Getenv("SITE_ROOT")
	if siteRoot == "" {
		siteRoot = "http://localhost:8666"
	}

	appleWebConfig = &appleOAuthConfig{
		ClientID:    clientID,
		TeamID:      teamID,
		KeyID:       keyID,
		Key:         key,
		RedirectURL: siteRoot + "/api/apple/callback",
	}

	return nil
}

func loadApplePrivateKey(keyPath string) (*ecdsa.PrivateKey, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Apple signing key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block from Apple signing key")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Apple signing key: %w", err)
	}

	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("Apple signing key is not an ECDSA private key")
	}

	return key, nil
}

// appleClientSecret mints the short-lived JWT Apple accepts in place of a
// static client secret. Apple never issues one, so this is the only way to
// authenticate the code exchange.
func (c *appleOAuthConfig) clientSecret(now time.Time) (string, error) {
	claims := jwt.MapClaims{
		"iss": c.TeamID,
		"iat": now.Unix(),
		"exp": now.Add(appleClientSecretLifetime).Unix(),
		"aud": appleIssuer,
		"sub": c.ClientID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = c.KeyID

	return token.SignedString(c.Key)
}

/* ---------- Apple's public keys ---------- */

type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type appleKeyCache struct {
	mu          sync.Mutex
	keys        map[string]*rsa.PublicKey
	fetchedAt   time.Time
	lastAttempt time.Time
}

// Apple rotates its signing keys without notice, so an unknown kid is a cache
// miss rather than an error. minRefetchInterval keeps a token signed with a
// forged kid from turning every login attempt into an outbound request.
const (
	appleKeyCacheTTL       = time.Hour
	appleKeyMinRefetchWait = time.Minute
)

var appleKeys = &appleKeyCache{}

func (c *appleKeyCache) lookup(kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	fresh := now.Sub(c.fetchedAt) < appleKeyCacheTTL
	if key, ok := c.keys[kid]; ok && fresh {
		return key, nil
	}

	if !fresh || now.Sub(c.lastAttempt) >= appleKeyMinRefetchWait {
		c.lastAttempt = now
		keys, err := fetchAppleKeys()
		if err != nil {
			if key, ok := c.keys[kid]; ok {
				// Serving a stale-but-known key beats failing every login
				// while Apple's key endpoint is unreachable.
				return key, nil
			}
			return nil, err
		}
		c.keys = keys
		c.fetchedAt = now
	}

	if key, ok := c.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("no Apple public key for kid %q", kid)
}

func (c *appleKeyCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = nil
	c.fetchedAt = time.Time{}
	c.lastAttempt = time.Time{}
}

// appleKeysEndpoint is a variable so tests can point the fetch at a local
// server instead of reaching out to Apple.
var appleKeysEndpoint = appleKeysURL

func fetchAppleKeys() (map[string]*rsa.PublicKey, error) {
	resp, err := appleHTTPClient.Get(appleKeysEndpoint)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("failed to fetch Apple public keys: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return nil, fmt.Errorf("Apple public keys request failed: %s", strings.TrimSpace(string(body)))
	}

	var payload struct {
		Keys []appleJWK `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode Apple public keys: %v", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, jwk := range payload.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" {
			continue
		}
		key, err := jwk.publicKey()
		if err != nil {
			continue
		}
		keys[jwk.Kid] = key
	}

	if len(keys) == 0 {
		return nil, errors.New("Apple returned no usable public keys")
	}

	return keys, nil
}

func (j appleJWK) publicKey() (*rsa.PublicKey, error) {
	modulus, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("invalid modulus: %v", err)
	}
	exponent, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, fmt.Errorf("invalid exponent: %v", err)
	}
	if len(exponent) == 0 || len(exponent) > 8 {
		return nil, errors.New("unsupported exponent size")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulus),
		E: int(new(big.Int).SetBytes(exponent).Int64()),
	}, nil
}

/* ---------- identity token verification ---------- */

// appleFlexBool reads a claim Apple sends as a JSON boolean in some responses
// and as the string "true" in others.
type appleFlexBool bool

func (b *appleFlexBool) UnmarshalJSON(data []byte) error {
	var asBool bool
	if err := json.Unmarshal(data, &asBool); err == nil {
		*b = appleFlexBool(asBool)
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err != nil {
		return err
	}
	*b = appleFlexBool(asString == "true")
	return nil
}

type appleIDTokenClaims struct {
	Iss            string        `json:"iss"`
	Sub            string        `json:"sub"`
	Aud            string        `json:"aud"`
	Exp            int64         `json:"exp"`
	Iat            int64         `json:"iat"`
	Nonce          string        `json:"nonce"`
	Email          string        `json:"email"`
	EmailVerified  appleFlexBool `json:"email_verified"`
	IsPrivateEmail appleFlexBool `json:"is_private_email"`
}

func (c appleIDTokenClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	if c.Exp == 0 {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.Exp, 0)), nil
}

func (c appleIDTokenClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	if c.Iat == 0 {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.Iat, 0)), nil
}

func (c appleIDTokenClaims) GetNotBefore() (*jwt.NumericDate, error) { return nil, nil }
func (c appleIDTokenClaims) GetIssuer() (string, error)              { return c.Iss, nil }
func (c appleIDTokenClaims) GetSubject() (string, error)             { return c.Sub, nil }
func (c appleIDTokenClaims) GetAudience() (jwt.ClaimStrings, error) {
	if c.Aud == "" {
		return nil, nil
	}
	return jwt.ClaimStrings{c.Aud}, nil
}

// appleAudiences lists every client ID a token may legitimately be minted for.
// A token addressed to some other relying party is a token we were handed, not
// one we were issued, so accepting it would let any Apple developer sign in as
// any of our users.
func appleAudiences() []string {
	var auds []string
	if appleWebConfig != nil && appleWebConfig.ClientID != "" {
		auds = append(auds, appleWebConfig.ClientID)
	}
	if appleIOSClientID != "" {
		auds = append(auds, appleIOSClientID)
	}
	return auds
}

func verifyAppleIDToken(idToken string) (*AppleTokenInfo, error) {
	audiences := appleAudiences()
	if len(audiences) == 0 {
		return nil, errors.New("Apple Sign In not configured")
	}

	var claims appleIDTokenClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithExpirationRequired(),
	)

	_, err := parser.ParseWithClaims(idToken, &claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("identity token has no key id")
		}
		return appleKeys.lookup(kid)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to verify identity token: %v", err)
	}

	audienceOK := false
	for _, aud := range audiences {
		if claims.Aud == aud {
			audienceOK = true
			break
		}
	}
	if !audienceOK {
		return nil, errors.New("invalid token audience")
	}

	if claims.Sub == "" {
		return nil, errors.New("identity token has no subject")
	}

	return &AppleTokenInfo{
		Sub:            claims.Sub,
		Aud:            claims.Aud,
		Email:          strings.ToLower(strings.TrimSpace(claims.Email)),
		EmailVerified:  bool(claims.EmailVerified),
		IsPrivateEmail: bool(claims.IsPrivateEmail),
		Nonce:          claims.Nonce,
	}, nil
}

/* ---------- provider discovery ---------- */

// AuthProviders tells the login page which third-party buttons are worth
// drawing. Google is unconditional because its credentials are required in
// release; Apple is optional, and a button that redirects to a 500 is worse
// than no button.
type AuthProviders struct {
	Google bool `json:"google"`
	Apple  bool `json:"apple"`
}

func authProvidersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthProviders{
		Google: oauthConf != nil,
		Apple:  appleWebConfig != nil,
	})
}

/* ---------- browser flow ---------- */

func appleLoginHandler(w http.ResponseWriter, r *http.Request) {
	if appleWebConfig == nil {
		http.Error(w, "Apple Sign In not configured", http.StatusInternalServerError)
		return
	}

	state, err := generateToken(20)
	if err != nil {
		http.Error(w, "Failed to start Apple sign-in", http.StatusInternalServerError)
		return
	}
	nonce, err := generateToken(20)
	if err != nil {
		http.Error(w, "Failed to start Apple sign-in", http.StatusInternalServerError)
		return
	}

	setAppleStateCookie(w, state, nonce)

	query := url.Values{
		"client_id":     {appleWebConfig.ClientID},
		"redirect_uri":  {appleWebConfig.RedirectURL},
		"response_type": {"code id_token"},
		"scope":         {"name email"},
		// Apple requires form_post whenever name or email is requested, which
		// is why the state cookie below has to survive a cross-site POST.
		"response_mode": {"form_post"},
		"state":         {state},
		"nonce":         {nonce},
	}

	http.Redirect(w, r, appleAuthorizeURL+"?"+query.Encode(), http.StatusTemporaryRedirect)
}

func setAppleStateCookie(w http.ResponseWriter, state, nonce string) {
	http.SetCookie(w, &http.Cookie{
		Name:     appleStateCookieName,
		Value:    state + "." + nonce,
		Path:     "/api",
		HttpOnly: true,
		Secure:   true,
		// Apple posts the callback from appleid.apple.com. A Lax cookie would
		// not be sent with that request, so there would be nothing to compare
		// the returned state against.
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int(appleStateLifetime.Seconds()),
	})
}

func clearAppleStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     appleStateCookieName,
		Value:    "",
		Path:     "/api",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Expires:  time.Unix(0, 0),
	})
}

func readAppleStateCookie(r *http.Request) (state, nonce string, ok bool) {
	cookie, err := r.Cookie(appleStateCookieName)
	if err != nil {
		return "", "", false
	}
	state, nonce, ok = strings.Cut(cookie.Value, ".")
	if !ok || state == "" || nonce == "" {
		return "", "", false
	}
	return state, nonce, true
}

// appleCallbackUser is the one-time payload Apple posts alongside the first
// authorization. Every later sign-in omits it entirely.
type appleCallbackUser struct {
	Name struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	} `json:"name"`
	Email string `json:"email"`
}

func appleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if appleWebConfig == nil {
		http.Error(w, "Apple Sign In not configured", http.StatusInternalServerError)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid callback payload", http.StatusBadRequest)
		return
	}

	// A user who cancels at Apple's prompt comes back here with an error and no
	// code. That is not a failure worth an error page.
	if appleErr := r.FormValue("error"); appleErr != "" {
		clearAppleStateCookie(w)
		LogInfoWithRequest(r, LogCategoryAuth, "Apple sign-in not completed", map[string]interface{}{
			"error": appleErr,
		})
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	expectedState, expectedNonce, ok := readAppleStateCookie(r)
	if !ok {
		http.Error(w, "Apple sign-in expired, please try again", http.StatusBadRequest)
		return
	}
	clearAppleStateCookie(w)

	if subtle.ConstantTimeCompare([]byte(r.FormValue("state")), []byte(expectedState)) != 1 {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	idToken, err := exchangeAppleCode(code)
	if err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Apple code exchange failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Apple sign-in failed", http.StatusInternalServerError)
		return
	}

	tokenInfo, err := verifyAppleIDToken(idToken)
	if err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Apple identity token verification failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Apple sign-in failed", http.StatusInternalServerError)
		return
	}

	if subtle.ConstantTimeCompare([]byte(tokenInfo.Nonce), []byte(expectedNonce)) != 1 {
		http.Error(w, "Apple sign-in failed", http.StatusBadRequest)
		return
	}

	var name string
	if raw := r.FormValue("user"); raw != "" {
		var payload appleCallbackUser
		if err := json.Unmarshal([]byte(raw), &payload); err == nil {
			name = strings.TrimSpace(payload.Name.FirstName + " " + payload.Name.LastName)
		}
	}

	user, err := upsertAppleUser(r, tokenInfo, name)
	if err != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Apple sign-in could not resolve an account", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Apple sign-in failed", http.StatusInternalServerError)
		return
	}

	if err := authenticateForUser(user.Id, w); err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	LogInfoWithRequest(r, LogCategoryAuth, "Apple sign-in successful", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// appleTokenEndpoint is a variable so tests can exercise the exchange without
// reaching appleid.apple.com.
var appleTokenEndpoint = appleTokenURL

func exchangeAppleCode(code string) (string, error) {
	secret, err := appleWebConfig.clientSecret(time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to sign client secret: %v", err)
	}

	form := url.Values{
		"client_id":     {appleWebConfig.ClientID},
		"client_secret": {secret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {appleWebConfig.RedirectURL},
	}

	resp, err := appleHTTPClient.PostForm(appleTokenEndpoint, form)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return "", fmt.Errorf("token request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request rejected: %s", strings.TrimSpace(string(body)))
	}

	var payload struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("failed to decode token response: %v", err)
	}
	if payload.Error != "" {
		return "", fmt.Errorf("token request rejected: %s", payload.Error)
	}
	if payload.IDToken == "" {
		return "", errors.New("token response contained no identity token")
	}

	return payload.IDToken, nil
}

/* ---------- native flow ---------- */

func appleTokenLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req AppleTokenLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid request body"})
		return
	}

	if req.IDToken == "" {
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Identity token is required"})
		return
	}

	tokenInfo, err := verifyAppleIDToken(req.IDToken)
	if err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Apple token verification failed", map[string]interface{}{
			"error": err.Error(),
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid Apple token"})
		return
	}

	user, err := upsertAppleUser(r, tokenInfo, req.Name)
	if err != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Apple sign-in could not resolve an account", map[string]interface{}{
			"error": err.Error(),
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Failed to create account"})
		return
	}

	token, err := generateAuthJwt(user, w)
	if err != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Failed to generate JWT for Apple sign-in", map[string]interface{}{
			"userId": user.Id,
			"error":  err.Error(),
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Failed to generate token"})
		return
	}

	LogInfoWithRequest(r, LogCategoryAuth, "Apple sign-in successful", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	json.NewEncoder(w).Encode(LoginResponse{Success: true, Token: token, Auth: GetAuthResponseForUser(user)})
}

/* ---------- account resolution ---------- */

// upsertAppleUser matches on the email in the identity token, the same key the
// password and Google paths use. Apple keeps that address stable for the life
// of the app's team, including the relay address issued to someone who chose to
// hide their real one.
func upsertAppleUser(r *http.Request, info *AppleTokenInfo, name string) (User, error) {
	if info.Email == "" {
		return User{}, errors.New("identity token carried no email address")
	}

	var userId int
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		userId = GetUserId(tx, info.Email)
	})

	var user User
	if userId > 0 {
		if info.EmailVerified {
			vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
				markEmailVerifiedTx(tx, userId)
				vbolt.TxCommit(tx)
			})
		}
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			user = GetUser(tx, userId)
		})
		if user.Id == 0 {
			return User{}, errors.New("user not found")
		}
		return user, nil
	}

	createAccountRequest := CreateAccountRequest{
		Name:  appleDisplayName(name, info.Email, info.IsPrivateEmail),
		Email: info.Email,
	}

	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, createAccountRequest, []byte{})
		// Apple asserts the address; a confirmation round trip would ask the
		// user to prove something the identity provider already proved. A relay
		// address is deliverable only through Apple, so a verification mail
		// there is worse than useless.
		if info.EmailVerified || info.IsPrivateEmail {
			user = markEmailVerifiedTx(tx, user.Id)
		} else {
			sendVerificationEmailTx(tx, user, time.Now())
		}
		vbolt.TxCommit(tx)
	})

	if user.Id == 0 {
		return User{}, errors.New("failed to create user account")
	}

	LogInfoWithRequest(r, LogCategoryAuth, "New user created via Apple sign-in", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	return user, nil
}

const appleFallbackName = "Family Member"

// appleDisplayName picks something a person would recognize on the dashboard.
// Apple hands over a name only on the first authorization, and a relay address
// has a random local part, so both of those can be nothing to work with.
func appleDisplayName(name, email string, isPrivateEmail bool) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	if !isPrivateEmail {
		if local, _, ok := strings.Cut(email, "@"); ok && local != "" {
			return local
		}
	}
	return appleFallbackName
}
