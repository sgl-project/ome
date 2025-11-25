'use client'

import { UseFormRegister, useFieldArray, Control } from 'react-hook-form'

interface VolumeFormProps {
  basePath: string
  register: UseFormRegister<any>
  control: Control<any>
}

export function VolumeForm({ basePath, register, control }: VolumeFormProps) {
  const {
    fields: volumeFields,
    append: appendVolume,
    remove: removeVolume,
  } = useFieldArray({
    control,
    name: basePath,
  })

  return (
    <div>
      <div className="mb-3 flex items-center justify-between">
        <h5 className="text-sm font-display font-semibold text-slate-700">Volumes</h5>
        <button
          type="button"
          onClick={() => appendVolume({ name: '', volumeType: 'emptyDir' })}
          className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
        >
          + Add Volume
        </button>
      </div>
      <div className="space-y-3">
        {volumeFields.map((field, index) => (
          <div key={field.id} className="rounded-lg border border-slate-200 bg-white/30 p-4">
            <div className="mb-3 flex items-center justify-between">
              <span className="text-sm font-medium text-slate-700">Volume {index + 1}</span>
              <button
                type="button"
                onClick={() => removeVolume(index)}
                className="text-xs font-medium text-red-600 hover:text-red-800 transition-colors"
              >
                Remove
              </button>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <label className="field-label block text-xs text-slate-600 mb-1.5">
                  Volume Name *
                </label>
                <input
                  type="text"
                  {...register(`${basePath}.${index}.name`)}
                  className="input-focus w-full rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                  placeholder="volume-name"
                />
              </div>
              <div>
                <label className="field-label block text-xs text-slate-600 mb-1.5">
                  Volume Type
                </label>
                <select
                  {...register(`${basePath}.${index}.volumeType`)}
                  className="input-focus w-full rounded-lg border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                >
                  <option value="emptyDir">Empty Dir</option>
                  <option value="configMap">ConfigMap</option>
                  <option value="secret">Secret</option>
                  <option value="persistentVolumeClaim">Persistent Volume Claim</option>
                  <option value="hostPath">Host Path</option>
                </select>
              </div>
            </div>
            <div className="mt-3 text-xs text-slate-500">
              Additional volume configuration can be added via YAML/JSON after creation
            </div>
          </div>
        ))}
        {volumeFields.length === 0 && (
          <p className="text-xs text-slate-500 italic">No volumes defined</p>
        )}
      </div>
    </div>
  )
}
