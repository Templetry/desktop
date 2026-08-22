package main

import (
	"strings"
	"testing"
)

func TestParseDeviceStart(t *testing.T) {
	ok := map[string]any{
		"device_code": "dc", "user_code": "ABCD-EFGH",
		"verification_uri": "https://gitlab.com/oauth/device",
		"interval":         float64(5), "expires_in": float64(600),
	}

	t.Run("reads a normal response", func(t *testing.T) {
		d, err := parseDeviceStart(ok, "gitlab.com")
		if err != nil {
			t.Fatal(err)
		}
		if d.deviceCode != "dc" || d.userCode != "ABCD-EFGH" {
			t.Errorf("got %+v", d)
		}
	})

	// The complete URI carries the code already filled in, which saves the
	// user typing it into a browser.
	t.Run("prefers the URI with the code already in it", func(t *testing.T) {
		with := map[string]any{}
		for k, v := range ok {
			with[k] = v
		}
		with["verification_uri_complete"] = "https://gitlab.com/oauth/device?user_code=ABCD-EFGH"
		d, _ := parseDeviceStart(with, "gitlab.com")
		if !strings.Contains(d.verURI, "user_code=") {
			t.Errorf("verURI = %q", d.verURI)
		}
	})

	// Polling faster than the forge allows gets the client slowed down or
	// cut off, so the floor matters more than the forge's suggestion.
	t.Run("never polls faster than every five seconds", func(t *testing.T) {
		fast := map[string]any{}
		for k, v := range ok {
			fast[k] = v
		}
		fast["interval"] = float64(1)
		d, _ := parseDeviceStart(fast, "gitlab.com")
		if d.interval != 5 {
			t.Errorf("interval = %v", d.interval)
		}
	})

	t.Run("supplies a deadline when none is given", func(t *testing.T) {
		bare := map[string]any{"device_code": "dc", "user_code": "u"}
		d, _ := parseDeviceStart(bare, "gitlab.com")
		if d.expires != 600 {
			t.Errorf("expires = %v", d.expires)
		}
	})

	// The whole point of the message: a new GitLab application has the
	// device grant off, and the response alone never says so.
	t.Run("names the likeliest cause when the forge refuses", func(t *testing.T) {
		_, err := parseDeviceStart(map[string]any{
			"error": "unauthorized_client", "error_description": "grant type is not allowed",
		}, "gitlab.com")
		if err == nil {
			t.Fatal("expected an error")
		}
		for _, want := range []string{"gitlab.com", "grant type is not allowed", "device authorization grant"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error missing %q: %v", want, err)
			}
		}
	})

	t.Run("still explains an empty response", func(t *testing.T) {
		_, err := parseDeviceStart(map[string]any{}, "gitlab.com")
		if err == nil || !strings.Contains(err.Error(), "device authorization grant") {
			t.Errorf("got %v", err)
		}
	})
}

func TestNormalizeHost(t *testing.T) {
	for _, in := range []string{"gitlab.com", "https://gitlab.com", "http://gitlab.com/", " gitlab.com "} {
		if got := normalizeHost(in); got != "gitlab.com" {
			t.Errorf("normalizeHost(%q) = %q", in, got)
		}
	}
}

// gitlab.com is the one instance an application can be shipped for; Gitea
// implements no device flow at all.
func TestDeviceLoginAvailability(t *testing.T) {
	if !supportsDeviceFlow("gitlab", "gitlab.com") {
		t.Error("gitlab.com should support device login now that an application is configured")
	}
	if supportsDeviceFlow("gitea", "codeberg.org") {
		t.Error("gitea must never claim device login")
	}
}
