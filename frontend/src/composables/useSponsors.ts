import { computed, onMounted, ref } from 'vue'
import { DismissSponsorSlot, GetSponsorConfig, RefreshSponsorConfig } from '../../wailsjs/go/main/App.js'
import type { SponsorConfig, SponsorSlot } from '../types/sponsors'

const QUICK_SLOT_ID = 'quick_connect_bottom'
const SIDEBAR_SLOT_PREFIX = 'sidebar_'

export function useSponsors() {
  const config = ref<SponsorConfig | null>(null)
  const loading = ref(false)

  async function load(force = false) {
    loading.value = true
    try {
      const data = force
        ? await RefreshSponsorConfig()
        : await GetSponsorConfig(false)
      config.value = data as SponsorConfig
    } catch (e) {
      console.error('[BoltShell] GetSponsorConfig failed', e)
    } finally {
      loading.value = false
    }
  }

  async function dismiss(slotID: string, days?: number) {
    try {
      await DismissSponsorSlot(slotID, days ?? 0)
      await load(false)
    } catch (e) {
      console.error('[BoltShell] DismissSponsorSlot failed', e)
    }
  }

  const quickSlot = computed(() =>
    config.value?.Slots.find((s) => s.SlotID === QUICK_SLOT_ID),
  )

  const sidebarSlots = computed(() =>
    (config.value?.Slots ?? []).filter(
      (s) => s.SlotID.startsWith(SIDEBAR_SLOT_PREFIX) && s.SlotID !== QUICK_SLOT_ID,
    ),
  )

  const showSponsors = computed(
    () => !config.value?.IsPro && (config.value?.Slots.length ?? 0) > 0,
  )

  onMounted(() => {
    load(false)
  })

  return {
    config,
    loading,
    load,
    dismiss,
    quickSlot,
    sidebarSlots,
    showSponsors,
  }
}
