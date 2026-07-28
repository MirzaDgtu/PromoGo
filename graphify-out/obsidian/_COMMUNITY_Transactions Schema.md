---
type: community
members: 2
---

# Transactions Schema

**Members:** 2 nodes

## Members
- [[00004_create_transactions.sql]] - code - migrations/sql/00004_create_transactions.sql
- [[transactions]] - code - migrations/sql/00004_create_transactions.sql

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Transactions_Schema
SORT file.name ASC
```
