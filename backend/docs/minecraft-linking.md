# Minecraft Account Linking

The Minecraft server talks to the website API with `X-Server-Token` set to `MINECRAFT_SERVER_TOKEN`.

## Generate a verification code

`POST /api/minecraft/verification-code`

```json
{
  "nickname": "PlayerName"
}
```

The backend looks for an accepted RP application with this nickname, creates or reuses a short-lived verification code, and stores it in PostgreSQL.

## Verify on the website

The player signs in through Discord and submits the code to:

`POST /api/auth/verify-minecraft`

```json
{
  "code": "A4K8M2Q9"
}
```

After a successful match, the backend writes `linked_minecraft` and `minecraft_verified_at` into `discord_users`.

## Sync RP name from the server

`POST /api/minecraft/rp-name`

```json
{
  "nickname": "PlayerName",
  "firstName": "Amy",
  "lastName": "Stone"
}
```

The backend updates the linked Discord user in PostgreSQL.
