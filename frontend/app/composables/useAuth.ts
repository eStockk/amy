export type RPApplicationSummary = {
  id: string
  status: 'pending' | 'call' | 'accepted' | 'canceled' | 'approved' | 'rejected'
  nickname: string
  rpName?: string
  race?: string
  gender?: string
  heightCm?: number
  birthDate?: string
  prisonReason?: string
  createdAt?: string
  updatedAt?: string
  moderatedAt?: string
}

export type AuthUser = {
  id: string
  username: string
  displayName?: string
  email?: string
  avatar?: string
  avatarUrl?: string
  rpFirstName?: string
  rpLastName?: string
  profileUrl?: string
  isOnline?: boolean
  isAmyDiscordMember?: boolean
  rpApplication?: RPApplicationSummary
}

type AuthResponse = {
  authenticated: boolean
  user?: AuthUser
}

type RPApplicationPayload = {
  nickname: string
  source?: string
  rpName?: string
  birthDate: string
  race: string
  gender: string
  heightCm: number
  skills: string
  plan: string
  biography: string
  prisonReason: string
  skinUrl: string
}

export function useAuth() {
  const config = useRuntimeConfig()

  const { data, pending, error, refresh } = useFetch<AuthResponse>(
    () => `${config.public.apiBase}/auth/me`,
    {
      key: 'auth-me',
      server: false,
      credentials: 'include',
      default: () => ({ authenticated: false })
    }
  )

  const authenticated = computed(() => Boolean(data.value?.authenticated))
  const user = computed(() => data.value?.user)
  const loginUrl = computed(() => `${config.public.apiBase}/auth/discord/start`)
  const profilePath = computed(() => (user.value?.id ? `/u/${user.value.id}` : '/profile'))

  const submitRPApplication = async (payload: RPApplicationPayload) => {
    await $fetch(`${config.public.apiBase}/rp/applications`, {
      method: 'POST',
      credentials: 'include',
      body: payload
    })
    await refresh()
  }

  const deleteRPApplication = async (applicationId: string) => {
    await $fetch(`${config.public.apiBase}/rp/applications/${applicationId}`, {
      method: 'DELETE',
      credentials: 'include'
    })
    await refresh()
  }

  const logout = async () => {
    await $fetch(`${config.public.apiBase}/auth/logout`, {
      method: 'POST',
      credentials: 'include'
    })
    await refresh()
  }

  return {
    authenticated,
    user,
    pending,
    error,
    refresh,
    loginUrl,
    profilePath,
    submitRPApplication,
    deleteRPApplication,
    logout
  }
}
