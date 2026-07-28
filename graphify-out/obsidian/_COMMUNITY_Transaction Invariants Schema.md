---
type: community
members: 2
---

# Transaction Invariants Schema

**Members:** 2 nodes

## Members
- [[00011_transaction_amount_and_sign_constraints.sql]] - code - migrations/sql/00011_transaction_amount_and_sign_constraints.sql
- [[transactions_4]] - code - migrations/sql/00011_transaction_amount_and_sign_constraints.sql

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Transaction_Invariants_Schema
SORT file.name ASC
```
