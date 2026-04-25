export type ServerStatus = {
  address: string
  online: boolean
  version?: string
  players?: {
    online: number
    max: number
  }
}

export function useServerStatus() {
  const config = useRuntimeConfig()

  const { data, pending, error, refresh } = useFetch<ServerStatus>(
    () => `${config.public.apiBase}/server/status`,
    {
      key: 'server-status',
      server: false,
      default: () => ({
        address: 'play.amy-world.ru',
        online: false,
        players: { online: 0, max: 0 }
      }),
      lazy: true
    }
  )

  const status = computed(() => data.value)

  return { status, pending, error, refresh }
}
