export type NewsItem = {
  id: string
  title: string
  intro: string
  tags: string[]
  source?: string
  url?: string
  imageUrl?: string
  category?: string
  author?: string
  authorId?: string
  likeCount?: number
  commentCount?: number
  likedByMe?: boolean
  createdAt?: string
  variant?: 'pink' | 'blue' | 'green'
}

export function useNews(limit = 3, category: string | Ref<string> = '') {
  const config = useRuntimeConfig()

  const { data, pending, error, refresh } = useFetch<NewsItem[]>(
    () => {
      const currentCategory = unref(category)
      return `${config.public.apiBase}/news?limit=${limit}${currentCategory ? `&category=${encodeURIComponent(currentCategory)}` : ''}`
    },
    {
      key: `news-${limit}-${unref(category) || 'all'}`,
      server: false,
      default: () => [],
      lazy: true
    }
  )

  const news = computed(() => data.value ?? [])

  return { news, pending, error, refresh }
}
