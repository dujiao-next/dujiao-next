/**
 * 区块化页面 schema
 *
 * 一条 posts 记录里存 { version, blocks: [] }，每个 block 有 type + props。
 * 文本字段用多语言对象（{ "zh-CN": "...", "en-US": "..." }），未填的语言回退中文。
 * 图片/视频不分语言。
 */

export type I18nText = string | Record<string, string>

export interface BlockCta {
  label: I18nText
  url: string
  /** primary = 实心蓝，ghost = 描边，link = 纯文字蓝链 */
  style?: 'primary' | 'ghost' | 'link'
}

export interface BlockMedia {
  kind: 'image' | 'video'
  src: string
  /** 视频封面，也用作图片懒加载占位 */
  poster?: string
  /** 手机端换图；不填就用 src */
  mobileSrc?: string
  alt?: string
}

export interface BlockBase {
  id: string
  type: string
  props: Record<string, any>
}

export interface HeroBlock extends BlockBase {
  type: 'hero'
  props: {
    theme?: 'dark' | 'light'
    /** overlay = 文字压在媒体上；stacked = 文字在上、媒体铺满下方 */
    variant?: 'overlay' | 'stacked'
    eyebrow?: I18nText
    title: I18nText
    subtitle?: I18nText
    media?: BlockMedia
    ctas?: BlockCta[]
    align?: 'left' | 'center'
  }
}

export interface PromoBlock extends BlockBase {
  type: 'promo'
  props: {
    items: Array<{
      theme?: 'dark' | 'light'
      eyebrow?: I18nText
      title: I18nText
      subtitle?: I18nText
      media?: BlockMedia
      ctas?: BlockCta[]
    }>
  }
}

export interface SectionHeaderBlock extends BlockBase {
  type: 'sectionHeader'
  props: {
    eyebrow?: I18nText
    accent?: string
    title: I18nText
    lead?: I18nText
    link?: BlockCta
  }
}

export interface FeatureGridBlock extends BlockBase {
  type: 'featureGrid'
  props: {
    columns?: 2 | 3 | 4
    theme?: 'light' | 'gray'
    items: Array<{ media?: BlockMedia; title: I18nText; body?: I18nText }>
  }
}

export interface CardScrollerBlock extends BlockBase {
  type: 'cardScroller'
  props: {
    title?: I18nText
    secondary?: I18nText
    cards: Array<{ media?: BlockMedia; label?: I18nText; title: I18nText; body?: I18nText; url?: string }>
  }
}

export interface GalleryBlock extends BlockBase {
  type: 'gallery'
  props: {
    title?: I18nText
    items: Array<{ media: BlockMedia; caption?: I18nText }>
  }
}

export interface VideoBlock extends BlockBase {
  type: 'video'
  props: {
    title?: I18nText
    caption?: I18nText
    media: BlockMedia
  }
}

export interface SpecsBlock extends BlockBase {
  type: 'specs'
  props: {
    title?: I18nText
    columns?: 2 | 3
    items: Array<{ label: I18nText; value: I18nText }>
  }
}

export interface CtaBlock extends BlockBase {
  type: 'cta'
  props: {
    title: I18nText
    subtitle?: I18nText
    price?: string
    media?: BlockMedia
    ctas?: BlockCta[]
  }
}

export interface RichTextBlock extends BlockBase {
  type: 'richText'
  props: { html: Record<string, string> }
}

export type PageBlock =
  | HeroBlock | PromoBlock | SectionHeaderBlock | FeatureGridBlock
  | CardScrollerBlock | GalleryBlock | VideoBlock | SpecsBlock | CtaBlock | RichTextBlock

export interface PageContent {
  version: number
  blocks: PageBlock[]
}
