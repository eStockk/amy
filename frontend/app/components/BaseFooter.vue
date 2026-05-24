<template>
  <footer class="site-footer">
    <div class="footer-inner">
      <section v-if="showDiscordInvite" class="discord-invite" aria-label="Discord сервер Amy">
        <div class="discord-mark">
          <img :src="discordIcon" alt="" />
        </div>
        <div class="discord-copy">
          <span class="eyebrow">Discord server</span>
          <h2>Amy Community</h2>
          <p>Новости, быстрые ответы поддержки, обсуждения и живое сообщество сервера.</p>
        </div>
        <a class="discord-button" href="https://discord.gg/wQc3kAZJWB" target="_blank" rel="noreferrer">
          Присоединиться
        </a>
      </section>

      <div class="footer-about">
        <div>
          <h3>Играй в Minecraft на сервере Amy</h3>
          <p>
            Приватный сервер с кастомным выживанием, самописными системами,
            голосовым чатом и спокойным сообществом.
          </p>
        </div>
        <span class="copyright">© 2023-2026 Комплекс игровых серверов Minecraft “Amy”.</span>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import discordIcon from '~/assets/discord.png'
import { useAuth } from '~/composables/useAuth'

const { authenticated, user } = useAuth()
const showDiscordInvite = computed(() => !authenticated.value || !user.value?.isAmyDiscordMember)
</script>

<style scoped>
.site-footer {
  width: 100%;
  padding: 0 24px 28px;
  color: var(--muted);
}

.footer-inner {
  display: grid;
  gap: 20px;
  width: 100%;
  margin: 0 auto;
  padding: 28px;
  border-top: 1px solid var(--stroke);
  background: rgba(18, 18, 22, 0.9);
  box-shadow: 0 -20px 50px rgba(0, 0, 0, 0.18);
}

.discord-invite {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 18px;
  padding: 18px;
  border-radius: var(--radius-md);
  border: 1px solid rgba(88, 101, 242, 0.42);
  background:
    linear-gradient(135deg, rgba(88, 101, 242, 0.34), rgba(35, 39, 42, 0.78)),
    rgba(35, 39, 42, 0.92);
}

.discord-mark {
  width: 58px;
  height: 58px;
  border-radius: 16px;
  display: grid;
  place-items: center;
  background: #5865f2;
  box-shadow: 0 16px 32px rgba(88, 101, 242, 0.32);
}

.discord-mark img {
  width: 34px;
  height: 34px;
  object-fit: contain;
}

.discord-copy {
  min-width: 0;
}

.eyebrow {
  display: block;
  margin-bottom: 4px;
  color: rgba(255, 255, 255, 0.68);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}

.discord-copy h2,
.footer-about h3 {
  margin: 0;
  color: var(--text);
}

.discord-copy p,
.footer-about p {
  margin: 6px 0 0;
  line-height: 1.55;
}

.discord-button {
  justify-self: end;
  padding: 11px 18px;
  border-radius: 999px;
  background: #fff;
  color: #23272a;
  font-weight: 800;
  text-decoration: none;
  white-space: nowrap;
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.28);
  transition: transform 160ms ease, box-shadow 160ms ease;
}

.discord-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 18px 34px rgba(0, 0, 0, 0.34);
}

.footer-about {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
}

.footer-about p {
  max-width: 720px;
}

.copyright {
  flex: 0 0 auto;
  font-size: 12px;
  text-align: right;
}

@media (max-width: 760px) {
  .site-footer {
    padding: 0 16px 24px;
  }

  .footer-inner {
    padding: 18px;
  }

  .discord-invite {
    grid-template-columns: auto minmax(0, 1fr);
    gap: 14px;
  }

  .discord-button {
    grid-column: 1 / -1;
    justify-self: stretch;
    text-align: center;
  }

  .footer-about {
    display: grid;
    gap: 14px;
  }

  .copyright {
    text-align: left;
  }
}

@media (max-width: 420px) {
  .discord-invite {
    grid-template-columns: 1fr;
  }

  .discord-mark {
    width: 52px;
    height: 52px;
  }
}
</style>
