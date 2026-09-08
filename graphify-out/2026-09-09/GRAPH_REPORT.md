# Graph Report - PromoGo  (2026-09-09)

## Corpus Check
- 163 files · ~90,600 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1592 nodes · 3676 edges · 110 communities (64 shown, 46 thin omitted)
- Extraction: 80% EXTRACTED · 20% INFERRED · 0% AMBIGUOUS · INFERRED: 720 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `c7956067`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- customerauth_test.go
- RequireStoreAPIKey
- Transaction
- StaffMembership
- IssueStaffAccessToken
- StaffAuthService
- CustomerAuthService
- Config
- Organization
- LoyaltyService
- staffauth_test.go
- handleGetMyTransactions
- Loyalty Platform Product Concept
- PromoGo audit remediation prompt
- Local Development Stack Skill
- Confirmed Decision Registry
- auth_customer.go
- newQRTestService
- doRequest
- handlerFor
- handleLookupClientByPhone
- Loyalty Mechanic Contract
- writeError
- handleAdminLookupClient
- handleGetLoyaltyConfig
- newTestLimiter
- New
- handleListAuditEvents
- Channel
- StaffUser
- handleStaffOIDCLogin
- Q: Что на данный момент не хватает в нашем проекте?
- Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?
- Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать
- PromoGo
- handleCreateStaffMembership
- Context
- must
- Q: Ознакомься с проектом. Его идеей и целью. Если видишь изъяны, допущения и возможности улучшения, то распиши их. Проведи полный аудит приложения
- Run
- graphify
- 00006_client_store_composite_fk.sql
- 00018_create_staff_and_org_rbac.sql
- LedgerRepository
- Mechanic
- New
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
- newTestSender
- handleRedeemTransaction
- testsupport_test.go
- Q: Создай для нашего проекта README.md
- loyalty_test.go
- LoyaltyConfig
- AuditEvent
- rateLimitRulesFor
- StaffMembershipRepository
- testFakes
- CustomerSessionRepository
- github.com/MirzaDgtu/PromoGo
- statusWriter
- BootstrapPlatformAdmin
- runBootstrapAdmin
- newFakeStaffMembershipRepo
- New
- Deps
- Q: Расскажи про наш проект для инвесторов. Попробуй донести цель данного проекта. Постарайся сделать это кратко. Расскажи что сделано, что планируется в будущем
- httpserver/qr.go
- refund.go
- 00022_transaction_refunds.sql
- 00023_create_customer_devices.sql
- Sender
- fakeLedgerRepo
- handleRegisterDevice
- fakeCustomerSessionRepo
- fakeCustomerAccountRepo
- fakeOrganizationRepo
- auditCreate
- fakeLoyaltyConfigRepo
- Build
- NotificationChannel
- Channel
- AuditEventRepository
- AuditEventRepository
- StaffUserRepository
- AuditEventRepository
- OrganizationRepository
- StaffUserRepository
- AuditEventRepository
- MapClaims
- PrivateKey

## God Nodes (most connected - your core abstractions)
1. `doRequest()` - 104 edges
2. `newTestServer()` - 95 edges
3. `itoa()` - 43 edges
4. `issueStaffToken()` - 43 edges
5. `writeError()` - 42 edges
6. `seedOrganization()` - 39 edges
7. `adminReq()` - 38 edges
8. `Transaction` - 37 edges
9. `writeJSON()` - 31 edges
10. `jsonBodyAny()` - 30 edges

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

## Communities (110 total, 46 thin omitted)

### Community 0 - "customerauth_test.go"
Cohesion: 0.10
Nodes (35): defaultOTPConfig(), discardLogger(), Context, CustomerAccount, CustomerAuthService, CustomerConsent, CustomerSession, fakeClientRepo (+27 more)

### Community 1 - "RequireStoreAPIKey"
Cohesion: 0.08
Nodes (44): StoreAPIKey, StoreAPIKeyRepository, fakeStoreAPIKeyRepo, fakeStoreRepo, storeAPIKeyContextKey, Time, constantTimeHashEqual(), Context (+36 more)

### Community 2 - "Transaction"
Cohesion: 0.06
Nodes (41): Transaction, TransactionCursor, TransactionRepository, TransactionType, fakeMeClientRepo, fakeMeTransactionRepo, meTransactionsResponse, Decimal (+33 more)

