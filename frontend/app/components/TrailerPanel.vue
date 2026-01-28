<template>
  <section class="panel">
    <h3>Трейлер сервера</h3>
    <div class="video">
      <div class="play" aria-hidden="true">
        <svg viewBox="0 0 24 24">
          <path d="M8 5v14l11-7z" fill="currentColor" />
        </svg>
      </div>
    </div>
  </section>
</template>

<style scoped>
.panel {
  padding: 20px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: var(--panel);
  display: grid;
  gap: 16px;
}

h3 {
  margin: 0;
}

.video {
  height: 180px;
  border-radius: 16px;
  background: linear-gradient(135deg, #2d3141, #4a3c5e);
  display: grid;
  place-items: center;
  position: relative;
  overflow: hidden;
}

.video::after {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 30% 30%, rgba(255, 255, 255, 0.2), transparent 60%);
}

.play {
  width: 46px;
  height: 46px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.2);
  display: grid;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, 0.4);
  font-size: 18px;
}

.play svg {
  width: 20px;
  height: 20px;
}
</style>