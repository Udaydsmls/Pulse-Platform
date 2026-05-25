package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthUserInfo contains the normalized user profile returned by an OAuth provider.
type OAuthUserInfo struct {
	ID       string
	Email    string
	Name     string
	Provider string
}

// OAuthProvider defines the interface for exchanging an authorization code for user info.
type OAuthProvider interface {
	ExchangeCode(ctx context.Context, code string) (*OAuthUserInfo, error)
}

// GoogleProvider implements OAuthProvider for Google OAuth2.
type GoogleProvider struct {
	config *oauth2.Config
}

// NewGoogleProvider creates a GoogleProvider with the given OAuth2 credentials.
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// ExchangeCode exchanges a Google authorization code for user profile information.
func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google token exchange: %w", err)
	}

	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read google userinfo response: %w", err)
	}

	var info struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse google userinfo: %w", err)
	}

	return &OAuthUserInfo{
		ID:       info.ID,
		Email:    info.Email,
		Name:     info.Name,
		Provider: "google",
	}, nil
}

// GithubProvider implements OAuthProvider for GitHub OAuth2.
type GithubProvider struct {
	config     *oauth2.Config
	httpClient *http.Client
}

// NewGithubProvider creates a GithubProvider with the given OAuth2 credentials.
func NewGithubProvider(clientID, clientSecret, redirectURL string) *GithubProvider {
	return &GithubProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     github.Endpoint,
		},
		httpClient: http.DefaultClient,
	}
}

// ExchangeCode exchanges a GitHub authorization code for user profile information.
func (p *GithubProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("github token exchange: %w", err)
	}

	client := p.config.Client(ctx, token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("create github user request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read github user response: %w", err)
	}

	var user struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse github user: %w", err)
	}

	name := user.Name
	if name == "" {
		name = user.Login
	}

	return &OAuthUserInfo{
		ID:       fmt.Sprintf("%d", user.ID),
		Email:    user.Email,
		Name:     name,
		Provider: "github",
	}, nil
}
