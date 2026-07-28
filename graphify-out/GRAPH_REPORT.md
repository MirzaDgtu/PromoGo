# Graph Report - PromoGo  (2026-07-29)

## Corpus Check
- 123 files · ~46,515 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 922 nodes · 1838 edges · 65 communities (38 shown, 27 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 280 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `dfa4057d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- handleLookupClientByPhone
- Loyalty Platform Product Concept
- LoyaltyService
- New
- loyalty_test.go
- Local Development Stack Skill
- Deps
- customerauth_test.go
- Confirmed Decision Registry
- StaffMembership
- Loyalty Mechanic Contract
- RequireStaff
- okHandler
- New
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
- NormalizePhone
- admin_organizations.go
- StoreAPIKeyRepository
- handleAdminLookupClient
- CustomerSessionRepository
- OrganizationRepository
- handleGetLoyaltyConfig
- clientIP
- handleListAuditEvents
- transactions.go
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

## God Nodes (most connected - your core abstractions)
1. `New()` - 41 edges
2. `writeError()` - 37 edges
3. `CustomerAuthService` - 28 edges
4. `writeJSON()` - 26 edges
5. `New()` - 24 edges
6. `newCustomerAuthTestDeps()` - 22 edges
7. `Client` - 20 edges
8. `okHandler()` - 20 edges
9. `Deps` - 20 edges
10. `RequireStoreAPIKey()` - 18 edges

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

## Communities (65 total, 27 thin omitted)

### Community 0 - "handleLookupClientByPhone"
Cohesion: 0.22
Nodes (11): balanceResponseBody, customerContextKey, staffContextKey, storeContextKey, BalanceRepository, ClientRepository, HandlerFunc, Logger (+3 more)

### Community 1 - "Loyalty Platform Product Concept"
Cohesion: 0.10
Nodes (31): Analytics and Customer Communications, Business Scale Discovery, Client Discovery Checklist, Customer Identity and Channels, Loyalty Mechanics Discovery, 1C and POS Integration Discovery, Security, Launch, and Governance, Cross-Cutting Product Requirements (+23 more)

### Community 2 - "LoyaltyService"
Cohesion: 0.16
Nodes (19): NotificationChannel, Build(), accrualFingerprint(), BalanceRepository, ClientRepository, Context, Decimal, Logger (+11 more)

### Community 3 - "New"
Cohesion: 0.07
Nodes (29): App, Client, ClientRepository, Context, Logger, Pool, Server, New() (+21 more)

### Community 4 - "loyalty_test.go"
Cohesion: 0.07
Nodes (40): Balance, BalanceRepository, LoyaltyConfig, LoyaltyConfigRepository, Transaction, TransactionRepository, TransactionType, Decimal (+32 more)

### Community 5 - "Local Development Stack Skill"
Cohesion: 0.12
Nodes (21): Add Notification Channel Skill, Best-Effort Notification Delivery, Notification Channel Contract, Notification Provider Fallback, Checked Increment Update Pattern, Database Migration Skill, PromoGo Schema Conventions, SQL-Only Migration Directory (+13 more)

### Community 6 - "Deps"
Cohesion: 0.07
Nodes (33): main(), AppConfig, AuthConfig, Config, FCMConfig, HTTPConfig, LoggerConfig, OIDCConfig (+25 more)

### Community 7 - "customerauth_test.go"
Cohesion: 0.06
Nodes (53): AuditActorType, AuditEvent, AuditEventRepository, CustomerAccount, CustomerAccountRepository, CustomerAccountStatus, CustomerConsent, CustomerConsentRepository (+45 more)

### Community 8 - "Confirmed Decision Registry"
Cohesion: 0.12
Nodes (21): Question-to-Decision Lifecycle, Post-Change Tests and Graph Update, Query-First Graphify Investigation, Source Verification of Graph Relationships, Shared Codex and Claude Knowledge Graph, Confirmed Decision Registry, DEC-001: 1C Is an Event Source, Not the Loyalty Core, DEC-002: Phase 1 Uses One Store and Points (+13 more)

### Community 9 - "StaffMembership"
Cohesion: 0.07
Nodes (43): Permission, Role, StaffMembership, StaffMembershipRepository, StaffStatus, StaffUser, StaffUserRepository, HasPermission() (+35 more)

### Community 10 - "Loyalty Mechanic Contract"
Cohesion: 0.14
Nodes (14): Graphify Skill Trigger, Graphify Workflow for Claude Code, Add Mechanic Skill, Decimal Point Calculation, Loyalty Mechanic Contract, Pure Mechanic Decision Logic, Graphify Rules for Codex, Scoped Graph Queries (+6 more)

### Community 11 - "RequireStaff"
Cohesion: 0.09
Nodes (47): accessClaims, tokenType, staffPrincipalResolver, staffScope, GenerateAPIKey(), GenerateRefreshToken(), Duration, RegisteredClaims (+39 more)

### Community 12 - "okHandler"
Cohesion: 0.09
Nodes (40): Store, StoreAPIKey, StoreAPIKeyRepository, StoreRepository, fakeStoreAPIKeyRepo, fakeStoreRepo, storeAPIKeyContextKey, Time (+32 more)

### Community 13 - "New"
Cohesion: 0.36
Nodes (5): New(), T, TestMechanic_Accrue(), TestMechanic_Name(), Mechanic

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
Nodes (21): authTokensResponseBody, otpRequestBody, otpVerifyBody, refreshTokenBody, HandlerFunc, Logger, Time, handleCustomerLogout() (+13 more)

### Community 38 - "handleCreateStaffMembership"
Cohesion: 0.17
Nodes (18): createStaffMembershipBody, staffMembershipResponseBody, updateStaffMembershipBody, AuditEventRepository, HandlerFunc, Logger, StaffMembershipRepository, StaffUserRepository (+10 more)

### Community 39 - "StaffAuthService"
Cohesion: 0.20
Nodes (12): fakeStaffResolver, Context, AuditEventRepository, Context, Duration, Logger, StaffMembershipRepository, StaffUserRepository (+4 more)

### Community 40 - "handleCreateStoreAPIKey"
Cohesion: 0.26
Nodes (16): createAPIKeyBody, storeAPIKeyResponseBody, apiKeyToBody(), AuditEventRepository, HandlerFunc, Logger, Request, ResponseWriter (+8 more)

### Community 41 - "handleGetMyBalance"
Cohesion: 0.19
Nodes (15): meBalanceItem, meResponseBody, meTransactionItem, customerFromContext(), Context, BalanceRepository, ClientRepository, CustomerAccountRepository (+7 more)

### Community 42 - "NormalizePhone"
Cohesion: 0.17
Nodes (11): MaskPhone(), NormalizePhone(), T, TestMaskPhone(), TestNormalizePhone(), TestNormalizePhoneIdempotent(), maskPhoneIf(), Context (+3 more)

### Community 43 - "admin_organizations.go"
Cohesion: 0.22
Nodes (12): createOrganizationBody, createStoreBody, organizationResponseBody, storeResponseBody, HandlerFunc, Logger, OrganizationRepository, StoreRepository (+4 more)

### Community 44 - "StoreAPIKeyRepository"
Cohesion: 0.26
Nodes (7): Context, Pool, Row, Time, NewStoreAPIKeyRepository(), scanStoreAPIKey(), StoreAPIKeyRepository

### Community 45 - "handleAdminLookupClient"
Cohesion: 0.23
Nodes (11): adminClientResponseBody, adminTransactionResponseBody, BalanceRepository, ClientRepository, HandlerFunc, Logger, StoreRepository, Time (+3 more)

### Community 46 - "CustomerSessionRepository"
Cohesion: 0.27
Nodes (6): Context, Pool, Row, NewCustomerSessionRepository(), scanCustomerSession(), CustomerSessionRepository

### Community 47 - "OrganizationRepository"
Cohesion: 0.25
Nodes (7): Organization, OrganizationRepository, Time, Context, Pool, NewOrganizationRepository(), OrganizationRepository

### Community 48 - "handleGetLoyaltyConfig"
Cohesion: 0.31
Nodes (10): loyaltyConfigResponseBody, putLoyaltyConfigBody, Decimal, HandlerFunc, Logger, LoyaltyConfigRepository, StoreRepository, handleGetLoyaltyConfig() (+2 more)

### Community 49 - "clientIP"
Cohesion: 0.22
Nodes (7): staffAuthResponseBody, staffOIDCLoginBody, HandlerFunc, Logger, handleStaffOIDCLogin(), clientIP(), Request

### Community 50 - "handleListAuditEvents"
Cohesion: 0.29
Nodes (6): auditEventResponseBody, AuditEventRepository, HandlerFunc, Logger, Time, handleListAuditEvents()

### Community 51 - "transactions.go"
Cohesion: 0.40
Nodes (5): accrueRequestBody, redeemRequestBody, redeemResponseBody, transactionResponseBody, Decimal

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
- **77 isolated node(s):** `python`, `github.com/MirzaDgtu/PromoGo`, `jwksDocument`, `StoreAPIKeyRepository`, `AuditEventRepository` (+72 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **27 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `CustomerAuthService`, `OIDCVerifier`, `loyalty_test.go`, `Deps`, `customerauth_test.go`, `StaffAuthService`, `StaffMembership`, `StoreAPIKeyRepository`, `okHandler`, `CustomerSessionRepository`, `OrganizationRepository`?**
  _High betweenness centrality (0.154) - this node is a cross-community bridge._
- **Why does `New()` connect `New` to `handleLookupClientByPhone`, `Deps`, `handleCreateStaffMembership`, `handleCreateStoreAPIKey`, `handleGetMyBalance`, `RequireStaff`, `okHandler`, `handleAdminLookupClient`, `admin_organizations.go`, `loggingMW`, `handleGetLoyaltyConfig`, `clientIP`, `handleListAuditEvents`?**
  _High betweenness centrality (0.143) - this node is a cross-community bridge._
- **Why does `CustomerAuthService` connect `CustomerAuthService` to `New`, `Deps`, `customerauth_test.go`?**
  _High betweenness centrality (0.127) - this node is a cross-community bridge._
- **Are the 37 inferred relationships involving `New()` (e.g. with `orgScopeFromPath()` and `storeScopeFromPath()`) actually correct?**
  _`New()` has 37 INFERRED edges - model-reasoned connections that need verification._
- **Are the 34 inferred relationships involving `writeError()` (e.g. with `handleCreateStoreAPIKey()` and `handleListStoreAPIKeys()`) actually correct?**
  _`writeError()` has 34 INFERRED edges - model-reasoned connections that need verification._
- **Are the 23 inferred relationships involving `writeJSON()` (e.g. with `handleCreateStoreAPIKey()` and `handleListStoreAPIKeys()`) actually correct?**
  _`writeJSON()` has 23 INFERRED edges - model-reasoned connections that need verification._
- **Are the 18 inferred relationships involving `New()` (e.g. with `NewOIDCVerifier()` and `NewAuditEventRepository()`) actually correct?**
  _`New()` has 18 INFERRED edges - model-reasoned connections that need verification._