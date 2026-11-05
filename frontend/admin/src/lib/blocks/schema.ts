/**
 * 积木编辑器 schema：每个区块类型声明有哪些字段，表单由 BlockField 递归渲染。
 * 字段 label 走 admin i18n（admin.blocks.fields.*），区块名走 admin.blocks.types.*。
 */

export type FieldKind = 'text' | 'textarea' | 'i18n' | 'url' | 'select' | 'media' | 'list' | 'strings'

export interface FieldDef {
  key: string
  labelKey: string
  kind: FieldKind
  placeholderKey?: string
  options?: Array<{ value: string; labelKey: string }>
  /** kind = list 时的行字段 */
  fields?: FieldDef[]
  /** 新行的初始值 */
  itemDefault?: () => any
  default?: any
  /** 字段下方的尺寸/安全区提示（admin.blocks.hints.*） */
  hintKey?: string
  /** 属性面板分组：内容（默认）/ 外观 / 链接 */
  group?: 'content' | 'look' | 'link'
  /** 多行文本（标题这类可以按回车换行的字段） */
  multiline?: boolean
}

export interface BlockDef {
  type: string
  labelKey: string
  fields: FieldDef[]
}

const i18nEmpty = () => ({ 'zh-CN': '', 'zh-TW': '', 'en-US': '' })

const themeField: FieldDef = {
  key: 'theme',
  labelKey: 'theme',
  kind: 'select',
  group: 'look',
  options: [
    { value: 'dark', labelKey: 'themeDark' },
    { value: 'light', labelKey: 'themeLight' },
  ],
  default: 'dark',
}

const mediaField: FieldDef = {
  key: 'media',
  labelKey: 'media',
  kind: 'media',
}

const mediaWith = (hintKey: string): FieldDef => ({ ...mediaField, hintKey })

const ctaList: FieldDef = {
  key: 'ctas',
  labelKey: 'ctas',
  kind: 'list',
  group: 'link',
  itemDefault: () => ({ label: i18nEmpty(), url: '', style: 'primary' }),
  fields: [
    { key: 'label', labelKey: 'ctaLabel', kind: 'i18n' },
    { key: 'url', labelKey: 'ctaUrl', kind: 'url', placeholderKey: '/products/example' },
    {
      key: 'style',
      labelKey: 'ctaStyle',
      kind: 'select',
      options: [
        { value: 'primary', labelKey: 'ctaPrimary' },
        { value: 'ghost', labelKey: 'ctaGhost' },
        { value: 'link', labelKey: 'ctaLink' },
      ],
      default: 'primary',
    },
  ],
}

