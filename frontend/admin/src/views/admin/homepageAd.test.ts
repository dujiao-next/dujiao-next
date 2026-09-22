import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')

describe('independent homepage ad settings', () => {
  it('offers a separate tab with its own save path', () => {
    const settings = read('./Settings.vue')
    expect(settings).toContain("value: 'homepage_ad'")
    expect(settings).toContain('SettingsHomepageAdTab')
    expect(settings).toContain("currentTab.value === 'homepage_ad'")
  })
  it('loads and saves the ad under homepage_ad rather than home_announcement', () => {
    const api = read('../../api/admin.ts')
    const tab = read('./components/SettingsHomepageAdTab.vue')
    expect(api).toContain("key: 'homepage_ad'")
    expect(tab).toContain('getHomepageAd()')
    expect(tab).toContain('updateHomepageAd(')
    expect(tab).toContain('RichEditor')
  })
})
