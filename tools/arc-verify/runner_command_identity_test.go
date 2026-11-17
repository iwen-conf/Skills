package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func commandIdentityProgram(t *testing.T, marker, tag string) []byte {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	return []byte(fmt.Sprintf("#!%s\nimport os, urllib.request\nwith open(%q, 'a') as marker: marker.write(%q + '\\n')\nrequest = urllib.request.Request(os.environ['ARC_VERIFY_TARGET'] + '/enroll', data=b'{}', method='POST')\nwith urllib.request.urlopen(request) as response: print(response.read().decode())\n", python, marker, tag))
}

func commandIdentityFixture(t *testing.T, kind string) (executionFixture, *runtimeBusinessTarget, RunnerManifest, string, string) {
	t.Helper()
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	if err := os.WriteFile(filepath.Join(fixture.root, ".gitignore"), []byte(".arc/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(fixture.root, ".arc", "artifacts", "round4-command")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	program, marker := filepath.Join(directory, "action.py"), filepath.Join(directory, "actual-invocations.txt")
	if err := os.WriteFile(program, commandIdentityProgram(t, marker, "original"), 0700); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(fixtureManifest(t, "round4-"+kind, "R1/POSITIVE"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest RunnerManifest
	if err := decodeStrictJSON(body, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Steps[0].Call = AdapterCall{Kind: adapterCommand, Args: []string{"./action.py"}, Directory: directory, SuccessExitCodes: []int{0}}
	return fixture, target, manifest, program, marker
}

func commandIdentityRegisterAndPlan(t *testing.T, fixture *executionFixture, manifest RunnerManifest) {
	t.Helper()
	if _, err := fixture.d.registerRunner(manifest); err != nil {
		t.Fatal(err)
	}
	round, err := fixture.d.createRound(manifest.Graph, "independent", fixture.commit, fixture.root, fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	fixture.round = round.ID
	if _, err := fixture.d.plan(manifest.Graph, 128); err != nil {
		t.Fatal(err)
	}
}

func commandIdentityExecute(fixture executionFixture, manifest RunnerManifest) (ExecuteResult, error) {
	return fixture.d.execute(context.Background(), ExecuteRequest{Task: manifest.Graph + "/" + manifest.Test, Runner: manifest.ID, Round: fixture.round, Environment: "independent", Commit: fixture.commit, Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture})
}

func TestRound4RunnerRelativeExecutableUsesFrozenDirectory(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("actual_script_changed=%t", changed), func(t *testing.T) {
			fixture, target, manifest, program, marker := commandIdentityFixture(t, "relative-executable")
			// The CLI is in a different directory from the registered command.
			// Both names exist, so looking up the wrong file succeeds silently.
			decoy := t.TempDir()
			if err := os.WriteFile(filepath.Join(decoy, "action.py"), []byte("#!/bin/sh\nprintf '{}\\n'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(decoy)
			commandIdentityRegisterAndPlan(t, &fixture, manifest)
			_, before, err := sourceFingerprint(fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				if err := os.WriteFile(program, commandIdentityProgram(t, marker, "changed-after-registration"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			_, after, err := sourceFingerprint(fixture.root)
			if err != nil || before != after {
				t.Fatalf("control needs an ignored command change outside the candidate fingerprint: %s %s %v", before, after, err)
			}
			result, runErr := commandIdentityExecute(fixture, manifest)
			target.mu.Lock()
			requests := target.requests
			target.mu.Unlock()
			invocations, _ := os.ReadFile(marker)
			if changed {
				if runErr == nil || result.Verdict != "BLOCKED" || requests != 0 || len(invocations) != 0 {
					t.Fatalf("changed execution-directory program escaped its frozen identity: verdict=%s requests=%d actual=%q error=%v", result.Verdict, requests, invocations, runErr)
				}
			} else {
				if runErr != nil || result.Verdict != "PASS" || requests != 1 || string(invocations) != "original\n" {
					t.Fatalf("legitimate explicit-directory program did not execute once: result=%+v requests=%d actual=%q error=%v", result, requests, invocations, runErr)
				}
				if facts, err := fixture.d.verifiedRunFacts(result.Run); err != nil || facts.EvidenceStatus != "PASS" {
					t.Fatalf("legitimate command RAW did not revalidate: %+v %v", facts, err)
				}
			}
		})
	}
}

func TestRound4RunnerSourceRelativeFileCannotEscapeFrozenInputs(t *testing.T) {
	fixture, target, manifest, program, marker := commandIdentityFixture(t, "source-relative-file")
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Steps[0].Call.Args = []string{python, "action.py"}
	manifest.Steps[0].Call.Directory = filepath.Join(".arc", "artifacts", "round4-command")
	t.Chdir(t.TempDir())
	// There is no SourceRoot in a registration. This relative directory must
	// not be inspected against the CLI cwd and later rebound under another root.
	if _, err := fixture.d.registerRunner(manifest); err != nil {
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatalf("rejected registration performed an action: %v", statErr)
		}
		return
	}
	round, err := fixture.d.createRound(manifest.Graph, "independent", fixture.commit, fixture.root, fixture.config)
	if err != nil {
		// Registration may defer a relative argument until SourceRoot is known.
		// Round admission must still refuse the undeclared file before any SUT IO.
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatalf("rejected round performed an action: %v", statErr)
		}
		return
	}
	fixture.round = round.ID
	if _, err := fixture.d.plan(manifest.Graph, 128); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, commandIdentityProgram(t, marker, "unfrozen-relative-file"), 0700); err != nil {
		t.Fatal(err)
	}
	result, runErr := commandIdentityExecute(fixture, manifest)
	target.mu.Lock()
	requests := target.requests
	target.mu.Unlock()
	invocations, _ := os.ReadFile(marker)
	t.Fatalf("registration accepted an unresolved source-relative command input: verdict=%s requests=%d actual=%q error=%v", result.Verdict, requests, invocations, runErr)
}

func TestRunnerDeclaredRelativeInputUsesBoundSourceRoot(t *testing.T) {
	for _, directory := range []string{"absolute", "relative", "default"} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("directory=%s/input_changed=%t", directory, changed), func(t *testing.T) {
				fixture, target, manifest, program, marker := commandIdentityFixture(t, "declared-input")
				python, err := exec.LookPath("python3")
				if err != nil {
					t.Fatal(err)
				}
				manifest.Steps[0].Call.Args = []string{python, "action.py"}
				manifest.InputFiles = []string{program}
				if directory != "absolute" {
					manifest.Steps[0].Call.Directory = filepath.Join(".arc", "artifacts", "round4-command")
				}
				if directory == "default" {
					manifest.Steps[0].Call.Directory = ""
					manifest.Steps[0].Call.Args[1] = filepath.Join(".arc", "artifacts", "round4-command", "action.py")
				}
				t.Chdir(t.TempDir())
				commandIdentityRegisterAndPlan(t, &fixture, manifest)
				if changed {
					if err := os.WriteFile(program, commandIdentityProgram(t, marker, "changed-declared-input"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				result, runErr := commandIdentityExecute(fixture, manifest)
				target.mu.Lock()
				requests := target.requests
				target.mu.Unlock()
				invocations, _ := os.ReadFile(marker)
				if changed {
					if runErr == nil || result.Verdict != "BLOCKED" || requests != 0 || len(invocations) != 0 {
						t.Fatalf("declared input change reached the target: result=%+v requests=%d actual=%q error=%v", result, requests, invocations, runErr)
					}
				} else {
					if runErr != nil || result.Verdict != "PASS" || requests != 1 || string(invocations) != "original\n" {
						t.Fatalf("declared relative input was misbound: result=%+v requests=%d actual=%q error=%v", result, requests, invocations, runErr)
					}
					if facts, err := fixture.d.verifiedRunFacts(result.Run); err != nil || facts.EvidenceStatus != "PASS" {
						t.Fatalf("declared-input evidence did not revalidate: %+v %v", facts, err)
					}
				}
			})
		}
	}
}

func TestRunnerRelativeExecutableNeedsKnownRegistrationDirectory(t *testing.T) {
	for _, directory := range []string{"", "relative-command-directory"} {
		t.Run(fmt.Sprintf("directory=%q", directory), func(t *testing.T) {
			fixture, _, manifest, _, _ := commandIdentityFixture(t, "unknown-directory")
			manifest.Steps[0].Call.Directory = directory
			decoy := t.TempDir()
			if err := os.WriteFile(filepath.Join(decoy, "action.py"), []byte("#!/bin/sh\nprintf '{}\\n'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(decoy)
			if _, err := fixture.d.registerRunner(manifest); err == nil {
				t.Fatal("registration guessed a relative executable from the CLI directory")
			}
		})
	}
}

func TestRunnerExecutableSymlinkRetargetCannotRetainFrozenIdentity(t *testing.T) {
	fixture, target, manifest, program, marker := commandIdentityFixture(t, "symlink-executable")
	original := filepath.Join(filepath.Dir(program), "original.py")
	if err := os.Rename(program, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("original.py", program); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	commandIdentityRegisterAndPlan(t, &fixture, manifest)
	changed := filepath.Join(filepath.Dir(program), "changed.py")
	if err := os.WriteFile(changed, commandIdentityProgram(t, marker, "retargeted"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(program); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("changed.py", program); err != nil {
		t.Fatal(err)
	}
	result, runErr := commandIdentityExecute(fixture, manifest)
	target.mu.Lock()
	requests := target.requests
	target.mu.Unlock()
	if runErr == nil || result.Verdict != "BLOCKED" || requests != 0 {
		t.Fatalf("symlink retarget reached the target: result=%+v requests=%d error=%v", result, requests, runErr)
	}
}

func TestRunnerRelativePATHExecutableStaysAtLookupLocation(t *testing.T) {
	fixture, target, manifest, program, marker := commandIdentityFixture(t, "relative-path")
	cli := t.TempDir()
	if err := os.Mkdir(filepath.Join(cli, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cli, "bin", "action.py"), commandIdentityProgram(t, marker, "frozen-PATH-program"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(filepath.Dir(program), "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(program), "bin", "action.py"), commandIdentityProgram(t, marker, "wrong-child-directory"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest.Steps[0].Call.Args = []string{"action.py"}
	t.Chdir(cli)
	// Go normally refuses relative PATH results. Even when the caller explicitly
	// opts into that mode, child chdir must not select a different executable.
	t.Setenv("GODEBUG", "execerrdot=0")
	t.Setenv("PATH", "bin"+string(os.PathListSeparator)+os.Getenv("PATH"))
	commandIdentityRegisterAndPlan(t, &fixture, manifest)
	result, runErr := commandIdentityExecute(fixture, manifest)
	target.mu.Lock()
	requests := target.requests
	target.mu.Unlock()
	invocations, _ := os.ReadFile(marker)
	if runErr != nil || result.Verdict != "PASS" || requests != 1 || string(invocations) != "frozen-PATH-program\n" {
		t.Fatalf("PATH lookup changed meaning after chdir: result=%+v requests=%d actual=%q error=%v", result, requests, invocations, runErr)
	}
}
