import DOMPurify from 'dompurify'

const RICH_CONTENT_TAGS = [
  'p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote',
  'ul', 'ol', 'li', 'a', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'span', 'div', 'img', 'hr', 'table', 'thead', 'tbody', 'tr', 'td',
  'th', 'colgroup', 'col',
]

const RICH_CONTENT_ATTRIBUTES = [
  'href', 'target', 'rel', 'src', 'alt', 'title', 'style', 'colspan',
  'rowspan', 'width',
]

/**
 * Converts stored image paths and sanitizes public storefront rich content.
 * Admin editor content must remain unsanitized while editing so sanitization
 * cannot silently alter persisted source HTML.
 */
export function sanitizeRichHtml(raw: unknown): string {
  const apiBaseUrl = import.meta.env?.VITE_API_BASE_URL || ''
  const withDisplayImages = String(raw || '').replace(
    /src=["'](\/uploads\/.*?)["']/g,
    (_, path: string) => `src="${apiBaseUrl}${path}"`,
  )
  return DOMPurify.sanitize(withDisplayImages, {
    ALLOWED_TAGS: RICH_CONTENT_TAGS,
    ALLOWED_ATTR: RICH_CONTENT_ATTRIBUTES,
    ALLOW_DATA_ATTR: false,
    ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|tel:|#|\/(?!\/))/i,
  })
}
