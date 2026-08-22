package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
)

// DeviceLogin is a forge sign-in in progress. One at a time: signing into two
// forges at once is not a thing anyone does, and a single status is far easier
// to reason about than a map keyed by host.
type DeviceLogin struct {
	State           string `json:"state"` // idle | pending | done | error
	Scheme          string `json:"scheme,omitempty"`
	Host            string `json:"host,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
	VerificationURI string `json:"verificationUri,omitempty"`
	Login           string `json:"login,omitempty"`
	Error           string `json:"error,omitempty"`
}

// gitlabScopes is what Templetry actually does with the token: read and
// create projects through the API, and push over HTTPS. Asking for more
// would be asking someone to grant access nothing here uses.
const gitlabScopes = "api write_repository"

func (a *App) setDeviceLogin(s DeviceLogin) {
	a.mu.Lock()
	a.deviceLogin = s
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "device-login", s)
}

// DeviceLoginStatus is the current state of a forge sign-in.
func (a *App) DeviceLoginStatus() DeviceLogin {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.deviceLogin
}

// SupportsDeviceLogin reports whether a host can be signed into without
// pasting a token, so the UI can offer the button only where it works.
func (a *App) SupportsDeviceLogin(scheme, host string) bool {
	return supportsDeviceFlow(strings.TrimSpace(scheme), normalizeHost(host))
}

func normalizeHost(host string) string {
	h := strings.TrimSpace(host)
	h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	return strings.TrimSuffix(h, "/")
}

// StartDeviceLogin begins the OAuth device flow against a forge, opens the
// verification page, and polls in the background. GitHub has its own entry
// point because its session is the app's primary identity, not one account
// among several.
func (a *App) StartDeviceLogin(scheme, host string) (DeviceLogin, error) {
	scheme, host = strings.TrimSpace(scheme), normalizeHost(host)
	if scheme != "gitlab" {
		// Gitea implements only the authorization code grant; Forgejo's
		// device flow exists but has no application we can present.
		return DeviceLogin{}, fmt.Errorf("%s signs in with a personal access token", scheme)
	}
	id := oauthClientID(scheme, host)
	if id == "" {
		return DeviceLogin{}, fmt.Errorf(
			"no OAuth application is configured for %s — add its application id in Settings, or sign in with a token", host)
	}

	res, err := postForm(deviceEndpoint(scheme, host), url.Values{
		"client_id": {id},
		"scope":     {gitlabScopes},
	})
	if err != nil {
		return DeviceLogin{}, fmt.Errorf("starting sign-in with %s: %w", host, err)
	}
	d, err := parseDeviceStart(res, host)
	if err != nil {
		return DeviceLogin{}, err
	}
	deviceCode, userCode, verURI := d.deviceCode, d.userCode, d.verURI
	interval, expires := d.interval, d.expires

	s := DeviceLogin{State: "pending", Scheme: scheme, Host: host, UserCode: userCode, VerificationURI: verURI}
	a.setDeviceLogin(s)
	runtime.BrowserOpenURL(a.ctx, verURI)

	go a.pollDeviceLogin(scheme, host, id, deviceCode, interval, time.Now().Add(time.Duration(expires)*time.Second))
	return s, nil
}

func (a *App) pollDeviceLogin(scheme, host, clientID, deviceCode string, interval float64, deadline time.Time) {
	for {
		time.Sleep(time.Duration(interval) * time.Second)
		if time.Now().After(deadline) {
			a.setDeviceLogin(DeviceLogin{State: "error", Scheme: scheme, Host: host,
				Error: "the sign-in code expired — start again"})
			return
		}

		res, err := postForm(tokenEndpoint(scheme, host), url.Values{
			"client_id":   {clientID},
			"device_code": {deviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		})
		if err != nil {
			continue // a blip on the network is not a refusal
		}

		var t oauthTokens
		b, _ := json.Marshal(res)
		_ = json.Unmarshal(b, &t)

		if t.AccessToken != "" {
			if err := a.finishDeviceLogin(scheme, host, t.credential(time.Now())); err != nil {
				a.setDeviceLogin(DeviceLogin{State: "error", Scheme: scheme, Host: host, Error: err.Error()})
			}
			return
		}

		switch t.Error {
		case "authorization_pending", "":
			continue
		case "slow_down":
			interval += 5
		case "expired_token", "access_denied":
			msg := t.ErrorDesc
			if msg == "" {
				msg = t.Error
			}
			a.setDeviceLogin(DeviceLogin{State: "error", Scheme: scheme, Host: host, Error: msg})
			return
		default:
			a.setDeviceLogin(DeviceLogin{State: "error", Scheme: scheme, Host: host, Error: t.Error})
			return
		}
	}
}

// finishDeviceLogin turns a granted token into a signed-in account: who it
// belongs to, then stored the same way an account added with a token is.
func (a *App) finishDeviceLogin(scheme, host string, c storedCredential) error {
	acc, err := forgeWhoami(scheme, host, c.Access)
	if err != nil {
		return fmt.Errorf("signed in to %s but could not read the account: %w", host, err)
	}
	if err := keyring.Set(keyringService, acc.Key(), c.encode()); err != nil {
		return fmt.Errorf("could not store the %s session: %w", host, err)
	}

	cfg := loadConfig()
	replaced := false
	for i, existing := range cfg.Accounts {
		if existing.Key() == acc.Key() {
			cfg.Accounts[i] = acc
			replaced = true
		}
	}
	if !replaced {
		cfg.Accounts = append(cfg.Accounts, acc)
	}
	saveConfig(cfg)

	a.setDeviceLogin(DeviceLogin{State: "done", Scheme: scheme, Host: host, Login: acc.Login})
	return nil
}

// deviceStart is what a forge answers when a device flow begins.
type deviceStart struct {
	deviceCode string
	userCode   string
	verURI     string
	interval   float64
	expires    float64
}

// parseDeviceStart reads that answer, or explains why there is not one.
func parseDeviceStart(res map[string]any, host string) (deviceStart, error) {
	d := deviceStart{}
	d.deviceCode, _ = res["device_code"].(string)
	d.userCode, _ = res["user_code"].(string)
	// The complete URI carries the code already filled in, which saves the
	// user typing it. Not every forge sends one.
	d.verURI, _ = res["verification_uri_complete"].(string)
	if d.verURI == "" {
		d.verURI, _ = res["verification_uri"].(string)
	}

	if d.deviceCode == "" || d.userCode == "" {
		msg, _ := res["error_description"].(string)
		if msg == "" {
			msg, _ = res["error"].(string)
		}
		if msg == "" {
			msg = "no device code in the response"
		}
		// By far the likeliest cause, and impossible to tell from the
		// response alone: the grant is off by default on a new application.
		return d, fmt.Errorf(
			"%s refused the sign-in (%s). Check that the OAuth application has the device authorization grant enabled", host, msg)
	}

	// A client that polls faster than the forge allows gets slowed down or
	// cut off, so the floor matters more than the forge's suggestion.
	d.interval, _ = res["interval"].(float64)
	if d.interval < 5 {
		d.interval = 5
	}
	d.expires, _ = res["expires_in"].(float64)
	if d.expires <= 0 {
		d.expires = 600
	}
	return d, nil
}
