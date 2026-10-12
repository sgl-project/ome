# Eviction planner

`Build(Input) (*Plan, error)` is a pure in-memory planner over
`artifactinventory.Inventory`. It reads no Kubernetes objects or node files,
changes no lifecycle state, and does not execute deletions.

The caller supplies a fixed evaluation time, capacity/available-byte snapshots,
per-node usage fractions `0 <= LowWatermark < HighWatermark <= 1`, and global
per-UID eligibility decisions covering serving, claims, and in-flight operations.
A missing decision excludes the entire associated artifact. For a pressure node,
a nil inventory means unevaluated; an empty non-nil inventory is a successful
empty mapping.

For example, with an already collected single-model `inventory`, its `uid`
(`types.UID`), and an observed `idleSince` (`*time.Time`, nil if unknown), after
the caller has checked global eligibility:

```go
now := time.Now().UTC()
plan, err := evictionplanner.Build(evictionplanner.Input{
	Now:    now,
	Policy: evictionplanner.DefaultPolicy(),
	Nodes: []evictionplanner.Node{{
		Name:           inventory.NodeName,
		CapacityBytes:  10 << 40,
		AvailableBytes: 1 << 40,
		HighWatermark:  .75,
		LowWatermark:   .70,
		Inventory:      inventory,
	}},
	EligibilityByUID: map[types.UID]evictionplanner.Eligibility{
		uid: {
			Eligible:  true,
			Reason:    "caller confirmed no serving, claim, or in-flight protection",
			IdleSince: idleSince,
		},
	},
})
if err != nil {
	return err
}
// Inspect plan; execution requires fresh revalidation.
```

A zero scoring policy uses version `v1`: 1/7/14-day age boundaries, weights
8/4/2/1, and a 64 GiB size scale. Missing or future idle history receives weight
8 and sorts after known history. A shared artifact uses the most recent known
idle start, the greatest associated weight, and the union of Ready node names
plus its current node. If any required history is unknown, group history is
unknown. Each normalized local directory is counted once.

For local size `S` in GiB, potential node count `N`, weight `W`, and remaining
local gap `R`, cost is `W * (N + S/64)` and greedy score is cost divided by
`min(S, R)`. The planner compares dynamic greedy, oldest-first, and every
sufficient single-artifact alternative after local redundancy removal. It
compares local total cost, excess bytes, group count, then stable ID sequence.
Insufficient candidate pools select all eligible groups in stable ID order and
report the shortfall without relaxing protection.

Pressure nodes are visited by decreasing initial utilization, then name. Already
selected UIDs earn a node's own artifact bytes only when they cover every known
reference in that artifact. A projected usage strictly below H skips further
selection; other nodes target L. H starts at `ceil(capacity * H)` bytes and L
allows `floor(capacity * L)` bytes, using the decimal fraction representation.
After all nodes, reverse insertion pruning preserves frozen goals: skipped nodes
stay below H, reachable nodes stay at or below L, and insufficient nodes keep
their final pre-pruning benefit.

`Plan` retains final ordered groups, deduplicated CR identities, candidate
weights/node sets/local sizes/initial scores, local alternatives, exclusions,
pruned group IDs, and final per-node byte projections and L shortfalls. An
alternative's `Selected` flag records the local decision before batch pruning;
its cost is not a global deletion-byte cost. Initially below-H and unevaluated
nodes are not batch goals and receive no credited benefit.
`InitialScore` uses the node's remaining gap when it is first considered; greedy
selection recomputes scores after every pick. Zero indicates no local scoring
was needed for that candidate (the node was skipped or its UIDs were covered).

Invalid capacity or policy snapshots, conflicting or empty eligible BaseModel
identities, duplicate normalized paths, and arithmetic overflow return an error. Known inventory
issues, mapping ineligibility, missing sizes, and nonpositive sizes exclude the
artifact. Nodes initially below H do not inspect inventories. Input slices, maps, and timestamps are not modified.

The inventory is only the supplied Ready snapshot, not a complete live reference
census. Execution must freshly revalidate CR identity, eligibility, references,
and capacity; reported bytes are estimates, not observed reclamation. Policy
versions should change whenever the caller changes scoring parameters.

Validation:

```sh
go test ./pkg/evictionplanner
go test -race ./pkg/evictionplanner
go vet ./pkg/evictionplanner
```
