'use client'

import { useRuntimeFormContext } from '../RuntimeFormContext'
import { RuntimeComponentSection } from '../components'

/**
 * Engine configuration section for RuntimeForm.
 * Uses the shared RuntimeComponentSection component.
 */
export function EngineConfigSection() {
  const { form, engineMultiNode, setEngineMultiNode } = useRuntimeFormContext()
  const { register, control } = form

  return (
    <RuntimeComponentSection
      title="Engine Configuration"
      description="Configure the inference engine component"
      basePath="spec.engineConfig"
      register={register}
      control={control}
      multiNodeEnabled={engineMultiNode}
      onMultiNodeToggle={setEngineMultiNode}
      leaderDescription="Coordinates distributed inference across worker nodes"
      workerDescription="Performs distributed processing tasks"
    />
  )
}
