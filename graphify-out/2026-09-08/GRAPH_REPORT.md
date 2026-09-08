# Graph Report - PromoGo  (2026-07-30)

## Corpus Check
- 160 files · ~85,928 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1515 nodes · 3672 edges · 92 communities (63 shown, 29 thin omitted)
- Extraction: 79% EXTRACTED · 21% INFERRED · 0% AMBIGUOUS · INFERRED: 771 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `121fea4c`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- customerauth_test.go
- okHandler
- Transaction
- StaffMembership
- handleGetMyTransactions
- StaffAuthService
- CustomerAuthService
- Config
- Organization
- LoyaltyService
- StaffUser
- customerFromContext
- Loyalty Platform Product Concept
- HashOpaqueToken
- Local Development Stack Skill
- Confirmed Decision Registry
- handlerFor
- newQRTestService
- doRequest
- handleCreateStoreAPIKey
- writeError
- Loyalty Mechanic Contract
- admin_organizations.go
- handleAdminLookupClient
- handleGetLoyaltyConfig
- newTestLimiter
- openapi_parity_test.go
- handleListAuditEvents
- Channel
- CustomerSession
- handleStaffOIDCLogin
- Q: Что на данный момент не хватает в нашем проекте?
- Q: Есть ли на данный момент регистрация и авторизация? Роли пользователей?
- Q: Давай обсудим данные дополнения. На мой взгляд это необходимо сделать
- PromoGo
- handleCreateStaffMembership
- Context
- must
- CustomerAccountRepository
- Run
- graphify
- 00006_client_store_composite_fk.sql
- 00018_create_staff_and_org_rbac.sql
- LedgerRepository
- Mechanic
- Client
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
- Sender
- transactions.go
- CustomerAccount
- Q: Создай для нашего проекта README.md
- loyalty_test.go
- New
- AuditEvent
- NormalizePhone
- testsupport_test.go
- CustomerDevice
- rateLimitRulesFor
- github.com/MirzaDgtu/PromoGo
- statusWriter
- otpStore
- runBootstrapAdmin
- CustomerSessionRepository
- New
- Deps
- Q: Расскажи про наш проект для инвесторов. Попробуй донести цель данного проекта. Постарайся сделать это кратко. Расскажи что сделано, что планируется в будущем
- httpserver/qr.go
- refund.go
- 00022_transaction_refunds.sql
- 00023_create_customer_devices.sql
- newTestSender
- HashOTP

## God Nodes (most connected - your core abstractions)
1. `doRequest()` - 96 edges
2. `newTestServer()` - 87 edges
3. `Client` - 48 edges
4. `writeError()` - 42 edges
5. `handlerFor()` - 38 edges
6. `Transaction` - 37 edges
7. `issueStaffToken()` - 36 edges
8. `itoa()` - 35 edges
9. `writeJSON()` - 31 edges
10. `seedOrganization()` - 31 edges

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

## Communities (92 total, 29 thin omitted)

### Community 0 - "customerauth_test.go"
Cohesion: 0.27
Nodes (20): defaultOTPConfig(), discardLogger(), Logger, T, newCustomerAuthTestDeps(), newFakeCustomerAccountRepo(), newFakeCustomerSessionRepo(), TestCustomerAuth_ConcurrentAccountCreateRaceRecovered() (+12 more)

### Community 1 - "okHandler"
Cohesion: 0.05
Nodes (66): Store, StoreAPIKey, StoreAPIKeyRepository, StoreRepository, fakeStoreAPIKeyRepo, fakeStoreRepo, staffPrincipalResolver, staffScope (+58 more)

### Community 2 - "Transaction"
Cohesion: 0.06
Nodes (32): Balance, BalanceRepository, Transaction, TransactionCursor, TransactionRepository, TransactionType, fakeMeTransactionRepo, Decimal (+24 more)

### Community 3 - "StaffMembership"
Cohesion: 0.15
Nodes (13): StaffMembership, StaffMembershipRepository, StaffStatus, StaffUserRepository, fakeStaffMembershipRepo, samePtr(), Context, Pool (+5 more)

