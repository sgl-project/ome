# InferenceReplica status-size baseline

`status_size_v1.csv` is a deterministic regression baseline generated from the
real API types and Kubernetes JSON encoder. The normalized request columns
include metadata, spec, and status, but omit API-server-generated
`managedFields`; those fields depend on the served schema, API-server version,
and whether the API server omitted them through its size fallback.

The marginal columns overlap with the total Instance-row column. The three live
observation fields are cleared in their CSV order; the other named families are
measured independently. These values are not separate pieces to sum with the
row total.

The envtest suite in
`tests/integration/singlecluster/controller/inferencereplica_status_size`
captures the authoritative full `/status` PUTs, including real
`managedFields`, for steady, conversion, and rehydration paths.
