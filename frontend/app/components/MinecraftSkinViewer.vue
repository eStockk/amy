<template>
  <div class="skin-viewer" :class="{ compact }">
    <canvas ref="canvasRef" aria-hidden="true"></canvas>
    <p v-if="failed" class="skin-fallback">Скин не загрузился</p>
  </div>
</template>

<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    skinUrl?: string
    compact?: boolean
  }>(),
  {
    skinUrl: '',
    compact: false
  }
)

const canvasRef = ref<HTMLCanvasElement | null>(null)
const failed = ref(false)

let viewer: any = null
let resizeObserver: ResizeObserver | null = null

const resizeViewer = () => {
  if (!viewer || !canvasRef.value?.parentElement) return
  const bounds = canvasRef.value.parentElement.getBoundingClientRect()
  viewer.width = Math.max(180, Math.round(bounds.width))
  viewer.height = Math.max(props.compact ? 220 : 300, Math.round(bounds.height))
}

const loadSkin = async () => {
  if (!viewer || !props.skinUrl) return
  failed.value = false
  try {
    await viewer.loadSkin(props.skinUrl)
  } catch {
    failed.value = true
  }
}

onMounted(async () => {
  if (!canvasRef.value || !props.skinUrl) return
  const skinview3d = await import('skinview3d')
  viewer = new skinview3d.SkinViewer({
    canvas: canvasRef.value,
    width: 260,
    height: props.compact ? 260 : 360,
    skin: props.skinUrl
  })
  viewer.autoRotate = true
  viewer.zoom = props.compact ? 0.78 : 0.66
  viewer.fov = 42
  viewer.globalLight.intensity = 1.1
  viewer.cameraLight.intensity = 0.75
  viewer.animation = new skinview3d.IdleAnimation()
  viewer.animation.speed = 0.45
  resizeViewer()
  resizeObserver = new ResizeObserver(resizeViewer)
  resizeObserver.observe(canvasRef.value.parentElement as Element)
})

watch(
  () => props.skinUrl,
  () => {
    void loadSkin()
  }
)

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (viewer) {
    viewer.dispose()
    viewer = null
  }
})
</script>

<style scoped>
.skin-viewer {
  position: relative;
  min-height: 340px;
  border-radius: 8px;
  overflow: hidden;
  background:
    linear-gradient(45deg, rgba(255, 255, 255, 0.045) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(255, 255, 255, 0.045) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgba(255, 255, 255, 0.045) 75%),
    linear-gradient(-45deg, transparent 75%, rgba(255, 255, 255, 0.045) 75%),
    rgba(255, 255, 255, 0.035);
  background-size: 32px 32px;
  background-position: 0 0, 0 16px, 16px -16px, -16px 0;
}

.skin-viewer.compact {
  min-height: 260px;
}

canvas {
  width: 100%;
  height: 100%;
  display: block;
}

.skin-fallback {
  position: absolute;
  inset: auto 12px 12px;
  margin: 0;
  color: #ffb6a8;
  font-size: 12px;
  text-align: center;
}
</style>
