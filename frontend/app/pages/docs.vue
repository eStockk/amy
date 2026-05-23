<template>
  <div class="page">
    <SectionTitle text="Документация" />

    <section class="hero">
      <img :src="docsImage" alt="Документация Amy" />
      <div class="hero-copy">
        <h1>Быстрый старт в Amy</h1>
        <p>Короткие инструкции для входа на сервер, первых шагов и спокойного погружения в RP.</p>
      </div>
    </section>

    <div class="docs-grid">
      <ContentPanel
        title="Как подключиться"
        text="IP сервера, базовая подготовка клиента и первые действия после входа."
      />
      <ContentPanel
        title="Гайд новичка"
        text="С чего начать, как вступить во фракцию, найти команду и не потеряться в правилах."
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import ContentPanel from '~/components/ContentPanel.vue'
import docsImage from '~/images/doc/doc1.png'
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.hero {
  position: relative;
  min-height: 360px;
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--stroke);
  box-shadow: var(--shadow);
  background: #111118;
}

.hero::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, rgba(10, 10, 14, 0.88), rgba(10, 10, 14, 0.44) 56%, rgba(10, 10, 14, 0.1));
}

.hero img {
  width: 100%;
  height: 100%;
  min-height: 360px;
  object-fit: cover;
}

.hero-copy {
  position: absolute;
  inset: auto auto 30px 30px;
  z-index: 1;
  max-width: 560px;
  display: grid;
  gap: 10px;
}

.hero-copy h1,
.hero-copy p {
  margin: 0;
}

.hero-copy h1 {
  font-size: 38px;
}

.hero-copy p {
  color: var(--muted);
  line-height: 1.6;
}

.docs-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

@media (max-width: 820px) {
  .docs-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .hero,
  .hero img {
    min-height: 300px;
  }

  .hero::after {
    background: linear-gradient(180deg, rgba(10, 10, 14, 0.28), rgba(10, 10, 14, 0.92));
  }

  .hero-copy {
    inset: auto 18px 20px;
  }

  .hero-copy h1 {
    font-size: 30px;
  }
}
</style>
