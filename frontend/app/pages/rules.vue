<template>
  <div class="page">
    <SectionTitle text="Правила сервера" />

    <section class="hero">
      <img :src="rulesImage" alt="Атмосфера правил Amy" />
      <div class="hero-copy">
        <h1>Честная игра и спокойный RP</h1>
        <p>Правила помогают сохранять живой мир, где конфликты интересны, а не токсичны.</p>
      </div>
    </section>

    <ContentPanel
      title="Основные правила"
      text="Уважайте других игроков, не используйте читы и соблюдайте атмосферу RP."
    >
      <ul>
        <li>Запрещены токсичность и оскорбления.</li>
        <li>Гриферство и разрушение чужих построек - под запретом.</li>
        <li>Экономика и торговля строятся на честной игре.</li>
      </ul>
    </ContentPanel>

    <ContentPanel
      title="Контент"
      text="Мы поддерживаем оригинальную ванильную атмосферу и бережно относимся к лору."
    />
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import ContentPanel from '~/components/ContentPanel.vue'
import rulesImage from '~/images/rules/rules1.png'
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.hero {
  position: relative;
  min-height: 340px;
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
  background: linear-gradient(90deg, rgba(10, 10, 14, 0.86), rgba(10, 10, 14, 0.36) 58%, rgba(10, 10, 14, 0.08));
}

.hero img {
  width: 100%;
  height: 100%;
  min-height: 340px;
  object-fit: cover;
}

.hero-copy {
  position: absolute;
  inset: auto auto 28px 28px;
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
  font-size: 36px;
}

.hero-copy p {
  color: var(--muted);
  line-height: 1.6;
}

ul {
  margin: 0;
  padding-left: 18px;
  color: var(--muted);
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
