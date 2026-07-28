---
type: community
members: 2
---

# Balances Schema

**Members:** 2 nodes

## Members
- [[00003_create_balances.sql]] - code - migrations/sql/00003_create_balances.sql
- [[balances]] - code - migrations/sql/00003_create_balances.sql

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Balances_Schema
SORT file.name ASC
```
