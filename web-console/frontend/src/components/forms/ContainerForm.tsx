'use client'

import { UseFormRegister, useFieldArray, Control } from 'react-hook-form'

interface ContainerFormProps {
  basePath: string
  register: UseFormRegister<any>
  control: Control<any>
  onRemove?: () => void
  showRemove?: boolean
  title?: string
}

export function ContainerForm({
  basePath,
  register,
  control,
  onRemove,
  showRemove = false,
  title,
}: ContainerFormProps) {
  // Environment variables
  const {
    fields: envFields,
    append: appendEnv,
    remove: removeEnv,
  } = useFieldArray({
    control,
    name: `${basePath}.env`,
  })

  // Arguments
  const {
    fields: argsFields,
    append: appendArg,
    remove: removeArg,
  } = useFieldArray({
    control,
    name: `${basePath}.args`,
  })

  // Command
  const {
    fields: commandFields,
    append: appendCommand,
    remove: removeCommand,
  } = useFieldArray({
    control,
    name: `${basePath}.command`,
  })

  // Volume Mounts
  const {
    fields: volumeMountFields,
    append: appendVolumeMount,
    remove: removeVolumeMount,
  } = useFieldArray({
    control,
    name: `${basePath}.volumeMounts`,
  })

  // Ports
  const {
    fields: portFields,
    append: appendPort,
    remove: removePort,
  } = useFieldArray({
    control,
    name: `${basePath}.ports`,
  })

  return (
    <div className="space-y-6 rounded-xl border border-slate-200 bg-white/50 p-6">
      {(title || showRemove) && (
        <div className="flex items-center justify-between border-b border-slate-200 pb-4">
          {title && (
            <h4 className="text-base font-display font-semibold text-slate-700">{title}</h4>
          )}
          {showRemove && onRemove && (
            <button
              type="button"
              onClick={onRemove}
              className="text-sm font-medium text-red-600 hover:text-red-800 transition-colors"
            >
              Remove Container
            </button>
          )}
        </div>
      )}

      {/* Basic Container Info */}
      <div>
        <h5 className="text-sm font-display font-semibold text-slate-700 mb-3">
          Basic Configuration
        </h5>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label className="field-label block text-sm text-slate-700 mb-2">
              Container Name *
            </label>
            <input
              type="text"
              {...register(`${basePath}.name`)}
              className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
              placeholder="container-name"
            />
          </div>
          <div>
            <label className="field-label block text-sm text-slate-700 mb-2">Image *</label>
            <input
              type="text"
              {...register(`${basePath}.image`)}
              className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
              placeholder="image:tag"
            />
          </div>
        </div>
      </div>

      {/* Command */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h5 className="text-sm font-display font-semibold text-slate-700">Command</h5>
          <button
            type="button"
            onClick={() => appendCommand('')}
            className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
          >
            + Add
          </button>
        </div>
        <div className="space-y-2">
          {commandFields.map((field, index) => (
            <div key={field.id} className="flex items-center gap-2">
              <input
                type="text"
                {...register(`${basePath}.command.${index}`)}
                className="input-focus flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="/bin/sh"
              />
              <button
                type="button"
                onClick={() => removeCommand(index)}
                className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-600 hover:bg-red-100 transition-colors"
              >
                Remove
              </button>
            </div>
          ))}
          {commandFields.length === 0 && (
            <p className="text-xs text-slate-500 italic">
              No command specified (uses image default)
            </p>
          )}
        </div>
      </div>

      {/* Arguments */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h5 className="text-sm font-display font-semibold text-slate-700">Arguments</h5>
          <button
            type="button"
            onClick={() => appendArg('')}
            className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
          >
            + Add
          </button>
        </div>
        <div className="space-y-2">
          {argsFields.map((field, index) => (
            <div key={field.id} className="flex items-center gap-2">
              <input
                type="text"
                {...register(`${basePath}.args.${index}`)}
                className="input-focus flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="--arg=value"
              />
              <button
                type="button"
                onClick={() => removeArg(index)}
                className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-600 hover:bg-red-100 transition-colors"
              >
                Remove
              </button>
            </div>
          ))}
          {argsFields.length === 0 && (
            <p className="text-xs text-slate-500 italic">No arguments specified</p>
          )}
        </div>
      </div>

      {/* Environment Variables */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h5 className="text-sm font-display font-semibold text-slate-700">
            Environment Variables
          </h5>
          <button
            type="button"
            onClick={() => appendEnv({ name: '', value: '' })}
            className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
          >
            + Add
          </button>
        </div>
        <div className="space-y-2">
          {envFields.map((field, index) => (
            <div key={field.id} className="flex items-center gap-2">
              <input
                type="text"
                {...register(`${basePath}.env.${index}.name`)}
                className="input-focus flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="VAR_NAME"
              />
              <input
                type="text"
                {...register(`${basePath}.env.${index}.value`)}
                className="input-focus flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="value"
              />
              <button
                type="button"
                onClick={() => removeEnv(index)}
                className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-600 hover:bg-red-100 transition-colors"
              >
                Remove
              </button>
            </div>
          ))}
          {envFields.length === 0 && (
            <p className="text-xs text-slate-500 italic">No environment variables defined</p>
          )}
        </div>
      </div>

      {/* Resources */}
      <div>
        <h5 className="text-sm font-display font-semibold text-slate-700 mb-3">Resources</h5>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label className="field-label block text-xs text-slate-600 mb-2">CPU Requests</label>
            <input
              type="text"
              {...register(`${basePath}.resources.requests.cpu`)}
              className="input-focus w-full rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
              placeholder="100m"
            />
          </div>
          <div>
            <label className="field-label block text-xs text-slate-600 mb-2">CPU Limits</label>
            <input
              type="text"
              {...register(`${basePath}.resources.limits.cpu`)}
              className="input-focus w-full rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
              placeholder="1000m"
            />
          </div>
          <div>
            <label className="field-label block text-xs text-slate-600 mb-2">Memory Requests</label>
            <input
              type="text"
              {...register(`${basePath}.resources.requests.memory`)}
              className="input-focus w-full rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
              placeholder="128Mi"
            />
          </div>
          <div>
            <label className="field-label block text-xs text-slate-600 mb-2">Memory Limits</label>
            <input
              type="text"
              {...register(`${basePath}.resources.limits.memory`)}
              className="input-focus w-full rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
              placeholder="512Mi"
            />
          </div>
        </div>
      </div>

      {/* Volume Mounts */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h5 className="text-sm font-display font-semibold text-slate-700">Volume Mounts</h5>
          <button
            type="button"
            onClick={() => appendVolumeMount({ name: '', mountPath: '' })}
            className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
          >
            + Add
          </button>
        </div>
        <div className="space-y-2">
          {volumeMountFields.map((field, index) => (
            <div key={field.id} className="flex items-center gap-2">
              <input
                type="text"
                {...register(`${basePath}.volumeMounts.${index}.name`)}
                className="input-focus flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="volume-name"
              />
              <input
                type="text"
                {...register(`${basePath}.volumeMounts.${index}.mountPath`)}
                className="input-focus flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="/mnt/path"
              />
              <label className="flex items-center gap-1.5 text-xs text-slate-600 whitespace-nowrap">
                <input
                  type="checkbox"
                  {...register(`${basePath}.volumeMounts.${index}.readOnly`)}
                  className="h-3 w-3 rounded border-slate-300 text-purple-600 focus:ring-purple-500"
                />
                Read-only
              </label>
              <button
                type="button"
                onClick={() => removeVolumeMount(index)}
                className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-600 hover:bg-red-100 transition-colors"
              >
                Remove
              </button>
            </div>
          ))}
          {volumeMountFields.length === 0 && (
            <p className="text-xs text-slate-500 italic">No volume mounts defined</p>
          )}
        </div>
      </div>

      {/* Ports */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h5 className="text-sm font-display font-semibold text-slate-700">Ports</h5>
          <button
            type="button"
            onClick={() => appendPort({ containerPort: '', protocol: 'TCP' })}
            className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
          >
            + Add
          </button>
        </div>
        <div className="space-y-2">
          {portFields.map((field, index) => (
            <div key={field.id} className="flex items-center gap-2">
              <input
                type="number"
                {...register(`${basePath}.ports.${index}.containerPort`, { valueAsNumber: true })}
                className="input-focus w-32 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="8080"
              />
              <select
                {...register(`${basePath}.ports.${index}.protocol`)}
                className="input-focus rounded-lg border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
              >
                <option value="TCP">TCP</option>
                <option value="UDP">UDP</option>
                <option value="SCTP">SCTP</option>
              </select>
              <input
                type="text"
                {...register(`${basePath}.ports.${index}.name`)}
                className="input-focus flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                placeholder="http (optional)"
              />
              <button
                type="button"
                onClick={() => removePort(index)}
                className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-600 hover:bg-red-100 transition-colors"
              >
                Remove
              </button>
            </div>
          ))}
          {portFields.length === 0 && (
            <p className="text-xs text-slate-500 italic">No ports exposed</p>
          )}
        </div>
      </div>
    </div>
  )
}
