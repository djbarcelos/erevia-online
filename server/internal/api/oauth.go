package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// provider é um login externo (Google, Discord).
type provider struct {
	name  string
	oauth *oauth2.Config
	// fetchUser busca o id estável (subject) e o nome de exibição com o cliente autenticado.
	fetchUser func(ctx context.Context, client *http.Client) (subject, displayName string, err error)
}

func callbackURL(publicURL, name string) string {
	return publicURL + "/api/auth/" + name + "/callback"
}

// NewGoogle configura o login com Google (escopos mínimos: openid + profile).
func NewGoogle(clientID, secret, publicURL string) *provider {
	return &provider{
		name: "google",
		oauth: &oauth2.Config{
			ClientID: clientID, ClientSecret: secret,
			Endpoint:    google.Endpoint,
			RedirectURL: callbackURL(publicURL, "google"),
			Scopes:      []string{"openid", "profile"},
		},
		fetchUser: func(ctx context.Context, c *http.Client) (string, string, error) {
			var u struct {
				Sub       string `json:"sub"`
				Name      string `json:"name"`
				GivenName string `json:"given_name"`
			}
			if err := getJSON(ctx, c, "https://openidconnect.googleapis.com/v1/userinfo", &u); err != nil {
				return "", "", err
			}
			name := u.GivenName
			if name == "" {
				name = u.Name
			}
			return u.Sub, name, nil
		},
	}
}

// NewDiscord configura o login com Discord (escopo mínimo: identify).
func NewDiscord(clientID, secret, publicURL string) *provider {
	return &provider{
		name: "discord",
		oauth: &oauth2.Config{
			ClientID: clientID, ClientSecret: secret,
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://discord.com/oauth2/authorize",
				TokenURL:  "https://discord.com/api/oauth2/token",
				AuthStyle: oauth2.AuthStyleInParams,
			},
			RedirectURL: callbackURL(publicURL, "discord"),
			Scopes:      []string{"identify"},
		},
		fetchUser: func(ctx context.Context, c *http.Client) (string, string, error) {
			var u struct {
				ID         string `json:"id"`
				Username   string `json:"username"`
				GlobalName string `json:"global_name"`
			}
			if err := getJSON(ctx, c, "https://discord.com/api/users/@me", &u); err != nil {
				return "", "", err
			}
			name := u.GlobalName
			if name == "" {
				name = u.Username
			}
			return u.ID, name, nil
		},
	}
}

func getJSON(ctx context.Context, c *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(out); err != nil {
		return err
	}
	return nil
}

var errNoSubject = errors.New("provider returned empty subject")
