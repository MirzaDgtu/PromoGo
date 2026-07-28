---
type: community
members: 2
---

# Loyalty Configuration Schema

**Members:** 2 nodes

## Members
- [[00005_create_loyalty_configs.sql]] - code - migrations/sql/00005_create_loyalty_configs.sql
- [[loyalty_configs]] - code - migrations/sql/00005_create_loyalty_configs.sql

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Loyalty_Configuration_Schema
SORT file.name ASC
```
