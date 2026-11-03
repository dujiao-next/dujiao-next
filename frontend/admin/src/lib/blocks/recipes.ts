/**
 * 版式模板：一键铺好整页结构，站长只填文字和图片。
 * 每个模板返回一组区块（文本为空、图片为空），字段结构与 schema 一致。
 */

import { createBlock, findBlockDef } from './schema'

export interface BlockRecipe {
  key: string
  labelKey: string
  descKey: string
  build: () => any[]
}

const block = (type: string, props: Record<string, any> = {}) => {
  const def = findBlockDef(type)
  if (!def) return null
  const created = createBlock(def)
  created.props = { ...created.props, ...props }
  return created
}

/** 造一个「列表字段」的占位行 */
const rows = (n: number, factory: (i: number) => Record<string, any>) =>
  Array.from({ length: n }, (_, i) => factory(i))

export const blockRecipes: BlockRecipe[] = [
  {
    key: 'productLaunch',
    labelKey: 'recipeLaunch',
    descKey: 'recipeLaunchDesc',
    build: () => [
      block('hero', { theme: 'dark', variant: 'stacked' }),
      block('sectionHeader', { eyebrow: { 'zh-CN': '设计' } }),
      block('featureGrid', {
        columns: '3',
        items: rows(3, () => ({ title: { 'zh-CN': '' }, body: { 'zh-CN': '' } })),
      }),
      block('gallery', { items: rows(3, () => ({ caption: { 'zh-CN': '' } })) }),
      block('cta', {
        ctas: [{ label: { 'zh-CN': '购买' }, url: '', style: 'primary' }],
      }),
    ].filter(Boolean),
  },
  {
    key: 'featureUpdate',
    labelKey: 'recipeUpdate',
    descKey: 'recipeUpdateDesc',
    build: () => [
      block('hero', { theme: 'dark', variant: 'overlay' }),
      block('sectionHeader', { eyebrow: { 'zh-CN': '更新' } }),
      block('cardScroller', {
        title: { 'zh-CN': '这次的改动，' },
        secondary: { 'zh-CN': '都在这里。' },
        cards: rows(3, () => ({
          label: { 'zh-CN': '' },
          title: { 'zh-CN': '' },
          body: { 'zh-CN': '' },
        })),
      }),
      block('specs', {
        columns: '2',
        items: rows(4, () => ({ label: { 'zh-CN': '' }, value: { 'zh-CN': '' } })),
      }),
    ].filter(Boolean),
  },
  {
    key: 'galleryStory',
    labelKey: 'recipeGallery',
    descKey: 'recipeGalleryDesc',
    build: () => [
      block('hero', { theme: 'dark', variant: 'stacked' }),
      block('gallery', { items: rows(4, () => ({ caption: { 'zh-CN': '' } })) }),
      block('richText', { html: { 'zh-CN': '' } }),
    ].filter(Boolean),
  },
  {
    key: 'storyPage',
    labelKey: 'recipeStory',
    descKey: 'recipeStoryDesc',
    build: () => [
      block('sectionHeader', { eyebrow: { 'zh-CN': '故事' } }),
      block('story', {
        theme: 'dark',
        steps: [
          { label: { 'zh-CN': '第一步' }, title: { 'zh-CN': '第一步标题' }, body: { 'zh-CN': '这一步的说明。' }, media: { kind: 'image', src: '', poster: '', mobileSrc: '' } },
          { label: { 'zh-CN': '第二步' }, title: { 'zh-CN': '第二步标题' }, body: { 'zh-CN': '这一步的说明。' }, media: { kind: 'image', src: '', poster: '', mobileSrc: '' } },
          { label: { 'zh-CN': '第三步' }, title: { 'zh-CN': '第三步标题' }, body: { 'zh-CN': '这一步的说明。' }, media: { kind: 'image', src: '', poster: '', mobileSrc: '' } },
        ],
      }),
    ].filter(Boolean),
  },
  {
    key: 'blank',
    labelKey: 'recipeBlank',
    descKey: 'recipeBlankDesc',
    build: () => [],
  },
]

export const findRecipe = (key: string) => blockRecipes.find((r) => r.key === key)
