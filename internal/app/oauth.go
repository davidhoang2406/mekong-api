package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	githubOAuth "golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"github.com/davidhoang2406/mekong-api/internal/store"
)

func (a *App) googleConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.Cfg.GoogleClientID,
		ClientSecret: a.Cfg.GoogleClientSecret,
		RedirectURL:  a.Cfg.OAuthRedirectBase + "/api/v1/auth/google/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (a *App) githubConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.Cfg.GitHubClientID,
		ClientSecret: a.Cfg.GitHubClientSecret,
		RedirectURL:  a.Cfg.OAuthRedirectBase + "/api/v1/auth/github/callback",
		Scopes:       []string{"user:email"},
		Endpoint:     githubOAuth.Endpoint,
	}
}

func oauthState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (a *App) issueJWT(userID, email string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"exp":   time.Now().Add(time.Duration(a.Cfg.JWTExpiryHours) * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	return token.SignedString([]byte(a.Cfg.JWTSecret))
}

// GoogleLogin redirects to Google OAuth consent screen.
func (a *App) GoogleLogin(c *gin.Context) {
	if a.Cfg.GoogleClientID == "" {
		abortError(c, http.StatusServiceUnavailable, "Google OAuth not configured", "NOT_CONFIGURED")
		return
	}
	state := oauthState()
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, a.googleConfig().AuthCodeURL(state))
}

// GoogleCallback handles the Google OAuth callback.
func (a *App) GoogleCallback(c *gin.Context) {
	if err := validateOAuthState(c); err != nil {
		abortError(c, http.StatusBadRequest, err.Error(), "INVALID_STATE")
		return
	}
	token, err := a.googleConfig().Exchange(context.Background(), c.Query("code"))
	if err != nil {
		abortError(c, http.StatusBadGateway, "failed to exchange code", "OAUTH_ERROR")
		return
	}
	info, err := fetchGoogleUserInfo(token.AccessToken)
	if err != nil {
		abortError(c, http.StatusBadGateway, "failed to fetch user info", "OAUTH_ERROR")
		return
	}
	user, err := store.FindOrCreateSocialUser(c.Request.Context(), a.PG, "google", info["sub"].(string), info["email"].(string), fmt.Sprintf("%v", info["name"]))
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	jwt, err := a.issueJWT(user.ID, user.Email)
	if err != nil {
		abortError(c, http.StatusInternalServerError, "token signing failed", "INTERNAL_ERROR")
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, a.Cfg.OAuthRedirectBase+"/auth/callback?token="+jwt)
}

// GitHubLogin redirects to GitHub OAuth consent screen.
func (a *App) GitHubLogin(c *gin.Context) {
	if a.Cfg.GitHubClientID == "" {
		abortError(c, http.StatusServiceUnavailable, "GitHub OAuth not configured", "NOT_CONFIGURED")
		return
	}
	state := oauthState()
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, a.githubConfig().AuthCodeURL(state))
}

// GitHubCallback handles the GitHub OAuth callback.
func (a *App) GitHubCallback(c *gin.Context) {
	if err := validateOAuthState(c); err != nil {
		abortError(c, http.StatusBadRequest, err.Error(), "INVALID_STATE")
		return
	}
	token, err := a.githubConfig().Exchange(context.Background(), c.Query("code"))
	if err != nil {
		abortError(c, http.StatusBadGateway, "failed to exchange code", "OAUTH_ERROR")
		return
	}
	info, err := fetchGitHubUserInfo(token.AccessToken)
	if err != nil {
		abortError(c, http.StatusBadGateway, "failed to fetch user info", "OAUTH_ERROR")
		return
	}
	providerID := fmt.Sprintf("%v", info["id"])
	email, _ := info["email"].(string)
	name, _ := info["name"].(string)
	if name == "" {
		name, _ = info["login"].(string)
	}
	if email == "" {
		email, _ = info["login"].(string)
	}
	user, err := store.FindOrCreateSocialUser(c.Request.Context(), a.PG, "github", providerID, email, name)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	jwt, err := a.issueJWT(user.ID, user.Email)
	if err != nil {
		abortError(c, http.StatusInternalServerError, "token signing failed", "INTERNAL_ERROR")
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, a.Cfg.OAuthRedirectBase+"/auth/callback?token="+jwt)
}

func validateOAuthState(c *gin.Context) error {
	cookie, err := c.Cookie("oauth_state")
	if err != nil || cookie != c.Query("state") {
		return fmt.Errorf("invalid OAuth state")
	}
	return nil
}

func fetchGoogleUserInfo(accessToken string) (map[string]interface{}, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v3/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info map[string]interface{}
	return info, json.Unmarshal(body, &info)
}

func fetchGitHubUserInfo(accessToken string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info map[string]interface{}
	return info, json.Unmarshal(body, &info)
}
