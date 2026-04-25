<template>
  <div class="page">
    <SectionTitle text="Магазин" />

    <section class="premium">
      <div>
        <p class="eyebrow">Подписка</p>
        <h1>Premium</h1>
        <p class="lead">Единый статус для игроков, которым нужны удобство, косметика и персонализация.</p>
      </div>

      <ul class="premium-list">
        <li>Ускоренная прокачка</li>
        <li>Кастомные сообщения о входе и выходе</li>
        <li>Косметические элементы</li>
      </ul>

      <button class="primary" type="button">Оформить Premium</button>
    </section>

    <section class="products">
      <h2>Товары</h2>
      <div class="grid">
        <article v-for="item in products" :key="item.title" class="product">
          <span class="product-type">{{ item.type }}</span>
          <h3>{{ item.title }}</h3>
          <p>{{ item.text }}</p>
          <button type="button">Подробнее</button>
        </article>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'

const products = [
  {
    type: 'Учебник',
    title: 'Академический учебник Воина',
    text: 'Материал для развития боевых навыков персонажа.'
  },
  {
    type: 'Учебник',
    title: 'Академический учебник Кузнеца',
    text: 'Подходит для ремесленного прогресса и кузнечного RP.'
  },
  {
    type: 'Учебник',
    title: 'Академический учебник Мушкетёра',
    text: 'Помогает развивать стрелковое направление персонажа.'
  },
  {
    type: 'Учебник',
    title: 'Академический учебник Мага',
    text: 'Предмет для магической специализации и косметического отыгрыша.'
  },
  {
    type: 'Набор',
    title: 'Набор "Строителя"',
    text: 'Комплект для игроков, которые активно развивают постройки и декор.'
  }
]
</script>

<style scoped>
.page {
  display: grid;
  gap: 24px;
}

.premium {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(260px, 0.8fr) auto;
  align-items: center;
  gap: 24px;
  padding: 28px;
  border-radius: var(--radius-lg);
  border: 1px solid rgba(228, 94, 56, 0.32);
  background: radial-gradient(circle at 12% 0%, rgba(228, 94, 56, 0.28), transparent 42%),
    linear-gradient(135deg, rgba(34, 35, 48, 0.98), rgba(18, 18, 24, 0.96));
  box-shadow: var(--shadow);
}

.eyebrow {
  margin: 0 0 6px;
  color: var(--accent);
  text-transform: uppercase;
  font-size: 12px;
}

h1,
h2,
h3,
p {
  margin: 0;
}

h1 {
  font-size: 38px;
}

.lead,
.product p {
  color: var(--muted);
}

.premium-list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.premium-list li {
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.primary,
.product button {
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 10px 16px;
  cursor: pointer;
  font-weight: 600;
}

.primary {
  color: #0b0b0f;
  background-image: linear-gradient(135deg, var(--accent), var(--accent-2));
}

.products {
  display: grid;
  gap: 16px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.product {
  display: grid;
  gap: 10px;
  padding: 18px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: var(--panel);
}

.product-type {
  width: max-content;
  padding: 4px 8px;
  border-radius: 999px;
  color: var(--muted);
  border: 1px solid var(--stroke);
  font-size: 12px;
}

.product button {
  width: max-content;
  color: var(--text);
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
}

@media (max-width: 1100px) {
  .premium {
    grid-template-columns: 1fr;
    align-items: stretch;
  }

  .primary {
    width: max-content;
  }
}

@media (max-width: 640px) {
  .premium {
    padding: 20px;
  }

  h1 {
    font-size: 32px;
  }

  .primary,
  .product button {
    width: 100%;
  }
}
</style>
