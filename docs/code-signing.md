# Code signing — what is needed, and what it changes

The Career Agent already signs every released executable with the project's
own key, which is what the updater verifies before replacing a binary. That
signature protects the update path. It is **not** what Windows and macOS
check when someone opens a download, so it does not stop either operating
system from blocking an install.

Making that stop requires certificates issued to Proofboard by Apple and by a
commercial certificate authority. Neither can be generated locally; both cost
money and require identity verification of the company. The release pipeline
is wired for them and activates automatically once the secrets below exist.

## What each platform blocks today, and why

| Platform | What the user sees now | What removes it |
| --- | --- | --- |
| macOS `.pkg` / `.dmg` / binary | "cannot be opened because the developer cannot be verified" | Developer ID certificate **and** notarization |
| Windows `.exe` / `.msi` | "Windows protected your PC" (SmartScreen) | Authenticode certificate |
| Windows `.msix` | Will not install at all | Authenticode certificate (mandatory) |
| Linux `.deb` / `.rpm` / AppImage | Nothing — installs normally | Optional GPG signing for repository trust |

macOS needs both halves. A Developer ID signature alone still trips Gatekeeper
on a downloaded file, because the check is for notarization — Apple's scan of
the uploaded artifact — not merely for a signature.

## Apple

1. Enrol in the Apple Developer Program (99 USD per year) as Proofboard, not
   as an individual, so the certificate carries the company name.
2. In the developer portal create a **Developer ID Application** certificate
   and a **Developer ID Installer** certificate. The first signs the
   executable and the disk image; the second signs the `.pkg`.
3. Export both from Keychain Access as one `.p12` with a password.
4. Create an **App Store Connect API key** with the Developer role, which is
   what notarization authenticates with. Note the key id and issuer id.

Secrets to add to the repository:

| Secret | Contents |
| --- | --- |
| `APPLE_CERTIFICATE_P12` | the `.p12`, base64 encoded |
| `APPLE_CERTIFICATE_PASSWORD` | the password used on export |
| `APPLE_DEVELOPER_ID_APP` | e.g. `Developer ID Application: Proofboard (TEAMID)` |
| `APPLE_DEVELOPER_ID_INSTALLER` | e.g. `Developer ID Installer: Proofboard (TEAMID)` |
| `APPLE_TEAM_ID` | the ten-character team identifier |
| `APPLE_API_KEY_ID` | App Store Connect key id |
| `APPLE_API_ISSUER_ID` | App Store Connect issuer id |
| `APPLE_API_KEY_P8` | the `.p8` key file, base64 encoded |

## Windows

Choose one:

- **Azure Trusted Signing** — about 10 USD per month, no hardware token, and
  the certificate is short-lived and reissued automatically. Requires an Azure
  subscription and identity validation of the company. This is the simplest
  option for a build pipeline.
- **EV certificate** from a CA such as DigiCert or Sectigo — 300 to 500 USD
  per year. Carries immediate SmartScreen reputation. Historically required a
  hardware token, which does not work in hosted CI; buy the cloud-HSM variant
  if you take this route.
- **OV certificate** — 200 to 400 USD per year, and the cheapest, but
  SmartScreen keeps warning until the certificate accumulates reputation
  across enough downloads. For a new product that can take weeks.

Secrets for the certificate-file route:

| Secret | Contents |
| --- | --- |
| `WINDOWS_CERTIFICATE_PFX` | the `.pfx`, base64 encoded |
| `WINDOWS_CERTIFICATE_PASSWORD` | its password |

For Azure Trusted Signing instead: `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`,
`AZURE_CLIENT_SECRET`, `AZURE_TRUSTED_SIGNING_ENDPOINT`,
`AZURE_TRUSTED_SIGNING_ACCOUNT`, `AZURE_TRUSTED_SIGNING_PROFILE`.

## Until the secrets exist

The signing steps skip and say so in the build log, and the release notes
carry the caveat. They do not fail the build, so releases continue — but they
also do not pretend to have run. Nothing reports success for work it did not
do.

The MSIX is the exception worth repeating: unsigned, it cannot be installed at
all except on a machine with developer mode enabled. It ships so that anyone
holding a certificate can sign it themselves without rebuilding.
