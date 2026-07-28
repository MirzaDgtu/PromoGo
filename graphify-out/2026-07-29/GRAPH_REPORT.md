# Graph Report - C:\Projects\Golang\github.com\MirzaDgtu\PromoGo  (2026-07-28)

## Corpus Check
- Corpus is ~19,541 words - fits in a single context window. You may not need a graph.

## Summary
- 344 nodes · 522 edges · 35 communities (18 shown, 17 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 60 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- HTTP API Layer
- Product Roadmap and Discovery
- Loyalty Service Core
- Application Lifecycle and Persistence
- Balances and Loyalty Configuration
- Development and Operations Workflows
- Runtime Configuration and Startup
- Loyalty Service Tests
- Knowledge and Decisions
- Transaction Persistence
- Agent and Mechanic Guidance
- Client Persistence
- Store Authentication and Persistence
- Points Mechanic Tests
- HTTP Logging Middleware
- Notification Log Channel
- Embedded Migrations
- Migration Runner
- Graphify MCP Integration
- Client Store Integrity Schema
- Ledger Contract
- Mechanic Contract
- Knowledge Base Hub
- Stores Schema
- Clients Schema
- Balances Schema
- Transactions Schema
- Loyalty Configuration Schema
- Idempotency Schema
- Loyalty Configuration Constraints
- Request Fingerprint Schema
- Transaction Invariants Schema
- Domain Errors
- Balance History Backfill
- Go Module Dependencies

## God Nodes (most connected - your core abstractions)
1. `LoyaltyService` - 18 edges
2. `newTestService()` - 14 edges
3. `New()` - 13 edges
4. `Transaction` - 12 edges
5. `Client` - 11 edges
6. `New()` - 11 edges
7. `LoyaltyConfig` - 10 edges
8. `Deps` - 10 edges
9. `New()` - 10 edges
10. `pointsConfig()` - 10 edges

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

## Communities (35 total, 17 thin omitted)

### Community 0 - "HTTP API Layer"
Cohesion: 0.08
Nodes (34): accrueRequestBody, balanceResponseBody, Deps, redeemRequestBody, redeemResponseBody, storeContextKey, transactionResponseBody, BalanceRepository (+26 more)

### Community 1 - "Product Roadmap and Discovery"
Cohesion: 0.10
Nodes (31): Analytics and Customer Communications, Business Scale Discovery, Client Discovery Checklist, Customer Identity and Channels, Loyalty Mechanics Discovery, 1C and POS Integration Discovery, Security, Launch, and Governance, Cross-Cutting Product Requirements (+23 more)

### Community 2 - "Loyalty Service Core"
Cohesion: 0.16
Nodes (19): NotificationChannel, Build(), accrualFingerprint(), BalanceRepository, ClientRepository, Context, Decimal, Logger (+11 more)

### Community 3 - "Application Lifecycle and Persistence"
Cohesion: 0.11
Nodes (17): App, Context, Logger, Pool, Server, New(), Context, Pool (+9 more)

### Community 4 - "Balances and Loyalty Configuration"
Cohesion: 0.12
Nodes (12): Balance, BalanceRepository, LoyaltyConfig, LoyaltyConfigRepository, Decimal, Context, Context, Pool (+4 more)

### Community 5 - "Development and Operations Workflows"
Cohesion: 0.12
Nodes (21): Add Notification Channel Skill, Best-Effort Notification Delivery, Notification Channel Contract, Notification Provider Fallback, Checked Increment Update Pattern, Database Migration Skill, PromoGo Schema Conventions, SQL-Only Migration Directory (+13 more)

### Community 6 - "Runtime Configuration and Startup"
Cohesion: 0.14
Nodes (16): main(), AppConfig, Config, FCMConfig, HTTPConfig, LoggerConfig, PostgresConfig, RedisConfig (+8 more)

### Community 7 - "Loyalty Service Tests"
Cohesion: 0.27
Nodes (17): T, newFakeBalanceRepo(), newFakeClientRepo(), newTestService(), pointsConfig(), TestAccrue_ConcurrentClientCreateRaceRecovered(), TestAccrue_ConflictingPhoneDoesNotCreateOrphanClient(), TestAccrue_IdempotencyConflictOnMismatchedAmount() (+9 more)

### Community 8 - "Knowledge and Decisions"
Cohesion: 0.12
Nodes (21): Question-to-Decision Lifecycle, Post-Change Tests and Graph Update, Query-First Graphify Investigation, Source Verification of Graph Relationships, Shared Codex and Claude Knowledge Graph, Confirmed Decision Registry, DEC-001: 1C Is an Event Source, Not the Loyalty Core, DEC-002: Phase 1 Uses One Store and Points (+13 more)

### Community 9 - "Transaction Persistence"
Cohesion: 0.20
Nodes (10): Transaction, TransactionRepository, TransactionType, Decimal, Time, Context, Pool, NewTransactionRepository() (+2 more)

### Community 10 - "Agent and Mechanic Guidance"
Cohesion: 0.14
Nodes (14): Graphify Skill Trigger, Graphify Workflow for Claude Code, Add Mechanic Skill, Decimal Point Calculation, Loyalty Mechanic Contract, Pure Mechanic Decision Logic, Graphify Rules for Codex, Scoped Graph Queries (+6 more)

### Community 11 - "Client Persistence"
Cohesion: 0.26
Nodes (7): Client, ClientRepository, Time, Context, Pool, NewClientRepository(), ClientRepository

### Community 12 - "Store Authentication and Persistence"
Cohesion: 0.29
Nodes (6): Store, StoreRepository, Context, Pool, NewStoreRepository(), StoreRepository

### Community 13 - "Points Mechanic Tests"
Cohesion: 0.36
Nodes (5): New(), T, TestMechanic_Accrue(), TestMechanic_Name(), Mechanic

### Community 14 - "HTTP Logging Middleware"
Cohesion: 0.29
Nodes (5): Handler, statusWriter, Logger, ResponseWriter, loggingMW()

### Community 15 - "Notification Log Channel"
Cohesion: 0.38
Nodes (4): Context, Logger, New(), Channel

## Knowledge Gaps
- **36 isolated node(s):** `python`, `github.com/MirzaDgtu/PromoGo`, `BalanceRepository`, `ClientRepository`, `LedgerRepository` (+31 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **17 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `LoyaltyService` connect `Loyalty Service Core` to `HTTP API Layer`, `Loyalty Service Tests`?**
  _High betweenness centrality (0.146) - this node is a cross-community bridge._
- **Why does `New()` connect `Application Lifecycle and Persistence` to `Balances and Loyalty Configuration`, `Runtime Configuration and Startup`, `Transaction Persistence`, `Client Persistence`, `Store Authentication and Persistence`?**
  _High betweenness centrality (0.100) - this node is a cross-community bridge._
- **Why does `Deps` connect `HTTP API Layer` to `Loyalty Service Core`, `Runtime Configuration and Startup`?**
  _High betweenness centrality (0.079) - this node is a cross-community bridge._
- **Are the 7 inferred relationships involving `New()` (e.g. with `NewBalanceRepository()` and `NewClientRepository()`) actually correct?**
  _`New()` has 7 INFERRED edges - model-reasoned connections that need verification._
- **What connects `python`, `github.com/MirzaDgtu/PromoGo`, `BalanceRepository` to the rest of the system?**
  _36 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `HTTP API Layer` be split into smaller, more focused modules?**
  _Cohesion score 0.08333333333333333 - nodes in this community are weakly interconnected._
- **Should `Product Roadmap and Discovery` be split into smaller, more focused modules?**
  _Cohesion score 0.0989247311827957 - nodes in this community are weakly interconnected._
