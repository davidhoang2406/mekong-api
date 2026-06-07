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
	"golang.org/x/oauth2/facebook"
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

func (a *App) facebookConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.Cfg.FacebookClientID,
		ClientSecret: a.Cfg.FacebookClientSecret,
		RedirectURL:  a.Cfg.OAuthRedirectBase + "/api/v1/auth/facebook/callback",
		Scopes:       []string{"email", "public_profile"},
		Endpoint:     facebook.Endpoint,
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

// FacebookLogin redirects to Facebook OAuth consent screen.
func (a *App) FacebookLogin(c *gin.Context) {
	if a.Cfg.FacebookClientID == "" {
		abortError(c, http.StatusServiceUnavailable, "Facebook OAuth not configured", "NOT_CONFIGURED")
		return
	}
	state := oauthState()
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, a.facebookConfig().AuthCodeURL(state))
}

// FacebookCallback handles the Facebook OAuth callback.
func (a *App) FacebookCallback(c *gin.Context) {
	if err := validateOAuthState(c); err != nil {
		abortError(c, http.StatusBadRequest, err.Error(), "INVALID_STATE")
		return
	}
	token, err := a.facebookConfig().Exchange(context.Background(), c.Query("code"))
	if err != nil {
		abortError(c, http.StatusBadGateway, "failed to exchange code", "OAUTH_ERROR")
		return
	}
	info, err := fetchFacebookUserInfo(token.AccessToken)
	if err != nil {
		abortError(c, http.StatusBadGateway, "failed to fetch user info", "OAUTH_ERROR")
		return
	}
	user, err := store.FindOrCreateSocialUser(c.Request.Context(), a.PG, "facebook", info["id"].(string), info["email"].(string), fmt.Sprintf("%v", info["name"]))
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

func fetchFacebookUserInfo(accessToken string) (map[string]interface{}, error) {
	resp, err := http.Get("https://graph.facebook.com/me?fields=id,name,email&access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info map[string]interface{}
	return info, json.Unmarshal(body, &info)
}
