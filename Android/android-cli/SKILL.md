---
name: android-cli
description: Provides instructions for installing and using the `android` CLI. The `android` command-line tool is a critical tool for Android development and helps you create new Android projects, run Android apps on devices, manage and interact with Android virtual devices (including screenshots and UI inspection), manage Android SDK components, look up official Android documentation, and discover and install official Android skills.
license: Complete terms in LICENSE.txt
metadata:
  author: Google LLC
  keywords:
  - sdk
  - emulator
  - skills
  - docs
  - knowledge base
  - project creation
  - screenshots
---
# Android CLI Specialist

This skill provides instructions for using the `android` CLI tool. The tool includes various commands for creating projects, running applications, interacting with devices, and managing the CLI environment.

## Installation

If the android tool is not in the path, install it. To install run the following command:
Linux: curl -fsSL https://dl.google.com/android/cli/latest/linux_x86_64/install.sh | bash
Mac Arm: curl -fsSL https://dl.google.com/android/cli/latest/darwin_arm64/install.sh | bash
Mac Intel: curl -fsSL https://dl.google.com/android/cli/latest/darwin_x86_64/install.sh | bash
Windows: curl -fsSL https://dl.google.com/android/cli/latest/windows_x86_64/install.cmd -o "%TEMP%\i.cmd" && "%TEMP%\i.cmd"

## SDK management
To manage the installation of Android SDKs and tools, use the `sdk` command. For example:

- `android sdk install <package>[@<version>]...`: Install specific packages. Multiple packages can be specified, separated by spaces. `<version>` defaults to latest. For example: `android sdk install platforms/android-30@2 platforms/android-34`
- `android sdk update [<pkg-name>]`: Update a specific package or all packages to the latest version.
- `android sdk remove <pkg-name>`: Remove a package from the local SDK.
- `android sdk list --all`: List installed and available SDK packages.

## Project creation
Create projects from templates using the `create` command.

For example: `android create empty-activity --name="My App" --output=./my-app`

## Interacting with devices
For more information on interacting with running devices, see [here](references/interact.md)

## Running journey tests
For more information on running journeys, see [here](references/journeys.md)

## Doc searching
The `docs` command searches authoritative, high-quality Android developer documentation in the Android Knowledge Base.
By providing a few keywords, this tool will return high quality articles that contain examples or guidance on how to use Android APIs or libraries.
Use this tool to obtain additional information on how to achieve Android-specific tasks or to know more about Android APIs, surfaces, libraries, or devices.

Always use this tool to get the most up-to-date information about Android concepts. Typical good use cases are:
  - Finding migration guides for APIs.
  - Finding examples for APIs.
  - Finding up-to-date information about Android APIs.
  - Finding best practices for Android concepts.

## Running APKs
Use the `run` command to run Android apps.

## Managing emulators

Apply [Android execution](../../docs/android-execution.md) first. Under the current host policy use remote redroid by default, and avdslim for local AVD lifecycle; do not use `android emulator` to bypass it.

## Capturing screenshots

Capture an image of the current screen of a connected Android device and output it to a file using the version-confirmed `android screen capture` command (check `android screen --help`); adb capture is also available under host policy.

## Managing skills

Manage antigravity agent skills for Android using the `android skills` command.

## Inspecting UI Layouts

Use the `android layout` command to inspect the UI layout of an Android application. It returns the layout tree of an Android application in JSON format. When debugging UI errors, this is often a much faster approach than taking a screenshot.

## Updating the CLI

Update the Android CLI using the `android update` command.

## Command discovery

Run `android --version` and the relevant `--help` once per session/version; cached help is reusable. Command names and flags come from that version, not a frozen help dump. SDK/docs/run/layout commands remain useful; emulator lifecycle follows host policy.
