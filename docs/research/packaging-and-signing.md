# Installers, packaging and signing

Research for #56 (part of map #49). Researched 2026-10-18 against primary sources (vendor docs and pricing pages). Prices are in USD. Where a fact comes from a secondary source or from memory, it is marked **(unverified)**.

## Question

What does it take to ship proz, one Go binary with a tray (fyne.io/systray, cgo on macOS) and a future GUI, to non-developers? Which installer, package manager and signing options exist per OS, what do they cost, and which fit the existing GitHub Actions release workflow?

## Where we are today

- `.github/workflows/release.yml` builds on a tag push with a `goos` x `goarch` matrix (linux and darwin amd64/arm64, windows amd64). Darwin runs on `macos-latest` with `CGO_ENABLED=1`; everything else runs on `ubuntu-latest` with cgo off. Each job uploads a `.tar.gz` or `.zip`, and a final job publishes them with `softprops/action-gh-release`.
- `install.sh` and `install.ps1` fetch `releases/latest/download/proz-<os>-<arch>.*` into `~/.local/bin` (and `%USERPROFILE%\.local\bin` on Windows). Nothing is signed, notarized or packaged.
- The archive names (`proz-<os>-<arch>.tar.gz|zip`) are an implicit contract with both install scripts. Any packaging change must keep them or update the scripts in the same change.
- The license is GPL-3.0 (`LICENSE`), which is OSI-approved. That matters for free OSS signing below.

## Constraint that decides Linux sandboxing

proz's job includes launching other programs (editors, terminals, a project's configured launch command). A sandboxed package format that cannot exec host binaries breaks that core feature.

