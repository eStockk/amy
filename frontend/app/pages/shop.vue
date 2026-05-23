<template>
  <div class="page">
    <SectionTitle text="Магазин" />

    <section class="premium">
      <div class="premium-copy">
        <p class="eyebrow">Подписка</p>
        <h1>Premium</h1>
        <p class="lead">
          Единый статус для игроков, которым нужны удобство, косметика и мягкая персонализация без влияния на честный прогресс.
        </p>
      </div>

      <ul class="premium-list">
        <li>Ускоренная прокачка бытовых навыков</li>
        <li>Кастомные сообщения о входе и выходе</li>
        <li>Косметические элементы для RP-образа</li>
      </ul>

      <button class="primary" type="button">Оформить Premium</button>
    </section>

    <section class="feature">
      <div class="feature-image">
        <img :src="buildKitImage" alt="Набор строителя" />
      </div>
      <div class="feature-copy">
        <p class="eyebrow">Набор</p>
        <h2>Набор строителя</h2>
        <p>
          Практичный комплект для тех, кто развивает постройки, интерьер и детали поселений. Хорошо смотрится как отдельный стартовый набор и как витрина магазина.
        </p>
      </div>
    </section>

    <section class="products">
      <h2>Товары</h2>
      <div class="grid">
        <article v-for="item in products" :key="item.title" class="product">
          <span class="product-type">{{ item.type }}</span>
          <h3>{{ item.title }}</h3>
          <p>{{ item.text }}</p>
        </article>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import buildKitImage from '~/images/shop/kit_build.png'

const products = [
  {
    type: 'Учебник',
    title: 'Академический учебник Воина',
    text: 'Материал для развития боевых навыков персонажа и уверенного участия в опасных RP-сценах.'
  },
  {
    type: 'Учебник',
    title: 'Академический учебник Кузнеца',
    text: 'Подходит для ремесленного прогресса, кузнечного RP и развития мастерской.'
  },
  {
    type: 'Учебник',
    title: 'Академический учебник Мага',
    text: 'Предмет для магической специализации, атмосферного отыгрыша и косметического образа.'
  },
  {
    type: 'Набор',
    title: 'Набор "Строителя"',
    text: 'Комплект для игроков, которые активно развивают постройки, декор и личные пространства.'
  }
]
</script>

<style scoped>
.page {
  display: grid;
  gap: 24px;
}

.premium,
.feature {
  border-radius: var(--radius-lg);
  border: 1px solid rgba(228, 94, 56, 0.28);
  box-shadow: var(--shadow);
  overflow: hidden;
}

.premium {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(260px, 0.8fr) auto;
  align-items: center;
  gap: 24px;
  padding: 28px;
  background: radial-gradient(circle at 12% 0%, rgba(228, 94, 56, 0.26), transparent 42%),
    linear-gradient(135deg, rgba(34, 35, 48, 0.98), rgba(18, 18, 24, 0.96));
}

.feature {
  display: grid;
  grid-template-columns: minmax(280px, 0.9fr) minmax(0, 1.1fr);
  min-height: 280px;
  background: linear-gradient(135deg, rgba(18, 18, 24, 0.96), rgba(34, 35, 48, 0.96));
}

.feature-image {
  min-height: 280px;
  background: #111118;
}

.feature-image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.feature-copy {
  display: grid;
  align-content: center;
  gap: 12px;
  padding: 28px;
}

.eyebrow {
  margin: 0;
  color: var(--accent);
  text-transform: uppercase;
  font-size: 12px;
  font-weight: 700;
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
.feature-copy p,
.product p {
  color: var(--muted);
  line-height: 1.6;
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

.primary {
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 10px 16px;
  cursor: pointer;
  font-weight: 600;
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
  align-content: start;
  gap: 10px;
  padding: 18px;
  border-radius: var(--radius-md);
  border: 1px solid var(--stroke);
  background: var(--panel);
  min-height: 178px;
}

.product-type {
  width: max-content;
  padding: 4px 8px;
  border-radius: 999px;
  color: var(--muted);
  border: 1px solid var(--stroke);
  font-size: 12px;
}

@media (max-width: 1100px) {
  .premium,
  .feature {
    grid-template-columns: 1fr;
  }

  .primary {
    width: max-content;
  }
}

@media (max-width: 640px) {
  .premium,
  .feature-copy {
    padding: 20px;
  }

  .feature-image {
    min-height: 220px;
  }

  h1 {
    font-size: 32px;
  }

  .primary {
    width: 100%;
  }
}
</style>
