import type { AdminPost } from '@/api/types'

/**
 * 列表 → 编辑器的数据交接。
 *
 * 从列表点进来时列表接口已经拿到了整篇文章（含 content_blocks），
 * 直接放内存里给编辑器用：秒开、不发请求、也不依赖 history.state
 * （history.state 在部分浏览器/history 模式下不可靠）。
 */
const cache = new Map<number, AdminPost>()

export const cachePost = (post: AdminPost) => {
  if (post?.id) cache.set(post.id, post)
}

export const readCachedPost = (id: number): AdminPost | undefined => cache.get(id)

export const dropCachedPost = (id: number) => cache.delete(id)
