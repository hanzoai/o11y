package identn

// THE ONE CONTROL THAT KEEPS IMPERSONATION HARMLESS.
//
// impersonationidentn.Test() returns true for every request and GetIdentity
// resolves every caller to the ROOT user. That is the mode's contract — a
// single-user local deployment with no authentication — and it is exactly as
// dangerous as it sounds if it is ever live next to a real identity provider.
//
// Nothing at request time stops it: the resolver tries providers in order and
// impersonation matches unconditionally, so if it is in the list at all, a
// request that fails to match IAM falls through to root. The ONLY thing standing
// between that and a production host is Config.Validate refusing to boot when
// impersonation is enabled alongside IAM or API keys — a check
// that pkg/o11y/config.go's validateConfig runs over every config at startup, so
// a deployment that tries the combination fails to start rather than serving.
//
// A control with no test is a control that can be relaxed by accident, and this
// one is load-bearing enough that relaxing it would not look like a security
// change. So it is pinned here, per forbidden pairing.
//
// (Requiring an opt-in header from impersonation would NOT add a factor: whoever
// can set Impersonation.Enabled can set a header just as easily. Boot-time
// mutual exclusion is the control that actually costs an attacker something,
// which is why it is the one under test.)

import "testing"

func TestImpersonationCannotBootBesideARealIdentityProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		with func(*Config)
	}{
		{"IAM", func(c *Config) { c.IAM.Enabled = true }},
		{"api key", func(c *Config) { c.APIKeyConfig.Enabled = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newConfig().(*Config)
			c.Impersonation.Enabled = true
			c.IAM.Enabled, c.APIKeyConfig.Enabled = false, false
			tc.with(c)

			if err := c.Validate(); err == nil {
				t.Fatalf("impersonation booted alongside %s — every request would resolve to "+
					"the root user whenever %s failed to match", tc.name, tc.name)
			}
		})
	}
}

// The mode itself must still be usable, or the guard above would be pinning a
// deployment nobody can run — and the pressure to relax it would come from there.
func TestImpersonationAloneIsValid(t *testing.T) {
	c := newConfig().(*Config)
	c.Impersonation.Enabled = true
	c.IAM.Enabled, c.APIKeyConfig.Enabled = false, false

	if err := c.Validate(); err != nil {
		t.Fatalf("impersonation alone is the local single-user mode and must validate: %v", err)
	}
}

// And the shipped default must not be it.
func TestImpersonationIsOffByDefault(t *testing.T) {
	if newConfig().(*Config).Impersonation.Enabled {
		t.Fatal("impersonation is enabled by default — every caller would resolve to root")
	}
}
