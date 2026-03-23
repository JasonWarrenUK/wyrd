## Roadmap files (`docs/roadmaps/`)

**Always analyse the full dependency chain before editing a roadmap file.** Each task carries explicit `depends on` annotations and the Mermaid progress map encodes the same graph. Adding, removing, or re-scoping a task can unblock or block other tasks downstream.

Checklist when editing a roadmap:
1. Identify every task that lists the changed task as a dependency.
2. Update their `depends on` annotations if the dependency is renamed or split.
3. Update the Mermaid graph nodes and edges to match.
4. Re-evaluate blocked/open status for all affected tasks.
