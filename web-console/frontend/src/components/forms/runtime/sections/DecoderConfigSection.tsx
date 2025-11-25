'use client'

import { useRuntimeFormContext } from '../RuntimeFormContext'
import { CollapsibleSection } from '../../CollapsibleSection'
import { ContainerForm } from '../../ContainerForm'
import { VolumeForm } from '../../VolumeForm'
import { ScalingConfig, MultiNodeConfig, ContainerListSection } from '../components'

/**
 * Decoder configuration section for RuntimeForm.
 * Handles prefill-decode disaggregated deployments with similar structure to engine config.
 */
export function DecoderConfigSection() {
  const { form, decoderMultiNode, setDecoderMultiNode } = useRuntimeFormContext()
  const { register, control } = form

  return (
    <CollapsibleSection
      title="Decoder Configuration"
      description="Configure the decoder component for prefill-decode disaggregated deployments"
    >
      <div className="space-y-8">
        {/* Scaling Configuration */}
        <ScalingConfig basePath="spec.decoderConfig" register={register} />

        {/* Runner (Main Container) */}
        <div>
          <h3 className="text-base font-semibold text-foreground mb-4">Runner (Main Container)</h3>
          <ContainerForm
            basePath="spec.decoderConfig.runner"
            register={register}
            control={control}
          />
        </div>

        {/* Multi-Node Configuration */}
        <MultiNodeConfig
          basePath="spec.decoderConfig"
          register={register}
          control={control}
          enabled={decoderMultiNode}
          onToggle={setDecoderMultiNode}
          leaderDescription="Coordinates distributed token generation across worker nodes"
          workerDescription="Performs distributed token generation tasks"
        />

        {/* Volumes */}
        <VolumeForm basePath="spec.decoderConfig.volumes" register={register} control={control} />

        {/* Init Containers */}
        <ContainerListSection
          title="Init Containers"
          description="Containers that run before the main container starts"
          basePath="spec.decoderConfig.initContainers"
          register={register}
          control={control}
          addButtonText="+ Add Init Container"
          emptyText="No init containers defined"
        />

        {/* Sidecar Containers */}
        <ContainerListSection
          title="Sidecar Containers"
          description="Containers that run alongside the main container"
          basePath="spec.decoderConfig.sidecars"
          register={register}
          control={control}
          addButtonText="+ Add Sidecar"
          emptyText="No sidecar containers defined"
        />
      </div>
    </CollapsibleSection>
  )
}
