<template>
  <section class="hero">
    <div class="hero-content">
      <p class="eyebrow">Добро пожаловать!</p>
      <h1>AMY</h1>
      <p class="lead">
        Приватный сервер с атмосферой ванили и авторскими сюжетами. Мы строим
        мир вокруг ролевых линий, редких механик и теплого комьюнити.
      </p>
      <div class="stats">
        <span class="status"><span class="dot"></span> 126 / 300 играют сейчас</span>
        <div class="pills">
          <span>mc.amyworld.com</span>
          <span>eu2.amyworld.com</span>
          <span>eu3.amyworld.com</span>
        </div>
      </div>
      <div class="actions">
        <button class="primary" type="button">Подать заявку</button>
        <button class="ghost" type="button">Трейлер</button>
      </div>
    </div>
    <div class="hero-media">
      <div class="glow"></div>
      <img class="character" :src="logo" alt="Amy logo" />
    </div>
  </section>
</template>

<script setup lang="ts">
import logo from '~/assets/amy-logo.png'
</script>

<style scoped>
.hero {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 24px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.02));
  border: 1px solid var(--stroke);
  border-radius: var(--radius-lg);
  padding: 24px;
  position: relative;
  overflow: hidden;
  min-height: 340px;
  box-shadow: var(--shadow);
  animation: heroIn 0.9s ease both;
}

.hero-content {
  display: grid;
  gap: 12px;
  align-content: center;
}

.eyebrow {
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 2px;
  font-size: 12px;
}

h1 {
  font-size: 38px;
  margin: 0;
}

.lead {
  color: var(--muted);
  margin: 0;
}

.stats {
  display: grid;
  gap: 10px;
}

.status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #48f579;
  box-shadow: 0 0 12px rgba(72, 245, 121, 0.8);
}

.pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pills span {
  padding: 6px 12px;
  border-radius: 999px;
  background: var(--panel);
  border: 1px solid var(--stroke);
  font-size: 12px;
}

.actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.primary,
.ghost {
  border-radius: 999px;
  padding: 10px 18px;
  border: 1px solid var(--stroke);
  background: var(--panel);
  color: var(--text);
  cursor: pointer;
}

.primary {
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  border: none;
}

.hero-media {
  position: relative;
  display: grid;
  place-items: center;
}

.glow {
  position: absolute;
  width: 220px;
  height: 220px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(240, 90, 60, 0.8), transparent 65%);
  filter: blur(10px);
  animation: pulse 3s ease-in-out infinite;
}

.character {
  width: 240px;
  height: 240px;
  object-fit: contain;
  border-radius: 28px;
  filter: drop-shadow(0 18px 32px rgba(0, 0, 0, 0.45));
  transform: rotate(-3deg);
  animation: floaty 5s ease-in-out infinite;
}

@keyframes heroIn {
  from {
    opacity: 0;
    transform: translateY(18px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes pulse {
  0%,
  100% {
    transform: scale(0.9);
    opacity: 0.7;
  }
  50% {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes floaty {
  0%,
  100% {
    transform: translateY(0) rotate(-3deg);
  }
  50% {
    transform: translateY(-10px) rotate(-1deg);
  }
}

@media (max-width: 1200px) {
  .hero {
    grid-template-columns: 1fr;
  }

  .hero-media {
    order: -1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hero,
  .glow,
  .character {
    animation: none;
  }
}
</style>
