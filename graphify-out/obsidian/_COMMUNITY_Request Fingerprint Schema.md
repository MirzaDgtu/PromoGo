---
type: community
members: 2
---

# Request Fingerprint Schema

**Members:** 2 nodes

## Members
- [[00010_transaction_request_fingerprint.sql]] - code - migrations/sql/00010_transaction_request_fingerprint.sql
- [[transactions_3]] - code - migrations/sql/00010_transaction_request_fingerprint.sql

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Request_Fingerprint_Schema
SORT file.name ASC
```
