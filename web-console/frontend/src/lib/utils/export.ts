import yaml from 'js-yaml'

/**
 * Clean an object by removing undefined, null, and empty values recursively.
 * This prepares data for YAML export by removing unnecessary fields.
 */
function cleanObject(obj: unknown): unknown {
  if (obj === null || obj === undefined) {
    return undefined
  }

  if (Array.isArray(obj)) {
    const cleaned = obj.map(cleanObject).filter((item) => item !== undefined)
    return cleaned.length > 0 ? cleaned : undefined
  }

  if (typeof obj === 'object') {
    const cleaned: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(obj as Record<string, unknown>)) {
      const cleanedValue = cleanObject(value)
      if (cleanedValue !== undefined && cleanedValue !== '') {
        cleaned[key] = cleanedValue
      }
    }
    return Object.keys(cleaned).length > 0 ? cleaned : undefined
  }

  return obj
}

/**
 * Convert a JavaScript object to YAML string.
 */
export function toYaml(data: unknown): string {
  const cleaned = cleanObject(data)
  return yaml.dump(cleaned, {
    indent: 2,
    lineWidth: -1, // Don't wrap long lines
    noRefs: true, // Don't use YAML references
    sortKeys: false, // Preserve key order
  })
}

/**
 * Download content as a file.
 */
export function downloadFile(content: string, filename: string, mimeType: string = 'text/yaml') {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Export data as a YAML file download.
 */
export function exportAsYaml(data: unknown, filename: string) {
  const yamlContent = toYaml(data)
  const sanitizedFilename = filename.endsWith('.yaml') ? filename : `${filename}.yaml`
  downloadFile(yamlContent, sanitizedFilename)
}
