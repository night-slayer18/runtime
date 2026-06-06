// Package plugin: security sandbox and integrity verification.
//
// This file layers a capability-restricted execution model and signature /
// checksum verification on top of the plugin registry defined in plugin.go.
//
// # Honest limitations of in-process sandboxing
//
// Go cannot truly sandbox code that runs in the same process. A plugin compiled
// into (or linked with) the host shares the host's address space, file
// descriptors, and OS privileges, so a determined plugin can always reach the
// standard library directly (os.Open, net.Dial, os/exec, unsafe, etc.). There
// is no in-process mechanism that can revoke those capabilities once the code
// is running in the same process.
//
// What this package therefore provides is a *capability/guard model*, not an OS
// sandbox:
//
//   - Plugins are expected to perform all privileged work through the
//     capability-mediated API surface (Capabilities) handed to them, rather
//     than calling the standard library directly.
//   - The Capabilities guard intercepts those mediated operations and denies
//     filesystem, network, and process access unless the host explicitly
//     granted that capability when constructing the sandbox.
//   - Plugin entry points run in isolated goroutines with panic recovery so a
//     misbehaving plugin cannot crash the host.
//
// True isolation (separate process, seccomp/landlock, WASM, or a container)
// must be layered underneath this model by the host if it loads untrusted
// third-party code. This package makes the *policy* explicit and enforceable at
// the API boundary; it does not, and cannot, enforce it against code that
// bypasses the boundary in-process.
package plugin

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Capability identifies a privileged operation class a plugin may request.
type Capability string

const (
	// CapFileSystem covers reading or writing the filesystem.
	CapFileSystem Capability = "filesystem"
	// CapNetwork covers opening network connections.
	CapNetwork Capability = "network"
	// CapProcess covers spawning or signalling OS processes.
	CapProcess Capability = "process"
)

// Sandbox errors. Callers can match these with errors.Is.
var (
	// ErrCapabilityDenied is returned when a plugin attempts a restricted
	// operation it was not granted.
	ErrCapabilityDenied = errors.New("plugin: capability denied")
	// ErrPluginPanic is returned when a plugin entry point panics; the
	// recovered value is wrapped in the error message.
	ErrPluginPanic = errors.New("plugin: panic recovered")
	// ErrSignatureInvalid is returned when signature/checksum verification
	// fails for a plugin's bytes.
	ErrSignatureInvalid = errors.New("plugin: signature verification failed")
	// ErrNotVerified is returned when a plugin is run through the sandbox
	// before its integrity has been verified.
	ErrNotVerified = errors.New("plugin: integrity not verified")
)

// Capabilities is the guard handed to a plugin. Every privileged operation a
// plugin performs is expected to flow through these methods so the host can
// allow or deny it according to the policy set when the sandbox was built.
//
// See the package-level note on in-process limitations: this guard enforces
// policy only for operations routed through it, not for direct standard-library
// calls.
type Capabilities interface {
	// OpenFile mediates filesystem access. It returns ErrCapabilityDenied
	// unless CapFileSystem was granted.
	OpenFile(path string) error
	// Dial mediates network access. It returns ErrCapabilityDenied unless
	// CapNetwork was granted.
	Dial(network, address string) error
	// StartProcess mediates process execution. It returns ErrCapabilityDenied
	// unless CapProcess was granted.
	StartProcess(name string, args ...string) error
	// Allowed reports whether a capability has been granted.
	Allowed(c Capability) bool
}

// guard is the default Capabilities implementation. By design it denies every
// capability that was not explicitly granted (deny-by-default / fail-closed).
type guard struct {
	granted map[Capability]bool
}

func newGuard(grants []Capability) *guard {
	g := &guard{granted: make(map[Capability]bool, len(grants))}
	for _, c := range grants {
		g.granted[c] = true
	}
	return g
}

func (g *guard) Allowed(c Capability) bool { return g.granted[c] }

func (g *guard) deny(c Capability, detail string) error {
	return fmt.Errorf("%w: %s (%s)", ErrCapabilityDenied, c, detail)
}

func (g *guard) OpenFile(path string) error {
	if !g.granted[CapFileSystem] {
		return g.deny(CapFileSystem, "open "+path)
	}
	return nil
}

func (g *guard) Dial(network, address string) error {
	if !g.granted[CapNetwork] {
		return g.deny(CapNetwork, network+" "+address)
	}
	return nil
}

func (g *guard) StartProcess(name string, args ...string) error {
	if !g.granted[CapProcess] {
		return g.deny(CapProcess, "exec "+name)
	}
	return nil
}

