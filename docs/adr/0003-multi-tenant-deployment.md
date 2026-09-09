# ADR-0003: multi-tenant SaaS vs. dedicated deployments (+ deployment platform / Firebase ADC)

Status: **proposed** — needs business sign-off (billing, contracts, data
residency commitments to retailers). Filed per
`docs/audit-remediation-prompt.md` Phase 5, third and seventh bullets
(the deployment-platform ADR is folded in here rather than split out,
since the ADC decision is downstream of, not independent from, this one).

## Context

`Organization` is already the tenant boundary used for RBAC scoping
(DEC-004) and every store/client/transaction row carries an
`organization_id`-reachable FK chain — this is architecturally a
shared-database, row-isolated multi-tenant model already, whether or not
that was a deliberate SaaS decision. There is no per-tenant provisioning,
billing, or database-selection code anywhere: `internal/config/config.go`
loads exactly one Postgres DSN and one Redis address per process
(`config.go:98-137`), and migration `00013` backfills a single "Default
Organization" for the pilot — i.e. today's actual deployment is a
single-tenant-in-practice instance of a schema that's shaped for
multi-tenancy. `Full-scope.md:63` names this exact fork as an open
question: shared SaaS with `tenant_id` isolation vs. dedicated
deployments per retailer.

FCM credentials today load from a static service-account JSON
(`config.FCMConfig.CredentialsJSON`,
`internal/notification/fcmchannel/fcmchannel.go:60-71`), with the ADC/
workload-identity migration already called out in a code comment as
tracked-but-deferred. Which cloud/platform this runs on, and whether it's
one shared deployment or N dedicated ones, determines what "migrate to
ADC" even means (GCP Workload Identity Federation is a GCP-specific
mechanism; a dedicated on-prem deployment for a data-residency-sensitive
retailer might not use GCP at all).

## Options considered

**A. Shared multi-tenant SaaS** — one deployment, one database, many
`Organization`s, isolation enforced entirely by application-layer
`organization_id` filtering (+ RBAC, Phase 1). Cheapest to operate and
scale; matches the schema as it already exists; fastest onboarding (no
per-customer infra step). Risk: a missed `organization_id` filter in a new
query is a cross-tenant data leak, not just a bug — this is exactly the
IDOR class Phase 1 was created to close, and every future query needs the
same discipline forever. Some retailers (especially ones citing 152-FZ
data-residency or sector-specific compliance) may contractually refuse
shared-database hosting regardless of how good the isolation is.

**B. Dedicated deployment per retailer** — separate database (and
possibly separate process/namespace) per `Organization`, same codebase.
Isolation is structural, not just a WHERE clause — a bug can't leak
cross-tenant because there's no cross-tenant connection to leak through.
Matches what a security-conscious enterprise retailer will ask for. Cost:
real per-tenant operational overhead (migration runs N times, monitoring/
alerting/backup drills scale linearly with tenant count, `organization_id`
columns become redundant single-org-per-DB dead weight), and it's a much
heavier onboarding step than "create an Organization row."

**C. Hybrid — shared SaaS by default, dedicated deployment as an
enterprise tier (recommended).** Same codebase and schema serve both,
selected per-customer at onboarding: small/pilot retailers land in the
shared multi-tenant deployment (Option A); a retailer that needs
structural isolation gets a dedicated deployment of the identical image
pointed at its own database via config (`internal/config/config.go`
already supports this trivially — it's already one DSN per process). No
code fork, no schema fork — the tenant-isolation discipline required by
Option A must be built regardless, since even "dedicated" customers share
infrastructure until they're deliberately split out.

## Decision

**Option C for the platform's shape; Option A (shared) is the pilot's
actual deployment**, since the pilot is a single retailer and the
dedicated-tier machinery (per-tenant provisioning/billing) doesn't exist
and isn't being built for one customer. This is `proposed` because it
commits the business to eventually building and pricing a dedicated tier,
which is a sales/contracts decision, not just an engineering one.

**Deployment platform / Firebase ADC**: no cloud platform has been
chosen yet (this ADR does not choose one) — but whichever is chosen,
static service-account JSON is the wrong long-term credential story for
either Option A or Option C: a leaked shared-SaaS credential is a
platform-wide incident, and a leaked dedicated-deployment credential is a
single-customer incident either way, but rotation/leak-blast-radius is
strictly better with workload identity (GCP Workload Identity Federation,
or the equivalent on whatever platform is actually chosen) than with a
committed/rotated static key. **Recommendation, not yet actionable**:
adopt workload identity for FCM (and any other GCP-adjacent credential)
as part of whichever cloud-platform ADR follows this one; until a
platform is chosen, keep the current static-credential path (already
correctly isolated behind `config.FCMConfig.CredentialsJSON` and loaded
via secret injection, not committed to the repo) rather than building
ADC wiring against a platform that might not be the final choice.

## Consequences / migration plan

1. No schema change required by this ADR alone — `organization_id` row
   scoping already serves both Option A and dedicated-tier Option B/C
   deployments identically; a dedicated deployment simply has exactly one
   `Organization` row in its database.
2. Tenant-isolation discipline (query review, regression tests asserting
   no cross-org data leak — see ADR-0001's consequence #2) is required
   regardless of which tier a given customer ends up on, since shared
   infrastructure exists at least until a customer is deliberately split
   into a dedicated deployment.
3. When a cloud platform is chosen (separate, future ADR): migrate FCM
   (and any other applicable credential) from
   `config.FCMConfig.CredentialsJSON` to that platform's workload-identity
   mechanism. Track this as the concrete follow-up item; do not close it
   here.
4. Billing/contract/onboarding process for a dedicated-tier customer is
   explicitly not designed here — flagged as a business-process gap, not
   an engineering one, consistent with Q-P1-121/122/123/124/125 (open).

## Not decided here

- Which cloud provider/platform.
- Pricing or contractual terms distinguishing shared vs. dedicated tiers.
- Whether existing/pilot customers can move tiers later, and how.
