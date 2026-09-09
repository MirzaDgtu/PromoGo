# Graph Report - PromoGo  (2026-09-09)

## Corpus Check
- 169 files · ~99,914 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1734 nodes · 3631 edges · 196 communities (68 shown, 128 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 489 edges (avg confidence: 0.81)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `5546094c`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- customerauth_test.go
- RequireStoreAPIKey
- handleGetMyTransactions
- StaffMembership
- RequireStaff
- StaffAuthService
- CustomerAuthService
- newTestServer
- Organization
- Transaction
- staffauth_test.go
- BalanceRepository
- Loyalty Platform Product Concept
- PromoGo audit remediation prompt
- Local Development Stack Skill
- Confirmed Decision Registry
- auth_customer.go
- newQRTestService
- doRequest
- resolveScopedStore
- MaskPhone
- Loyalty Mechanic Contract
- admin_organizations.go
- ClientRepository
- Deps
- newTestLimiter
- rateLimitRulesFor
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
- newTestChannel
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
- Config
- transactions.go
- AuditEvent
- Q: Создай для нашего проекта README.md
- loyalty_test.go
- .Accrue
- context.go
- openapi_parity_test.go
- StaffMembershipRepository
- testsupport_test.go
- Pool
- github.com/MirzaDgtu/PromoGo
- statusWriter
- BootstrapPlatformAdmin
- ClientRepository
- newFakeStaffMembershipRepo
- fakeStaffMembershipRepo
- BalanceRepository
- Q: Расскажи про наш проект для инвесторов. Попробуй донести цель данного проекта. Постарайся сделать это кратко. Расскажи что сделано, что планируется в будущем
- httpserver/qr.go
- refund.go
- 00022_transaction_refunds.sql
- 00023_create_customer_devices.sql
- Sender
- fakeLedgerRepo
- handleRegisterDevice
- 00026_loyalty_config_versioning.sql
- CustomerAccount
- testFakes
- 00025_refund_replay_snapshot.sql
- Client
- Build
- NotificationChannel
- Channel
- AuditEventRepository
- AuditEventRepository
- StaffUserRepository
- fakeStaffUserRepo
- OrganizationRepository
- StaffUserRepository
- AuditEventRepository
- MapClaims
- PrivateKey
- 00027_transaction_currency.sql
- SMSSender
- CustomerConsentRepository
- ResponseRecorder
- HandlerFunc
- CustomerAccountRepository
- CustomerAuthService
- CustomerDeviceRepository
- LoyaltyConfigRepository
- Deps
- StaffAuthService
- StaffMembershipRepository
- StoreAPIKeyRepository
- StoreRepository
- TransactionRepository
- AuditEvent
- CustomerAccount
- CustomerConsent
- CustomerSession
- LoyaltyConfig
- Organization
- Role
- Row
- CustomerAccountRepository
- CustomerAccount
- CustomerAuthService
- CustomerConsent
- CustomerSession
- fakeClientRepo
- OTPConfig
- writeError
- Sender
- config_test.go
- runBootstrapAdmin
- newTestSender
- New
- AppConfig
- HTTPConfig
- Client
- Pool
- AntiFraudConfig
- Request
- Request
- AuditEventRepository
- ClientRepository
- RateLimitConfig
- Prefix
- Server
- fakeCustomerDeviceRepo
- 00024_audit_events_append_only_and_index.sql
- StoreRepository
- Time
- TransactionRepository
- Decimal
- StoreRepository
- AuditEvent
- Handler
- Miniredis
- Mutex
- RateLimitConfig
- Transaction
- TransactionCursor
- TransactionType
- LedgerRepository
- Row
- BalanceRepository
- ClientRepository
- ClientRepository
- TransactionRepository
- AntiFraudConfig
- LoyaltyConfig
- LoyaltyConfig
- CustomerAccountRepository
- Logger
- TransactionRepository
- Client
- fakeCustomerAccountRepo
- Request
- T
- Transaction
- TransactionCursor
- TransactionType
- Balance
- Duration
- Pool
- meTransactionItem

## God Nodes (most connected - your core abstractions)
1. `newTestServer()` - 104 edges
2. `issueStaffToken()` - 49 edges
3. `adminReq()` - 43 edges
4. `Transaction` - 38 edges
5. `doRequest()` - 38 edges
6. `writeError()` - 31 edges
7. `testFakes` - 27 edges
8. `CustomerAuthService` - 27 edges
9. `pointsConfig()` - 26 edges
10. `Deps` - 25 edges

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

## Communities (196 total, 128 thin omitted)

### Community 0 - "customerauth_test.go"
Cohesion: 0.06
Nodes (54): App, CustomerAccount, CustomerAccountStatus, CustomerConsent, CustomerSession, CustomerSessionRepository, fakeClientRepo, Context (+46 more)

### Community 1 - "RequireStoreAPIKey"
Cohesion: 0.08
Nodes (44): StoreAPIKey, StoreAPIKeyRepository, fakeStoreAPIKeyRepo, fakeStoreRepo, storeAPIKeyContextKey, Time, constantTimeHashEqual(), Context (+36 more)

### Community 2 - "handleGetMyTransactions"
Cohesion: 0.14
Nodes (31): CustomerAccountRepository, Deps, meBalanceItem, meResponseBody, meTransactionItem, meTransactionsResponse, decodeTransactionCursor(), encodeTransactionCursor() (+23 more)

### Community 3 - "StaffMembership"
Cohesion: 0.16
Nodes (10): StaffMembership, StaffMembershipRepository, StaffStatus, StaffUserRepository, Role, Context, Mutex, Role (+2 more)

### Community 4 - "RequireStaff"
Cohesion: 0.08
Nodes (50): accessClaims, tokenType, Permission, Role, staffPrincipalResolver, staffScope, GenerateOTPCode(), HashOTP() (+42 more)

### Community 5 - "StaffAuthService"
Cohesion: 0.09
Nodes (36): jwksDocument, OIDCClaims, oidcIDTokenClaims, OIDCVerifier, fakeStaffResolver, Context, Duration, Mutex (+28 more)

### Community 6 - "CustomerAuthService"
Cohesion: 0.08
Nodes (35): AuditActorType, CustomerSessionRepository, CustomerAccountRepository, CustomerConsentRepository, HandlerFunc, RequireCustomerSession(), activeCustomerAccounts(), fakeCustomerAccountRepo (+27 more)

### Community 7 - "newTestServer"
Cohesion: 0.08
Nodes (91): authTokensResponseBody, adminReq(), Request, T, TestHandleAdminListClientTransactions_NotFoundWrongStore(), TestHandleAdminListClientTransactions_Success(), TestHandleAdminLookupClient_MaskedForSupportViewer(), TestHandleAdminLookupClient_NotFound() (+83 more)

### Community 8 - "Organization"
Cohesion: 0.18
Nodes (10): Organization, OrganizationRepository, Time, Context, Pool, NewOrganizationRepository(), Context, Mutex (+2 more)

### Community 9 - "Transaction"
Cohesion: 0.07
Nodes (42): Balance, BalanceRepository, ClientRepository, Decimal, Transaction, TransactionCursor, TransactionRepository, TransactionType (+34 more)

### Community 10 - "staffauth_test.go"
Cohesion: 0.23
Nodes (17): fakeAuditEventRepo, Server, StaffAuthService, T, newStaffAuthTestDeps(), signStaffTestIDToken(), startStaffTestJWKSServer(), TestStaffAuth_DisabledAccountRejected() (+9 more)

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
Cohesion: 0.25
Nodes (13): authTokensResponseBody, otpRequestBody, otpVerifyBody, refreshTokenBody, HandlerFunc, Logger, Time, handleCustomerLogout() (+5 more)

### Community 17 - "newQRTestService"
Cohesion: 0.08
Nodes (43): AuditEventRepository, fakeBalanceRepo, fakeCustomerAccountRepo, Balance, BalanceRepository, Client, ClientRepository, Context (+35 more)

### Community 18 - "doRequest"
Cohesion: 0.09
Nodes (66): T, TestHandleStaffOIDCLogin_InvalidToken(), TestHandleStaffOIDCLogin_MalformedBody(), Request, T, staffLoginReq(), TestRateLimit_AccrualPrincipalIsolatedByStoreAPIKey(), TestRateLimit_AdminStaffPrincipalExceeded() (+58 more)

### Community 19 - "resolveScopedStore"
Cohesion: 0.27
Nodes (15): createAPIKeyBody, storeAPIKeyResponseBody, apiKeyToBody(), HandlerFunc, Logger, Request, ResponseWriter, Store (+7 more)

### Community 20 - "MaskPhone"
Cohesion: 0.17
Nodes (11): MaskPhone(), NormalizePhone(), T, TestMaskPhone(), TestNormalizePhone(), TestNormalizePhoneIdempotent(), maskPhoneIf(), Context (+3 more)

### Community 21 - "Loyalty Mechanic Contract"
Cohesion: 0.14
Nodes (14): Graphify Skill Trigger, Graphify Workflow for Claude Code, Add Mechanic Skill, Decimal Point Calculation, Loyalty Mechanic Contract, Pure Mechanic Decision Logic, Graphify Rules for Codex, Scoped Graph Queries (+6 more)

### Community 22 - "admin_organizations.go"
Cohesion: 0.22
Nodes (12): createOrganizationBody, createStoreBody, organizationResponseBody, storeResponseBody, HandlerFunc, Logger, OrganizationRepository, StoreRepository (+4 more)

### Community 25 - "newTestLimiter"
Cohesion: 0.09
Nodes (40): Client, ClientRepository, Time, Context, Duration, New(), Miniredis, T (+32 more)

### Community 26 - "rateLimitRulesFor"
Cohesion: 0.15
Nodes (24): Handler, clientIPContextKey, Duration, Prefix, ipRule(), ParseTrustedProxies(), phoneQueryRule(), principalRule() (+16 more)

### Community 27 - "handleListAuditEvents"
Cohesion: 0.29
Nodes (6): auditEventResponseBody, AuditEventRepository, HandlerFunc, Logger, Time, handleListAuditEvents()

### Community 28 - "Channel"
Cohesion: 0.38
Nodes (4): Context, Logger, New(), Channel

### Community 29 - "StaffUser"
Cohesion: 0.28
Nodes (8): StaffUser, Time, Context, Pool, Row, NewStaffUserRepository(), scanStaffUser(), StaffUserRepository

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
Cohesion: 0.25
Nodes (5): CustomerSession, fakeCustomerSessionRepo, fakeFullClientRepo, Client, Context

### Community 38 - "Q: Ознакомься с проектом. Его идеей и целью. Если видишь изъяны, допущения и возможности улучшения, то распиши их. Проведи полный аудит приложения"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Ознакомься с проектом. Его идеей и целью. Если видишь изъяны, допущения и возможности улучшения, то распиши их. Проведи полный аудит приложения, Source Nodes

### Community 45 - "newTestChannel"
Cohesion: 0.08
Nodes (30): CustomerDevice, CustomerDeviceRepository, Channel, fakeClientRepo, fakeDeviceRepo, fakeMessagingSender, messagingSender, Time (+22 more)

### Community 66 - "Config"
Cohesion: 0.15
Nodes (16): AntiFraudConfig, AppConfig, AuthConfig, Config, FCMConfig, HTTPConfig, LoggerConfig, OIDCConfig (+8 more)

### Community 67 - "transactions.go"
Cohesion: 0.40
Nodes (5): accrueRequestBody, redeemRequestBody, redeemResponseBody, transactionResponseBody, Decimal

### Community 68 - "AuditEvent"
Cohesion: 0.14
Nodes (14): AuditActorType, AuditEvent, fakeAuditEventRepo, Time, auditCreate(), AuditEventRepository, Context, Logger (+6 more)

### Community 69 - "Q: Создай для нашего проекта README.md"
Cohesion: 0.40
Nodes (4): Answer, Outcome, Q: Создай для нашего проекта README.md, Source Nodes

### Community 70 - "loyalty_test.go"
Cohesion: 0.08
Nodes (49): LoyaltyConfig, LoyaltyConfigRepository, LoyaltyConfigVersion, Decimal, Time, Context, Pool, NewLoyaltyConfigRepository() (+41 more)

### Community 71 - ".Accrue"
Cohesion: 0.13
Nodes (12): Balance, BalanceRepository, Context, New(), T, TestMechanic_Accrue(), TestMechanic_Name(), Context (+4 more)

### Community 72 - "context.go"
Cohesion: 0.15
Nodes (13): Store, StoreRepository, customerContextKey, staffContextKey, storeContextKey, customerFromContext(), Context, staffFromContext() (+5 more)

### Community 73 - "openapi_parity_test.go"
Cohesion: 0.25
Nodes (13): contour, openAPIDoc, openAPIOperation, openAPIOperationEntry, routeMeta, staffScopeKind, flattenOpenAPIDoc(), T (+5 more)

### Community 74 - "StaffMembershipRepository"
Cohesion: 0.27
Nodes (8): Context, Pool, Role, Row, NewStaffMembershipRepository(), queryStaffMemberships(), scanStaffMembership(), StaffMembershipRepository

### Community 75 - "testsupport_test.go"
Cohesion: 0.21
Nodes (13): fakeNotifier, Deps, newFakeAuditEventRepo(), newFakeBalanceRepo(), newFakeCustomerAccountRepo(), newFakeCustomerDeviceRepo(), newFakeCustomerSessionRepo(), newFakeFullClientRepo() (+5 more)

### Community 79 - "statusWriter"
Cohesion: 0.29
Nodes (5): statusWriter, Handler, Logger, ResponseWriter, loggingMW()

### Community 80 - "BootstrapPlatformAdmin"
Cohesion: 0.42
Nodes (10): BootstrapPlatformAdmin(), Context, OrganizationRepository, StaffMembershipRepository, StaffUserRepository, resolveOrCreatePlatformAdminMembership(), resolveOrCreateStaffUser(), resolveOrganization() (+2 more)

### Community 82 - "newFakeStaffMembershipRepo"
Cohesion: 0.55
Nodes (9): T, newFakeOrganizationRepo(), TestBootstrapPlatformAdmin_ByOrgID(), TestBootstrapPlatformAdmin_FreshCreateSucceeds(), TestBootstrapPlatformAdmin_RerunIsIdempotent(), TestBootstrapPlatformAdmin_UnknownOrgNameReturnsError(), TestBootstrapPlatformAdmin_UpgradesExistingLesserRoleMembership(), newFakeStaffMembershipRepo() (+1 more)

### Community 83 - "fakeStaffMembershipRepo"
Cohesion: 0.24
Nodes (5): fakeStaffMembershipRepo, samePtr(), Role, StaffMembership, StaffStatus

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

### Community 93 - "00026_loyalty_config_versioning.sql"
Cohesion: 0.50
Nodes (3): loyalty_config_history, loyalty_configs, transactions

### Community 94 - "CustomerAccount"
Cohesion: 0.23
Nodes (8): CustomerAccount, fakeCustomerAccountRepo, Context, Pool, Row, NewCustomerAccountRepository(), scanCustomerAccount(), CustomerAccountRepository

### Community 95 - "testFakes"
Cohesion: 0.17
Nodes (10): CustomerConsent, fakeStoreAPIKeyRepo, fakeStoreRepo, fakeCustomerConsentRepo, fakeLoyaltyConfigRepo, fakeOrganizationRepo, fakeSMSSender, testFakes (+2 more)

### Community 97 - "Client"
Cohesion: 0.07
Nodes (41): Client, adminClientResponseBody, adminTransactionResponseBody, balanceResponseBody, fakeMeClientRepo, loyaltyConfigResponseBody, loyaltyConfigVersionResponseBody, putLoyaltyConfigBody (+33 more)

### Community 119 - "Deps"
Cohesion: 0.12
Nodes (16): AuditEventRepository, CustomerAuthService, CustomerDeviceRepository, Deps, Context, Logger, Prefix, Limiter (+8 more)

### Community 140 - "writeError"
Cohesion: 0.27
Nodes (15): HandlerFunc, Logger, handleIssueQR(), handleResolveQR(), HandlerFunc, Logger, handleRefundTransaction(), ResponseWriter (+7 more)

### Community 141 - "Sender"
Cohesion: 0.19
Nodes (10): SMSConfig, retryableError, Sender, smsRequest, smsResponse, Client, Context, Logger (+2 more)

### Community 142 - "config_test.go"
Cohesion: 0.34
Nodes (14): T, TestPostgresConfig_DSN_EscapesSpecialCharacters(), TestValidatePostgres_DisableAllowedInDevelopment(), TestValidatePostgres_DisableRejectedOutsideDevelopment(), TestValidatePostgres_PreferRejectedOutsideDevelopment(), TestValidatePostgres_RequireAllowedOutsideDevelopment(), TestValidateRedis_TLSNotRequiredInDevelopment(), TestValidateRedis_TLSRequiredOutsideDevelopment() (+6 more)

### Community 143 - "runBootstrapAdmin"
Cohesion: 0.27
Nodes (7): runBootstrapAdmin(), main(), resolveConfigPath(), runServer(), Context, Pool, NewPool()

### Community 144 - "newTestSender"
Cohesion: 0.50
Nodes (8): Buffer, T, newTestSender(), TestHTTPSMS_Send_ClientErrorNotRetried(), TestHTTPSMS_Send_ServerErrorRetriedThenFails(), TestHTTPSMS_Send_ServerErrorThenSuccessRecovers(), TestHTTPSMS_Send_Success(), TestHTTPSMS_Send_TimeoutIsRetried()

### Community 145 - "New"
Cohesion: 0.50
Nodes (4): Logger, New(), parseLevel(), Level

## Knowledge Gaps
- **132 isolated node(s):** `meBalanceItem`, `transactions`, `LoyaltyConfigRepository`, `rollbackLoyaltyConfigBody`, `loyalty_configs` (+127 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **128 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Work-memory lessons

**Preferred sources** — corroborated by past sessions; start here.
- `Config` (3× useful, score=2.367564945)
- `RequireStoreAPIKey()` (3× useful, score=1.749407382)
- `PromoGo` (2× useful, score=1.38109161)
- `Docker Compose Local Stack` (2× useful, score=1.373010599)
- `Reliability, Operations, Quality, and Release Readiness` (2× useful, score=1.371791254)
- `Phase 2: Mechanics and Multistore Product` (2× useful, score=0.76398443)
- `Phased Product Roadmap` (2× useful, score=0.763477674)
- `Current Implementation and Scope Gaps` (2× useful, score=0.763477674)
- `Web Configuration and Administrative Management` (2× useful, score=0.754853036)
- `Customer Registration, Identity, Mobile, and Notifications` (2× useful, score=0.754853036)

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `writeError()` connect `writeError` to `RequireStoreAPIKey`, `handleCreateStaffMembership`, `RequireStaff`, `auth_customer.go`, `resolveScopedStore`, `admin_organizations.go`, `handleListAuditEvents`, `handleRegisterDevice`, `handleStaffOIDCLogin`?**
  _High betweenness centrality (0.128) - this node is a cross-community bridge._
- **Why does `New()` connect `doRequest` to `customerauth_test.go`, `RequireStoreAPIKey`, `RequireStaff`, `Deps`, `rateLimitRulesFor`?**
  _High betweenness centrality (0.114) - this node is a cross-community bridge._
- **Why does `New()` connect `Transaction` to `loyalty_test.go`, `newTestServer`, `testsupport_test.go`, `newQRTestService`, `doRequest`?**
  _High betweenness centrality (0.092) - this node is a cross-community bridge._
- **Are the 99 inferred relationships involving `newTestServer()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`newTestServer()` has 99 INFERRED edges - model-reasoned connections that need verification._
- **Are the 44 inferred relationships involving `issueStaffToken()` (e.g. with `TestHandleAdminListClientTransactions_NotFoundWrongStore()` and `TestHandleAdminListClientTransactions_Success()`) actually correct?**
  _`issueStaffToken()` has 44 INFERRED edges - model-reasoned connections that need verification._
- **Are the 2 inferred relationships involving `adminReq()` (e.g. with `TestRateLimit_AdminStaffPrincipalExceeded()` and `TestRateLimit_AdminStaffPrincipalIsolatedBetweenStaffUsers()`) actually correct?**
  _`adminReq()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `meBalanceItem`, `transactions`, `LoyaltyConfigRepository` to the rest of the system?**
  _132 weakly-connected nodes found - possible documentation gaps or missing edges._