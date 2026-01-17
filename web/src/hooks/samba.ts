import { ref, type Ref } from 'vue'
import toast from '@/components/toaster'
import { useI18n } from 'vue-i18n'
import { initLazyQuery } from '@/lib/api/query'
import { initMutation, runMutation } from '@/lib/api/mutation'
import { sambaSettingsGQL } from '@/lib/api/query'
import { setSambaSettingsGQL, setSambaUserPasswordGQL } from '@/lib/api/mutation'

export type SambaShareAuth = 'GUEST' | 'PASSWORD'

export interface SambaShare {
    name: string
    sharePath: string
    auth: SambaShareAuth
    readOnly: boolean
}

export interface SambaSettings {
    enabled: boolean
    username: string
    hasPassword: boolean
    shares: SambaShare[]
    serviceName: string
    serviceActive: boolean
    serviceEnabled: boolean
}

type SambaSettingsResult = {
    settings: Ref<SambaSettings | null>
    loading: Ref<boolean>
    saving: Ref<boolean>
    toggling: Ref<boolean>
    passwordSaving: Ref<boolean>
    fetch: () => void
    saveSettings: (input: { enabled: boolean; shares: SambaShare[] }) => Promise<boolean>
    setEnabled: (enabled: boolean) => Promise<boolean>
    setUserPassword: (password: string) => Promise<boolean>
}

let sambaSettingsCache: SambaSettingsResult | null = null

export const useSambaSettings = (): SambaSettingsResult => {
    if (sambaSettingsCache) return sambaSettingsCache

    const { t } = useI18n()
    const settings = ref<SambaSettings | null>(null)

    const { fetch, loading } = initLazyQuery<{ sambaSettings: SambaSettings }>({
        document: sambaSettingsGQL,
        variables: () => ({}),
        handle: (data, error) => {
            if (error) {
                toast(t(error), 'error')
                return
            }
            settings.value = data?.sambaSettings ?? null
        },
    })

    const { mutate: saveMutation, loading: saving, onDone: onSaveDone, onError: onSaveError } = initMutation({
        document: setSambaSettingsGQL,
    })

    const { mutate: toggleMutation, loading: toggling, onDone: onToggleDone, onError: onToggleError } = initMutation({
        document: setSambaSettingsGQL,
    })

    const { mutate: setPasswordMutation, loading: passwordSaving, onDone: onSetPasswordDone, onError: onSetPasswordError } = initMutation({
        document: setSambaUserPasswordGQL,
    })

    const saveSettings = async (input: { enabled: boolean; shares: SambaShare[] }) => {
        const ok = await runMutation(
            saveMutation,
            onSaveDone,
            onSaveError,
            {
                input: {
                    enabled: !!input.enabled,
                    shares: (input.shares ?? []).map((s) => ({
                        name: String(s.name || ''),
                        sharePath: String(s.sharePath || ''),
                        auth: s.auth,
                        readOnly: !!s.readOnly,
                    })),
                },
            }
        )

        if (ok) fetch()
        return ok
    }

    const setEnabled = async (enabled: boolean) => {
        const current = settings.value
        if (!current) return false

        const ok = await runMutation(
            toggleMutation,
            onToggleDone,
            onToggleError,
            {
                input: {
                    enabled: !!enabled,
                    shares: (current.shares ?? []).map((s) => ({
                        name: String(s.name || ''),
                        sharePath: String(s.sharePath || ''),
                        auth: s.auth,
                        readOnly: !!s.readOnly,
                    })),
                },
            }
        )

        if (ok) fetch()
        return ok
    }

    const setUserPassword = async (password: string) => {
        const ok = await runMutation(setPasswordMutation, onSetPasswordDone, onSetPasswordError, { password: String(password || '') })
        if (ok) fetch()
        return ok
    }

    sambaSettingsCache = { settings, loading, saving, toggling, passwordSaving, fetch, saveSettings, setEnabled, setUserPassword }
    sambaSettingsCache.fetch()
    return sambaSettingsCache
}
