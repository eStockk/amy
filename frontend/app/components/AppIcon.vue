<template>
  <svg
    class="app-icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="path in paths" :key="path" :d="path" />
    <circle v-for="circle in circles" :key="`${circle.cx}-${circle.cy}-${circle.r}`" :cx="circle.cx" :cy="circle.cy" :r="circle.r" />
    <line v-for="line in lines" :key="`${line.x1}-${line.y1}-${line.x2}-${line.y2}`" :x1="line.x1" :y1="line.y1" :x2="line.x2" :y2="line.y2" />
  </svg>
</template>

<script setup lang="ts">
type IconName = 'home' | 'news' | 'rules' | 'shop' | 'faq' | 'docs' | 'support' | 'apply' | 'play'

const props = defineProps<{
  name: IconName
}>()

const icons: Record<IconName, { paths?: string[]; circles?: Array<{ cx: number; cy: number; r: number }>; lines?: Array<{ x1: number; y1: number; x2: number; y2: number }> }> = {
  home: {
    paths: ['M3 10.5 12 3l9 7.5', 'M5 10v10h14V10', 'M9 20v-6h6v6']
  },
  news: {
    paths: ['M4 4h14a2 2 0 0 1 2 2v14H6a2 2 0 0 1-2-2V4Z', 'M8 8h8', 'M8 12h8', 'M8 16h5']
  },
  rules: {
    paths: ['M12 3 5 6v5c0 4.4 2.8 8.4 7 10 4.2-1.6 7-5.6 7-10V6l-7-3Z', 'm9.5 12 1.8 1.8 3.7-4.1']
  },
  shop: {
    paths: ['M6 8h12l-1 13H7L6 8Z', 'M9 8a3 3 0 0 1 6 0']
  },
  faq: {
    paths: ['M9.1 9a3 3 0 1 1 5.8 1.4c-.9.9-1.9 1.4-2.4 2.6'],
    circles: [{ cx: 12, cy: 12, r: 10 }, { cx: 12, cy: 17, r: 0.7 }]
  },
  docs: {
    paths: ['M4 19.5A2.5 2.5 0 0 1 6.5 17H20', 'M4 4.5A2.5 2.5 0 0 1 6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5Z', 'M8 7h8', 'M8 11h6']
  },
  support: {
    paths: ['M4 12a8 8 0 0 1 16 0', 'M4 12v4a2 2 0 0 0 2 2h1v-6H4Z', 'M20 12v4a2 2 0 0 1-2 2h-1v-6h3Z', 'M15 20h-3']
  },
  apply: {
    paths: ['M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z', 'M14 2v6h6', 'M9 15l2 2 4-5']
  },
  play: {
    paths: ['M8 5v14l11-7-11-7Z']
  }
}

const current = computed(() => icons[props.name])
const paths = computed(() => current.value.paths || [])
const circles = computed(() => current.value.circles || [])
const lines = computed(() => current.value.lines || [])
</script>

<style scoped>
.app-icon {
  width: 1em;
  height: 1em;
  flex: 0 0 auto;
}
</style>
