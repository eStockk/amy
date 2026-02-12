export type AuthUser = {
  id: string
  username: string
  displayName?: string
  email?: string
  avatar?: string
  avatarUrl?: string
  linkedMinecraft?: string
}

type AuthResponse = {
  authenticated: boolean
  user?: AuthUser
}

export function useAuth() {
  const config = useRuntimeConfig()

  const { data, pending, error, refresh } = useFetch<AuthResponse>(
    () => `${config.public.apiBase}/api/auth/me`,
    {
      key: 'auth-me',
      server: false,
      credentials: 'include',
      default: () => ({ authenticated: false })
    }
  )

  const authenticated = computed(() => Boolean(data.value?.authenticated))
  const user = computed(() => data.value?.user)
  const loginUrl = computed(() => `${config.public.apiBase}/api/auth/discord/start`)

  const linkMinecraft = async (nickname: string) => {
    await $fetch(`${config.public.apiBase}/api/auth/link-minecraft`, {
      method: 'POST',
      credentials: 'include',
      body: { nickname }
    })
    await refresh()
  }

  return { authenticated, user, pending, error, refresh, loginUrl, linkMinecraft }
}
