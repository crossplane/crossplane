/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package marketplace

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/golang-jwt/jwt/v5"
	"github.com/upbound/up-sdk-go/service/userinfo"
	"golang.org/x/term"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/cmd/crank/internal/upbound"
	"github.com/crossplane/crossplane/cmd/crank/internal/upbound/config"
)

const (
	defaultTimeout     = 30 * time.Second
	loginPath          = "/v1/login"
	defaultProfileName = "default"
)

type loginCmd struct {
	stdin  *os.File
	client *http.Client

	Username string `short:"u" env:"MARKETPLACE_USER" xor:"identifier" help:"Username used to execute command."`
	Password string `short:"p" env:"MARKETPLACE_PASSWORD" help:"Password for specified user. '-' to read from stdin."`
	Token    string `short:"t" env:"MARKETPLACE_TOKEN" xor:"identifier" help:"Token used to execute command. '-' to read from stdin."`

	upbound.Flags `embed:""`
}

// BeforeApply sets default values in login before assignment and validation.
func (c *loginCmd) BeforeApply() error {
	c.stdin = os.Stdin
	return nil
}

func (c *loginCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	c.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: upCtx.InsecureSkipTLSVerify, //nolint:gosec // we need to support insecure connections if requested
			},
		},
	}
	kongCtx.Bind(upCtx)
	if c.Token != "" {
		return nil
	}
	if err := c.setupCredentials(); err != nil {
		return errors.Wrapf(err, "failed to get credentials")
	}
	return nil
}

// Run executes the login command.
func (c *loginCmd) Run(k *kong.Context, upCtx *upbound.Context) error { //nolint:gocyclo // TODO(phisco): refactor
	auth, profType, err := constructAuth(c.Username, c.Token, c.Password)
	if err != nil {
		return errors.Wrap(err, "failed to construct auth")
	}
	jsonStr, err := json.Marshal(auth)
	if err != nil {
		return errors.Wrap(err, "failed to marshal auth")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	loginEndpoint := *upCtx.APIEndpoint
	loginEndpoint.Path = loginPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginEndpoint.String(), bytes.NewReader(jsonStr))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send request")
	}
	defer res.Body.Close() //nolint:errcheck // we don't care about the error here
	session, err := extractSession(res, upbound.CookieName)
	if err != nil {
		return errors.Wrap(err, "failed to extract session")
	}

	// Set session early so that it can be used to fetch user info if necessary.
	upCtx.Profile.Session = session

	// If the default account is not set, the user's personal account is used.
	if upCtx.Account == "" {
		conf, err := upCtx.BuildSDKConfig()
		if err != nil {
			return errors.Wrap(err, "failed to build SDK config")
		}
		info, err := userinfo.NewClient(conf).Get(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get user info")
		}
		upCtx.Account = info.User.Username
	}

	// If profile name was not provided and no default exists, set name to 'default'.
	if upCtx.ProfileName == "" {
		upCtx.ProfileName = defaultProfileName
	}

	upCtx.Profile.ID = auth.ID
	upCtx.Profile.Type = profType
	upCtx.Profile.Account = upCtx.Account

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return errors.Wrap(err, "failed to add or update profile")
	}
	if err := upCtx.Cfg.SetDefaultUpboundProfile(upCtx.ProfileName); err != nil {
		return errors.Wrap(err, "failed to set default profile")
	}
	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, "failed to update config")
	}
	fmt.Fprintln(k.Stdout, "Login successful.")
	return nil
}

func (c *loginCmd) setupCredentials() error {
	if c.Token == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.Token = strings.TrimSpace(string(b))
	}
	if c.Password == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.Password = strings.TrimSpace(string(b))
	}
	if c.Token == "" {
		if c.Username == "" {
			username, err := getUsername(c.stdin)
			if err != nil {
				return errors.Wrap(err, "failed to get username")
			}
			c.Username = username
		}
		if c.Password == "" {
			password, err := getPassword(c.stdin)
			if err != nil {
				return errors.Wrap(err, "failed to get password")
			}
			c.Password = password
		}
	}
	return nil
}

func getPassword(f *os.File) (string, error) {
	if !term.IsTerminal(int(f.Fd())) {
		return "", errors.New("not a terminal")
	}
	fmt.Fprintf(f, "Password: ")
	password, err := term.ReadPassword(int(f.Fd()))
	if err != nil {
		return "", err
	}
	// Print a new line because ReadPassword does not.
	_, _ = fmt.Fprintf(f, "\n")
	return string(password), nil

}
func getUsername(f *os.File) (string, error) {
	if !term.IsTerminal(int(f.Fd())) {
		return "", errors.New("not a terminal")
	}
	fmt.Fprintf(f, "Username: ")
	reader := bufio.NewReader(f)
	s, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// auth is the request body sent to authenticate a user or token.
type auth struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// constructAuth constructs the body of an Upbound Cloud authentication request
// given the provided credentials.
func constructAuth(username, token, password string) (*auth, config.ProfileType, error) {
	if username == "" && token == "" {
		return nil, "", errors.New("no user or token provided")
	}
	id, profType, err := parseID(username, token)
	if err != nil {
		return nil, "", err
	}
	if profType == config.TokenProfileType {
		password = token
	}
	return &auth{
		ID:       id,
		Password: password,
		Remember: true,
	}, profType, nil
}

// parseID gets a user ID by either parsing a token or returning the username.
func parseID(user, token string) (string, config.ProfileType, error) {
	if token != "" {
		p := jwt.Parser{}
		claims := &jwt.RegisteredClaims{}
		_, _, err := p.ParseUnverified(token, claims)
		if err != nil {
			return "", "", err
		}
		if claims.ID == "" {
			return "", "", errors.New("no id in token")
		}
		return claims.ID, config.TokenProfileType, nil
	}
	return user, config.UserProfileType, nil
}

// extractSession extracts the specified cookie from an HTTP response. The
// caller is responsible for closing the response body.
func extractSession(res *http.Response, cookieName string) (string, error) {
	for _, cook := range res.Cookies() {
		if cook.Name == cookieName {
			return cook.Value, nil
		}
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read body")
	}
	return "", errors.Errorf("failed to read cookie format: %v", string(b))
}
