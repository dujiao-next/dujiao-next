const SUPPORTED_LOCALES = ['zh-CN', 'zh-TW', 'en-US'] as const

const normalizeText = (value: unknown) => String(value ?? '').trim()

const normalizeLocaleCode = (locale?: unknown) => normalizeText(locale).toLowerCase()

const localeFallbacks = (locale?: string) => {
  const normalized = normalizeLocaleCode(locale)
  switch (normalized) {
    case 'zh-tw':
    case 'zh-hk':
    case 'zh-mo':
      return ['zh-TW', 'zh-CN', 'en-US']
    case 'en':
    case 'en-us':
      return ['en-US', 'zh-CN', 'zh-TW']
    case 'zh':
    case 'zh-cn':
    default:
      return ['zh-CN', 'zh-TW', 'en-US']
  }
}

const isLocalizedObject = (value: unknown): value is Record<string, unknown> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  const keys = Object.keys(value)
  if (keys.length === 0) return false
  return keys.every((key) => SUPPORTED_LOCALES.includes(key as typeof SUPPORTED_LOCALES[number]))
}

const resolveLocalizedText = (value: unknown, locale?: string) => {
  if (!isLocalizedObject(value)) return ''
  const row = value as Record<string, unknown>
  const chain = localeFallbacks(locale)
  for (const code of chain) {
    const text = normalizeText(row[code])
    if (text) return text
  }
  for (const code of SUPPORTED_LOCALES) {
    const text = normalizeText(row[code])
    if (text) return text
  }
  return ''
}

const formatSpecValue = (value: unknown, locale?: string): string => {
  if (Array.isArray(value)) {
    return value.map((entry) => formatSpecValue(entry, locale)).filter(Boolean).join(', ')
  }
  if (value === null || value === undefined) return ''
  if (isLocalizedObject(value)) {
    return resolveLocalizedText(value, locale)
  }
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch {
      return ''
    }
  }
  return normalizeText(value)
}

export const formatSkuSpecValues = (specValues: unknown, locale?: string) => {
  if (!specValues || typeof specValues !== 'object' || Array.isArray(specValues)) return ''
  if (isLocalizedObject(specValues)) {
    return resolveLocalizedText(specValues, locale)
  }
  return Object.entries(specValues as Record<string, unknown>)
    .map(([key, value]) => {
      const keyText = normalizeText(key)
      const valueText = formatSpecValue(value, locale)
      if (!valueText) return ''
      if (!keyText) return valueText
      return `${keyText}: ${valueText}`
    })
    .filter(Boolean)
    .join(' / ')
}

export const formatCurrentLocaleSkuSpec = (specValues: Record<string, string> | undefined, locale: string) => {
  if (specValues && SUPPORTED_LOCALES.some((code) => code in specValues)) {
    return normalizeText(specValues[locale])
  }
  return formatSkuSpecValues(specValues, locale)
}

export const resolveSkuCodeFromSnapshot = (snapshot: unknown, options?: { defaultLabel?: string }) => {
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) return ''
  const row = snapshot as Record<string, unknown>
  const rawCode = normalizeText(row.sku_code)
  if (!rawCode) return ''
  if (rawCode.toUpperCase() === 'DEFAULT') {
    return normalizeText(options?.defaultLabel)
  }
  return rawCode
}

export const resolveSkuSpecFromSnapshot = (snapshot: unknown, locale?: string) => {
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) return ''
  const row = snapshot as Record<string, unknown>
  return formatSkuSpecValues(row.spec_values, locale)
}

// formatSkuDisplayLabel 构造"规格 · 编码"展示文本，spec 和 code 有任一显示，都有则拼接。
// 用于订单列表、退款列表等后台管理界面统一 SKU 展示。
export const formatSkuDisplayLabel = (snapshot: unknown, locale?: string) => {
  const spec = resolveSkuSpecFromSnapshot(snapshot, locale)
  const code = resolveSkuCodeFromSnapshot(snapshot)
  if (spec && code) return `${spec} · ${code}`
  return spec || code
}
