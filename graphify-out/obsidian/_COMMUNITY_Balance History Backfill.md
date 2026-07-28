---
type: community
members: 1
---

# Balance History Backfill

**Members:** 1 nodes

## Members
- [[00009_backfill_transaction_balance_after.sql]] - code - migrations/sql/00009_backfill_transaction_balance_after.sql

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Balance_History_Backfill
SORT file.name ASC
```
