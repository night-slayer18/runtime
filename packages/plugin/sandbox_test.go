package plugin

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func TestGuardDeniesAllByDefault(t *testing.T) {
	g := newGuard(nil)
	if err := g.OpenFile("/etc/passwd"); !errors.Is(err, ErrCapabilityDenied) {
		t.Errorf("OpenFile = %v, want ErrCapabilityDenied", err)
	}
	if err := g.Dial("tcp", "example.com:443"); !errors.Is(err, ErrCapabilityDenied) {
		t.Errorf("Dial = %v, want ErrCapabilityDenied", err)
	}
	if err := g.StartProcess("sh", "-c", "echo hi"); !errors.Is(err, ErrCapabilityDenied) {
		t.Errorf("StartProcess = %v, want ErrCapabilityDenied", err)
	}
	for _, c := range []Capability{CapFileSystem, CapNetwork, CapProcess} {
		if g.Allowed(c) {
			t.Errorf("Allowed(%s) = true, want false", c)
		}
	}
}

func TestGuardGrantsListedCapabilities(t *testing.T) {
	g := newGuard([]Capability{CapNetwork})
	if err := g.Dial("tcp", "example.com:443"); err != nil {
		t.Errorf("Dial with grant = %v, want nil", err)
	}
	if !g.Allowed(CapNetwork) {
		t.Error("Allowed(CapNetwork) = false, want true")
	}
	// Ungranted capabilities are still denied.
	if err := g.OpenFile("/tmp/x"); !errors.Is(err, ErrCapabilityDenied) {
		t.Errorf("OpenFile without grant = %v, want ErrCapabilityDenied", err)
	}
}