### Community 3 - "StaffMembership"
Cohesion: 0.12
Nodes (12): StaffMembership, StaffMembershipRepository, StaffStatus, StaffUserRepository, fakeStaffMembershipRepo, Role, Role, samePtr() (+4 more)

### Community 4 - "IssueStaffAccessToken"
Cohesion: 0.07
Nodes (57): accessClaims, tokenType, Permission, Role, staffPrincipalResolver, staffScope, GenerateOTPCode(), HashOTP() (+49 more)

### Community 5 - "StaffAuthService"
Cohesion: 0.09
Nodes (36): jwksDocument, OIDCClaims, oidcIDTokenClaims, OIDCVerifier, fakeStaffResolver, Context, Duration, Mutex (+28 more)

### Community 6 - "CustomerAuthService"
Cohesion: 0.05
Nodes (48): CustomerConsentRepository, CustomerSessionRepository, Client, ClientRepository, CustomerAccount, CustomerAccountRepository, CustomerAccountStatus, CustomerConsent (+40 more)

### Community 7 - "Config"
Cohesion: 0.16
Nodes (16): AntiFraudConfig, AppConfig, AuthConfig, Config, FCMConfig, HTTPConfig, OIDCConfig, PostgresConfig (+8 more)

### Community 8 - "Organization"
Cohesion: 0.18
Nodes (10): Organization, OrganizationRepository, Time, Context, Pool, NewOrganizationRepository(), Context, Mutex (+2 more)

### Community 9 - "LoyaltyService"
Cohesion: 0.16
Nodes (23): accrualFingerprint(), BalanceRepository, Client, ClientRepository, Context, Decimal, Duration, Logger (+15 more)

### Community 10 - "staffauth_test.go"
Cohesion: 0.23
Nodes (17): fakeAuditEventRepo, Server, StaffAuthService, T, newStaffAuthTestDeps(), signStaffTestIDToken(), startStaffTestJWKSServer(), TestStaffAuth_DisabledAccountRejected() (+9 more)

### Community 11 - "handleGetMyTransactions"
Cohesion: 0.20
Nodes (16): meBalanceItem, meResponseBody, meTransactionItem, customerFromContext(), encodeTransactionCursor(), BalanceRepository, ClientRepository, CustomerAccountRepository (+8 more)

### Community 12 - "Loyalty Platform Product Concept"
Cohesion: 0.10
Nodes (31): Analytics and Customer Communications, Business Scale Discovery, Client Discovery Checklist, Customer Identity and Channels, Loyalty Mechanics Discovery, 1C and POS Integration Discovery, Security, Launch, and Governance, Cross-Cutting Product Requirements (+23 more)

### Community 13 - "PromoGo audit remediation prompt"
Cohesion: 0.14
Nodes (13): Configuration and transport security, OTP and customer sessions, Phase 0: baseline and release gates, Phase 1: security and tenant-isolation blockers, Phase 2: loyalty and concurrency correctness, Phase 3: QR, notifications and availability, Phase 4: infrastructure, CI and operations, Phase 5: product decisions and full MVP (+5 more)

### Community 14 - "Local Development Stack Skill"
Cohesion: 0.12
Nodes (21): Add Notification Channel Skill, Best-Effort Notification Delivery, Notification Channel Contract, Notification Provider Fallback, Checked Increment Update Pattern, Database Migration Skill, PromoGo Schema Conventions, SQL-Only Migration Directory (+13 more)

### Community 15 - "Confirmed Decision Registry"
Cohesion: 0.12
Nodes (21): Question-to-Decision Lifecycle, Post-Change Tests and Graph Update, Query-First Graphify Investigation, Source Verification of Graph Relationships, Shared Codex and Claude Knowledge Graph, Confirmed Decision Registry, DEC-001: 1C Is an Event Source, Not the Loyalty Core, DEC-002: Phase 1 Uses One Store and Points (+13 more)

### Community 16 - "auth_customer.go"
Cohesion: 0.21
Nodes (14): authTokensResponseBody, otpRequestBody, otpVerifyBody, refreshTokenBody, HandlerFunc, Logger, Time, handleCustomerLogout() (+6 more)

### Community 17 - "newQRTestService"
Cohesion: 0.08
Nodes (43): AuditEventRepository, fakeBalanceRepo, fakeCustomerAccountRepo, Balance, BalanceRepository, Client, ClientRepository, Context (+35 more)

