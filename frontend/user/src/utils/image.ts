/**
 * 获取完整的图片 URL
 * 如果传入的是相对路径（如 /uploads/xxx.png），则拼接 API 基础 URL
 * 如果传入的是完整 URL（如 http://...），则直接返回
 */
export function getImageUrl(path: string | undefined | null): string {
    if (!path) return ''

    // 如果已经是完整 URL，直接返回
    if (path.startsWith('http://') || path.startsWith('https://')) {
        return path
    }

    // 获取 API 基础 URL
    const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || ''

    // 确保路径以 / 开头
    const normalizedPath = path.startsWith('/') ? path : `/${path}`

    return `${apiBaseUrl}${normalizedPath}`
}

/**
 * 从 images 数组中获取第一张图片的完整 URL
 */
export function getFirstImageUrl(images: any): string {
    if (!images) return ''

    let imageUrl = ''

    if (Array.isArray(images)) {
        imageUrl = images[0] || ''
    } else if (images.images && Array.isArray(images.images)) {
        imageUrl = images.images[0] || ''
    }

    return getImageUrl(imageUrl)
}

/** 无图占位的色相：由 id 派生，同一条内容在列表和详情里颜色一致 */
export const placeholderHue = (id: unknown): number => ((Number(id) || 0) * 47) % 360
