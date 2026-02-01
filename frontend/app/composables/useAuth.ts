export type AuthUser = {
  id: string
  username: string
  email?: string
  avatar?: string
  avatarUrl?: string
}

export function useAuth() {
  const config = useRuntimeConfig()

  const { data, pending, error, refresh } = useFetch<{ authenticated: boolean; user?: AuthUser }>(
    () => `${config.public.apiBase}/api/auth/me`,
    {
      server: false,
      credentials: 'include',
      default: () => ({ authenticated: false })
    }
  )

  const authenticated = computed(() => Boolean(data.value?.authenticated))
  const user = computed(() => data.value?.user)
  const loginUrl = computed(() => `${config.public.apiBase}/api/auth/discord/start`)

  return { authenticated, user, pending, error, refresh, loginUrl }
}
