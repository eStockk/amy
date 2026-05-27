<template>
  <article class="card">
    <div class="card-header">
      <div class="tags">
        <span v-for="tag in tags" :key="tag"># {{ tag }}</span>
      </div>
      <button class="like" type="button" :class="{ active: localLiked }" @click="toggleLike">
        {{ localLiked ? '♥' : '♡' }} <span>{{ localLikeCount }}</span>
      </button>
    </div>
    <div v-if="source || formattedDate" class="meta">
      <span v-if="source">{{ source }}</span>
      <span v-if="author">{{ author }}</span>
      <span v-if="formattedDate">{{ formattedDate }}</span>
    </div>
    <h4>{{ title }}</h4>
    <p>{{ intro }}</p>
    <a v-if="url" class="primary" :href="url" target="_blank" rel="noreferrer">Открыть пост</a>
    <button v-else class="primary" type="button">Подробнее</button>
    <img v-if="imageUrl" class="media image" :src="imageUrl" alt="" loading="lazy" referrerpolicy="no-referrer" />
    <div v-else class="media" :class="variant"></div>
    <div class="comments">
      <button class="comment-toggle" type="button" @click="toggleComments">
        Комментарии {{ commentsLoaded ? comments.length : localCommentCount }}
      </button>
      <div v-if="commentsOpen" class="comment-panel">
        <p v-if="commentsPending" class="comment-muted">Загружаем...</p>
        <p v-else-if="!comments.length" class="comment-muted">Комментариев пока нет.</p>
        <article v-for="comment in comments" :key="comment.id" class="comment">
          <strong>{{ comment.author }}</strong>
          <span>{{ comment.message }}</span>
        </article>
        <form class="comment-form" @submit.prevent="sendComment">
          <input v-model.trim="commentText" maxlength="800" placeholder="Комментарий..." />
          <button type="submit" :disabled="commentPending || !commentText">Отправить</button>
        </form>
        <p v-if="commentError" class="comment-error">{{ commentError }}</p>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
const props = defineProps<{
  id?: string
  title: string
  intro: string
  tags: string[]
  source?: string
  url?: string
  imageUrl?: string
  author?: string
  likeCount?: number
  commentCount?: number
  likedByMe?: boolean
  createdAt?: string
  variant?: 'pink' | 'blue' | 'green'
}>()

type NewsComment = {
  id: number
  author: string
  message: string
  createdAt: string
}

const config = useRuntimeConfig()
const localLiked = ref(Boolean(props.likedByMe))
const localLikeCount = ref(props.likeCount || 0)
const localCommentCount = ref(props.commentCount || 0)
const commentsOpen = ref(false)
const commentsLoaded = ref(false)
const commentsPending = ref(false)
const commentPending = ref(false)
const commentError = ref('')
const commentText = ref('')
const comments = ref<NewsComment[]>([])

watch(() => props.likedByMe, (value) => {
  localLiked.value = Boolean(value)
})

watch(() => props.likeCount, (value) => {
  localLikeCount.value = value || 0
})

watch(() => props.commentCount, (value) => {
  localCommentCount.value = value || 0
})

const formattedDate = computed(() => {
  if (!props.createdAt) return ''
  const parsed = new Date(props.createdAt)
  if (Number.isNaN(parsed.getTime())) return ''

  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: 'short'
  }).format(parsed)
})

const toggleLike = async () => {
  if (!props.id) return
  try {
    const response = await $fetch<{ liked: boolean; likeCount: number }>(`${config.public.apiBase}/news/likes`, {
      method: 'POST',
      credentials: 'include',
      body: { newsId: props.id }
    })
    localLiked.value = response.liked
    localLikeCount.value = response.likeCount
  } catch {
    commentError.value = 'Войдите через Discord, чтобы поставить лайк.'
  }
}

const loadComments = async () => {
  if (!props.id || commentsLoaded.value) return
  commentsPending.value = true
  commentError.value = ''
  try {
    const response = await $fetch<{ comments: NewsComment[] }>(`${config.public.apiBase}/news/comments`, {
      credentials: 'include',
      query: { newsId: props.id }
    })
    comments.value = response.comments
    commentsLoaded.value = true
  } catch {
    commentError.value = 'Не удалось загрузить комментарии.'
  } finally {
    commentsPending.value = false
  }
}

const toggleComments = async () => {
  commentsOpen.value = !commentsOpen.value
  if (commentsOpen.value) await loadComments()
}

const sendComment = async () => {
  if (!props.id || !commentText.value) return
  commentPending.value = true
  commentError.value = ''
  try {
    const response = await $fetch<{ comment: NewsComment }>(`${config.public.apiBase}/news/comments`, {
      method: 'POST',
      credentials: 'include',
      body: { newsId: props.id, message: commentText.value }
    })
    comments.value.push(response.comment)
    commentsLoaded.value = true
    localCommentCount.value += 1
    commentText.value = ''
  } catch {
    commentError.value = 'Войдите через Discord, чтобы комментировать.'
  } finally {
    commentPending.value = false
  }
}
</script>

<style scoped>
.card {
  padding: 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: var(--panel);
  display: grid;
  gap: 10px;
  transition: transform 0.2s ease, border-color 0.2s ease;
}

.card:hover {
  transform: translateY(-4px);
  border-color: rgba(240, 90, 60, 0.4);
}

.card-header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
}

.tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.tags span {
  border: 1px solid var(--stroke);
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 11px;
  color: var(--muted);
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  color: var(--muted);
  font-size: 12px;
}

.meta span + span::before {
  content: '/';
  margin-right: 8px;
  color: rgba(255, 255, 255, 0.28);
}

.like {
  border-radius: 999px;
  border: 1px solid var(--stroke);
  background: transparent;
  color: var(--muted);
  min-width: 44px;
  height: 32px;
  cursor: pointer;
}

.like.active {
  color: #ff9fba;
  border-color: rgba(255, 159, 186, 0.5);
  background: rgba(255, 159, 186, 0.12);
}

h4 {
  margin: 0;
  font-size: 18px;
}

p {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}

.primary {
  border-radius: 999px;
  padding: 8px 14px;
  border: none;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  color: #fff;
  width: max-content;
  cursor: pointer;
  text-decoration: none;
  font-size: 13px;
  font-weight: 600;
}

.media {
  height: 140px;
  border-radius: 9px;
  background: linear-gradient(135deg, #725c74, #d59abf);
}

.media.image {
  width: 100%;
  object-fit: cover;
  background: rgba(255, 255, 255, 0.04);
}

.comments {
  display: grid;
  gap: 8px;
}

.comment-toggle {
  width: max-content;
  border: 1px solid var(--stroke);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.05);
  color: var(--muted);
  padding: 7px 11px;
  cursor: pointer;
}

.comment-panel {
  display: grid;
  gap: 8px;
  border-top: 1px solid var(--stroke);
  padding-top: 10px;
}

.comment {
  display: grid;
  gap: 3px;
  font-size: 13px;
}

.comment span,
.comment-muted {
  color: var(--muted);
}

.comment-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.comment-form input {
  min-width: 0;
  border: 1px solid var(--stroke);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text);
  padding: 8px 10px;
}

.comment-form button {
  border: 0;
  border-radius: 8px;
  background: rgba(240, 90, 60, 0.85);
  color: #fff;
  padding: 8px 10px;
  cursor: pointer;
}

.comment-error {
  color: #ff9f9f;
}

.media.blue {
  background: linear-gradient(135deg, #394a6f, #7ea0ff);
}

.media.green {
  background: linear-gradient(135deg, #2d5e58, #65d2b2);
}
</style>
