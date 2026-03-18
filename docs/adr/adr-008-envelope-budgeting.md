# ADR-008: Envelope Budgeting Without Financial Tracking

**Status:** Accepted

## Context

Financial awareness is a real need, but full financial tracking (bank account sync, transaction import, CSV mapping) adds significant complexity for a personal tool. The question was whether financial data belongs inside the graph at all, and if so, at what granularity.

## Decision

Wyrd implements envelope budgeting. Budget envelopes are nodes with a `budget` type. Spending is logged directly on the envelope via `wyrd spend` commands. No accounts, no transactions as separate nodes, no bank import, no privacy partitioning.

Each envelope has a category, period, allocated amount, and a `spend_log` array of dated entries. Budget views aggregate the log and display remaining balance per envelope.

Edges to other nodes (projects, tasks) are optional, enabling queries like "how much have I spent on nursery prep?"

**Rejected alternatives:** full financial tracking with transaction nodes and CSV import (over-engineered for the use case). Separate privacy-scoped store partitions (unnecessary when there's no sensitive financial data to protect). Time budgets (removed; the user found no value in the feature).

## Consequences

**Positive:** minimal implementation surface. `wyrd spend records 18.50 "Discogs"` is the entire input interface. Envelopes are regular nodes; they work with the existing query engine, views, and ritual system. The budget view follows the "consequences visible" philosophy (progress bars, status glyphs).

**Negative:** no automated data entry. Every spend is manual. No reconciliation against actual bank transactions. Users wanting real financial tracking would need to build it as a plugin or use a separate tool.