### Community 4 - "handleGetMyTransactions"
Cohesion: 0.13
Nodes (30): meBalanceItem, meResponseBody, meTransactionItem, meTransactionsResponse, decodeTransactionCursor(), encodeTransactionCursor(), Time, TransactionRepository (+22 more)

### Community 5 - "StaffAuthService"
Cohesion: 0.09
Nodes (36): jwksDocument, OIDCClaims, oidcIDTokenClaims, OIDCVerifier, fakeStaffResolver, Context, Duration, Mutex (+28 more)

### Community 6 - "CustomerAuthService"
Cohesion: 0.17
Nodes (15): CustomerConsentRepository, CustomerSessionRepository, SMSSender, AuditEventRepository, ClientRepository, Context, CustomerAccountRepository, Duration (+7 more)

### Community 7 - "Config"
Cohesion: 0.17
Nodes (16): AntiFraudConfig, AppConfig, AuthConfig, Config, FCMConfig, LoggerConfig, OIDCConfig, PostgresConfig (+8 more)

### Community 8 - "Organization"
Cohesion: 0.07
Nodes (50): Organization, OrganizationRepository, fakeOrganizationRepo, Time, Context, Pool, NewOrganizationRepository(), BootstrapPlatformAdmin() (+42 more)

### Community 9 - "LoyaltyService"
Cohesion: 0.13
Nodes (24): NotificationChannel, Build(), accrualFingerprint(), BalanceRepository, ClientRepository, Context, Decimal, Duration (+16 more)

### Community 10 - "StaffUser"
Cohesion: 0.19
Nodes (9): StaffUser, fakeStaffUserRepo, Time, Context, Pool, Row, NewStaffUserRepository(), scanStaffUser() (+1 more)

### Community 11 - "customerFromContext"
Cohesion: 0.13
Nodes (18): customerContextKey, registerDeviceRequestBody, registerDeviceResponseBody, staffContextKey, storeContextKey, customerFromContext(), CustomerDeviceRepository, HandlerFunc (+10 more)

### Community 12 - "Loyalty Platform Product Concept"
Cohesion: 0.10
Nodes (31): Analytics and Customer Communications, Business Scale Discovery, Client Discovery Checklist, Customer Identity and Channels, Loyalty Mechanics Discovery, 1C and POS Integration Discovery, Security, Launch, and Governance, Cross-Cutting Product Requirements (+23 more)

### Community 13 - "HashOpaqueToken"
Cohesion: 0.21
Nodes (21): accessClaims, tokenType, GenerateAPIKey(), GenerateRefreshToken(), Duration, RegisteredClaims, HashOpaqueToken(), issueAccessToken() (+13 more)

### Community 14 - "Local Development Stack Skill"
Cohesion: 0.12
Nodes (21): Add Notification Channel Skill, Best-Effort Notification Delivery, Notification Channel Contract, Notification Provider Fallback, Checked Increment Update Pattern, Database Migration Skill, PromoGo Schema Conventions, SQL-Only Migration Directory (+13 more)

### Community 15 - "Confirmed Decision Registry"
Cohesion: 0.12
Nodes (21): Question-to-Decision Lifecycle, Post-Change Tests and Graph Update, Query-First Graphify Investigation, Source Verification of Graph Relationships, Shared Codex and Claude Knowledge Graph, Confirmed Decision Registry, DEC-001: 1C Is an Event Source, Not the Loyalty Core, DEC-002: Phase 1 Uses One Store and Points (+13 more)

### Community 16 - "handlerFor"
Cohesion: 0.20
Nodes (16): authTokensResponseBody, otpRequestBody, otpVerifyBody, refreshTokenBody, HandlerFunc, Logger, Time, handleCustomerLogout() (+8 more)

