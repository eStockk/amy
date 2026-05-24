<template>
  <div class="page">
    <SectionTitle text="Правила сервера" />

    <ContentPanel
      class="image-panel"
      title="Общие правила"
      text="Нарушения ниже относятся к Minecraft-серверу и сообществу Amy: Restore. Администрация оценивает ситуацию по контексту и истории игрока."
      :style="{ '--panel-image': `url(${rulesImage})` }"
    >
      <div class="rules-list">
        <article v-for="rule in rules" :key="rule.number" class="rule">
          <span class="rule-number">{{ rule.number }}</span>
          <div>
            <h3>{{ rule.title }}</h3>
            <p>{{ rule.penalty }}</p>
          </div>
        </article>
      </div>
    </ContentPanel>

    <ContentPanel
      title="Пояснение"
      text="Если правило кажется спорным, сначала уточните у администрации. Попытки обхода наказаний и повторные нарушения считаются отягчающими обстоятельствами."
    />
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import ContentPanel from '~/components/ContentPanel.vue'
import rulesImage from '~/images/rules/rules1.png'

const rules = [
  {
    number: '1',
    title: 'Запрещено намеренно нарушать правила',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '2',
    title: 'Запрещена продажа и покупка товаров или услуг за реальные средства',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '3',
    title: 'Запрещены действия, направленные на возбуждение ненависти либо вражды',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '4',
    title: 'Запрещён доксинг',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '5',
    title: 'Запрещён мультиаккаунт',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '6',
    title: 'Запрещена реклама сторонних ресурсов без одобрения администрации',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '7',
    title: 'Запрещено гриферство, обман на ресурсы и дюп в любых проявлениях',
    penalty: 'Наказание: бан до 14 дней. За повторное нарушение: перманентный бан.'
  },
  {
    number: '8',
    title: 'Запрещены политические высказывания, связанные с реальной жизнью',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '9',
    title: 'Запрещено использовать модификации и ПО, облегчающие игру',
    penalty: 'Наказание: перманентный бан.'
  },
  {
    number: '10',
    title: 'Запрещено использовать баги в своих целях',
    penalty: 'Наказание определяется администрацией по тяжести нарушения.'
  }
]
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.image-panel {
  min-height: 360px;
  align-content: start;
  overflow: hidden;
  background-image: linear-gradient(90deg, rgba(20, 20, 27, 0.98), rgba(20, 20, 27, 0.86) 58%, rgba(20, 20, 27, 0.34)),
    var(--panel-image);
  background-position: center, center;
  background-size: cover, cover;
  box-shadow: var(--shadow);
}

.rules-list {
  display: grid;
  gap: 12px;
  margin-top: 4px;
}

.rule {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(10, 10, 14, 0.62);
}

.rule-number {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  color: var(--text);
  background: rgba(228, 94, 56, 0.28);
  border: 1px solid rgba(228, 94, 56, 0.34);
  font-weight: 700;
}

h3,
p {
  margin: 0;
}

h3 {
  font-size: 16px;
}

p {
  margin-top: 6px;
  color: #e9a39a;
  font-weight: 600;
}

@media (max-width: 640px) {
  .image-panel {
    background-image: linear-gradient(180deg, rgba(20, 20, 27, 0.8), rgba(20, 20, 27, 0.98)),
      var(--panel-image);
  }
}
</style>
