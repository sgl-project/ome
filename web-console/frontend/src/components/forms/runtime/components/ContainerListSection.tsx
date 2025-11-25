'use client'

import { UseFormRegister, Control, useFieldArray } from 'react-hook-form'
import { ContainerForm } from '../../ContainerForm'

interface ContainerListSectionProps {
  title: string
  description: string
  basePath: string
  register: UseFormRegister<any>
  control: Control<any>
  addButtonText: string
  emptyText: string
}

/**
 * Reusable component for managing a list of containers (init containers, sidecars).
 * Includes add button, list rendering, and remove functionality.
 */
export function ContainerListSection({
  title,
  description,
  basePath,
  register,
  control,
  addButtonText,
  emptyText,
}: ContainerListSectionProps) {
  const { fields, append, remove } = useFieldArray({
    control,
    name: basePath,
  })

  return (
    <div>
      <div className="mb-3 flex items-center justify-between">
        <div>
          <h5 className="text-sm font-semibold text-foreground">{title}</h5>
          <p className="text-xs text-muted-foreground mt-1">{description}</p>
        </div>
        <button
          type="button"
          onClick={() => append({ name: '', image: '' })}
          className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
        >
          {addButtonText}
        </button>
      </div>
      <div className="space-y-4">
        {fields.map((field, index) => (
          <ContainerForm
            key={field.id}
            basePath={`${basePath}.${index}`}
            register={register}
            control={control}
            showRemove={true}
            onRemove={() => remove(index)}
            title={`${title.replace('s', '')} ${index + 1}`}
          />
        ))}
        {fields.length === 0 && <p className="text-xs text-muted-foreground italic">{emptyText}</p>}
      </div>
    </div>
  )
}
