import { getImageUrl } from './image'

const SITE_ICON_LINK_ID = 'site-favicon'
const DEFAULT_SITE_ICON = '/favicon-v5.svg'

export function resolveSiteIconHref(value: unknown): string {
  const icon = String(value || '').trim()
  // Legacy site configuration points at the old domain-level favicon URL.
  // Use the versioned icon so Safari does not reuse its cached avatar.
  return !icon || icon === '/favicon.svg' ? DEFAULT_SITE_ICON : getImageUrl(icon)
}

export function applySiteIcon(value: unknown) {
  const link = document.getElementById(SITE_ICON_LINK_ID) as HTMLLinkElement | null
  if (link) {
    link.href = resolveSiteIconHref(value)
  }
}
