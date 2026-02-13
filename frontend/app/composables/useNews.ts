export type NewsItem = {
  id: string
  title: string
  intro: string
  tags: string[]
  variant?: 'pink' | 'blue' | 'green'
}

export function useNews() {
  const config = useRuntimeConfig()

  const { data, pending, error, refresh } = useFetch<NewsItem[]>(
    () => `${config.public.apiBase}/news?limit=3`,
    {
      server: false,
      default: () => [],
      lazy: true
    }
  )

  const news = computed(() => data.value ?? [])

  return { news, pending, error, refresh }
}