### Community 17 - "newQRTestService"
Cohesion: 0.10
Nodes (38): fakeBalanceRepo, fakeCustomerAccountRepo, AuditEventRepository, BalanceRepository, ClientRepository, Context, CustomerAccountRepository, Duration (+30 more)

### Community 18 - "doRequest"
Cohesion: 0.07
Nodes (136): adminReq(), Request, T, TestHandleAdminListClientTransactions_NotFoundWrongStore(), TestHandleAdminListClientTransactions_Success(), TestHandleAdminLookupClient_MaskedForSupportViewer(), TestHandleAdminLookupClient_NotFound(), TestHandleAdminLookupClient_UnmaskedForRetailerAdmin() (+128 more)

### Community 19 - "handleCreateStoreAPIKey"
Cohesion: 0.26
Nodes (16): createAPIKeyBody, storeAPIKeyResponseBody, apiKeyToBody(), AuditEventRepository, HandlerFunc, Logger, Request, ResponseWriter (+8 more)

### Community 20 - "writeError"
Cohesion: 0.17
Nodes (24): balanceResponseBody, BalanceRepository, ClientRepository, HandlerFunc, Logger, handleGetClientBalance(), handleLookupClientByPhone(), Context (+16 more)

### Community 21 - "Loyalty Mechanic Contract"
Cohesion: 0.14
Nodes (14): Graphify Skill Trigger, Graphify Workflow for Claude Code, Add Mechanic Skill, Decimal Point Calculation, Loyalty Mechanic Contract, Pure Mechanic Decision Logic, Graphify Rules for Codex, Scoped Graph Queries (+6 more)

### Community 22 - "admin_organizations.go"
Cohesion: 0.22
Nodes (12): createOrganizationBody, createStoreBody, organizationResponseBody, storeResponseBody, HandlerFunc, Logger, OrganizationRepository, StoreRepository (+4 more)

### Community 23 - "handleAdminLookupClient"
Cohesion: 0.23
Nodes (11): adminClientResponseBody, adminTransactionResponseBody, BalanceRepository, ClientRepository, HandlerFunc, Logger, StoreRepository, Time (+3 more)

### Community 24 - "handleGetLoyaltyConfig"
Cohesion: 0.31
Nodes (10): loyaltyConfigResponseBody, putLoyaltyConfigBody, Decimal, HandlerFunc, Logger, LoyaltyConfigRepository, StoreRepository, handleGetLoyaltyConfig() (+2 more)

### Community 25 - "newTestLimiter"
Cohesion: 0.11
Nodes (34): Context, Duration, New(), Miniredis, T, newTestLimiter(), TestLimiter_AllowsUpToLimit(), TestLimiter_BackendErrorPropagates() (+26 more)

### Community 26 - "openapi_parity_test.go"
Cohesion: 0.27
Nodes (12): contour, openAPIDoc, openAPIOperation, openAPIOperationEntry, routeMeta, staffScopeKind, flattenOpenAPIDoc(), T (+4 more)

### Community 27 - "handleListAuditEvents"
Cohesion: 0.29
Nodes (6): auditEventResponseBody, AuditEventRepository, HandlerFunc, Logger, Time, handleListAuditEvents()

### Community 28 - "Channel"
Cohesion: 0.38
Nodes (4): Context, Logger, New(), Channel

### Community 29 - "CustomerSession"
Cohesion: 0.14
Nodes (11): CustomerConsent, CustomerSession, Time, Context, fakeClientRepo, Miniredis, Mutex, customerAuthTestDeps (+3 more)

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
Cohesion: 0.12
Nodes (21): Permission, Role, createStaffMembershipBody, staffMembershipResponseBody, updateStaffMembershipBody, HasPermission(), AuditEventRepository, HandlerFunc (+13 more)

### Community 36 - "Context"
Cohesion: 0.15
Nodes (5): fakeCustomerSessionRepo, fakeFullClientRepo, fakeNotifier, Context, newFakeCustomerSessionRepo()