### Community 18 - "doRequest"
Cohesion: 0.07
Nodes (131): adminReq(), Request, T, TestHandleAdminListClientTransactions_NotFoundWrongStore(), TestHandleAdminListClientTransactions_Success(), TestHandleAdminLookupClient_MaskedForSupportViewer(), TestHandleAdminLookupClient_NotFound(), TestHandleAdminLookupClient_UnmaskedForRetailerAdmin() (+123 more)

### Community 19 - "handlerFor"
Cohesion: 0.24
Nodes (17): createAPIKeyBody, storeAPIKeyResponseBody, apiKeyToBody(), HandlerFunc, Logger, Request, ResponseWriter, Store (+9 more)

### Community 20 - "handleLookupClientByPhone"
Cohesion: 0.07
Nodes (30): Store, StoreRepository, balanceResponseBody, customerContextKey, staffContextKey, storeContextKey, MaskPhone(), NormalizePhone() (+22 more)

### Community 21 - "Loyalty Mechanic Contract"
Cohesion: 0.14
Nodes (14): Graphify Skill Trigger, Graphify Workflow for Claude Code, Add Mechanic Skill, Decimal Point Calculation, Loyalty Mechanic Contract, Pure Mechanic Decision Logic, Graphify Rules for Codex, Scoped Graph Queries (+6 more)

### Community 22 - "writeError"
Cohesion: 0.15
Nodes (23): createOrganizationBody, createStoreBody, organizationResponseBody, storeResponseBody, HandlerFunc, Logger, OrganizationRepository, StoreRepository (+15 more)

### Community 23 - "handleAdminLookupClient"
Cohesion: 0.23
Nodes (11): adminClientResponseBody, adminTransactionResponseBody, BalanceRepository, ClientRepository, HandlerFunc, Logger, StoreRepository, Time (+3 more)

### Community 24 - "handleGetLoyaltyConfig"
Cohesion: 0.31
Nodes (10): loyaltyConfigResponseBody, putLoyaltyConfigBody, Decimal, HandlerFunc, Logger, LoyaltyConfigRepository, StoreRepository, handleGetLoyaltyConfig() (+2 more)

### Community 25 - "newTestLimiter"
Cohesion: 0.10
Nodes (37): Context, Duration, New(), Miniredis, T, newTestLimiter(), TestLimiter_AllowsUpToLimit(), TestLimiter_BackendErrorPropagates() (+29 more)

### Community 26 - "New"
Cohesion: 0.35
Nodes (16): Request, T, staffLoginReq(), TestRateLimit_AccrualPrincipalIsolatedByStoreAPIKey(), TestRateLimit_AdminStaffPrincipalExceeded(), TestRateLimit_AdminStaffPrincipalIsolatedBetweenStaffUsers(), TestRateLimit_BackendUnavailableReturns503(), TestRateLimit_ClientLookupPhoneDimensionExceeded() (+8 more)

### Community 27 - "handleListAuditEvents"
Cohesion: 0.29
Nodes (6): auditEventResponseBody, AuditEventRepository, HandlerFunc, Logger, Time, handleListAuditEvents()

### Community 28 - "Channel"
Cohesion: 0.38
Nodes (4): Context, Logger, New(), Channel

### Community 29 - "StaffUser"
Cohesion: 0.23
Nodes (9): StaffUser, Time, Context, Pool, Row, NewStaffUserRepository(), scanStaffUser(), StaffUserRepository (+1 more)

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
Cohesion: 0.09
Nodes (22): 1С / POS, API, PromoGo, QR-флоу, Refund-флоу, SMS и push, Staff / admin, Архитектура (+14 more)

### Community 35 - "handleCreateStaffMembership"
Cohesion: 0.26
Nodes (14): createStaffMembershipBody, staffMembershipResponseBody, updateStaffMembershipBody, HandlerFunc, Logger, StaffMembershipRepository, Time, handleCreateStaffMembership() (+6 more)

### Community 36 - "Context"
Cohesion: 0.27
Nodes (4): fakeFullClientRepo, fakeStaffUserRepo, Client, Context

### Community 38 - "Q: Ознакомься с проектом. Его идеей и целью. Если видишь изъяны, допущения и возможности улучшения, то распиши их. Проведи полный аудит приложения"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Ознакомься с проектом. Его идеей и целью. Если видишь изъяны, допущения и возможности улучшения, то распиши их. Проведи полный аудит приложения, Source Nodes

