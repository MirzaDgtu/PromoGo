# Graph Report - PromoGo  (2026-07-29)

## Corpus Check
- 123 files · ~46,511 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 928 nodes · 1776 edges · 67 communities (36 shown, 31 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 218 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `33c61bcf`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- staffFromContext
- Loyalty Platform Product Concept
- LoyaltyService
- New
- Client
- Local Development Stack Skill
- Config
- customerauth_test.go
- Confirmed Decision Registry
- StaffMembership
- Loyalty Mechanic Contract
- IssueStaffAccessToken
- okHandler
- LoyaltyConfig
- loggingMW
- Channel
- must
- Run
- graphify
- 00006_client_store_composite_fk.sql
- LedgerRepository
- Mechanic
- Graphified Code, Schema, Configuration, and Documentation
- 00001_create_stores.sql
- 00002_create_clients.sql
- 00003_create_balances.sql
- 00004_create_transactions.sql
- 00005_create_loyalty_configs.sql
- 00007_transaction_type_scoped_idempotency.sql
- 00008_loyalty_configs_constraints.sql
- 00010_transaction_request_fingerprint.sql
- 00011_transaction_amount_and_sign_constraints.sql
- github.com/MirzaDgtu/PromoGo
- CustomerAuthService
- OIDCVerifier
- New
- handleCreateStaffMembership
- StaffAuthService
- handleCreateStoreAPIKey
- handleGetMyBalance
- handleLookupClientByPhone
- admin_organizations.go
- AuditEvent
- Deps
- writeError
- NewPool
- Build
- handleStaffOIDCLogin
- handleListAuditEvents
- NotificationChannel
- Q: Что на данный момент не хватает в нашем проекте?
- Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?
- Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать
- 00018_create_staff_and_org_rbac.sql
- 00012_create_organizations.sql
- 00013_stores_add_organization_id.sql
- 00014_create_customer_accounts.sql
- 00015_clients_add_customer_account_id.sql
- customer_sessions
- 00017_create_customer_consents.sql
- 00019_create_store_api_keys.sql
- 00020_create_audit_events.sql
- 00021_stores_api_key_hash_nullable.sql
- ResponseWriter
- LedgerRepository

## God Nodes (most connected - your core abstractions)
1. `New()` - 39 edges
2. `CustomerAuthService` - 28 edges
3. `newCustomerAuthTestDeps()` - 22 edges
4. `New()` - 21 edges
5. `Client` - 20 edges
6. `okHandler()` - 20 edges
7. `Deps` - 20 edges
8. `LoyaltyService` - 18 edges
9. `RequireStoreAPIKey()` - 17 edges
10. `StaffAuthService` - 16 edges

## Surprising Connections (you probably didn't know these)
- `Loyalty Mechanics Discovery` --semantically_similar_to--> `Loyalty Backend and Configurable Mechanics`  [INFERRED] [semantically similar]
  ClientChecklist.md → Idea.md
- `1C and POS Integration Discovery` --semantically_similar_to--> `Reliable 1C Event Integration`  [INFERRED] [semantically similar]
  ClientChecklist.md → Idea.md
- `Phase 2: Mechanics and Multistore Product` --semantically_similar_to--> `Loyalty Backend and Configurable Mechanics`  [INFERRED] [semantically similar]
  Full-scope.md → Idea.md
- `Customer Identity and Channels` --semantically_similar_to--> `Mobile Client Experience and Identity`  [INFERRED] [semantically similar]
  ClientChecklist.md → Idea.md
- `Analytics and Customer Communications` --semantically_similar_to--> `Retailer Analytics and Marketing`  [INFERRED] [semantically similar]
  ClientChecklist.md → Idea.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Client Discovery Dimensions** — clientchecklist_business_scale_discovery, clientchecklist_loyalty_mechanics_discovery, clientchecklist_pos_1c_integration_discovery, clientchecklist_customer_identity_and_channels, clientchecklist_analytics_and_communications, clientchecklist_security_launch_governance [EXTRACTED 1.00]
- **Phased Product Maturation** — full_scope_phase_1_mvp, full_scope_phase_2_multistore_product, full_scope_phase_3_analytics_marketing, full_scope_phase_4_platform_scaling [EXTRACTED 1.00]
- **MVP End-to-End Loyalty Flow** — mvp_scope_backend_points_api, mvp_scope_mobile_customer_app, mvp_scope_web_configurator, mvp_scope_pos_identity_and_1c_integration, mvp_scope_minimum_security_compliance [EXTRACTED 1.00]
- **PromoGo Knowledge Governance Cycle** — knowledge_home_knowledge_base_hub, knowledge_agent_workflow_decision_lifecycle, knowledge_project_questions_pilot_governance, knowledge_decisions_confirmed_decision_registry [EXTRACTED 1.00]
- **Pilot Production Readiness** — knowledge_project_questions_pilot_governance, knowledge_project_questions_security_privacy, knowledge_project_questions_operational_release_readiness, knowledge_project_questions_implementation_scope_gaps [EXTRACTED 1.00]
- **Shared Graphify Agent Workflow** — _claude_claude_graphify_workflow, agents_graphify_rules, claude_graphify_guidance [INFERRED 0.95]
- **Local PromoGo Runtime Stack** — _claude_skills_dev_stack_skill_local_development_stack, configs_config_runtime_configuration, deployments_docker_compose_local_stack, deployments_docker_compose_app_service [INFERRED 0.95]
- **PromoGo Extension Workflows** — _claude_skills_add_mechanic_skill_add_mechanic, _claude_skills_add_notification_channel_skill_add_notification_channel, _claude_skills_db_migrate_skill_database_migrations [INFERRED 0.75]

## Communities (67 total, 31 thin omitted)

### Community 0 - "staffFromContext"
Cohesion: 0.33
Nodes (6): customerContextKey, staffContextKey, storeContextKey, customerFromContext(), Context, staffFromContext()

### Community 1 - "Loyalty Platform Product Concept"
Cohesion: 0.10
Nodes (31): Analytics and Customer Communications, Business Scale Discovery, Client Discovery Checklist, Customer Identity and Channels, Loyalty Mechanics Discovery, 1C and POS Integration Discovery, Security, Launch, and Governance, Cross-Cutting Product Requirements (+23 more)

### Community 2 - "LoyaltyService"
Cohesion: 0.15
Nodes (21): Balance, Context, Pool, NewLedgerRepository(), accrualFingerprint(), BalanceRepository, ClientRepository, Context (+13 more)

### Community 3 - "New"
Cohesion: 0.08
Nodes (23): App, Organization, OrganizationRepository, Context, Logger, Pool, Server, New() (+15 more)

### Community 4 - "Client"
Cohesion: 0.07
Nodes (40): Client, ClientRepository, Transaction, TransactionRepository, TransactionType, Time, Decimal, Time (+32 more)

### Community 5 - "Local Development Stack Skill"
Cohesion: 0.12
Nodes (21): Add Notification Channel Skill, Best-Effort Notification Delivery, Notification Channel Contract, Notification Provider Fallback, Checked Increment Update Pattern, Database Migration Skill, PromoGo Schema Conventions, SQL-Only Migration Directory (+13 more)

### Community 6 - "Config"
Cohesion: 0.13
Nodes (19): main(), AppConfig, AuthConfig, Config, FCMConfig, HTTPConfig, LoggerConfig, OIDCConfig (+11 more)

### Community 7 - "customerauth_test.go"
Cohesion: 0.08
Nodes (43): CustomerAccount, CustomerAccountRepository, CustomerAccountStatus, CustomerConsent, CustomerConsentRepository, CustomerSession, CustomerSessionRepository, Time (+35 more)

### Community 8 - "Confirmed Decision Registry"
Cohesion: 0.12
Nodes (21): Question-to-Decision Lifecycle, Post-Change Tests and Graph Update, Query-First Graphify Investigation, Source Verification of Graph Relationships, Shared Codex and Claude Knowledge Graph, Confirmed Decision Registry, DEC-001: 1C Is an Event Source, Not the Loyalty Core, DEC-002: Phase 1 Uses One Store and Points (+13 more)

### Community 9 - "StaffMembership"
Cohesion: 0.07
Nodes (40): StaffMembership, StaffMembershipRepository, StaffStatus, StaffUser, StaffUserRepository, Time, Context, Pool (+32 more)

### Community 10 - "Loyalty Mechanic Contract"
Cohesion: 0.14
Nodes (14): Graphify Skill Trigger, Graphify Workflow for Claude Code, Add Mechanic Skill, Decimal Point Calculation, Loyalty Mechanic Contract, Pure Mechanic Decision Logic, Graphify Rules for Codex, Scoped Graph Queries (+6 more)

### Community 11 - "IssueStaffAccessToken"
Cohesion: 0.08
Nodes (49): accessClaims, tokenType, Permission, staffPrincipalResolver, staffScope, GenerateAPIKey(), GenerateRefreshToken(), Duration (+41 more)

### Community 12 - "okHandler"
Cohesion: 0.07
Nodes (47): Store, StoreAPIKey, StoreAPIKeyRepository, StoreRepository, fakeStoreAPIKeyRepo, fakeStoreRepo, storeAPIKeyContextKey, Time (+39 more)

### Community 13 - "LoyaltyConfig"
Cohesion: 0.09
Nodes (19): Balance, BalanceRepository, LoyaltyConfig, LoyaltyConfigRepository, Decimal, Context, New(), T (+11 more)

### Community 14 - "loggingMW"
Cohesion: 0.29
Nodes (5): Handler, statusWriter, Logger, ResponseWriter, loggingMW()

### Community 15 - "Channel"
Cohesion: 0.38
Nodes (4): Context, Logger, New(), Channel

### Community 35 - "CustomerAuthService"
Cohesion: 0.09
Nodes (29): CustomerConsentRepository, CustomerSessionRepository, SMSSender, GenerateOTPCode(), HashOTP(), T, TestGenerateOTPCodeIsSixDigits(), TestVerifyOTP() (+21 more)

### Community 36 - "OIDCVerifier"
Cohesion: 0.15
Nodes (24): jwksDocument, OIDCClaims, oidcIDTokenClaims, OIDCVerifier, Context, Duration, Mutex, RegisteredClaims (+16 more)

### Community 37 - "New"
Cohesion: 0.20
Nodes (16): authTokensResponseBody, otpRequestBody, otpVerifyBody, refreshTokenBody, HandlerFunc, Logger, Time, handleCustomerLogout() (+8 more)

### Community 38 - "handleCreateStaffMembership"
Cohesion: 0.24
Nodes (15): Role, createStaffMembershipBody, staffMembershipResponseBody, updateStaffMembershipBody, AuditEventRepository, HandlerFunc, Logger, StaffMembershipRepository (+7 more)

### Community 39 - "StaffAuthService"
Cohesion: 0.20
Nodes (12): fakeStaffResolver, Context, AuditEventRepository, Context, Duration, Logger, StaffMembershipRepository, StaffUserRepository (+4 more)

### Community 40 - "handleCreateStoreAPIKey"
Cohesion: 0.08
Nodes (41): adminClientResponseBody, adminTransactionResponseBody, createAPIKeyBody, loyaltyConfigResponseBody, putLoyaltyConfigBody, storeAPIKeyResponseBody, apiKeyToBody(), AuditEventRepository (+33 more)

### Community 41 - "handleGetMyBalance"
Cohesion: 0.21
Nodes (13): meBalanceItem, meResponseBody, meTransactionItem, BalanceRepository, ClientRepository, CustomerAccountRepository, HandlerFunc, Logger (+5 more)

### Community 42 - "handleLookupClientByPhone"
Cohesion: 0.08
Nodes (28): accrueRequestBody, balanceResponseBody, redeemRequestBody, redeemResponseBody, transactionResponseBody, MaskPhone(), NormalizePhone(), T (+20 more)

### Community 43 - "admin_organizations.go"
Cohesion: 0.22
Nodes (12): createOrganizationBody, createStoreBody, organizationResponseBody, storeResponseBody, HandlerFunc, Logger, OrganizationRepository, StoreRepository (+4 more)

### Community 44 - "AuditEvent"
Cohesion: 0.20
Nodes (10): AuditActorType, AuditEvent, AuditEventRepository, Time, Context, Pool, Row, NewAuditEventRepository() (+2 more)

### Community 45 - "Deps"
Cohesion: 0.13
Nodes (14): Deps, AuditEventRepository, BalanceRepository, ClientRepository, Context, CustomerAccountRepository, Logger, LoyaltyConfigRepository (+6 more)

### Community 46 - "writeError"
Cohesion: 0.83
Nodes (3): ResponseWriter, writeError(), writeJSON()

### Community 47 - "NewPool"
Cohesion: 0.50
Nodes (3): Context, Pool, NewPool()

### Community 49 - "handleStaffOIDCLogin"
Cohesion: 0.33
Nodes (5): staffAuthResponseBody, staffOIDCLoginBody, HandlerFunc, Logger, handleStaffOIDCLogin()

### Community 50 - "handleListAuditEvents"
Cohesion: 0.29
Nodes (6): auditEventResponseBody, AuditEventRepository, HandlerFunc, Logger, Time, handleListAuditEvents()

### Community 52 - "Q: Что на данный момент не хватает в нашем проекте?"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Что на данный момент не хватает в нашем проекте?, Source Nodes

### Community 53 - "Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?, Source Nodes

### Community 54 - "Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать, Source Nodes

## Knowledge Gaps
- **78 isolated node(s):** `python`, `github.com/MirzaDgtu/PromoGo`, `jwksDocument`, `StoreAPIKeyRepository`, `AuditEventRepository` (+73 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **31 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `handleCreateStaffMembership`, `handleCreateStoreAPIKey`, `handleGetMyBalance`, `handleLookupClientByPhone`, `IssueStaffAccessToken`, `okHandler`, `Deps`, `admin_organizations.go`, `handleStaffOIDCLogin`, `handleListAuditEvents`?**
  _High betweenness centrality (0.226) - this node is a cross-community bridge._
- **Why does `New()` connect `New` to `LoyaltyService`, `CustomerAuthService`, `OIDCVerifier`, `Client`, `Config`, `customerauth_test.go`, `StaffAuthService`, `StaffMembership`, `AuditEvent`, `okHandler`?**
  _High betweenness centrality (0.131) - this node is a cross-community bridge._
- **Why does `CustomerAuthService` connect `CustomerAuthService` to `New`, `Deps`, `customerauth_test.go`?**
  _High betweenness centrality (0.115) - this node is a cross-community bridge._
- **Are the 35 inferred relationships involving `New()` (e.g. with `orgScopeFromPath()` and `storeScopeFromPath()`) actually correct?**
  _`New()` has 35 INFERRED edges - model-reasoned connections that need verification._
- **Are the 2 inferred relationships involving `newCustomerAuthTestDeps()` (e.g. with `NewCustomerAuthService()` and `newFakeClientRepo()`) actually correct?**
  _`newCustomerAuthTestDeps()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **Are the 15 inferred relationships involving `New()` (e.g. with `NewOIDCVerifier()` and `NewAuditEventRepository()`) actually correct?**
  _`New()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **What connects `python`, `github.com/MirzaDgtu/PromoGo`, `jwksDocument` to the rest of the system?**
  _78 weakly-connected nodes found - possible documentation gaps or missing edges._