### Community 38 - "CustomerAccountRepository"
Cohesion: 0.33
Nodes (6): Context, Pool, Row, NewCustomerAccountRepository(), scanCustomerAccount(), CustomerAccountRepository

### Community 45 - "Client"
Cohesion: 0.07
Nodes (32): Channel, Client, ClientRepository, Channel, fakeClientRepo, fakeDeviceRepo, fakeMessagingSender, messagingSender (+24 more)

### Community 66 - "Sender"
Cohesion: 0.21
Nodes (9): SMSConfig, retryableError, Sender, smsRequest, smsResponse, Context, Logger, isRetryable() (+1 more)

### Community 67 - "transactions.go"
Cohesion: 0.40
Nodes (5): accrueRequestBody, redeemRequestBody, redeemResponseBody, transactionResponseBody, Decimal

### Community 68 - "CustomerAccount"
Cohesion: 0.19
Nodes (7): CustomerAccount, CustomerAccountRepository, CustomerAccountStatus, CustomerConsentRepository, CustomerSessionRepository, fakeCustomerAccountRepo, fakeCustomerAccountRepo

### Community 69 - "Q: Создай для нашего проекта README.md"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Создай для нашего проекта README.md, Source Nodes

### Community 70 - "loyalty_test.go"
Cohesion: 0.10
Nodes (41): LoyaltyConfig, LoyaltyConfigRepository, Decimal, Context, Pool, NewLoyaltyConfigRepository(), AntiFraudConfig, Context (+33 more)

### Community 71 - "New"
Cohesion: 0.08
Nodes (23): App, Context, Logger, Pool, Server, New(), Context, Pool (+15 more)

### Community 72 - "AuditEvent"
Cohesion: 0.16
Nodes (11): AuditActorType, AuditEvent, AuditEventRepository, Time, Context, Pool, Row, NewAuditEventRepository() (+3 more)

### Community 73 - "NormalizePhone"
Cohesion: 0.17
Nodes (11): MaskPhone(), NormalizePhone(), T, TestMaskPhone(), TestNormalizePhone(), TestNormalizePhoneIdempotent(), maskPhoneIf(), Context (+3 more)

### Community 74 - "testsupport_test.go"
Cohesion: 0.22
Nodes (18): fakeAuditEventRepo, fakeBalanceRepo, fakeCustomerConsentRepo, fakeFullTransactionRepo, fakeLedgerRepo, fakeLoyaltyConfigRepo, fakeSMSSender, testFakes (+10 more)

### Community 75 - "CustomerDevice"
Cohesion: 0.24
Nodes (5): CustomerDevice, CustomerDeviceRepository, fakeCustomerDeviceRepo, Time, newFakeCustomerDeviceRepo()

### Community 76 - "rateLimitRulesFor"
Cohesion: 0.30
Nodes (14): clientIPForRateLimit(), Duration, Prefix, Request, ipRule(), ParseTrustedProxies(), phoneQueryRule(), principalRule() (+6 more)

### Community 79 - "statusWriter"
Cohesion: 0.29
Nodes (5): statusWriter, Handler, Logger, ResponseWriter, loggingMW()

### Community 80 - "otpStore"
Cohesion: 0.33
Nodes (7): Context, Duration, ipKey(), newOTPStore(), phoneKey(), OTPConfig, otpStore

### Community 81 - "runBootstrapAdmin"
Cohesion: 0.53
Nodes (4): runBootstrapAdmin(), main(), resolveConfigPath(), runServer()

### Community 82 - "CustomerSessionRepository"
Cohesion: 0.27
Nodes (6): Context, Pool, Row, NewCustomerSessionRepository(), scanCustomerSession(), CustomerSessionRepository

### Community 83 - "New"
Cohesion: 0.50
Nodes (4): Logger, New(), parseLevel(), Level

### Community 84 - "Deps"
Cohesion: 0.11
Nodes (17): HTTPConfig, Deps, AuditEventRepository, BalanceRepository, ClientRepository, Context, CustomerAccountRepository, CustomerDeviceRepository (+9 more)