// Verifier validates plugin integrity before execution. Two stdlib-backed
// strategies are supported and may be combined:
//
//   - An ed25519 public key verifies a detached signature over the plugin
//     bytes (authenticity + integrity).
//   - A SHA-256 checksum from a trusted manifest verifies integrity against a
//     known-good digest.
//
// At least one strategy must be configured, otherwise Verify fails closed.
type Verifier struct {
	// PublicKey, when set, is used to verify an ed25519 signature over the
	// plugin bytes.
	PublicKey ed25519.PublicKey
	// Checksum, when non-empty, is the trusted SHA-256 digest the plugin bytes
	// must match.
	Checksum []byte
}

// Verify reports whether data satisfies the configured integrity checks. When a
// public key is configured, sig must be a valid ed25519 signature over data.
// When a checksum is configured, the SHA-256 of data must equal it. If both are
// configured, both must pass.
//
// Verify fails closed: with no strategy configured it returns ErrNotVerified.
func (v Verifier) Verify(data, sig []byte) error {
	checked := false

	if len(v.PublicKey) > 0 {
		if len(v.PublicKey) != ed25519.PublicKeySize {
			return fmt.Errorf("%w: malformed public key", ErrSignatureInvalid)
		}
		if !ed25519.Verify(v.PublicKey, data, sig) {
			return fmt.Errorf("%w: ed25519 signature mismatch", ErrSignatureInvalid)
		}
		checked = true
	}

	if len(v.Checksum) > 0 {
		sum := sha256.Sum256(data)
		// Constant-time compare to avoid leaking digest bytes via timing.
		if subtle.ConstantTimeCompare(sum[:], v.Checksum) != 1 {
			return fmt.Errorf("%w: sha-256 checksum mismatch", ErrSignatureInvalid)
		}
		checked = true
	}

	if !checked {
		return ErrNotVerified
	}
	return nil
}

// Checksum returns the SHA-256 digest of data. Hosts use it to build a trusted
// manifest of known-good plugin digests.
func Checksum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// Sandbox runs verified plugin code under a capability guard, isolating each
// entry point in its own goroutine with panic recovery.
//
// A Sandbox is created per plugin via NewSandbox, verified once via Verify, and
// then used to run the plugin's entry points. Run refuses to execute until
// verification has succeeded.
type Sandbox struct {
	caps     *guard
	verifier Verifier
	timeout  time.Duration

	mu       sync.RWMutex
	verified bool
}

// SandboxOption configures a Sandbox.
type SandboxOption func(*Sandbox)

// WithCapabilities grants the listed capabilities. Anything not listed is
// denied. With no option the sandbox denies filesystem, network, and process
// access entirely.
func WithCapabilities(caps ...Capability) SandboxOption {
	return func(s *Sandbox) { s.caps = newGuard(caps) }
}

// WithVerifier sets the integrity verifier used by Verify.
func WithVerifier(v Verifier) SandboxOption {
	return func(s *Sandbox) { s.verifier = v }
}

// WithTimeout bounds how long a single Run may take before it is reported as
// timed out. A non-positive timeout means no limit.
func WithTimeout(d time.Duration) SandboxOption {
	return func(s *Sandbox) { s.timeout = d }
}

// NewSandbox builds a sandbox. By default it denies all capabilities and has no
// timeout; use options to grant capabilities and configure verification.
func NewSandbox(opts ...SandboxOption) *Sandbox {
	s := &Sandbox{caps: newGuard(nil)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Capabilities returns the guard the sandbox enforces. Plugins route privileged
// operations through it.
func (s *Sandbox) Capabilities() Capabilities { return s.caps }

// Verify checks the plugin bytes against the configured Verifier and, on
// success, marks the sandbox as cleared to run. It must be called before Run.
func (s *Sandbox) Verify(data, sig []byte) error {
	if err := s.verifier.Verify(data, sig); err != nil {
		return err
	}
	s.mu.Lock()
	s.verified = true
	s.mu.Unlock()
	return nil
}

// Verified reports whether the sandbox has passed integrity verification.
func (s *Sandbox) Verified() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.verified
}

// Run executes fn in an isolated goroutine with panic recovery. The plugin
// receives the capability guard so any privileged work is mediated by policy.
//
// Run returns ErrNotVerified if the sandbox has not been verified, ErrPluginPanic
// if fn panics, the deadline error if the configured timeout elapses, or
// whatever error fn returns.
func (s *Sandbox) Run(fn func(caps Capabilities) error) error {
	s.mu.RLock()
	verified := s.verified
	s.mu.RUnlock()
	if !verified {
		return ErrNotVerified
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("%w: %v", ErrPluginPanic, r)
			}
		}()
		done <- fn(s.caps)
	}()

	if s.timeout <= 0 {
		return <-done
	}

	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		// The goroutine is left to finish on its own; in-process Go code cannot
		// be force-killed. The host is told the entry point exceeded its budget.
		return fmt.Errorf("plugin: run exceeded %s", s.timeout)
	}
}
