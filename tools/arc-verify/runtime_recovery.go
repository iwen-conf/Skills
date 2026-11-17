package main

const (
	recoveryRestore    = "restore"
	recoveryCheckpoint = "checkpoint"
)

// The omitted mode preserves historic restoration semantics. A checkpoint
// retains the current isolated business instance; it never asserts restoration
// and does not import variables or sessions from an earlier run.
func fixtureRecoveryMode(protocol *FixtureRecovery) string {
	if protocol == nil || protocol.Mode == "" {
		return recoveryRestore
	}
	return protocol.Mode
}

func fixtureRecoveryTarget(protocol *FixtureRecovery, fixture Fixture) string {
	if fixtureRecoveryMode(protocol) == recoveryCheckpoint {
		return protocol.State
	}
	return fixture.State
}
