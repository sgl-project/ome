'use client'

import { useRuntimeFormContext } from '../RuntimeFormContext'
import { CollapsibleSection } from '../../CollapsibleSection'
import { ContainerForm } from '../../ContainerForm'
import { VolumeForm } from '../../VolumeForm'
import { ScalingConfig, MultiNodeConfig, ContainerListSection } from '../components'

/**
 * Engine configuration section for RuntimeForm.
 * Handles scaling, runner container, multi-node setup, volumes, and additional containers.
 */
export function EngineConfigSection() {
  const { form, engineMultiNode, setEngineMultiNode } = useRuntimeFormContext()
  const { register, control } = form

  return (
    <CollapsibleSection
      title="Engine Configuration"
      description="Configure the inference engine component"
    >
      <div className="space-y-8">
        {/* Scaling Configuration */}
        <ScalingConfig basePath="spec.engineConfig" register={register} />

        {/* Runner (Main Container) */}
        <div>
          <h3 className="text-base font-semibold text-foreground mb-4">Runner (Main Container)</h3>
          <ContainerForm
            basePath="spec.engineConfig.runner"
            register={register}
            control={control}
          />
        </div>

        {/* Multi-Node Configuration */}
        <MultiNodeConfig
          basePath="spec.engineConfig"
          register={register}
          control={control}
          enabled={engineMultiNode}
          onToggle={setEngineMultiNode}
          leaderDescription="Coordinates distributed inference across worker nodes"
          workerDescription="Performs distributed processing tasks"
        />

        {/* Volumes */}
        <VolumeForm basePath="spec.engineConfig.volumes" register={register} control={control} />

        {/* Init Containers */}
        <ContainerListSection
          title="Init Containers"
          description="Containers that run before the main container starts"
          basePath="spec.engineConfig.initContainers"
          register={register}
          control={control}
          addButtonText="+ Add Init Container"
          emptyText="No init containers defined"
        />

        {/* Sidecar Containers */}
        <ContainerListSection
          title="Sidecar Containers"
          description="Containers that run alongside the main container"
          basePath="spec.engineConfig.sidecars"
          register={register}
          control={control}
          addButtonText="+ Add Sidecar"
          emptyText="No sidecar containers defined"
        />
      </div>
    </CollapsibleSection>
  )
}
