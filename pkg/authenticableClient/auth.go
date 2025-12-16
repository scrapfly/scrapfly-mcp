package authenticableClient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/go-scrapfly"
	scrapflyprovider "github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly"
)

var digestRegex = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

func IsDigestRegex(s string) bool {
	return digestRegex.MatchString(s)
}

type TokenInfo struct {
	Client *scrapfly.Client
	ApiKey string
}

var ErrInvalidToken = errors.New("invalid token")

type TokenVerifier func(ctx context.Context, req *http.Request) (*TokenInfo, error)

type tokenInfoKey struct{}

func TokenInfoFromContext(ctx context.Context) *TokenInfo {
	ti := ctx.Value(tokenInfoKey{})
	if ti == nil {
		return nil
	}
	return ti.(*TokenInfo)
}

func TokenInfoFromRequest(req *http.Request) *TokenInfo {
	return TokenInfoFromContext(req.Context())
}

func apikeyVerifier(ctx context.Context, req *http.Request) (*TokenInfo, error) {
	apiKey := parseForToken(req)
	return tokenInfoFromApiKey(apiKey)
}

func tokenInfoFromApiKey(apiKey string) (*TokenInfo, error) {
	client, err := scrapfly.New(apiKey)
	if err != nil {
		return nil, fmt.Errorf("%w: API key not found", auth.ErrInvalidToken)
	}
	_, err = client.VerifyAPIKey()
	if err != nil {
		return nil, fmt.Errorf("%w: Invalid Credentials: %s", auth.ErrInvalidToken, err.Error())
	}
	return &TokenInfo{
			ApiKey: apiKey,
			Client: client,
		},
		nil
}

func RequireBearerToken(apikeyVerifier TokenVerifier) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenInfo, errmsg, code := verify(r, apikeyVerifier)
			if code != 0 {
				http.Error(w, errmsg, code)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), tokenInfoKey{}, tokenInfo))
			handler.ServeHTTP(w, r)
		})
	}
}

func parseForToken(req *http.Request) string {
	token := req.URL.Query().Get("key")
	if token != "" {
		return token
	}
	token = req.URL.Query().Get("apiKey")
	if token != "" {
		return token
	}
	authHeader := req.Header.Get("Authorization")
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || strings.ToLower(fields[0]) != "bearer" {
		return ""
	}
	return fields[1]
}

func verify(req *http.Request, apikeyVerifier TokenVerifier) (_ *TokenInfo, errmsg string, code int) {

	token := parseForToken(req)
	if token == "" {
		return nil, "no api key", http.StatusUnauthorized
	}

	if !strings.HasPrefix(token, "scp-") && !IsDigestRegex(token) {
		return nil, fmt.Errorf("invalid token").Error(), http.StatusUnauthorized
	}

	tokenInfo, err := apikeyVerifier(req.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return nil, err.Error(), http.StatusUnauthorized
		}
		return nil, err.Error(), http.StatusInternalServerError
	}
	return tokenInfo, "", 0
}

func CorsMiddleware(handler http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, *")
			w.Header().Set("Access-Control-Expose-Headers", "mcp-session-id, mcp-protocol-version")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			handler.ServeHTTP(w, r)
		})
	}(handler)
}

// Middleware to verify Scrapfly API key
func ScrapflyAuthMiddleware(handler *mcp.StreamableHTTPHandler) http.Handler {
	return RequireBearerToken(apikeyVerifier)(handler)
}
func GetStreamableScrapflyClient(p *scrapflyprovider.ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
	TokenInfo := TokenInfoFromContext(ctx)
	if TokenInfo == nil {
		return nil, fmt.Errorf("client not found (missing token info)")
	}
	client := TokenInfo.Client
	if client == nil {
		return nil, fmt.Errorf("client not found")
	}
	return client, nil
}

func CorsAndAuthenticatedStreamableServerFunction(mcpHandler *mcp.StreamableHTTPHandler, httpAddr *string) {
	authenticationHandler := ScrapflyAuthMiddleware(mcpHandler)
	corsHandler := CorsMiddleware(authenticationHandler)
	http.HandleFunc("/mcp", corsHandler.ServeHTTP)
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}
