export type AuthUser = {
  id: string
  username: string
  displayName?: string
  email?: string
  avatar?: string
  avatarUrl?: string
  linkedMinecraft?: string
  profileUrl?: string
}

type AuthResponse = {
  authenticated: boolean
  user?: AuthUser
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

  const linkMinecraft = async (nickname: string) => {
    await $fetch(`${config.public.apiBase}/auth/link-minecraft`, {
      method: 'POST',
      credentials: 'include',
      body: { nickname }
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

  return { authenticated, user, pending, error, refresh, loginUrl, profilePath, linkMinecraft, logout }
}