### Community 45 - "New"
Cohesion: 0.06
Nodes (37): App, CustomerDevice, CustomerDeviceRepository, Channel, fakeClientRepo, fakeDeviceRepo, fakeMessagingSender, messagingSender (+29 more)

### Community 66 - "newTestSender"
Cohesion: 0.15
Nodes (18): Buffer, SMSConfig, retryableError, Sender, smsRequest, smsResponse, Client, Context (+10 more)

### Community 67 - "handleRedeemTransaction"
Cohesion: 0.27
Nodes (9): accrueRequestBody, redeemRequestBody, redeemResponseBody, transactionResponseBody, Decimal, HandlerFunc, Logger, handleAccrueTransaction() (+1 more)

### Community 68 - "testsupport_test.go"
Cohesion: 0.23
Nodes (12): fakeNotifier, newFakeAuditEventRepo(), newFakeBalanceRepo(), newFakeCustomerAccountRepo(), newFakeCustomerDeviceRepo(), newFakeCustomerSessionRepo(), newFakeFullClientRepo(), newFakeLoyaltyConfigRepo() (+4 more)

### Community 69 - "Q: Создай для нашего проекта README.md"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Создай для нашего проекта README.md, Source Nodes

### Community 70 - "loyalty_test.go"
Cohesion: 0.13
Nodes (39): AntiFraudConfig, Balance, Client, Context, Duration, LoyaltyConfig, T, newFakeBalanceRepo() (+31 more)

### Community 71 - "LoyaltyConfig"
Cohesion: 0.09
Nodes (19): Balance, BalanceRepository, LoyaltyConfig, LoyaltyConfigRepository, Decimal, Context, New(), T (+11 more)

### Community 72 - "AuditEvent"
Cohesion: 0.20
Nodes (10): AuditActorType, AuditEvent, fakeAuditEventRepo, Time, Context, Pool, Row, NewAuditEventRepository() (+2 more)

### Community 73 - "rateLimitRulesFor"
Cohesion: 0.15
Nodes (25): contour, openAPIDoc, openAPIOperation, openAPIOperationEntry, routeMeta, staffScopeKind, flattenOpenAPIDoc(), T (+17 more)

### Community 74 - "StaffMembershipRepository"
Cohesion: 0.27
Nodes (8): Context, Pool, Role, Row, NewStaffMembershipRepository(), queryStaffMemberships(), scanStaffMembership(), StaffMembershipRepository

### Community 75 - "testFakes"
Cohesion: 0.23
Nodes (7): fakeCustomerConsentRepo, fakeCustomerDeviceRepo, fakeFullTransactionRepo, fakeSMSSender, testFakes, CustomerConsent, Mutex

### Community 76 - "CustomerSessionRepository"
Cohesion: 0.27
Nodes (7): CustomerSession, Context, Pool, Row, NewCustomerSessionRepository(), scanCustomerSession(), CustomerSessionRepository

### Community 79 - "statusWriter"
Cohesion: 0.29
Nodes (5): statusWriter, Handler, Logger, ResponseWriter, loggingMW()

### Community 80 - "BootstrapPlatformAdmin"
Cohesion: 0.42
Nodes (10): BootstrapPlatformAdmin(), Context, OrganizationRepository, StaffMembershipRepository, StaffUserRepository, resolveOrCreatePlatformAdminMembership(), resolveOrCreateStaffUser(), resolveOrganization() (+2 more)

### Community 81 - "runBootstrapAdmin"
Cohesion: 0.27
Nodes (7): runBootstrapAdmin(), main(), resolveConfigPath(), runServer(), Context, Pool, NewPool()

### Community 82 - "newFakeStaffMembershipRepo"
Cohesion: 0.55
Nodes (9): T, newFakeOrganizationRepo(), TestBootstrapPlatformAdmin_ByOrgID(), TestBootstrapPlatformAdmin_FreshCreateSucceeds(), TestBootstrapPlatformAdmin_RerunIsIdempotent(), TestBootstrapPlatformAdmin_UnknownOrgNameReturnsError(), TestBootstrapPlatformAdmin_UpgradesExistingLesserRoleMembership(), newFakeStaffMembershipRepo() (+1 more)

