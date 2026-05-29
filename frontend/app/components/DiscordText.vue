<template>
  <div class="discord-markdown" v-html="formattedHtml"></div>
</template>

<script setup lang="ts">
const props = defineProps<{
  text?: string
}>()

const formattedHtml = computed(() => formatDiscordMarkdown(props.text || ''))

function formatDiscordMarkdown(raw: string) {
  const lines = raw.replace(/\r\n?/g, '\n').split('\n')
  const blocks: string[] = []
  let index = 0

  while (index < lines.length) {
    const line = lines[index]

    if (line.trim().startsWith('```')) {
      const code: string[] = []
      index += 1
      while (index < lines.length && !lines[index].trim().startsWith('```')) {
        code.push(lines[index])
        index += 1
      }
      if (index < lines.length) index += 1
      blocks.push(`<pre class="discord-code-block"><code>${escapeHtml(code.join('\n'))}</code></pre>`)
      continue
    }

    if (line.trim() === '') {
      blocks.push('<div class="discord-blank"></div>')
      index += 1
      continue
    }

    const heading = /^(#{1,3})\s+(.+)$/.exec(line.trim())
    if (heading) {
      const level = heading[1].length
      blocks.push(`<h${level} class="discord-heading">${formatDiscordInline(heading[2])}</h${level}>`)
      index += 1
      continue
    }

    if (/^\s*>/.test(line)) {
      const quote: string[] = []
      while (index < lines.length && /^\s*>/.test(lines[index])) {
        quote.push(lines[index].replace(/^\s*>\s?/, ''))
        index += 1
      }
      blocks.push(`<blockquote class="discord-quote">${quote.map(formatDiscordInline).join('<br>')}</blockquote>`)
      continue
    }

    blocks.push(`<p>${formatDiscordInline(line)}</p>`)
    index += 1
  }

  return blocks.join('')
}

function formatDiscordInline(text: string, depth = 0): string {
  if (!text) return ''
  if (depth > 6) return escapeHtml(text)

  let output = ''
  let cursor = 0

  while (cursor < text.length) {
    const next = findNextToken(text, cursor)
    if (!next) {
      output += escapeHtml(text.slice(cursor))
      break
    }

    if (next.index > cursor) {
      output += escapeHtml(text.slice(cursor, next.index))
    }

    if (next.type === 'link' || next.type === 'maskedLink') {
      const url = next.href || text.slice(next.index, next.end)
      const label = next.label ? formatDiscordInline(next.label, depth + 1) : escapeHtml(url)
      output += `<a href="${escapeAttribute(url)}" target="_blank" rel="noreferrer">${label}</a>`
      cursor = next.end
      continue
    }

    if (next.type === 'timestamp') {
      output += `<time>${escapeHtml(formatDiscordTimestamp(next.timestamp || 0, next.timestampStyle || 'f'))}</time>`
      cursor = next.end
      continue
    }

    if (next.type === 'mention') {
      output += `<span class="discord-mention">${escapeHtml(next.label || '')}</span>`
      cursor = next.end
      continue
    }

    const inner = text.slice(next.index + next.token.length, next.end)
    const content = next.type === 'code' ? escapeHtml(inner) : formatDiscordInline(inner, depth + 1)
    output += wrapInline(next.type, content)
    cursor = next.end + next.token.length
  }

  return output
}

type InlineToken = {
  type: 'bold' | 'italic' | 'underline' | 'strike' | 'spoiler' | 'code' | 'link' | 'maskedLink' | 'timestamp' | 'mention'
  token: string
  index: number
  end: number
  href?: string
  label?: string
  timestamp?: number
  timestampStyle?: string
}

function findNextToken(text: string, cursor: number): InlineToken | null {
  const candidates: InlineToken[] = []
  const pairs: Array<Pick<InlineToken, 'type' | 'token'>> = [
    { type: 'bold', token: '**' },
    { type: 'underline', token: '__' },
    { type: 'strike', token: '~~' },
    { type: 'spoiler', token: '||' },
    { type: 'code', token: '`' },
    { type: 'italic', token: '*' }
  ]

  for (const pair of pairs) {
    const start = text.indexOf(pair.token, cursor)
    if (start < 0) continue
    if (pair.token === '*' && text[start + 1] === '*') continue
    const end = text.indexOf(pair.token, start + pair.token.length)
    if (end > start + pair.token.length - 1) {
      candidates.push({ ...pair, index: start, end })
    }
  }

  const linkMatch = /https?:\/\/[^\s<]+/i.exec(text.slice(cursor))
  if (linkMatch?.index !== undefined) {
    const start = cursor + linkMatch.index
    const cleanURL = linkMatch[0].replace(/[),.!?:;]+$/, '')
    candidates.push({ type: 'link', token: '', index: start, end: start + cleanURL.length })
  }

  const maskedLinkMatch = /\[([^\]\n]{1,140})\]\(\s*(https?:\/\/[^\s)]+)\s*\)/i.exec(text.slice(cursor))
  if (maskedLinkMatch?.index !== undefined) {
    const start = cursor + maskedLinkMatch.index
    candidates.push({
      type: 'maskedLink',
      token: '',
      index: start,
      end: start + maskedLinkMatch[0].length,
      label: maskedLinkMatch[1],
      href: maskedLinkMatch[2]
    })
  }

  const timestampMatch = /<t:(\d{1,12})(?::([tTdDfFR]))?>/.exec(text.slice(cursor))
  if (timestampMatch?.index !== undefined) {
    const start = cursor + timestampMatch.index
    candidates.push({
      type: 'timestamp',
      token: '',
      index: start,
      end: start + timestampMatch[0].length,
      timestamp: Number(timestampMatch[1]),
      timestampStyle: timestampMatch[2] || 'f'
    })
  }

  const mentionMatch = /@(everyone|here)\b/i.exec(text.slice(cursor))
  if (mentionMatch?.index !== undefined) {
    const start = cursor + mentionMatch.index
    candidates.push({
      type: 'mention',
      token: '',
      index: start,
      end: start + mentionMatch[0].length,
      label: mentionMatch[0]
    })
  }

  candidates.sort((a, b) => a.index - b.index || b.token.length - a.token.length)
  return candidates[0] || null
}

