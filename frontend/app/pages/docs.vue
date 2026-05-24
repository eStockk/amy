<template>
  <div class="page">
    <SectionTitle text="Документация" />

    <ContentPanel
      class="image-panel image-panel-connect"
      title="Как подключиться"
      text="IP сервера, базовая настройка клиента и первые шаги после входа."
      :style="{ '--panel-image': `url(${docsImage})` }"
    >
      <div class="note-grid">
        <span>Зайдите на сервер и получите код привязки.</span>
        <span>Авторизуйтесь через Discord на сайте.</span>
        <span>Введите код в профиле, чтобы связать Minecraft и Discord.</span>
      </div>
    </ContentPanel>

    <ContentPanel
      class="image-panel image-panel-craft"
      title="Прокачка ремесла"
      text="В мире можно найти книги обучения. Они дают очки, которые тратятся на Алтаре Просвещения."
      :style="{ '--panel-image': `url(${craftImage})` }"
    >
      <div class="doc-list">
        <div>
          <b>Руководство для чайников</b>
          <span>Даёт от 10% до 30% очков обучения.</span>
        </div>
        <div>
          <b>Продвинутое руководство</b>
          <span>Даёт от 30% до 70% очков обучения.</span>
        </div>
        <div>
          <b>Академический учебник</b>
          <span>Даёт от 50% до 130% очков обучения.</span>
        </div>
      </div>
    </ContentPanel>

    <ContentPanel
      class="image-panel image-panel-runes"
      title="Система рун"
      text="Статуи в мире могут дропнуть неактивную руну. Её нужно активировать на Алтаре Рун с помощью Аркания."
      :style="{ '--panel-image': `url(${runesImage})` }"
    >
      <div class="doc-list">
        <div>
          <b>Активация</b>
          <span>Положите неактивную руну в первый слот, Арканий во второй и нажмите «Совместить».</span>
        </div>
        <div>
          <b>Установка</b>
          <span>Поместите активную руну в оружие. Для использования возьмите оружие в руку и нажмите F.</span>
        </div>
        <div>
          <b>Прокачка</b>
          <span>Используйте Маяк душ: 50 душ для второго уровня и 100 душ для третьего.</span>
        </div>
      </div>
    </ContentPanel>

    <ContentPanel
      title="Руны и различия по уровням"
      text="Каждая руна усиливается по мере прокачки. Эффекты ниже указаны в формате 1/2/3 уровень."
    >
      <div class="rune-grid">
        <div>
          <b>Бастион</b>
          <span>Игрок перестаёт получать урон, но не может двигаться 2/3/5 секунд.</span>
        </div>
        <div>
          <b>Ядовитый сад</b>
          <span>Создаёт поле 5 блоков с отравлением на 10 секунд с 1/2/3 уровнем.</span>
        </div>
        <div>
          <b>Кривое зеркало</b>
          <span>Даёт невидимость, а сущности вокруг получают замедление на 10/20/40 секунд.</span>
        </div>
        <div>
          <b>Огненный котёл</b>
          <span>Создаёт поле 5 блоков и поджигает врагов на 10/15/20 секунд.</span>
        </div>
        <div>
          <b>Притягивание</b>
          <span>Притягивает мобов в радиусе 5/10/15 блоков к игроку.</span>
        </div>
      </div>
    </ContentPanel>
  </div>
</template>

<script setup lang="ts">
import SectionTitle from '~/components/SectionTitle.vue'
import ContentPanel from '~/components/ContentPanel.vue'
import docsImage from '~/images/doc/doc1.png'
import craftImage from '~/images/doc/doc2.png'
import runesImage from '~/images/doc/doc3.png'
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.image-panel {
  min-height: 320px;
  align-content: end;
  overflow: hidden;
  background-image: linear-gradient(90deg, rgba(20, 20, 27, 0.96), rgba(20, 20, 27, 0.76) 52%, rgba(20, 20, 27, 0.28)),
    var(--panel-image);
  background-position: center, center;
  background-size: cover, cover;
  box-shadow: var(--shadow);
}

.image-panel-craft {
  background-position: center, center 45%;
}

.image-panel-runes {
  background-position: center, center;
}

.note-grid,
.doc-list,
.rune-grid {
  display: grid;
  gap: 10px;
}

.note-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.note-grid span,
.doc-list div,
.rune-grid div {
  padding: 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(10, 10, 14, 0.56);
}

.doc-list div,
.rune-grid div {
  display: grid;
  gap: 4px;
}

.doc-list span,
.rune-grid span {
  color: var(--muted);
}

.rune-grid {
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

@media (max-width: 760px) {
  .note-grid {
    grid-template-columns: 1fr;
  }

  .image-panel {
    min-height: 360px;
    background-image: linear-gradient(180deg, rgba(20, 20, 27, 0.62), rgba(20, 20, 27, 0.96)),
      var(--panel-image);
  }
}
</style>
