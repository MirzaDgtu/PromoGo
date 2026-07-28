---
type: community
members: 3
---

# Client Store Integrity Schema

**Members:** 3 nodes

## Members
- [[00006_client_store_composite_fk.sql]] - code - migrations/sql/00006_client_store_composite_fk.sql
- [[clients_1]] - code - migrations/sql/00006_client_store_composite_fk.sql
- [[transactions_1]] - code - migrations/sql/00006_client_store_composite_fk.sql

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Client_Store_Integrity_Schema
SORT file.name ASC
```
