# Graph Report - PromoGo  (2026-07-29)

## Corpus Check
- 129 files · ~51,254 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1032 nodes · 1955 edges · 84 communities (38 shown, 46 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 225 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2b44991e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- customerauth_test.go
- okHandler
- loyalty_test.go
- staffauth_test.go
- IssueStaffAccessToken
- StaffAuthService
- CustomerAuthService
- Deps
- BootstrapPlatformAdmin
- LoyaltyService
- New
- handleGetMyTransactions
- Loyalty Platform Product Concept
- LoyaltyConfig
- Local Development Stack Skill
- Confirmed Decision Registry
- clientIP
- StaffMembership
- OrganizationRepository
- handleCreateStoreAPIKey
- handleLookupClientByPhone
- Loyalty Mechanic Contract
- New
- handleAdminLookupClient
- handleGetLoyaltyConfig
- CustomerSessionRepository
- statusWriter
- handleListAuditEvents
- Channel
- Transaction
- handleStaffOIDCLogin
- Q: Что на данный момент не хватает в нашем проекте?
- Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?
- Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать
- PromoGo
- writeError
- NewPool
- must
- Build
- Run
- graphify
- 00006_client_store_composite_fk.sql
- 00018_create_staff_and_org_rbac.sql
- LedgerRepository
- Mechanic
- NotificationChannel
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
- 00012_create_organizations.sql
- 00013_stores_add_organization_id.sql
- 00014_create_customer_accounts.sql
- 00015_clients_add_customer_account_id.sql
- customer_sessions
- 00017_create_customer_consents.sql
- 00019_create_store_api_keys.sql
- 00020_create_audit_events.sql
- 00021_stores_api_key_hash_nullable.sql
- Decimal
- ResponseWriter
- BalanceRepository
- ClientRepository
- CustomerAccountRepository
- HandlerFunc
- Logger
- TransactionRepository
- StaffMembershipRepository
- Balance
- LoyaltyConfig
- github.com/MirzaDgtu/PromoGo
- Transaction
- TransactionType
- StaffUserRepository
- Mutex
- Pool

## God Nodes (most connected - your core abstractions)
1. `New()` - 37 edges
2. `CustomerAuthService` - 28 edges
3. `newCustomerAuthTestDeps()` - 22 edges
4. `okHandler()` - 20 edges
5. `Deps` - 20 edges
6. `Transaction` - 19 edges
7. `New()` - 19 edges
8. `RequireStoreAPIKey()` - 17 edges
9. `LoyaltyService` - 17 edges
10. `BootstrapPlatformAdmin()` - 16 edges

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

## Communities (84 total, 46 thin omitted)

### Community 0 - "customerauth_test.go"
Cohesion: 0.06
Nodes (53): AuditActorType, AuditEvent, AuditEventRepository, CustomerAccount, CustomerAccountRepository, CustomerAccountStatus, CustomerConsent, CustomerConsentRepository (+45 more)

### Community 1 - "okHandler"
Cohesion: 0.07
Nodes (47): Store, StoreAPIKey, StoreAPIKeyRepository, StoreRepository, fakeStoreAPIKeyRepo, fakeStoreRepo, storeAPIKeyContextKey, Time (+39 more)

### Community 2 - "loyalty_test.go"
Cohesion: 0.15
Nodes (23): Balance, Client, Context, T, newFakeBalanceRepo(), newFakeClientRepo(), newTestService(), pointsConfig() (+15 more)

### Community 3 - "staffauth_test.go"
Cohesion: 0.10
Nodes (29): StaffUser, Time, Context, Pool, Row, NewStaffUserRepository(), scanStaffUser(), Context (+21 more)

### Community 4 - "IssueStaffAccessToken"
Cohesion: 0.08
Nodes (49): accessClaims, tokenType, Permission, staffPrincipalResolver, staffScope, GenerateAPIKey(), GenerateRefreshToken(), Duration (+41 more)

### Community 5 - "StaffAuthService"
Cohesion: 0.09
Nodes (36): jwksDocument, OIDCClaims, oidcIDTokenClaims, OIDCVerifier, fakeStaffResolver, Context, Duration, Mutex (+28 more)

### Community 6 - "CustomerAuthService"
Cohesion: 0.09
Nodes (29): CustomerConsentRepository, CustomerSessionRepository, SMSSender, GenerateOTPCode(), HashOTP(), T, TestGenerateOTPCodeIsSixDigits(), TestVerifyOTP() (+21 more)

### Community 7 - "Deps"
Cohesion: 0.07
Nodes (32): AppConfig, AuthConfig, Config, FCMConfig, HTTPConfig, LoggerConfig, OIDCConfig, PostgresConfig (+24 more)

### Community 8 - "BootstrapPlatformAdmin"
Cohesion: 0.09
Nodes (33): runBootstrapAdmin(), main(), resolveConfigPath(), runServer(), Organization, OrganizationRepository, Time, Context (+25 more)

### Community 9 - "LoyaltyService"
Cohesion: 0.12
Nodes (26): accrueRequestBody, redeemRequestBody, redeemResponseBody, transactionResponseBody, Decimal, HandlerFunc, Logger, handleAccrueTransaction() (+18 more)

### Community 10 - "New"
Cohesion: 0.11
Nodes (19): App, Client, ClientRepository, Context, Logger, Pool, Server, New() (+11 more)

### Community 11 - "handleGetMyTransactions"
Cohesion: 0.13
Nodes (30): BalanceRepository, ClientRepository, CustomerAccountRepository, HandlerFunc, meBalanceItem, meResponseBody, meTransactionItem, meTransactionsResponse (+22 more)

### Community 12 - "Loyalty Platform Product Concept"
Cohesion: 0.10
Nodes (31): Analytics and Customer Communications, Business Scale Discovery, Client Discovery Checklist, Customer Identity and Channels, Loyalty Mechanics Discovery, 1C and POS Integration Discovery, Security, Launch, and Governance, Cross-Cutting Product Requirements (+23 more)

### Community 13 - "LoyaltyConfig"
Cohesion: 0.09
Nodes (19): Balance, BalanceRepository, LoyaltyConfig, LoyaltyConfigRepository, Decimal, Context, New(), T (+11 more)

### Community 14 - "Local Development Stack Skill"
Cohesion: 0.12
Nodes (21): Add Notification Channel Skill, Best-Effort Notification Delivery, Notification Channel Contract, Notification Provider Fallback, Checked Increment Update Pattern, Database Migration Skill, PromoGo Schema Conventions, SQL-Only Migration Directory (+13 more)

### Community 15 - "Confirmed Decision Registry"
Cohesion: 0.12
Nodes (21): Question-to-Decision Lifecycle, Post-Change Tests and Graph Update, Query-First Graphify Investigation, Source Verification of Graph Relationships, Shared Codex and Claude Knowledge Graph, Confirmed Decision Registry, DEC-001: 1C Is an Event Source, Not the Loyalty Core, DEC-002: Phase 1 Uses One Store and Points (+13 more)

### Community 16 - "clientIP"
Cohesion: 0.21
Nodes (14): authTokensResponseBody, otpRequestBody, otpVerifyBody, refreshTokenBody, HandlerFunc, Logger, Time, handleCustomerLogout() (+6 more)

### Community 17 - "StaffMembership"
Cohesion: 0.10
Nodes (31): Role, StaffMembership, StaffMembershipRepository, StaffStatus, StaffUserRepository, createStaffMembershipBody, staffMembershipResponseBody, updateStaffMembershipBody (+23 more)

### Community 19 - "handleCreateStoreAPIKey"
Cohesion: 0.28
Nodes (15): createAPIKeyBody, storeAPIKeyResponseBody, apiKeyToBody(), AuditEventRepository, HandlerFunc, Logger, Request, StoreAPIKeyRepository (+7 more)

### Community 20 - "handleLookupClientByPhone"
Cohesion: 0.09
Nodes (24): balanceResponseBody, customerContextKey, staffContextKey, storeContextKey, MaskPhone(), NormalizePhone(), T, TestMaskPhone() (+16 more)

### Community 21 - "Loyalty Mechanic Contract"
Cohesion: 0.14
Nodes (14): Graphify Skill Trigger, Graphify Workflow for Claude Code, Add Mechanic Skill, Decimal Point Calculation, Loyalty Mechanic Contract, Pure Mechanic Decision Logic, Graphify Rules for Codex, Scoped Graph Queries (+6 more)

### Community 22 - "New"
Cohesion: 0.20
Nodes (14): createOrganizationBody, createStoreBody, organizationResponseBody, storeResponseBody, HandlerFunc, Logger, OrganizationRepository, StoreRepository (+6 more)

### Community 23 - "handleAdminLookupClient"
Cohesion: 0.23
Nodes (11): adminClientResponseBody, adminTransactionResponseBody, BalanceRepository, ClientRepository, HandlerFunc, Logger, StoreRepository, Time (+3 more)

### Community 24 - "handleGetLoyaltyConfig"
Cohesion: 0.27
Nodes (11): loyaltyConfigResponseBody, putLoyaltyConfigBody, Decimal, HandlerFunc, Logger, LoyaltyConfig, LoyaltyConfigRepository, StoreRepository (+3 more)

### Community 25 - "CustomerSessionRepository"
Cohesion: 0.27
Nodes (6): Context, Pool, Row, NewCustomerSessionRepository(), scanCustomerSession(), CustomerSessionRepository

### Community 26 - "statusWriter"
Cohesion: 0.29
Nodes (5): Handler, statusWriter, Logger, ResponseWriter, loggingMW()

### Community 27 - "handleListAuditEvents"
Cohesion: 0.29
Nodes (6): auditEventResponseBody, AuditEventRepository, HandlerFunc, Logger, Time, handleListAuditEvents()

### Community 28 - "Channel"
Cohesion: 0.38
Nodes (4): Context, Logger, New(), Channel

### Community 29 - "Transaction"
Cohesion: 0.10
Nodes (20): Decimal, Transaction, TransactionCursor, TransactionRepository, TransactionType, fakeMeClientRepo, fakeMeTransactionRepo, Time (+12 more)

### Community 30 - "handleStaffOIDCLogin"
Cohesion: 0.33
Nodes (5): staffAuthResponseBody, staffOIDCLoginBody, HandlerFunc, Logger, handleStaffOIDCLogin()

### Community 31 - "Q: Что на данный момент не хватает в нашем проекте?"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Что на данный момент не хватает в нашем проекте?, Source Nodes

### Community 32 - "Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?, Source Nodes

### Community 33 - "Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать, Source Nodes

### Community 34 - "PromoGo"
Cohesion: 0.10
Nodes (19): 1С / POS, API, PromoGo, Staff / admin, Архитектура, Быстрый запуск через Docker Compose, Возможности, Конфигурация (+11 more)

### Community 35 - "writeError"
Cohesion: 0.83
Nodes (3): ResponseWriter, writeError(), writeJSON()

### Community 36 - "NewPool"
Cohesion: 0.50
Nodes (3): Context, Pool, NewPool()

## Knowledge Gaps
- **95 isolated node(s):** `Возможности`, `Стек`, `Архитектура`, `Быстрый запуск через Docker Compose`, `Локальная разработка` (+90 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **46 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Work-memory lessons

**Preferred sources** — corroborated by past sessions; start here.
- `RequireStoreAPIKey()` (2× useful, score=1.980102225)
- `Web Configuration and Administrative Management` (2× useful, score=1.980102225)
- `Customer Registration, Identity, Mobile, and Notifications` (2× useful, score=1.980102225)

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `okHandler`, `IssueStaffAccessToken`, `Deps`, `handleGetMyTransactions`, `clientIP`, `StaffMembership`, `handleCreateStoreAPIKey`, `handleLookupClientByPhone`, `handleAdminLookupClient`, `handleGetLoyaltyConfig`, `handleListAuditEvents`, `handleStaffOIDCLogin`?**
  _High betweenness centrality (0.279) - this node is a cross-community bridge._
- **Why does `New()` connect `New` to `customerauth_test.go`, `okHandler`, `staffauth_test.go`, `StaffAuthService`, `CustomerAuthService`, `Deps`, `BootstrapPlatformAdmin`, `StaffMembership`, `CustomerSessionRepository`?**
  _High betweenness centrality (0.132) - this node is a cross-community bridge._
- **Why does `CustomerAuthService` connect `CustomerAuthService` to `clientIP`, `customerauth_test.go`, `Deps`?**
  _High betweenness centrality (0.126) - this node is a cross-community bridge._
- **Are the 33 inferred relationships involving `New()` (e.g. with `orgScopeFromPath()` and `storeScopeFromPath()`) actually correct?**
  _`New()` has 33 INFERRED edges - model-reasoned connections that need verification._
- **Are the 2 inferred relationships involving `newCustomerAuthTestDeps()` (e.g. with `NewCustomerAuthService()` and `newFakeClientRepo()`) actually correct?**
  _`newCustomerAuthTestDeps()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **Are the 10 inferred relationships involving `okHandler()` (e.g. with `TestRequireCustomerSession_ExpiredTokenRejected()` and `TestRequireCustomerSession_MissingTokenRejected()`) actually correct?**
  _`okHandler()` has 10 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Возможности`, `Стек`, `Архитектура` to the rest of the system?**
  _95 weakly-connected nodes found - possible documentation gaps or missing edges._