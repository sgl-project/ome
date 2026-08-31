package audit

// OutcomeOwnerTornDown is the Outcome stamped when teardown closes a
// dangling in-flight (Phase=Started) migration entry: the workload
// owning the migration's instances was torn down mid-flight, so the
// migration can never complete. Closed terminal (Phase=Failed) because
// ledger rows are RECORDS, not work orders — leaving the row Started
// would let the in-flight readers (InFlightEntry,
// HasInFlightMigrationForInstance) and the adapter's upgrade import
// (which synthesizes status.migrations entries from Started rows
// lacking a terminal counterpart) resume it as a phantom migration on
// a freshly recreated workload.
const OutcomeOwnerTornDown = "owner-torn-down"
