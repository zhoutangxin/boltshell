/** 赞助位配置（与 Go SponsorConfigView / sponsors.json 对齐） */
export type SponsorSlot = {
  SlotID: string
  enabled: boolean
  type: 'banner' | 'compact' | 'self_promo' | string
  badge: string
  title: string
  desc: string
  linkUrl: string
  imageUrl?: string
  dismissDays?: number
}

export type SponsorConfig = {
  Version: number
  UpdatedAt: string
  CacheTTLSeconds: number
  ProUpgradeURL: string
  IsPro: boolean
  Slots: SponsorSlot[]
  DismissedUntil: Record<string, number>
}