### Community 85 - "Q: Расскажи про наш проект для инвесторов. Попробуй донести цель данного проекта. Постарайся сделать это кратко. Расскажи что сделано, что планируется в будущем"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Расскажи про наш проект для инвесторов. Попробуй донести цель данного проекта. Постарайся сделать это кратко. Расскажи что сделано, что планируется в будущем, Source Nodes

### Community 86 - "httpserver/qr.go"
Cohesion: 0.40
Nodes (4): issueQRResponseBody, resolveQRRequestBody, resolveQRResponseBody, Time

### Community 87 - "refund.go"
Cohesion: 0.67
Nodes (3): refundRequestBody, refundResponseBody, Decimal

### Community 90 - "newTestSender"
Cohesion: 0.42
Nodes (9): Buffer, T, newTestSender(), TestHTTPSMS_Send_ClientErrorNotRetried(), TestHTTPSMS_Send_ServerErrorRetriedThenFails(), TestHTTPSMS_Send_ServerErrorThenSuccessRecovers(), TestHTTPSMS_Send_Success(), TestHTTPSMS_Send_TimeoutIsRetried() (+1 more)

### Community 91 - "HashOTP"
Cohesion: 0.33
Nodes (7): GenerateOTPCode(), HashOTP(), T, TestGenerateOTPCodeIsSixDigits(), TestVerifyOTP(), VerifyOTP(), constantTimeEqual()

## Knowledge Gaps
- **114 isolated node(s):** `python`, `github.com/MirzaDgtu/PromoGo`, `jwksDocument`, `StoreAPIKeyRepository`, `AuditEventRepository` (+109 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **29 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Work-memory lessons

**Preferred sources** — corroborated by past sessions; start here.
- `RequireStoreAPIKey()` (2× useful, score=1.953914705)
- `Web Configuration and Administrative Management` (2× useful, score=1.953914705) _(code changed — re-verify)_
- `Customer Registration, Identity, Mobile, and Notifications` (2× useful, score=1.953914705) _(code changed — re-verify)_

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Client` connect `Client` to `Sender`, `Context`, `StaffAuthService`, `CustomerAuthService`, `New`, `loyalty_test.go`, `LoyaltyService`, `testsupport_test.go`, `otpStore`, `newQRTestService`, `newTestLimiter`?**
  _High betweenness centrality (0.141) - this node is a cross-community bridge._
- **Why does `testFakes` connect `testsupport_test.go` to `okHandler`, `StaffMembership`, `Context`, `CustomerAccount`, `Organization`, `StaffUser`, `CustomerDevice`, `Client`, `doRequest`?**
  _High betweenness centrality (0.114) - this node is a cross-community bridge._
- **Why does `handlerFor()` connect `handlerFor` to `handleCreateStaffMembership`, `handleGetMyTransactions`, `customerFromContext`, `doRequest`, `handleCreateStoreAPIKey`, `Deps`, `writeError`, `admin_organizations.go`, `handleAdminLookupClient`, `handleGetLoyaltyConfig`, `newTestLimiter`, `openapi_parity_test.go`, `handleListAuditEvents`, `handleStaffOIDCLogin`?**
  _High betweenness centrality (0.088) - this node is a cross-community bridge._
- **Are the 85 inferred relationships involving `doRequest()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`doRequest()` has 85 INFERRED edges - model-reasoned connections that need verification._
- **Are the 82 inferred relationships involving `newTestServer()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`newTestServer()` has 82 INFERRED edges - model-reasoned connections that need verification._
- **Are the 39 inferred relationships involving `writeError()` (e.g. with `handleCreateStoreAPIKey()` and `handleListStoreAPIKeys()`) actually correct?**
  _`writeError()` has 39 INFERRED edges - model-reasoned connections that need verification._
- **What connects `python`, `github.com/MirzaDgtu/PromoGo`, `jwksDocument` to the rest of the system?**
  _114 weakly-connected nodes found - possible documentation gaps or missing edges._