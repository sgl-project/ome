import { useState, useMemo, useCallback, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { exportAsYaml } from '@/lib/utils'
import { Icons } from '@/components/ui/Icons'
import type { BulkAction } from '@/components/ui/BulkActionDropdown'

interface ResourceWithMetadata {
  metadata: { name: string }
}

interface UseBulkSelectionOptions<T extends ResourceWithMetadata> {
  items: T[]
  resourceType: string
  basePath: string
  deleteMutation: { mutateAsync: (name: string) => Promise<unknown> }
}

/**
 * Hook for managing bulk selection and actions in resource list pages.
 */
export function useBulkSelection<T extends ResourceWithMetadata>({
  items,
  resourceType,
  basePath,
  deleteMutation,
}: UseBulkSelectionOptions<T>) {
  const router = useRouter()
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set())
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

  // Prune selection when items change (namespace filter, data refresh)
  useEffect(() => {
    const currentNames = new Set(items.map((item) => item.metadata.name))
    setSelectedItems((prev) => {
      const pruned = new Set([...prev].filter((name) => currentNames.has(name)))
      return pruned.size !== prev.size ? pruned : prev
    })
  }, [items])

  const allSelected = items.length > 0 && selectedItems.size === items.length
  const someSelected = selectedItems.size > 0 && selectedItems.size < items.length

  const handleSelectAll = useCallback(
    (checked: boolean) => {
      setSelectedItems(checked ? new Set(items.map((item) => item.metadata.name)) : new Set())
    },
    [items]
  )

  const handleSelectItem = useCallback((name: string, checked: boolean) => {
    setSelectedItems((prev) => {
      const next = new Set(prev)
      checked ? next.add(name) : next.delete(name)
      return next
    })
  }, [])

  const handleBulkDelete = useCallback(async () => {
    setIsDeleting(true)
    try {
      for (const name of selectedItems) {
        await deleteMutation.mutateAsync(name)
      }
      setSelectedItems(new Set())
      setShowDeleteModal(false)
    } catch (err) {
      console.error(`Failed to delete ${resourceType}s:`, err)
    } finally {
      setIsDeleting(false)
    }
  }, [selectedItems, deleteMutation, resourceType])

  const bulkActions: BulkAction[] = useMemo(() => {
    const selectedData = items.filter((item) => selectedItems.has(item.metadata.name))
    return [
      {
        id: 'export',
        label: 'Export YAML',
        icon: <Icons.downloadFile size="sm" />,
        onClick: () =>
          selectedData.forEach((item) => exportAsYaml(item, `${item.metadata.name}.yaml`)),
        disabled: selectedItems.size === 0,
      },
      {
        id: 'edit',
        label: 'Edit',
        icon: <Icons.pencil size="sm" />,
        onClick: () => {
          if (selectedItems.size === 1) {
            router.push(`${basePath}/${[...selectedItems][0]}/edit`)
          }
        },
        disabled: selectedItems.size !== 1,
      },
      {
        id: 'delete',
        label: 'Delete',
        icon: <Icons.trash size="sm" />,
        onClick: () => setShowDeleteModal(true),
        disabled: selectedItems.size === 0,
        variant: 'destructive',
      },
    ]
  }, [items, selectedItems, basePath, router])

  const deleteModalResourceName =
    selectedItems.size === 1 ? [...selectedItems][0] : `${selectedItems.size} ${resourceType}s`

  return {
    selectedItems,
    showDeleteModal,
    isDeleting,
    allSelected,
    someSelected,
    handleSelectAll,
    handleSelectItem,
    handleBulkDelete,
    closeDeleteModal: () => setShowDeleteModal(false),
    bulkActions,
    deleteModalResourceName,
  }
}
