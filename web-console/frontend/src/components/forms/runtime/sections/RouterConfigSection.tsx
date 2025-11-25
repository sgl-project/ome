'use client'

import { useRuntimeFormContext } from '../RuntimeFormContext'
import { CollapsibleSection } from '../../CollapsibleSection'
import { ContainerForm } from '../../ContainerForm'
import { VolumeForm } from '../../VolumeForm'
import { ScalingConfig, ContainerListSection } from '../components'
import { textareaClassName, labelStyles, helpTextClassName } from '../../styles'

/**
 * Router configuration section for RuntimeForm.
 * Handles request routing and load balancing configuration.
 */
export function RouterConfigSection() {
  const { form } = useRuntimeFormContext()
  const { register, control } = form

  return (
    <CollapsibleSection
      title="Router Configuration"
      description="Configure the router component for request routing and load balancing"
    >
      <div className="space-y-8">
        {/* Scaling Configuration */}
        <ScalingConfig basePath="spec.routerConfig" register={register} />

        {/* Runner (Main Container) */}
        <div>
          <h3 className="text-base font-semibold text-foreground mb-4">Runner (Main Container)</h3>
          <ContainerForm
            basePath="spec.routerConfig.runner"
            register={register}
            control={control}
          />
        </div>

        {/* Router Configuration Parameters */}
        <div>
          <h3 className="text-base font-semibold text-foreground mb-4">
            Router Configuration Parameters
          </h3>
          <p className={helpTextClassName}>
            Additional configuration parameters as key-value pairs (JSON format)
          </p>
          <textarea
            {...register('spec.routerConfig.config')}
            className={`${textareaClassName} font-mono`}
            placeholder={`{\n  "timeout": "30s",\n  "maxConnections": "1000"\n}`}
          />
        </div>

        {/* Volumes */}
        <VolumeForm basePath="spec.routerConfig.volumes" register={register} control={control} />

        {/* Init Containers */}
        <ContainerListSection
          title="Init Containers"
          description="Containers that run before the main container starts"
          basePath="spec.routerConfig.initContainers"
          register={register}
          control={control}
          addButtonText="+ Add Init Container"
          emptyText="No init containers defined"
        />

        {/* Sidecar Containers */}
        <ContainerListSection
          title="Sidecar Containers"
          description="Containers that run alongside the main container"
          basePath="spec.routerConfig.sidecars"
          register={register}
          control={control}
          addButtonText="+ Add Sidecar"
          emptyText="No sidecar containers defined"
        />
      </div>
    </CollapsibleSection>
  )
}
