package plugin

import (
	"errors"
	"testing"
	"testing/quick"
)

// Feature: runtime-ecosystem, Property 6: Plugin sandboxing
//
// For any plugin attempting a restricted operation (filesystem, network,
// process), the operation is blocked and an error is returned unless the host
// explicitly granted the matching capability. Conversely, any operation whose
// capability was granted is permitted (returns nil).
//
// Validates: Requirements 8.1

// allCapabilities is the full set of restricted operation classes a plugin may
// attempt. The property generator picks grant subsets and operations from here.
var allCapabilities = []Capability{CapFileSystem, CapNetwork, CapProcess}

// grantSet decodes a random bitmask into the subset of capabilities that the
// host grants. Using a bitmask lets testing/quick explore every one of the 2^3
// possible grant combinations (none, single, pair, all) across iterations.
func grantSet(mask uint8) []Capability {
	var grants []Capability
	for i, c := range allCapabilities {
		if mask&(1<<uint(i)) != 0 {
			grants = append(grants, c)
		}
	}
	return grants
}

// attempt routes a restricted operation of the given class through the guard,
// returning whatever error the guard produces (nil means the operation was
// permitted).
func attempt(caps Capabilities, op Capability) error {
	switch op {
	case CapFileSystem:
		return caps.OpenFile("/etc/passwd")
	case CapNetwork:
		return caps.Dial("tcp", "example.com:443")
	case CapProcess:
		return caps.StartProcess("sh", "-c", "echo hi")
	default:
		return nil
	}
}

// TestProperty6_PluginSandboxing verifies that the capability guard blocks every
// restricted operation that was not granted (returning ErrCapabilityDenied) and
// permits every operation that was granted, across randomized grant sets and
// operations.
func TestProperty6_PluginSandboxing(t *testing.T) {
	// Property: for a random grant set and a random restricted operation, the
	// guard denies the operation iff its capability was not granted.
	prop := func(grantMask uint8, opPick uint8) bool {
		grants := grantSet(grantMask)
		guardObj := newGuard(grants)

		op := allCapabilities[int(opPick)%len(allCapabilities)]
		granted := guardObj.Allowed(op)

		err := attempt(guardObj, op)

		if granted {
			// A granted capability must permit the operation.
			if err != nil {
				t.Errorf("granted %s but operation returned %v, want nil", op, err)
				return false
			}
			return true
		}

		// An ungranted capability must block the operation with the typed error.
		if !errors.Is(err, ErrCapabilityDenied) {
			t.Errorf("ungranted %s returned %v, want ErrCapabilityDenied", op, err)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(prop, cfg); err != nil {
		t.Error(err)
	}
}

// TestProperty6_PluginSandboxing_ViaSandbox verifies the same invariant when the
// plugin runs through a verified Sandbox: operations the sandbox did not grant
// are blocked from inside the isolated run, while granted ones succeed.
//
// Validates: Requirements 8.1
func TestProperty6_PluginSandboxing_ViaSandbox(t *testing.T) {
	prop := func(grantMask uint8, opPick uint8) bool {
		grants := grantSet(grantMask)
		op := allCapabilities[int(opPick)%len(allCapabilities)]

		data := []byte("plugin-under-test")
		s := NewSandbox(
			WithCapabilities(grants...),
			WithVerifier(Verifier{Checksum: Checksum(data)}),
		)
		if err := s.Verify(data, nil); err != nil {
			t.Errorf("Verify: %v", err)
			return false
		}

		granted := s.Capabilities().Allowed(op)

		runErr := s.Run(func(caps Capabilities) error {
			return attempt(caps, op)
		})

		if granted {
			if runErr != nil {
				t.Errorf("granted %s but sandboxed op returned %v, want nil", op, runErr)
				return false
			}
			return true
		}

		if !errors.Is(runErr, ErrCapabilityDenied) {
			t.Errorf("ungranted %s in sandbox returned %v, want ErrCapabilityDenied", op, runErr)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(prop, cfg); err != nil {
		t.Error(err)
	}
}
