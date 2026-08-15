package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Templetry/engine/manifest"
	"github.com/Templetry/engine/planner"
	"github.com/Templetry/engine/render"
	"github.com/Templetry/engine/verify"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Verify events the frontend subscribes to.
const (
	verifyLogEvent  = "verify:log"
	verifyDoneEvent = "verify:done"
)

// VerifyInfo tells the UI whether a form can be verified at all, so the
// button can explain itself instead of failing after a click.
type VerifyInfo struct {
	// Available is true when the form declares a verify block.
	Available bool   `json:"available"`
	Image     string `json:"image,omitempty"`
	Run       string `json:"run,omitempty"`
	// Reason explains an unavailable verify: no block, or no Docker.
	Reason string `json:"reason,omitempty"`
}

// GetVerifyInfo reports whether the form declares a verify block and whether
// this machine can run it (ADR-0004: verification happens in containers, so
// no toolchain has to be installed).
func (a *App) GetVerifyInfo(cat, ref string) (VerifyInfo, error) {
	b, err := a.fetchBundle(cat, ref)
	if err != nil {
		return VerifyInfo{}, err
	}
	v := b.manifest.Verify
	if v == nil || v.Image == "" || v.Run == "" {
		return VerifyInfo{Reason: "this form declares no verify block"}, nil
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return VerifyInfo{
			Image: v.Image, Run: v.Run,
			Reason: "verify runs the build in a container — Docker was not found on this machine",
		}, nil
	}
	return VerifyInfo{Available: true, Image: v.Image, Run: v.Run}, nil
}

// StartVerify renders the current inputs into a temporary directory and runs
// the form's verify command in Docker, streaming output to the UI. It
// returns as soon as the container starts; completion arrives as a
// "verify:done" event.
//
// There is no cancel: the engine's verify.Run has no context to cancel, and
// reimplementing it here to gain one would put a second copy of the docker
// invocation in the app (ADR-0012 embeds the engine, it does not restate it).
func (a *App) StartVerify(cat, ref string, vars map[string]string, feats map[string]bool) (string, error) {
	a.mu.Lock()
	busy := a.verifying
	a.mu.Unlock()
	if busy {
		return "", fmt.Errorf("a verify is already running")
	}

	info, err := a.GetVerifyInfo(cat, ref)
	if err != nil {
		return "", err
	}
	if !info.Available {
		return "", fmt.Errorf("%s", info.Reason)
	}

	b, err := a.fetchBundle(cat, ref)
	if err != nil {
		return "", err
	}
	p, err := planner.Build(b.manifest, manifest.Inputs{Variables: vars, Features: feats}, b.files)
	if err != nil {
		return "", err
	}
	result, err := render.Apply(p, b.files)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "templetry-verify-")
	if err != nil {
		return "", err
	}
	if err := render.WriteDir(result, dir); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	a.mu.Lock()
	a.verifying = true
	a.mu.Unlock()

	go func() {
		defer os.RemoveAll(dir)
		defer func() {
			a.mu.Lock()
			a.verifying = false
			a.mu.Unlock()
		}()
		w := &eventWriter{app: a}
		err := verify.Run(info.Image, info.Run, dir, w, w)
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		wruntime.EventsEmit(a.ctx, verifyDoneEvent, map[string]any{
			"ok":    err == nil,
			"error": msg,
		})
	}()

	return fmt.Sprintf("Running %s in %s…", info.Run, info.Image), nil
}

// eventWriter streams a container's output to the UI as it arrives. Chunks
// are emitted as they come rather than split into lines: a build log is
// appended verbatim, and waiting for newlines would stall progress behind
// tools that write without them.
type eventWriter struct{ app *App }

func (w *eventWriter) Write(p []byte) (int, error) {
	wruntime.EventsEmit(w.app.ctx, verifyLogEvent, string(p))
	return len(p), nil
}