### Community 83 - "New"
Cohesion: 0.40
Nodes (5): LoggerConfig, Logger, New(), parseLevel(), Level

### Community 84 - "Deps"
Cohesion: 0.11
Nodes (17): Deps, BalanceRepository, ClientRepository, Context, CustomerAccountRepository, CustomerAuthService, CustomerDeviceRepository, Logger (+9 more)

### Community 85 - "Q: Расскажи про наш проект для инвесторов. Попробуй донести цель данного проекта. Постарайся сделать это кратко. Расскажи что сделано, что планируется в будущем"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Расскажи про наш проект для инвесторов. Попробуй донести цель данного проекта. Постарайся сделать это кратко. Расскажи что сделано, что планируется в будущем, Source Nodes

### Community 86 - "httpserver/qr.go"
Cohesion: 0.40
Nodes (4): issueQRResponseBody, resolveQRRequestBody, resolveQRResponseBody, Time

### Community 87 - "refund.go"
Cohesion: 0.67
Nodes (3): refundRequestBody, refundResponseBody, Decimal

### Community 91 - "fakeLedgerRepo"
Cohesion: 0.33
Nodes (4): fakeBalanceRepo, fakeLedgerRepo, Balance, Duration

### Community 92 - "handleRegisterDevice"
Cohesion: 0.36
Nodes (7): registerDeviceRequestBody, registerDeviceResponseBody, CustomerDeviceRepository, HandlerFunc, Logger, handleRegisterDevice(), handleRevokeDevice()

### Community 96 - "auditCreate"
Cohesion: 0.40
Nodes (4): auditCreate(), AuditEventRepository, Context, Logger

## Knowledge Gaps
- **127 isolated node(s):** `Возможности`, `SMS и push`, `Стек`, `Архитектура`, `Быстрый запуск через Docker Compose` (+122 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **46 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Work-memory lessons

**Preferred sources** — corroborated by past sessions; start here.
- `RequireStoreAPIKey()` (3× useful, score=1.758707657) _(code changed — re-verify)_
- `PromoGo` (2× useful, score=1.388433829) _(code changed — re-verify)_
- `Config` (2× useful, score=1.380011784)
- `Phase 2: Mechanics and Multistore Product` (2× useful, score=0.768045957)
- `Phased Product Roadmap` (2× useful, score=0.767536507)
- `Current Implementation and Scope Gaps` (2× useful, score=0.767536507)
- `Web Configuration and Administrative Management` (2× useful, score=0.758866018)
- `Customer Registration, Identity, Mobile, and Notifications` (2× useful, score=0.758866018)

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `writeError()` connect `writeError` to `RequireStoreAPIKey`, `handleCreateStaffMembership`, `IssueStaffAccessToken`, `handleRedeemTransaction`, `handleGetMyTransactions`, `auth_customer.go`, `handlerFor`, `handleLookupClientByPhone`, `handleAdminLookupClient`, `handleGetLoyaltyConfig`, `handleListAuditEvents`, `handleRegisterDevice`, `handleStaffOIDCLogin`?**
  _High betweenness centrality (0.164) - this node is a cross-community bridge._
- **Why does `handlerFor()` connect `handlerFor` to `handleCreateStaffMembership`, `handleRedeemTransaction`, `rateLimitRulesFor`, `Deps`, `writeError`, `newTestLimiter`, `New`, `handleRegisterDevice`?**
  _High betweenness centrality (0.113) - this node is a cross-community bridge._
- **Why does `Deps` connect `Deps` to `StaffMembership`, `testsupport_test.go`, `Config`, `LoyaltyService`, `newQRTestService`, `handlerFor`, `New`?**
  _High betweenness centrality (0.091) - this node is a cross-community bridge._
- **Are the 93 inferred relationships involving `doRequest()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`doRequest()` has 93 INFERRED edges - model-reasoned connections that need verification._
- **Are the 90 inferred relationships involving `newTestServer()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`newTestServer()` has 90 INFERRED edges - model-reasoned connections that need verification._
- **Are the 39 inferred relationships involving `itoa()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`itoa()` has 39 INFERRED edges - model-reasoned connections that need verification._
- **Are the 38 inferred relationships involving `issueStaffToken()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`issueStaffToken()` has 38 INFERRED edges - model-reasoned connections that need verification._