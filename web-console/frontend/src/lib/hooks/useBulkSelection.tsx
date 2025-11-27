import { useState, useMemo, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { exportAsYaml } from '@/lib/utils'
import { Icons } from '@/components/ui/Icons'
import type { BulkAction } from '@/components/ui/BulkActionDropdown'

interface ResourceWithMetadata {
  metadata: {
    name: string
  }
}

interface UseBulkSelectionOptions<T extends ResourceWithMetadata> {
  /** The list of items to select from */
  items: T[]
  /** The resource type for display (e.g., 'model', 'runtime', 'service') */
  resourceType: string
  /** The base path for editing (e.g., '/models', '/runtimes', '/services') */
  basePath: string
  /** Delete mutation function */
  deleteMutation: {
    mutateAsync: (name: string) => Promise<unknown>
  }
}

interface UseBulkSelectionReturn<T extends ResourceWithMetadata> {
  /** Set of selected item names */
  selectedItems: Set<string>
  /** Whether the delete modal is shown */
  showDeleteModal: boolean
  /** Whether deletion is in progress */
  isDeleting: boolean
  /** Whether all items are selected */
  allSelected: boolean
  /** Whether some (but not all) items are selected */
  someSelected: boolean
  /** The selected items as full objects */
  selectedData: T[]
  /** Handler for select all checkbox */
  handleSelectAll: (checked: boolean) => void
  /** Handler for individual item checkbox */
  handleSelectItem: (name: string, checked: boolean) => void
  /** Handler for bulk delete confirmation */
  handleBulkDelete: () => Promise<void>
  /** Open the delete modal */
  openDeleteModal: () => void
  /** Close the delete modal */
  closeDeleteModal: () => void
  /** Get the bulk actions array */
  bulkActions: BulkAction[]
  /** Get the resource name for the delete modal */
  deleteModalResourceName: string
}

/**
 * Hook for managing bulk selection, actions, and deletion in resource list pages.
 * Provides consistent selection behavior across models, runtimes, and services pages.
 */
export function useBulkSelection<T extends ResourceWithMetadata>({
  items,
  resourceType,
  basePath,
  deleteMutation,
}: UseBulkSelectionOptions<T>): UseBulkSelectionReturn<T> {
  const router = useRouter()
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set())
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

  // Computed selection state
  const allSelected = items.length > 0 && selectedItems.size === items.length
  const someSelected = selectedItems.size > 0 && selectedItems.size < items.length

  // Get selected items as full objects
  const selectedData = useMemo(() => {
    return items.filter((item) => selectedItems.has(item.metadata.name))
  }, [items, selectedItems])

  // Selection handlers
  const handleSelectAll = useCallback(
    (checked: boolean) => {
      if (checked) {
        setSelectedItems(new Set(items.map((item) => item.metadata.name)))
      } else {
        setSelectedItems(new Set())
      }
    },
    [items]
  )

  const handleSelectItem = useCallback((name: string, checked: boolean) => {
    setSelectedItems((prev) => {
      const newSelected = new Set(prev)
      if (checked) {
        newSelected.add(name)
      } else {
        newSelected.delete(name)
      }
      return newSelected
    })
  }, [])

  // Bulk action handlers
  const handleBulkExport = useCallback(() => {
    selectedData.forEach((item) => {
      exportAsYaml(item, `${item.metadata.name}.yaml`)
    })
  }, [selectedData])

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

  const handleEdit = useCallback(() => {
    if (selectedItems.size === 1) {
      const name = Array.from(selectedItems)[0]
      router.push(`${basePath}/${name}/edit`)
    }
  }, [selectedItems, basePath, router])

  const openDeleteModal = useCallback(() => setShowDeleteModal(true), [])
  const closeDeleteModal = useCallback(() => setShowDeleteModal(false), [])

  // Build bulk actions array
  const bulkActions: BulkAction[] = useMemo(
    () => [
      {
        id: 'export',
        label: 'Export YAML',
        icon: <Icons.downloadFile size="sm" />,
        onClick: handleBulkExport,
        disabled: selectedItems.size === 0,
      },
      {
        id: 'edit',
        label: 'Edit',
        icon: <Icons.pencil size="sm" />,
        onClick: handleEdit,
        disabled: selectedItems.size !== 1,
      },
      {
        id: 'delete',
        label: 'Delete',
        icon: <Icons.trash size="sm" />,
        onClick: openDeleteModal,
        disabled: selectedItems.size === 0,
        variant: 'destructive',
      },
    ],
    [selectedItems.size, handleBulkExport, handleEdit, openDeleteModal]
  )

  // Resource name for delete modal
  const deleteModalResourceName = useMemo(() => {
    if (selectedItems.size === 1) {
      return Array.from(selectedItems)[0]
    }
    return `${selectedItems.size} ${resourceType}s`
  }, [selectedItems, resourceType])

  return {
    selectedItems,
    showDeleteModal,
    isDeleting,
    allSelected,
    someSelected,
    selectedData,
    handleSelectAll,
    handleSelectItem,
    handleBulkDelete,
    openDeleteModal,
    closeDeleteModal,
    bulkActions,
    deleteModalResourceName,
  }
}
