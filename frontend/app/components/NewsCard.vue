<template>
  <article class="card">
    <div class="card-header">
      <div class="tags">
        <span v-for="tag in tags" :key="tag"># {{ tag }}</span>
      </div>
      <button class="like" type="button" :class="{ active: localLiked }" @click="toggleLike">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M20.8 4.9a5.5 5.5 0 0 0-7.8 0L12 6l-1-1.1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21l7.8-7.3 1-1a5.5 5.5 0 0 0 0-7.8Z" />
        </svg>
        <span>{{ localLikeCount }}</span>
      </button>
    </div>
    <div v-if="source || formattedDate" class="meta">
      <span v-if="source">{{ source }}</span>
      <span v-if="author">{{ author }}</span>
      <span v-if="formattedDate">{{ formattedDate }}</span>
    </div>
    <h4>{{ title }}</h4>
    <p class="discord-text">{{ intro }}</p>
    <button class="primary" type="button" @click="openModal">Подробнее</button>
    <img v-if="imageUrl" class="media image" :src="imageUrl" alt="" loading="lazy" referrerpolicy="no-referrer" />
    <div v-else class="media" :class="variant"></div>
    <div class="comments">
      <button class="comment-toggle" type="button" @click="toggleComments">
        Комментарии {{ commentsLoaded ? comments.length : localCommentCount }}
      </button>
      <div v-if="commentsOpen" class="comment-panel compact">
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
    <Teleport to="body">
      <div v-if="modalOpen" class="modal-backdrop" @click.self="closeModal">
        <article class="post-modal" role="dialog" aria-modal="true" aria-label="Новость">
          <section class="modal-main">
            <img v-if="imageUrl" class="modal-image" :src="imageUrl" alt="" loading="lazy" referrerpolicy="no-referrer" />
            <div v-else class="modal-placeholder" :class="variant"></div>
            <div class="modal-copy">
              <div class="tags">
                <span v-for="tag in tags" :key="tag"># {{ tag }}</span>
              </div>
              <h3>{{ title }}</h3>
              <p class="discord-text">{{ intro }}</p>
              <a v-if="url" class="discord-link" :href="url" target="_blank" rel="noreferrer">Открыть в Discord</a>
            </div>
          </section>
          <aside class="modal-side">
            <NuxtLink v-if="authorId" class="author-card" :to="`/u/${authorId}`">
              <img :src="authorAvatar || fallbackAvatar" alt="" />
              <span>{{ author || 'Автор' }}</span>
            </NuxtLink>
            <div v-else class="author-card inert">
              <img :src="authorAvatar || fallbackAvatar" alt="" />
              <span>{{ author || 'Автор' }}</span>
            </div>
            <div class="modal-stats">
              <button class="like" type="button" :class="{ active: localLiked }" @click="toggleLike">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M20.8 4.9a5.5 5.5 0 0 0-7.8 0L12 6l-1-1.1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21l7.8-7.3 1-1a5.5 5.5 0 0 0 0-7.8Z" />
                </svg>
                <span>{{ localLikeCount }}</span>
              </button>
              <span class="comment-stat">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M5 6.5A3.5 3.5 0 0 1 8.5 3h7A3.5 3.5 0 0 1 19 6.5v5A3.5 3.5 0 0 1 15.5 15H13l-4.5 4v-4A3.5 3.5 0 0 1 5 11.5v-5Z" />
                  <path d="M9 8h6M9 11h4" />
                </svg>
                {{ commentsLoaded ? comments.length : localCommentCount }}
              </span>
            </div>
            <div class="modal-comments">
              <p v-if="commentsPending" class="comment-muted">Загружаем...</p>
              <p v-else-if="!comments.length" class="comment-muted">Комментариев пока нет.</p>
              <article v-for="comment in comments" :key="comment.id" class="comment rich-comment">
                <NuxtLink v-if="comment.authorId" class="comment-author" :to="`/u/${comment.authorId}`">
                  <img :src="comment.authorAvatar || fallbackAvatar" alt="" />
                  <strong>{{ comment.author }}</strong>
                </NuxtLink>
                <div v-else class="comment-author">
                  <img :src="comment.authorAvatar || fallbackAvatar" alt="" />
                  <strong>{{ comment.author }}</strong>
                </div>
                <span>{{ comment.message }}</span>
              </article>
            </div>
            <form class="comment-form" @submit.prevent="sendComment">
              <input v-model.trim="commentText" maxlength="800" placeholder="Комментарий..." />
              <button type="submit" :disabled="commentPending || !commentText">Отправить</button>
            </form>
            <p v-if="commentError" class="comment-error">{{ commentError }}</p>
          </aside>
        </article>
      </div>
    </Teleport>
  </article>
