'use client'

import { useRuntimeFormContext } from '../RuntimeFormContext'
import { RuntimeComponentSection } from '../components'

/**
 * Decoder configuration section for RuntimeForm.
 * Uses the shared RuntimeComponentSection component.
 * Handles prefill-decode disaggregated deployments.
 */
export function DecoderConfigSection() {
  const { form, decoderMultiNode, setDecoderMultiNode } = useRuntimeFormContext()
  const { register, control } = form

  return (
    <RuntimeComponentSection
      title="Decoder Configuration"
      description="Configure the decoder component for prefill-decode disaggregated deployments"
      basePath="spec.decoderConfig"
      register={register}
      control={control}
      multiNodeEnabled={decoderMultiNode}
      onMultiNodeToggle={setDecoderMultiNode}
      leaderDescription="Coordinates distributed token generation across worker nodes"
      workerDescription="Performs distributed token generation tasks"
    />
  )
}
