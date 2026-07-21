# Security Policy

Itinerarium is a self-hosted, server-side permission system: the whole product
is a set of rules deciding who may see which documents, inventories, locations,
and activity. A bug that lets the wrong player see hidden content, or lets an
unauthenticated request reach protected data, is a security issue — not a
cosmetic one. We take those reports seriously and would rather hear about them
privately first.

## Supported versions

Itinerarium is pre-1.0 and ships from `main`. Security fixes land on `main` and
in the next tagged release; there is no back-porting to older tags yet. If you
run a self-hosted instance, track the latest release.

## Reporting a vulnerability

**Please do not open a public GitHub issue for a security vulnerability.**

Report privately through GitHub's coordinated-disclosure flow:

1. Go to the repository's **Security** tab → **Report a vulnerability**
   (GitHub Private Vulnerability Reporting), or open
   <https://github.com/DaanV2/itinerarium/security/advisories/new>.
2. Describe the issue with enough detail to reproduce it: affected endpoint or
   feature, the permission rule you believe is bypassed, a proof-of-concept
   request or steps, and the impact (what data is exposed or what action is
   possible).

If you cannot use GitHub's reporting flow, open a regular issue titled
"Security contact request" **without any vulnerability details** and ask a
maintainer for a private channel.

### What to expect

- **Acknowledgement** within a few days that the report was received.
- An **initial assessment** (confirmed / needs-info / not-a-vuln) once we have
  reproduced it.
- Coordinated disclosure: we will agree on a timeline with you and credit you
  in the advisory unless you prefer to remain anonymous.

## Scope

In scope — anything that breaks the server-side permission model, for example:

- A character seeing a document, inventory, location, journal, or activity
  entry it should not (game-day gating, repository access, or share rules
  bypassed).
- GM-only content (`gm_only` document sections, the `actor` on announced
  activity entries) reaching a non-GM client.
- The existence of a hidden entity leaking through a response, search result,
  or hit count (it should 404, never 403).
- Authentication or token-revocation bypass.
- Request handling that lets a single caller exhaust server resources.

Out of scope — issues that require an already-compromised host or GM account,
findings against a deployment's reverse proxy / TLS termination (that is the
operator's responsibility), and best-practice suggestions with no concrete
exploit (open those as normal issues).

## Hardening notes for operators

Itinerarium ships with sensible security defaults, but you own the deployment:

- **Serve behind TLS.** Terminate HTTPS at a reverse proxy and set
  `security.hsts: true` (env `SECURITY_HSTS`) so `Strict-Transport-Security` is
  sent. See [docs/deployment.md](docs/deployment.md).
- **Throttle auth endpoints at the proxy.** Itinerarium does not rate-limit
  `/api/login` or the GM password-reset path in-process; put per-IP/per-account
  throttling on the reverse proxy in front of an internet-exposed instance.
- **Body-size limit.** `security.body-limit` caps request bodies; raise it only
  if large Obsidian imports need it.