export const blockDefs: BlockDef[] = [
  {
    type: 'hero',
    labelKey: 'hero',
    fields: [
      themeField,
      {
        key: 'variant',
        labelKey: 'variant',
        group: 'look', kind: 'select',
        options: [
          { value: 'stacked', labelKey: 'variantStacked' },
          { value: 'overlay', labelKey: 'variantOverlay' },
        ],
        default: 'stacked',
      },
      { key: 'eyebrow', labelKey: 'eyebrow', kind: 'i18n' },
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true, default: i18nEmpty() },
      { key: 'subtitle', labelKey: 'subtitle', kind: 'i18n', multiline: true },
        mediaWith('hintHero'),
      ctaList,
    ],
  },
  {
    type: 'promo',
    labelKey: 'promo',
    fields: [
      {
        key: 'items',
        labelKey: 'promoItems',
        kind: 'list',
        itemDefault: () => ({ theme: 'dark', eyebrow: i18nEmpty(), title: i18nEmpty(), subtitle: i18nEmpty(), ctas: [] }),
        fields: [
          themeField,
          { key: 'eyebrow', labelKey: 'eyebrow', kind: 'i18n' },
          { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
          { key: 'subtitle', labelKey: 'subtitle', kind: 'i18n', multiline: true },
        mediaWith('hintPromo'),
          ctaList,
        ],
      },
    ],
  },
  {
    type: 'sectionHeader',
    labelKey: 'sectionHeader',
    fields: [
      { key: 'eyebrow', labelKey: 'eyebrow', kind: 'i18n' },
      { key: 'accent', labelKey: 'accent', group: 'look', kind: 'text', placeholderKey: '#c76a00' },
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      { key: 'lead', labelKey: 'lead', kind: 'i18n', multiline: true },
    ],
  },
  {
    type: 'featureGrid',
    labelKey: 'featureGrid',
    fields: [
      {
        key: 'columns',
        labelKey: 'columns',
        group: 'look',
        kind: 'select',
        options: [
          { value: '2', labelKey: 'col2' },
          { value: '3', labelKey: 'col3' },
          { value: '4', labelKey: 'col4' },
        ],
        default: '3',
      },
      {
        key: 'items',
        labelKey: 'featureItems',
        kind: 'list',
        itemDefault: () => ({ title: i18nEmpty(), body: i18nEmpty() }),
        fields: [
        mediaWith('hintCard'),
          { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
          { key: 'body', labelKey: 'body', kind: 'i18n' },
        ],
      },
    ],
  },
  {
    type: 'cardScroller',
    labelKey: 'cardScroller',
    fields: [
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      { key: 'secondary', labelKey: 'secondary', kind: 'i18n', multiline: true },
      {
        key: 'cards',
        labelKey: 'cards',
        kind: 'list',
        itemDefault: () => ({ label: i18nEmpty(), title: i18nEmpty(), body: i18nEmpty(), url: '' }),
        fields: [
        mediaWith('hintCard'),
          { key: 'label', labelKey: 'label', kind: 'i18n' },
          { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
          { key: 'body', labelKey: 'body', kind: 'i18n' },
          { key: 'url', labelKey: 'url', kind: 'url', group: 'link' },
        ],
      },
    ],
  },
  {
    type: 'gallery',
    labelKey: 'gallery',
    fields: [
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      {
        key: 'items',
        labelKey: 'galleryItems',
        kind: 'list',
        itemDefault: () => ({ caption: i18nEmpty() }),
        fields: [mediaWith('hintGallery'), { key: 'caption', labelKey: 'caption', kind: 'i18n', multiline: true }],
      },
    ],
  },
  {
    type: 'compare',
    labelKey: 'compare',
    fields: [
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      { key: 'subtitle', labelKey: 'lead', kind: 'i18n', multiline: true },
      {
        key: 'columns',
        labelKey: 'compareColumns',
        kind: 'list',
        itemDefault: () => ({ media: { kind: 'image', src: '', poster: '', mobileSrc: '' }, title: i18nEmpty(), subtitle: i18nEmpty(), price: '', ctaLabel: i18nEmpty(), ctaUrl: '' }),
        fields: [
          mediaWith('hintCard'),
          { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
          { key: 'subtitle', labelKey: 'caption', kind: 'i18n', multiline: true },
          { key: 'price', labelKey: 'priceLabel', kind: 'text' },
          { key: 'ctaLabel', labelKey: 'ctaLabel', kind: 'i18n', multiline: true },
          { key: 'ctaUrl', labelKey: 'ctaUrl', kind: 'url', group: 'link' },
        ],
      },
      {
        key: 'rows',
        labelKey: 'compareRows',
        kind: 'list',
        itemDefault: () => ({ label: i18nEmpty(), values: [] }),
        fields: [
          { key: 'label', labelKey: 'compareRowLabel', kind: 'i18n' },
          { key: 'values', labelKey: 'compareRowValues', kind: 'strings' },
        ],
      },
    ],
  },
  {
    type: 'story',
    labelKey: 'story',
    fields: [
      themeField,
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      {
        key: 'steps',
        labelKey: 'storySteps',
        kind: 'list',
        itemDefault: () => ({ label: i18nEmpty(), title: i18nEmpty(), body: i18nEmpty(), media: { kind: 'image', src: '', poster: '', mobileSrc: '' } }),
        fields: [
          { key: 'label', labelKey: 'storyStepLabel', kind: 'i18n' },
          { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
          { key: 'body', labelKey: 'body', kind: 'textarea' },
          mediaWith('hintStory'),
        ],
      },
    ],
  },
  {
    type: 'video',
    labelKey: 'video',
    fields: [
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      { key: 'caption', labelKey: 'caption', kind: 'i18n', multiline: true },
        mediaWith('hintVideo'),
    ],
  },
  {
    type: 'specs',
    labelKey: 'specs',
    fields: [
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      {
        key: 'columns',
        labelKey: 'columns',
        group: 'look',
        kind: 'select',
        options: [
          { value: '2', labelKey: 'col2' },
          { value: '3', labelKey: 'col3' },
        ],
        default: '2',
      },
      {
        key: 'items',
        labelKey: 'specItems',
        kind: 'list',
        itemDefault: () => ({ label: i18nEmpty(), value: i18nEmpty() }),
        fields: [
          { key: 'label', labelKey: 'label', kind: 'i18n' },
          { key: 'value', labelKey: 'value', kind: 'i18n', multiline: true },
        ],
      },
    ],
  },
  {
    type: 'cta',
    labelKey: 'cta',
    fields: [
      { key: 'title', labelKey: 'title', kind: 'i18n', multiline: true },
      { key: 'subtitle', labelKey: 'subtitle', kind: 'i18n', multiline: true },
      { key: 'price', labelKey: 'price', group: 'look', kind: 'text', placeholderKey: '9.00 CNY' },
        mediaWith('hintCard'),
      ctaList,
    ],
  },
  {
    type: 'richText',
    labelKey: 'richText',
    fields: [{ key: 'html', labelKey: 'html', kind: 'textarea' }],
  },
]

export const findBlockDef = (type: string) => blockDefs.find((d) => d.type === type)

/** 按 schema 生成一个新区块的初始 props */
export function defaultProps(def: BlockDef): Record<string, any> {
  const props: Record<string, any> = {}
  for (const field of def.fields) {
    if (field.kind === 'i18n') props[field.key] = field.default ?? i18nEmpty()
    else if (field.kind === 'media') props[field.key] = { kind: 'image', src: '' }
    else if (field.kind === 'list') props[field.key] = []
    else if (field.kind === 'select') props[field.key] = field.default ?? field.options?.[0]?.value ?? ''
    else props[field.key] = field.default ?? ''
  }
  return props
}

export function createBlock(def: BlockDef): { id: string; type: string; props: Record<string, any> } {
  return {
    id: `blk_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`,
    type: def.type,
    props: defaultProps(def),
  }
}

export const emptyPageContent = () => ({ version: 1, blocks: [] as any[] })
