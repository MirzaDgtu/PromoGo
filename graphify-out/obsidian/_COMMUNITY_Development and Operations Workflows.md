---
type: community
members: 21
---

# Development and Operations Workflows

**Members:** 21 nodes

## Members
- [[Add Notification Channel Skill]] - document - .claude/skills/add-notification-channel/SKILL.md
- [[Application Health Verification]] - concept - .claude/skills/dev-stack/SKILL.md
- [[Best-Effort Notification Delivery]] - rationale - .claude/skills/add-notification-channel/SKILL.md
- [[Checked Increment Update Pattern]] - rationale - .claude/skills/db-migrate/SKILL.md
- [[Database Migration Skill]] - document - .claude/skills/db-migrate/SKILL.md
- [[Docker Compose Local Stack]] - document - deployments/docker-compose.yml
- [[FCM Log-Channel Fallback]] - rationale - configs/config.yaml
- [[Idempotent Webhook Replay Test]] - concept - .claude/skills/dev-stack/SKILL.md
- [[Local Development Stack Skill]] - document - .claude/skills/dev-stack/SKILL.md
- [[Notification Channel Contract]] - concept - .claude/skills/add-notification-channel/SKILL.md
- [[Notification Provider Fallback]] - rationale - .claude/skills/add-notification-channel/SKILL.md
- [[PostgreSQL Configuration]] - concept - configs/config.yaml
- [[PostgreSQL Service]] - concept - deployments/docker-compose.yml
- [[PromoGo Application Service]] - concept - deployments/docker-compose.yml
- [[PromoGo Runtime Configuration]] - document - configs/config.yaml
- [[PromoGo Schema Conventions]] - concept - .claude/skills/db-migrate/SKILL.md
- [[Redis Configuration]] - concept - configs/config.yaml
- [[Redis Service]] - concept - deployments/docker-compose.yml
- [[SQL-Only Migration Directory]] - rationale - .claude/skills/db-migrate/SKILL.md
- [[Test Store Seed Workflow]] - concept - .claude/skills/dev-stack/SKILL.md
- [[Transaction Idempotency Constraint]] - rationale - .claude/skills/db-migrate/SKILL.md

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Development_and_Operations_Workflows
SORT file.name ASC
```
