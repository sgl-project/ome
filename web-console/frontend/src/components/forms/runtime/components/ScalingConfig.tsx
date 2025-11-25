'use client'

import { UseFormRegister } from 'react-hook-form'
import { inputMonoClassName, selectClassName, labelStyles, gridStyles } from '../../styles'

interface ScalingConfigProps {
  basePath: string
  register: UseFormRegister<any>
}

const SCALE_METRICS = [
  { value: '', label: 'Select metric...' },
  { value: 'cpu', label: 'CPU' },
  { value: 'memory', label: 'Memory' },
  { value: 'concurrency', label: 'Concurrency' },
  { value: 'rps', label: 'RPS' },
]

/**
 * Reusable scaling configuration component for engine/decoder/router configs.
 * Handles minReplicas, maxReplicas, scaleTarget, and scaleMetric fields.
 */
export function ScalingConfig({ basePath, register }: ScalingConfigProps) {
  return (
    <div>
      <h3 className="text-base font-semibold text-foreground mb-4">Scaling Configuration</h3>
      <div className={gridStyles.cols4}>
        <div>
          <label className={labelStyles.base}>Min Replicas</label>
          <input
            type="number"
            {...register(`${basePath}.minReplicas`, { valueAsNumber: true })}
            className={inputMonoClassName}
            placeholder="0"
          />
        </div>
        <div>
          <label className={labelStyles.base}>Max Replicas</label>
          <input
            type="number"
            {...register(`${basePath}.maxReplicas`, { valueAsNumber: true })}
            className={inputMonoClassName}
            placeholder="5"
          />
        </div>
        <div>
          <label className={labelStyles.base}>Scale Target</label>
          <input
            type="number"
            {...register(`${basePath}.scaleTarget`, { valueAsNumber: true })}
            className={inputMonoClassName}
            placeholder="80"
          />
        </div>
        <div>
          <label className={labelStyles.base}>Scale Metric</label>
          <select {...register(`${basePath}.scaleMetric`)} className={selectClassName}>
            {SCALE_METRICS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      </div>
    </div>
  )
}
