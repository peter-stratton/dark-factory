# Apple Code Signing and Notarization

## Overview

macOS Gatekeeper blocks or warns users when running unsigned binaries. To distribute godark via Homebrew without warnings, macOS binaries are signed with an Apple Developer ID certificate and submitted to Apple for notarization during the release pipeline.

Signing proves the binary comes from an identified developer. Notarization submits the signed binary to Apple for malware scanning — once approved, Gatekeeper trusts it without warnings.

## Prerequisites

- **Apple Developer Program enrollment** ($99/year) at https://developer.apple.com/programs/
- **Developer ID Application certificate** — created in Certificates, Identifiers & Profiles
- **App-specific password** for notarization — generated at https://appleid.apple.com/

## GitHub Actions Secrets

All credentials are stored as GitHub Actions **repository secrets** (Settings > Secrets and variables > Actions). No secrets are committed to the repository.

| Secret | Description |
|--------|-------------|
| `APPLE_DEVELOPER_ID_P12` | Base64-encoded `.p12` certificate file. Export the Developer ID Application certificate from Keychain Access as a `.p12`, then encode: `base64 < certificate.p12 \| pbcopy` |
| `APPLE_DEVELOPER_ID_PASSWORD` | Password used when exporting the `.p12` file |
| `APPLE_DEVELOPER_ID_IDENTITY` | Signing identity string. Find it with: `security find-identity -v -p codesigning`. Looks like `Developer ID Application: Name (TEAMID)` |
| `APPLE_ID` | Apple ID email address used for notarization |
| `APPLE_ID_PASSWORD` | App-specific password (not your account password). Generate at https://appleid.apple.com/ under Sign-In and Security > App-Specific Passwords |
| `APPLE_TEAM_ID` | Apple Developer Team ID. Visible in the signing identity string after your name, or at https://developer.apple.com/account under Membership Details |

## How the Pipeline Works

The release workflow (`.github/workflows/release.yml`) runs on `macos-latest` when a version tag is pushed:

1. **Certificate import** — the `.p12` certificate is decoded from the secret and imported into a temporary keychain on the runner
2. **Build** — GoReleaser compiles binaries for all targets (darwin/amd64, darwin/arm64, linux/amd64)
3. **Sign** — a post-build hook runs `codesign` on darwin binaries with the hardened runtime and secure timestamp. Linux binaries are skipped
4. **Notarize** — a second post-build hook zips each signed darwin binary, submits it to Apple via `xcrun notarytool submit --wait`, and waits for approval. The zip is cleaned up after submission
5. **Release** — GoReleaser archives and publishes all artifacts to GitHub Releases and the Homebrew tap
6. **Cleanup** — the temporary keychain is deleted (runs even if the job fails)

Since the distributed binaries are bare Mach-O executables (not `.dmg` or `.pkg`), the notarization ticket cannot be stapled. Gatekeeper checks the ticket server-side on first launch.

## Local Verification

After downloading a release binary:

```bash
# Verify the signature is valid
codesign --verify --deep --strict godark

# Check the signing identity and flags (look for "runtime" in flags)
codesign -dv godark

# Verify Gatekeeper accepts the binary (requires notarization)
spctl --assess --type execute godark

# Check notarization status for a submission
xcrun notarytool info <submission-id> --apple-id <email> --password <app-specific-password> --team-id <team-id>
```

## Certificate Renewal

Developer ID Application certificates are valid for **5 years**. When a certificate expires:

1. Create a new Developer ID Application certificate in [Certificates, Identifiers & Profiles](https://developer.apple.com/account/resources/certificates/list)
2. Export the new certificate from Keychain Access as a `.p12` file
3. Base64-encode it: `base64 < new-certificate.p12 | pbcopy`
4. Update the `APPLE_DEVELOPER_ID_P12` and `APPLE_DEVELOPER_ID_PASSWORD` secrets in GitHub
5. Update `APPLE_DEVELOPER_ID_IDENTITY` if the identity string changed
6. Push a new tag to verify the pipeline works with the new certificate

Previously signed binaries remain valid — the old certificate's signature is still trusted as long as it was valid at signing time (the `--timestamp` flag records when signing occurred).

## Troubleshooting

**Notarization fails with "Invalid signature"**
The binary must be signed with `--options runtime` (hardened runtime) and `--timestamp`. Verify with `codesign -dv` that both are present.

**Notarization fails with "The software is not signed with a valid Developer ID certificate"**
The certificate may have expired or the wrong certificate type was used. You need a **Developer ID Application** certificate, not a development or distribution certificate.

**`spctl --assess` returns "rejected"**
Notarization may not have completed, or the binary was modified after signing. Re-download and check again. Gatekeeper caches results, so test on a fresh download or clear the cache with `sudo spctl --reset`.

**Keychain access denied during CI**
The `security set-key-partition-list` step may have failed. Check that the temporary keychain was created and unlocked before the import step runs.

**App-specific password expired or revoked**
Generate a new app-specific password at https://appleid.apple.com/ and update the `APPLE_ID_PASSWORD` secret.
