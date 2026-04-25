export type NewsItem = {
  id: string
  title: string
  intro: string
  tags: string[]
  source?: string
  url?: string
  createdAt?: string
  variant?: 'pink' | 'blue' | 'green'
}

export function useNews(limit = 3) {
  const config = useRuntimeConfig()

  const { data, pending, error, refresh } = useFetch<NewsItem[]>(
    () => `${config.public.apiBase}/news?limit=${limit}`,
    {
      key: `news-${limit}`,
      server: false,
      default: () => [],
      lazy: true
    }
  )

  const news = computed(() => data.value ?? [])

  return { news, pending, error, refresh }
}
