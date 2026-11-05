import DOMPurify from 'dompurify'
import { getImageUrl } from './image'

/**
 * 将 HTML 内容中的图片路径转换为显示用的绝对路径，并清除脚本
 * /uploads/xxx -> http://domain/uploads/xxx
 */
export function processHtmlForDisplay(html: string): string {
    if (!html) return ''

    // 匹配 src="/uploads/..."，支持单引号和双引号
    // 使用非贪婪匹配 .*?
    const withImages = html.replace(/src=["'](\/uploads\/.*?)["']/g, (_, path) => {
        return `src="${getImageUrl(path)}"`
    })
    // 结果直接进 v-html，且与后台同源（localStorage 里有 admin_token）；上游导入的商品详情也走这里
    return DOMPurify.sanitize(withImages, { ADD_ATTR: ['target'] })
}

/**
 * 将 HTML 内容中的图片路径转换为存储用的相对路径
 * http://domain/uploads/xxx -> /uploads/xxx
 */
export function processHtmlForStorage(html: string): string {
    if (!html) return ''

    const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || ''
    let apiHost = ''
    try {
        if (apiBaseUrl) {
            // 提取域名 (例如 localhost:8080 或 domain.com)
            apiHost = new URL(apiBaseUrl).host
        } else {
            // 如果未配置 API_BASE_URL，则认为是同源，使用当前域名
            apiHost = window.location.host
        }
    } catch (e) {
        // Fallback
        apiHost = window.location.host
    }

    // 匹配 src="value" 或 src='value'
    return html.replace(/src=["'](.*?)["']/g, (match, src) => {
        try {
            // 如果是完整 URL (http:// 或 https://)
            if (src.startsWith('http://') || src.startsWith('https://')) {
                const url = new URL(src)

                // 检查域名是否匹配 API 域名，且路径以 /uploads/ 开头
                // url.host 会自动处理端口号 (例如 localhost:8080)
                if (url.host === apiHost && url.pathname.startsWith('/uploads/')) {
                    // 返回相对路径，例如 src="/uploads/xxx.png"
                    // 始终标准化为双引号
                    return `src="${url.pathname}"`
                }
            }
        } catch (e) {
            // URL 解析失败，忽略，返回原样
        }
        return match
    })
}
