package canary

// Event reasons recorded on the InferenceService during a canary rollout, so the
// step-by-step progression is visible via `kubectl describe isvc` / events.
const (
	// EventReasonCanaryStepAdvanced is recorded when the canary advances to the
	// next step.
	EventReasonCanaryStepAdvanced = "CanaryStepAdvanced"
	// EventReasonCanaryCompleted is recorded when the canary reaches Stable.
	EventReasonCanaryCompleted = "CanaryCompleted"
)
