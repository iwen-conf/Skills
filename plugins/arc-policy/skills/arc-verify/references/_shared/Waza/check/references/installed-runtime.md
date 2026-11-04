# Installed Runtime Evidence

Adapted from Waza's packaged-install and CLI review methods; [MIT license](../../LICENSE).
Apply when an installer, skill, plugin, CLI, package manifest or shipped archive changes.

1. Identify what a real user installs: package manager, release archive, marketplace source,
   installer default ref or generated mirror. Record the package/candidate identity and lane.
2. Build or regenerate using the project's documented rule. Inspect the actual artifact for
   required scripts, references, templates, manifests, archive roots and executable permissions.
   A correct source tree does not prove an allowlist or tracked-file packager included new files.
3. Run a smoke check in a fresh temporary install/config/cache location using the supported host
   options. Do not repurpose the operator's `HOME` or overwrite active configuration. Exercise the
   real install path, list/discover the result, then invoke the smallest relevant entrypoint and
   resolve its references from the installed location. For instructions, link validation proves
   packaging only; observing an agent's tool actions and artifacts proves the behavioral claim.
4. Check archive listings for accidental cache files, bytecode, local logs, screenshots and OS
   metadata. Use archive/package listing tools; local repository discovery follows code-intel.
5. If a required host, dependency, network or credential is unavailable, record the missing layer.
   Source tests, manifest parsing and successful imports cannot substitute for installation proof.

For a CLI entrypoint, additionally verify the changed contract: help/version, flags/subcommands,
exit status, stdout/stderr, machine-readable output, TTY versus redirected stdin, config/env
precedence, shebang, executable bit and wrapper/PATH target. Re-run visible CLI output after
editing its presentation.

For mutating commands, compare the displayed plan/dry-run selection with the actual executor's
input. Exercise partial failure, cancellation and retry where they can leave state changed.
Test cleanup against owned disposable fixtures. An unknown install state or a timed-out probe
never proves that user data is orphaned or safe to remove.
