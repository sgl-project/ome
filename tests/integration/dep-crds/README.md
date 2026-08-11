# dep-crds — CRD YAMLs vendored on demand from go.mod modules

This directory holds optional CRDs that integration suites install
when they exercise features backed by an external operator (KEDA
autoscaling, Gateway API routing, gang scheduling, Kueue admission).
The YAMLs themselves are **not committed** — they're regenerated from
go.mod by `make dep-crds`.

The single source of truth for CRD versions is `go.mod`. Checking the
YAMLs in would create a drift surface and bloat the repo without
buying anything that the go.mod pin doesn't already give us.

## Layout (after `make dep-crds`)

```
dep-crds/
├── keda/                 # keda.sh CRDs
├── gateway-api/          # gateway.networking.k8s.io HTTPRoute
├── scheduler-plugins/    # scheduling.x-k8s.io PodGroup
├── kueue/                # kueue.x-k8s.io CRDs
└── README.md
```

All bundles are sourced from Go modules and gitignored under
`tests/integration/dep-crds/*/`.

## Bumping a vendored dep

```sh
go get <module>@<version> && go mod tidy
make dep-crds
```

CI runs `make dep-crds` so a module restructuring its CRD paths after
a bump fails fast instead of surfacing later in a test run.
