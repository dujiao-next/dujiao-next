/**
 * 将格式化后的价格字符串拆分为整数与小数两段，用于大小字号等宽排印。
 */
export const splitPrice = (formatted: string): { int: string; dec: string } => {
  const dotIdx = formatted.lastIndexOf('.')
  if (dotIdx === -1) return { int: formatted, dec: '' }
  return {
    int: formatted.slice(0, dotIdx),
    dec: formatted.slice(dotIdx + 1),
  }
}