</template>

<script setup lang="ts">
import fallbackAvatar from '~/assets/amy-logo.png'

const props = defineProps<{
  id?: string
  title: string
  intro: string
  tags: string[]
  source?: string
  url?: string
  imageUrl?: string
  author?: string
  authorId?: string
  authorAvatar?: string
  likeCount?: number
  commentCount?: number
  likedByMe?: boolean
  createdAt?: string
  variant?: 'pink' | 'blue' | 'green'
}>()

type NewsComment = {
  id: number
  author: string
  authorId?: string
  authorAvatar?: string
  message: string
  createdAt: string
}

const config = useRuntimeConfig()
const localLiked = ref(Boolean(props.likedByMe))
const localLikeCount = ref(props.likeCount || 0)
const localCommentCount = ref(props.commentCount || 0)
const commentsOpen = ref(false)
const modalOpen = ref(false)
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
  await openModal()
  commentsOpen.value = false
}

const openModal = async () => {
  modalOpen.value = true
  await loadComments()
}

const closeModal = () => {
  modalOpen.value = false
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
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  border-radius: 999px;
  border: 1px solid var(--stroke);
  background: transparent;
  color: var(--muted);
  min-width: 44px;
  height: 32px;
  cursor: pointer;
}

.like svg {
  width: 15px;
  height: 15px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.like.active svg {
  fill: currentColor;
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

.comment-panel.compact {
  display: none;
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

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 70;
  display: grid;
  place-items: center;
  padding: 18px;
  background: rgba(7, 6, 10, 0.78);
  backdrop-filter: blur(8px);
}

.post-modal {
  position: relative;
  width: min(1080px, 100%);
  max-height: min(760px, calc(100dvh - 36px));
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(280px, 0.8fr);
  border: 1px solid var(--stroke);
  border-radius: 8px;
  background: #15151b;
  overflow: hidden;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.55);
}

.comment-stat svg {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.modal-main {
  display: grid;
  grid-template-rows: minmax(260px, 1fr) auto;
  min-height: 0;
}

.modal-image,
.modal-placeholder {
  width: 100%;
  height: 100%;
  max-height: 520px;
  object-fit: contain;
  background: rgba(255, 255, 255, 0.04);
}

.modal-placeholder {
  min-height: 340px;
  background: linear-gradient(135deg, #725c74, #d59abf);
}

.modal-copy,
.modal-side {
  display: grid;
  gap: 12px;
  padding: 16px;
}

.modal-copy h3 {
  margin: 0;
  font-size: 24px;
}

.discord-text {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.discord-link {
  color: var(--accent);
  text-decoration: none;
  font-weight: 700;
}

.modal-side {
  border-left: 1px solid var(--stroke);
  grid-template-rows: auto auto minmax(0, 1fr) auto auto;
  min-height: 0;
}

.author-card {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
  padding: 10px;
  border: 1px solid var(--stroke);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.05);
  color: var(--text);
  text-decoration: none;
}

.author-card.inert {
  pointer-events: none;
}

.author-card img {
  width: 42px;
  height: 42px;
  border-radius: 8px;
  object-fit: cover;
}

.modal-stats {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--muted);
}

.comment-stat {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.modal-comments {
  display: grid;
  gap: 10px;
  align-content: start;
  overflow: auto;
  min-height: 160px;
  padding-right: 4px;
}

.rich-comment {
  gap: 6px;
}

.comment-author {
  width: max-content;
  max-width: 100%;
  display: inline-grid;
  grid-template-columns: 28px minmax(0, max-content);
  gap: 8px;
  align-items: center;
  color: var(--text);
  text-decoration: none;
}

.comment-author img {
  width: 28px;
  height: 28px;
  border-radius: 7px;
  object-fit: cover;
}

@media (max-width: 820px) {
  .post-modal {
    grid-template-columns: 1fr;
    overflow: auto;
  }

  .modal-side {
    border-left: 0;
    border-top: 1px solid var(--stroke);
  }
}

.media.blue {
  background: linear-gradient(135deg, #394a6f, #7ea0ff);
}

.media.green {
  background: linear-gradient(135deg, #2d5e58, #65d2b2);
}
</style>
