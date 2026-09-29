package app

import "testing"

// TestDumpSMCPowerKeysSmoke exercises the cgo wiring for the SMC power
// diagnostic and asserts it completes against live SMC hardware. The type and
// value assertions live in C and depend on the host Mac, so they are verified
// by running --dump-smc-power, not here.
func TestDumpSMCPowerKeysSmoke(t *testing.T) {
	DumpSMCPowerKeys()
}