- **Flatpak:** by default an app has "No access to processes outside the sandbox". Running host commands needs `flatpak-spawn --host`, which needs `--talk-name=org.freedesktop.Flatpak`, a permission outside the default bus policy ([Flatpak sandbox permissions](https://docs.flatpak.org/en/latest/sandbox-permissions.html)). Flathub says static permissions "must be kept to an absolute minimum" and rejects apps "disabling or bypassing security mechanisms" or that "rely on host components ... for core functionality", with case-by-case exceptions ([Flathub requirements](https://docs.flathub.org/docs/for-app-authors/requirements)). That permission is effectively a full sandbox escape, so Flatpak buys nothing and risks rejection. **(The "full escape" framing is our reading, not a quote.)**
- **Snap, strict:** sandboxed, so it cannot run arbitrary host programs. **Snap, classic:** not isolated from the host at runtime, but publishing to the Snap Store requires an explicit request and manual approval for classic confinement ([snapcraft classic confinement](https://raw.githubusercontent.com/canonical/snapcraft/main/docs/explanation/classic-confinement.rst)). The approval criteria page is behind a login redirect, so whether a project launcher qualifies is **unverified**.

Conclusion: Linux ships native packages (deb, rpm, Arch) plus the existing tarball, not Flatpak or strict Snap.

## Options per OS

### Windows

| Option | What it gives | Cost | CI fit |
| --- | --- | --- | --- |
| Portable zip (today) | Works; no Start menu entry, no uninstaller | Free | Already done |
| Inno Setup `.exe` | Classic wizard installer. `PrivilegesRequired=lowest` gives a per-user install with no UAC prompt ([Inno docs](https://jrsoftware.org/ishelp/topic_setup_privilegesrequired.htm)). winget understands `InstallerType: inno` and sets silent switches automatically ([winget manifest](https://learn.microsoft.com/en-us/windows/package-manager/package/manifest)) | Free | Needs a `windows-latest` job (Inno is a Windows tool) |
| WiX `.msi` | MSI, good for enterprise deployment | WiX v6+ asks organizations with annual revenue above $10,000 to pay the Open Source Maintenance Fee via GitHub sponsorship; v7 enforces EULA acceptance ([FireGiant OSMF](https://docs.firegiant.com/wix/osmf/)) | Windows job, or GoReleaser Pro's MSI pipe via msitools on Linux ([GoReleaser MSI](https://goreleaser.com/customization/msi/)) |
| MSIX via Microsoft Store | Store re-signs MSIX for free and hosts it; Store apps "are never subject to SmartScreen download warnings" ([SmartScreen doc](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation), [Store package requirements](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/msi/app-package-requirements)). Store registration is free for individual developers since September 2025 ([Windows Developer Blog](https://blogs.windows.com/windowsdeveloper/2025/09/10/free-developer-registration-for-individual-developers-on-microsoft-store/)) | Free | Store submission is a separate, mostly manual step. Whether a full-trust MSIX tray app that spawns arbitrary processes passes certification is **unverified** |
| EXE/MSI via Microsoft Store | Store lists your own installer | Free, but every PE file must be signed with a cert from the Microsoft Trusted Root Program, and install must be silent ([Store MSI/EXE requirements](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/msi/app-package-requirements)) | Needs signing first |
| winget | `winget install proz`; manifests are PRs to `microsoft/winget-pkgs`; all tools must support silent install ([winget manifest](https://learn.microsoft.com/en-us/windows/package-manager/package/manifest)) | Free | GoReleaser OSS has a winget publisher that commits a manifest and can open the PR; choosing installer type (`use: archive`, `nsis`, `msi`) is Pro-only ([GoReleaser winget](https://goreleaser.com/customization/winget/)) |
| Scoop | Per-user installs from a bucket; manifests verify a SHA256 `hash`, no signing required ([Scoop manifests](https://github.com/ScoopInstaller/Scoop/wiki/App-Manifests)) | Free | GoReleaser `scoops` publisher to our own bucket repo (OSS, from memory, **unverified** on the page) |

**Console window:** a tray or GUI binary on Windows needs the `-H windowsgui` linker flag or it opens a console window on start. Today's release builds without it. A GUI and a CLI in one `.exe` conflict here; the usual answer is two binaries or a separate `proz-tray.exe`. **(From general Go knowledge, not checked against a source in this pass.)**

#### Windows signing

| Option | Cost (2026) | Eligibility | SmartScreen | CI fit |
| --- | --- | --- | --- | --- |
| Unsigned | Free | Anyone | "Windows protected your PC", user must pick "Run anyway"; Smart App Control on Windows 11 blocks unsigned files without reputation ([SmartScreen doc](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)) | n/a |
| Azure Artifact Signing (formerly Trusted Signing) | Basic $9.99/month for 5,000 signatures, Premium $99.99/month for 100,000; $0.005 per extra signature. Microsoft's own docs say "Starts at $9.99/month" ([SmartScreen doc](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)); the pricing page renders amounts client side, so the Premium and overage figures come from search snippets of it and [DevClass, Jan 2026](https://devclass.com/2026/01/14/code-signing-windows-apps-may-be-easier-and-more-secure-with-new-azure-artifact-service/) **(partly unverified)**. Not pro-rated; needs a paid Azure subscription, no free or trial ([FAQ](https://learn.microsoft.com/en-us/azure/artifact-signing/faq)) | **Individuals: US or Canada only.** Organizations: US, Canada, EU, UK, Australia, NZ, Japan, South Korea, Singapore, Switzerland, Norway, Israel ([quickstart](https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart)). Validation takes 1 to 20 business days | Builds reputation over time; no EV certs, and EV no longer bypasses SmartScreen anyway | Official GitHub Actions integration; works from `ubuntu-latest` **(Linux runner support unverified; the documented paths are SignTool, PowerShell, GitHub Actions and an SDK)** |
| SignPath Foundation | Free for OSS | OSI license "without commercial dual-licensing for all components", no proprietary components, already released, automated verifiable builds from source ([SignPath terms](https://signpath.org/terms), [signpath.org](https://signpath.org/)) | Certificate is issued to SignPath Foundation, not to us **(from memory, unverified)** | CI-based; GitHub Actions connector exists **(unverified on the pages read)** |
| Traditional OV certificate (Sectigo, DigiCert, SSL.com, Certum) | Roughly $200 to $500 per year, plus a hardware token or cloud HSM since the 2023 CA/B Forum key-storage rule **(unverified, no vendor page read)** | Anyone who passes validation | Same as Artifact Signing: reputation accumulates | Cloud HSM vendors offer CLI signing usable from Actions **(unverified)** |
| Microsoft Store MSIX | Free | Individuals, ~200 markets | No warnings | Manual submission |

If the maintainer lives outside the US or Canada, Artifact Signing as an individual is out, unless an organization in a listed country holds the account. SignPath Foundation fits GPL-3.0, but if the paid hosted layer ever ships proprietary code inside the signed binary, or the project adds a commercial dual license, it stops qualifying.

### macOS

| Option | What it gives | Cost | CI fit |
| --- | --- | --- | --- |
| Bare binary in tar.gz (today) | Works from a terminal; quarantined downloads are blocked by Gatekeeper | Free | Done |
| `.app` bundle with `LSUIElement` | Required for a double-clickable tray/GUI app; `LSUIElement = true` in Info.plist hides the Dock icon for menu-bar agents ([Apple LSUIElement](https://developer.apple.com/documentation/bundleresources/information-property-list/lsuielement); description from general knowledge, the page did not render) | Free | Hand-built Info.plist and folder layout in the macOS job, or GoReleaser Pro `app_bundles` (since v2.4, used together with DMGs) ([GoReleaser app bundles](https://goreleaser.com/customization/app_bundles/)) |
| `.dmg` | Drag-to-Applications installer | Free | `hdiutil` on the macOS runner, or GoReleaser Pro `dmg` (mkisofs on Linux) ([GoReleaser DMG](https://goreleaser.com/customization/dmg/)) |
| Developer ID signing + notarization | Gatekeeper accepts it; required for a smooth double-click open. Since macOS Sequoia, users "will no longer be able to Control-click to override Gatekeeper"; they must go to System Settings > Privacy & Security ([Apple news](https://developer.apple.com/news/?id=saqachfa)) | Apple Developer Program, "$99 annual membership" ([Apple](https://developer.apple.com/programs/)) | Native: `codesign --options runtime` + `xcrun notarytool` on the existing `macos-latest` job. Cross-platform: `rcodesign` signs, notarizes and staples `.app` and `.dmg` from Linux ([apple-codesign README](https://github.com/indygreg/apple-platform-rs/blob/main/apple-codesign/README.md)); GoReleaser OSS-side quill notarizes bare binaries only and warns against it for app bundles ([GoReleaser notarize](https://goreleaser.com/customization/notarize/)) |
| Homebrew cask | `brew install --cask proz` | Free | GoReleaser `homebrew_casks` (since v2.10; `homebrew_formulas` is now listed as deprecated) to our own tap ([GoReleaser casks](https://goreleaser.com/customization/homebrew_casks/)) |

Homebrew 5.0.0 deprecated `--no-quarantine` and "will disable all Homebrew/homebrew-cask casks that fail Gatekeeper checks in September 2026" ([Homebrew 5.0.0](https://brew.sh/2025/11/12/homebrew-5.0.0/)). That rule names the official cask repo; whether our own tap is affected is not stated. GoReleaser documents an `xattr -dr com.apple.quarantine` post-install hook for unsigned casks in a custom tap and warns it may stop working ([GoReleaser casks](https://goreleaser.com/customization/homebrew_casks/)). A stapled ticket is only possible on `.app`, `.dmg` and `.pkg`, not on a bare binary or zip **(from general knowledge; Apple's notarization page did not render)**.

Practical upshot: there is no free way to give non-developers on macOS a clean install. Without the $99 per year, the GUI is a terminal-and-System-Settings experience.

### Linux

| Option | What it gives | Cost | CI fit |
| --- | --- | --- | --- |
| tar.gz + `install.sh` (today) | Works everywhere | Free | Done |
| `.deb`, `.rpm`, `.apk`, Arch packages via nfpm | Native packages with dependencies, `.desktop` file and icon via `contents` ([GoReleaser nfpm](https://goreleaser.com/customization/nfpm/)) | Free | GoReleaser OSS `nfpms` on `ubuntu-latest`; no cgo needed on Linux |
| AUR `proz-bin` | `yay -S proz-bin` | Free | GoReleaser `aurs` publisher, needs an unencrypted SSH key with push to the AUR repo as a secret ([GoReleaser AUR](https://goreleaser.com/customization/aur/)) |
| apt/yum repository | `apt install proz` with updates | Free on e.g. Cloudsmith/Gemfury free tiers or self-hosted **(unverified)** | Extra publish step; can wait |
| Flatpak, strict Snap | Sandboxed; breaks launching host programs (see above) | Free | Not recommended |
| Classic Snap | Works, needs Store approval | Free | `snapcraft` action; optional later |

Linux signing: package managers check repository signatures, not binaries. Signing `checksums.txt` with cosign keyless via GitHub OIDC is free and supported by GoReleaser `signs` ([GoReleaser sign](https://goreleaser.com/customization/sign/)).

## Fit with the existing workflow

Two shapes are possible.

1. **Keep the hand-written matrix and add steps.** Linux: run `nfpm` directly. macOS: on the existing `macos-latest` job, build arm64 and amd64, `lipo` them, write the `.app` with Info.plist, `codesign`, `hdiutil create`, `notarytool submit --wait`, `stapler staple`. Windows: add a `windows-latest` job for Inno Setup and signing. Everything here is free tooling; only the certificates cost money. More YAML to own.
2. **Move to GoReleaser OSS for Linux and publishing, keep a macOS job for cgo.** GoReleaser OSS covers archives, checksums, nfpm, AUR, Scoop, winget (default installer type), Homebrew casks and cosign. The macOS tray needs cgo, so GoReleaser's split/merge or a separate macOS job is still needed **(split/merge is a Pro feature from memory, unverified)**. App bundles, DMG, MSI, NSIS, native notarization and winget installer choice are Pro: $165/year Personal ([GoReleaser Pro](https://goreleaser.com/pro/)).

Either way the release job should keep the current archive names so `install.sh` and `install.ps1` keep working.

## Recommendation

1. **Linux, now, free:** add nfpm `.deb` and `.rpm` (and `archlinux`) plus AUR `proz-bin`. Ship a `.desktop` file and an XDG autostart entry in the package. Skip Flatpak and strict Snap for the launcher reason above.
2. **Windows, now, free:** Inno Setup per-user installer (`PrivilegesRequired=lowest`) built on a `windows-latest` job, published to winget (`InstallerType: inno`) and a Scoop bucket. Apply to SignPath Foundation for free signing while the binary stays fully GPL-3.0. In parallel, try the free Microsoft Store MSIX route, which removes SmartScreen entirely if certification accepts a process-launching tray app.
3. **macOS, when the GUI ships, $99/year:** buy the Apple Developer Program, build a universal `.app` with `LSUIElement`, sign with hardened runtime, notarize and staple a `.dmg` on the existing macOS runner, and publish a cask to our own tap. Until then, keep the terminal install and document the System Settings step.
4. **Tooling:** adopt GoReleaser OSS for Linux packages and package-manager publishing; do the macOS bundle/DMG/notarize and the Inno step as plain workflow steps rather than paying for Pro. Revisit Pro ($165/year) only if the hand-written steps become a maintenance burden.
5. **Skip:** WiX MSI (no need for enterprise MSI yet, and the OSMF applies once revenue passes $10,000), EV certificates (no SmartScreen benefit), Azure Artifact Signing as an individual outside the US or Canada.

## Open questions

- Does the maintainer (or a future proz organization) sit in an Artifact Signing country? If an org forms in the EU or UK, $9.99/month becomes the simplest Windows answer.
- Will the paid hosted layer put proprietary code in the shipped binary? If yes, SignPath Foundation is out.
- One binary or two on Windows (`-H windowsgui` for the tray/GUI vs a console CLI)?
- Does Microsoft Store certification accept a full-trust MSIX that launches arbitrary user programs?

## Sources

- Azure Artifact Signing: [quickstart and eligibility](https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart), [FAQ](https://learn.microsoft.com/en-us/azure/artifact-signing/faq), [pricing page](https://azure.microsoft.com/en-us/pricing/details/artifact-signing/), [DevClass, Jan 2026](https://devclass.com/2026/01/14/code-signing-windows-apps-may-be-easier-and-more-secure-with-new-azure-artifact-service/)
- [SmartScreen reputation for Windows app developers](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)
- [Microsoft Store MSI/EXE package requirements](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/msi/app-package-requirements), [free individual Store registration](https://blogs.windows.com/windowsdeveloper/2025/09/10/free-developer-registration-for-individual-developers-on-microsoft-store/)
- [winget manifest docs](https://learn.microsoft.com/en-us/windows/package-manager/package/manifest), [Scoop app manifests](https://github.com/ScoopInstaller/Scoop/wiki/App-Manifests)
- [Inno Setup PrivilegesRequired](https://jrsoftware.org/ishelp/topic_setup_privilegesrequired.htm), [WiX Open Source Maintenance Fee](https://docs.firegiant.com/wix/osmf/)
- [SignPath Foundation](https://signpath.org/), [SignPath terms](https://signpath.org/terms)
- [Apple Developer Program](https://developer.apple.com/programs/), [Gatekeeper change in macOS Sequoia](https://developer.apple.com/news/?id=saqachfa), [LSUIElement](https://developer.apple.com/documentation/bundleresources/information-property-list/lsuielement), [apple-codesign (rcodesign)](https://github.com/indygreg/apple-platform-rs/blob/main/apple-codesign/README.md)
- [Homebrew 5.0.0 release notes](https://brew.sh/2025/11/12/homebrew-5.0.0/)
- GoReleaser: [notarize](https://goreleaser.com/customization/notarize/), [app bundles](https://goreleaser.com/customization/app_bundles/), [DMG](https://goreleaser.com/customization/dmg/), [MSI](https://goreleaser.com/customization/msi/), [winget](https://goreleaser.com/customization/winget/), [Homebrew casks](https://goreleaser.com/customization/homebrew_casks/), [nfpm](https://goreleaser.com/customization/nfpm/), [AUR](https://goreleaser.com/customization/aur/), [signing](https://goreleaser.com/customization/sign/), [Pro pricing](https://goreleaser.com/pro/)
- [Flatpak sandbox permissions](https://docs.flatpak.org/en/latest/sandbox-permissions.html), [Flathub requirements](https://docs.flathub.org/docs/for-app-authors/requirements), [snapcraft classic confinement](https://raw.githubusercontent.com/canonical/snapcraft/main/docs/explanation/classic-confinement.rst)
