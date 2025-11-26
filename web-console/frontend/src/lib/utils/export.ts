import yaml from 'js-yaml'

type CleanableValue =
  | Record<string, unknown>
  | unknown[]
  | string
  | number
  | boolean
  | null
  | undefined

/**
 * Recursively cleans an object by removing undefined, null, and empty values.
 * This prepares data for YAML export by removing unnecessary fields.
 *
 * @param obj - The value to clean
 * @returns The cleaned value, or undefined if the value should be removed
 */
function cleanObject(obj: CleanableValue): CleanableValue {
  if (obj === null || obj === undefined) {
    return undefined
  }

  if (Array.isArray(obj)) {
    const cleaned = obj
      .map((item) => cleanObject(item as CleanableValue))
      .filter((item): item is NonNullable<CleanableValue> => item !== undefined)
    return cleaned.length > 0 ? cleaned : undefined
  }

  if (typeof obj === 'object') {
    const cleaned: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(obj)) {
      const cleanedValue = cleanObject(value as CleanableValue)
      if (cleanedValue !== undefined && cleanedValue !== '') {
        cleaned[key] = cleanedValue
      }
    }
    return Object.keys(cleaned).length > 0 ? cleaned : undefined
  }

  return obj
}

/**
 * Converts a JavaScript object to a YAML string.
 *
 * @param data - The data to convert to YAML
 * @returns The YAML string representation
 *
 * @example
 * ```ts
 * const yamlString = toYaml({ name: 'example', value: 42 })
 * // Returns: "name: example\nvalue: 42\n"
 * ```
 */
export function toYaml(data: unknown): string {
  const cleaned = cleanObject(data as CleanableValue)
  return yaml.dump(cleaned, {
    indent: 2,
    lineWidth: -1, // Don't wrap long lines
    noRefs: true, // Don't use YAML references
    sortKeys: false, // Preserve key order
  })
}

/**
 * Triggers a file download in the browser.
 *
 * @param content - The content to download
 * @param filename - The name of the file to download
 * @param mimeType - The MIME type of the file (defaults to 'text/yaml')
 */
export function downloadFile(
  content: string,
  filename: string,
  mimeType: string = 'text/yaml'
): void {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)

  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.style.display = 'none'

  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)

  URL.revokeObjectURL(url)
}

/**
 * Exports data as a YAML file download.
 *
 * @param data - The data to export
 * @param filename - The name of the file (will add .yaml extension if missing)
 *
 * @example
 * ```ts
 * exportAsYaml(runtimeConfig, 'my-runtime')
 * // Downloads: my-runtime.yaml
 * ```
 */
export function exportAsYaml(data: unknown, filename: string): void {
  const yamlContent = toYaml(data)
  const sanitizedFilename = filename.endsWith('.yaml') ? filename : `${filename}.yaml`
  downloadFile(yamlContent, sanitizedFilename)
}