function formatDiscordTimestamp(seconds: number, style: string) {
  const date = new Date(seconds * 1000)
  if (Number.isNaN(date.getTime())) return ''

  if (style === 't') {
    return new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' }).format(date)
  }
  if (style === 'T') {
    return new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(date)
  }
  if (style === 'd') {
    return new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' }).format(date)
  }
  if (style === 'D') {
    return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }).format(date)
  }
  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

function wrapInline(type: InlineToken['type'], content: string) {
  switch (type) {
    case 'bold':
      return `<strong>${content}</strong>`
    case 'italic':
      return `<em>${content}</em>`
    case 'underline':
      return `<span class="discord-underline">${content}</span>`
    case 'strike':
      return `<s>${content}</s>`
    case 'spoiler':
      return `<span class="discord-spoiler">${content}</span>`
    case 'code':
      return `<code class="discord-inline-code">${content}</code>`
    default:
      return content
  }
}

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function escapeAttribute(value: string) {
  return escapeHtml(value).replaceAll('`', '&#96;')
}
</script>

<style scoped>
.discord-markdown {
  display: grid;
  gap: 6px;
  overflow-wrap: anywhere;
  white-space: normal;
}

.discord-markdown :deep(p) {
  margin: 0;
}

.discord-markdown :deep(.discord-heading) {
  margin: 0;
  color: var(--text);
  line-height: 1.2;
}

.discord-markdown :deep(h1.discord-heading) {
  font-size: 1.35em;
}

.discord-markdown :deep(h2.discord-heading) {
  font-size: 1.2em;
}

.discord-markdown :deep(h3.discord-heading) {
  font-size: 1.08em;
}

.discord-markdown :deep(a) {
  color: #6aa8ff;
  text-decoration: none;
}

.discord-markdown :deep(a:hover) {
  text-decoration: underline;
}

.discord-markdown :deep(.discord-underline) {
  text-decoration: underline;
}

.discord-markdown :deep(.discord-spoiler) {
  border-radius: 4px;
  background: #202225;
  color: transparent;
  padding: 0 3px;
  transition: color 0.15s ease;
}

.discord-markdown :deep(.discord-spoiler:hover) {
  color: inherit;
}

.discord-markdown :deep(.discord-inline-code),
.discord-markdown :deep(.discord-code-block) {
  border-radius: 4px;
  background: #1f2128;
  color: #e7e9f2;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.discord-markdown :deep(.discord-inline-code) {
  padding: 1px 4px;
  font-size: 0.92em;
}

.discord-markdown :deep(.discord-mention) {
  border-radius: 4px;
  background: rgba(88, 101, 242, 0.26);
  color: #dbe0ff;
  padding: 0 4px;
  font-weight: 600;
}

.discord-markdown :deep(.discord-code-block) {
  margin: 0;
  padding: 10px;
  overflow: auto;
}

.discord-markdown :deep(.discord-quote) {
  margin: 0;
  padding: 2px 0 2px 10px;
  border-left: 4px solid #4e5361;
  color: #c5c7d1;
}

.discord-markdown :deep(.discord-blank) {
  height: 6px;
}
</style>