func TestVerifierChecksum(t *testing.T) {
	data := []byte("plugin-bytes-v1")
	v := Verifier{Checksum: Checksum(data)}

	if err := v.Verify(data, nil); err != nil {
		t.Errorf("Verify good checksum = %v, want nil", err)
	}
	if err := v.Verify([]byte("tampered"), nil); !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("Verify tampered = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifierEd25519(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	data := []byte("signed-plugin")
	sig := ed25519.Sign(priv, data)

	v := Verifier{PublicKey: pub}
	if err := v.Verify(data, sig); err != nil {
		t.Errorf("Verify valid signature = %v, want nil", err)
	}

	// Wrong signature.
	bad := make([]byte, len(sig))
	copy(bad, sig)
	bad[0] ^= 0xFF
	if err := v.Verify(data, bad); !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("Verify bad signature = %v, want ErrSignatureInvalid", err)
	}

	// Tampered data.
	if err := v.Verify([]byte("tampered"), sig); !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("Verify tampered data = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifierBothStrategies(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data := []byte("dual-checked")
	sig := ed25519.Sign(priv, data)
	v := Verifier{PublicKey: pub, Checksum: Checksum(data)}

	if err := v.Verify(data, sig); err != nil {
		t.Errorf("Verify both strategies = %v, want nil", err)
	}
	// Checksum matches but signature wrong -> fail.
	if err := v.Verify(data, []byte("short")); !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("Verify bad sig, good checksum = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifierFailsClosedWithNoStrategy(t *testing.T) {
	v := Verifier{}
	if err := v.Verify([]byte("anything"), nil); !errors.Is(err, ErrNotVerified) {
		t.Errorf("Verify no strategy = %v, want ErrNotVerified", err)
	}
}

func TestVerifierMalformedPublicKey(t *testing.T) {
	v := Verifier{PublicKey: ed25519.PublicKey{1, 2, 3}}
	if err := v.Verify([]byte("data"), make([]byte, ed25519.SignatureSize)); !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("Verify malformed key = %v, want ErrSignatureInvalid", err)
	}
}

func TestSandboxRunRequiresVerification(t *testing.T) {
	s := NewSandbox(WithVerifier(Verifier{Checksum: Checksum([]byte("x"))}))
	err := s.Run(func(Capabilities) error { return nil })
	if !errors.Is(err, ErrNotVerified) {
		t.Errorf("Run before Verify = %v, want ErrNotVerified", err)
	}
	if s.Verified() {
		t.Error("Verified() = true before Verify")
	}
}

func TestSandboxVerifyThenRun(t *testing.T) {
	data := []byte("trusted-plugin")
	s := NewSandbox(WithVerifier(Verifier{Checksum: Checksum(data)}))
	if err := s.Verify(data, nil); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !s.Verified() {
		t.Error("Verified() = false after successful Verify")
	}

	ran := false
	err := s.Run(func(caps Capabilities) error {
		ran = true
		// Default sandbox denies filesystem.
		if ferr := caps.OpenFile("/etc/passwd"); !errors.Is(ferr, ErrCapabilityDenied) {
			t.Errorf("OpenFile inside run = %v, want ErrCapabilityDenied", ferr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ran {
		t.Error("fn was not executed")
	}
}

func TestSandboxVerifyFailsLeavesUnverified(t *testing.T) {
	s := NewSandbox(WithVerifier(Verifier{Checksum: Checksum([]byte("real"))}))
	if err := s.Verify([]byte("fake"), nil); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("Verify = %v, want ErrSignatureInvalid", err)
	}
	if s.Verified() {
		t.Error("Verified() = true after failed Verify")
	}
	if err := s.Run(func(Capabilities) error { return nil }); !errors.Is(err, ErrNotVerified) {
		t.Errorf("Run after failed Verify = %v, want ErrNotVerified", err)
	}
}

func TestSandboxRecoversPanic(t *testing.T) {
	data := []byte("p")
	s := NewSandbox(WithVerifier(Verifier{Checksum: Checksum(data)}))
	if err := s.Verify(data, nil); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	err := s.Run(func(Capabilities) error {
		panic("boom")
	})
	if !errors.Is(err, ErrPluginPanic) {
		t.Errorf("Run panicking fn = %v, want ErrPluginPanic", err)
	}
}

func TestSandboxPropagatesError(t *testing.T) {
	data := []byte("p")
	s := NewSandbox(WithVerifier(Verifier{Checksum: Checksum(data)}))
	if err := s.Verify(data, nil); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	want := errors.New("plugin failure")
	if err := s.Run(func(Capabilities) error { return want }); !errors.Is(err, want) {
		t.Errorf("Run = %v, want %v", err, want)
	}
}

func TestSandboxGrantsCapabilities(t *testing.T) {
	data := []byte("net-plugin")
	s := NewSandbox(
		WithCapabilities(CapNetwork),
		WithVerifier(Verifier{Checksum: Checksum(data)}),
	)
	if err := s.Verify(data, nil); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	err := s.Run(func(caps Capabilities) error {
		if derr := caps.Dial("tcp", "example.com:443"); derr != nil {
			return derr
		}
		// Process still denied.
		if perr := caps.StartProcess("sh"); !errors.Is(perr, ErrCapabilityDenied) {
			t.Errorf("StartProcess = %v, want ErrCapabilityDenied", perr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestSandboxTimeout(t *testing.T) {
	data := []byte("slow")
	s := NewSandbox(
		WithVerifier(Verifier{Checksum: Checksum(data)}),
		WithTimeout(20*time.Millisecond),
	)
	if err := s.Verify(data, nil); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	err := s.Run(func(Capabilities) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	if err == nil {
		t.Fatal("Run = nil, want timeout error")
	}
}

func TestSandboxCapabilitiesAccessor(t *testing.T) {
	s := NewSandbox(WithCapabilities(CapFileSystem))
	caps := s.Capabilities()
	if !caps.Allowed(CapFileSystem) {
		t.Error("Allowed(CapFileSystem) = false, want true")
	}
	if caps.Allowed(CapNetwork) {
		t.Error("Allowed(CapNetwork) = true, want false")
	}
}
