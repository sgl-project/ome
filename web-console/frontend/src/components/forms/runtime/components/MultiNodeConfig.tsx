'use client'

import { UseFormRegister, Control } from 'react-hook-form'
import { ContainerForm } from '../../ContainerForm'
import { inputMonoClassName, labelStyles } from '../../styles'

interface MultiNodeConfigProps {
  basePath: string
  register: UseFormRegister<any>
  control: Control<any>
  enabled: boolean
  onToggle: (enabled: boolean) => void
  leaderDescription?: string
  workerDescription?: string
}

/**
 * Reusable multi-node (leader/worker) configuration component.
 * Used in both engine and decoder configurations.
 */
export function MultiNodeConfig({
  basePath,
  register,
  control,
  enabled,
  onToggle,
  leaderDescription = 'Coordinates distributed inference across worker nodes',
  workerDescription = 'Performs distributed processing tasks',
}: MultiNodeConfigProps) {
  return (
    <>
      {/* Multi-Node Toggle */}
      <div className="rounded-lg bg-purple-50 border border-purple-200 p-4">
        <label className="flex items-center gap-3 cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => onToggle(e.target.checked)}
            className="h-5 w-5 rounded border-slate-300 text-purple-600 focus:ring-purple-500"
          />
          <div>
            <span className="text-sm font-medium text-foreground block">
              Enable Multi-Node Deployment
            </span>
            <span className="text-xs text-muted-foreground">
              Configure leader and worker nodes for distributed inference
            </span>
          </div>
        </label>
      </div>

      {enabled && (
        <>
          {/* Leader Node */}
          <div>
            <h3 className="text-base font-semibold text-foreground mb-4">
              Leader Node Configuration
            </h3>
            <p className="text-xs text-muted-foreground mb-4">{leaderDescription}</p>
            <ContainerForm
              basePath={`${basePath}.leader.runner`}
              register={register}
              control={control}
            />
          </div>

          {/* Worker Node */}
          <div>
            <h3 className="text-base font-semibold text-foreground mb-4">
              Worker Node Configuration
            </h3>
            <p className="text-xs text-muted-foreground mb-4">{workerDescription}</p>
            <div className="mb-4">
              <label className={labelStyles.base}>Worker Size (Number of Pods)</label>
              <input
                type="number"
                {...register(`${basePath}.worker.size`, { valueAsNumber: true })}
                className={`${inputMonoClassName} max-w-xs`}
                placeholder="1"
              />
            </div>
            <ContainerForm
              basePath={`${basePath}.worker.runner`}
              register={register}
              control={control}
            />
          </div>
        </>
      )}
    </>
  )
}
