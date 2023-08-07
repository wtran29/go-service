// Package auth provides authentication and authorization support.
package auth

import (
	"context"

	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/wtran29/go-service/business/core/user"

	"github.com/jmoiron/sqlx"

	"github.com/open-policy-agent/opa/rego"
	"go.uber.org/zap"

	"github.com/golang-jwt/jwt/v4"
)

// ErrForbidden is returned when a auth issue is identified.
var ErrForbidden = errors.New("attempted action is not allowed")

// Claims represents the authorization claims transmitted via a JWT.
type Claims struct {
	jwt.RegisteredClaims
	Roles []user.Role `json:"roles"`
}

// KeyLookup declares a method set of behavior for looking up
// private and public keys for JWT use. The return could be a
// PEM encoded string or a JWS based key.
type KeyLookup interface {
	PrivateKey(kid string) (key string, err error)
	PublicKey(kid string) (key string, err error)
}

// Config represents information required to initialize auth.
type Config struct {
	// Log       *logger.Logger
	Log       *zap.SugaredLogger
	DB        *sqlx.DB
	KeyLookup KeyLookup
	Issuer    string
}

// Auth is used to authenticate clients. It can generate a token for a
// set of user claims and recreate the claims by parsing the token.
type Auth struct {
	// log       *logger.Logger
	log       *zap.SugaredLogger
	keyLookup KeyLookup
	// userCore  *user.Core
	method jwt.SigningMethod
	parser *jwt.Parser
	issuer string
	mu     sync.RWMutex
	cache  map[string]string
}

// New creates an Auth to support authentication/authorization.
func New(cfg Config) (*Auth, error) {
	// If a database connection is not provided, we won't perform the
	// user enabled check.
	// var usrCore *user.Core
	// if cfg.DB != nil {
	// 	evnCore := event.NewCore(cfg.Log)
	// 	usrCore = user.NewCore(cfg.Log, evnCore, userdb.NewStore(cfg.Log, cfg.DB))
	// }

	a := Auth{
		// log: cfg.Log,
		log:       cfg.Log,
		keyLookup: cfg.KeyLookup,
		// user:      usr,
		method: jwt.GetSigningMethod("RS256"),
		parser: jwt.NewParser(jwt.WithValidMethods([]string{"RS256"})),
		cache:  make(map[string]string),
	}

	return &a, nil
}

// GenerateToken generates a signed JWT token string representing the user Claims.
func (a *Auth) GenerateToken(kid string, claims Claims) (string, error) {
	token := jwt.NewWithClaims(a.method, claims)
	token.Header["kid"] = kid

	privateKeyPEM, err := a.keyLookup.PrivateKey(kid)
	if err != nil {
		return "", fmt.Errorf("private key: %w", err)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return "", fmt.Errorf("parsing private pem: %w", err)
	}

	str, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return str, nil
}

// // ValidateToken recreates the Claims that were used to generate a token. It
// // verifies that the token was signed using our key.
// func (a *Auth) ValidateToken(tokenStr string) (Claims, error) {
// 	var claims Claims
// 	token, err := a.parser.ParseWithClaims(tokenStr, &claims, a.keyFunc)
// 	if err != nil {
// 		return Claims{}, fmt.Errorf("parsing token: %w", err)
// 	}

// 	if !token.Valid {
// 		return Claims{}, errors.New("invalid token")
// 	}

// 	return claims, nil
// }

// Authenticate processes the token to validate the sender's token is valid.
func (a *Auth) Authenticate(ctx context.Context, bearerToken string) (Claims, error) {
	parts := strings.Split(bearerToken, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return Claims{}, errors.New("expected authorization header format: Bearer <token>")
	}

	var claims Claims
	token, _, err := a.parser.ParseUnverified(parts[1], &claims)
	if err != nil {
		return Claims{}, fmt.Errorf("error parsing token: %w", err)
	}

	// Perform an extra level of authentication verification with OPA.

	kidRaw, exists := token.Header["kid"]
	if !exists {
		return Claims{}, fmt.Errorf("kid missing from header: %w", err)
	}

	kid, ok := kidRaw.(string)
	if !ok {
		return Claims{}, fmt.Errorf("kid malformed: %w", err)
	}

	pem, err := a.keyLookup.PublicKey(kid)
	if err != nil {
		return Claims{}, fmt.Errorf("fetch public key: %w", err)
	}

	input := map[string]any{
		"Key":   pem,
		"Token": parts[1],
		"ISS":   a.issuer,
	}

	if err := a.opaPolicyEvaluation(ctx, opaAuthentication, RuleAuthenticate, input); err != nil {
		return Claims{}, fmt.Errorf("authentication failed : %w", err)
	}

	// Check the database for this user to verify they are still enabled.

	// if !a.isUserEnabled(ctx, claims) {
	// 	return Claims{}, fmt.Errorf("user not enabled : %w", err)
	// }

	return claims, nil
}

// Authorize attempts to authorize the user with the provided input roles, if
// none of the input roles are within the user's claims, we return an error
// otherwise the user is authorized.
func (a *Auth) Authorize(ctx context.Context, claims Claims, rule string) error {
	input := map[string]any{
		"Roles":   claims.Roles,
		"Subject": claims.Subject,
		"UserID":  claims.Subject,
	}

	if err := a.opaPolicyEvaluation(ctx, opaAuthorization, rule, input); err != nil {
		return fmt.Errorf("rego evaluation failed : %w", err)
	}

	return nil
}

// =============================================================================

// publicKeyLookup performs a lookup for the public pem for the specified kid.
func (a *Auth) publicKeyLookup(kid string) (string, error) {
	pem, err := func() (string, error) {
		a.mu.RLock()
		defer a.mu.RUnlock()

		pem, exists := a.cache[kid]
		if !exists {
			return "", errors.New("not found")
		}
		return pem, nil
	}()
	if err == nil {
		return pem, nil
	}

	pem, err = a.keyLookup.PublicKey(kid)
	if err != nil {
		return "", fmt.Errorf("fetching public key: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache[kid] = pem

	return pem, nil
}

// opaPolicyEvaluation asks opa to evaulate the token against the specified token
// policy and public key.
func (a *Auth) opaPolicyEvaluation(ctx context.Context, opaPolicy string, rule string, input any) error {
	query := fmt.Sprintf("x = data.%s.%s", opaPackage, rule)

	q, err := rego.New(
		rego.Query(query),
		rego.Module("policy.rego", opaPolicy),
	).PrepareForEval(ctx)
	if err != nil {
		return err
	}

	results, err := q.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if len(results) == 0 {
		return errors.New("no results")
	}

	result, ok := results[0].Bindings["x"].(bool)
	if !ok || !result {
		return fmt.Errorf("bindings results[%v] ok[%v]", results, ok)
	}

	return nil
}

// isUserEnabled hits the database and checks the user is not disabled. If the
// no database connection was provided, this check is skipped.
// func (a *Auth) isUserEnabled(ctx context.Context, claims Claims) bool {
// 	if a.user == nil {
// 		return true
// 	}

// 	userID, err := uuid.Parse(claims.Subject)
// 	if err != nil {
// 		return false
// 	}

// 	usr, err := a.user.QueryByID(ctx, userID)
// 	if err != nil {
// 		return false
// 	}

// 	return usr.Enabled
// }
