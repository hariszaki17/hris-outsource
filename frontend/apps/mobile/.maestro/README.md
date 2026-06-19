# Maestro E2E tests (mobile)

Black-box E2E for the agent/shift-leader mobile app. Flows are plain YAML under
`flows/`; they drive the **installed app** on a running simulator/emulator and
assert against `testID`s + on-screen text.

## Install Maestro (one-time)

```bash
curl -fsSL "https://get.maestro.mobile.dev" | bash
# adds ~/.maestro/bin to PATH — restart shell or: export PATH="$HOME/.maestro/bin:$PATH"
maestro --version
```

## Prerequisites for a green run

The login flow does a **real** auth round-trip — there is no mock server. You need:

1. **Backend up** — Go API reachable at `EXPO_PUBLIC_API_BASE_URL` (`.env`), with the
   seed personas loaded (`backend/cmd/seed`). Default flow logs in as the agent
   persona `agent.budi@swp.test`. `.env` must point at the host's **current LAN IP**
   (the iOS sim resolves it) — stale IP = login hangs.
2. **App installed** on a booted simulator/emulator:
   ```bash
   pnpm ios        # or: pnpm android  (builds the dev client and installs it)
   ```
   The flows target the dev-build bundle id `com.hariszaki17.swphris`.
3. **Metro running** (`pnpm start`) — the dev build loads its JS bundle from Metro.
   The flows handle the Expo dev-launcher + first-run dev-menu automatically (the
   `when: "Development Build"` block), so `clearState: true` still works on a dev build.

## Run

```bash
pnpm e2e            # all flows in .maestro/
pnpm e2e:login      # just the happy-path login
pnpm e2e:studio     # interactive flow builder + view-hierarchy inspector
```

## Flows

| File | Asserts |
|------|---------|
| `flows/login.yaml` | valid creds → lands on Absen home (`clock-action` visible) |
| `flows/login-invalid.yaml` | wrong password → error banner, stays on login |

## Notes

- **App id** is **hardcoded** in each flow's `appId:` — Maestro does not interpolate
  env vars in that field. For Expo Go, change it to `host.exp.Exponent`. (The `pnpm e2e*`
  scripts still pass `MAESTRO_APP_ID`, but only flows that reference it via a command would
  use it; the `appId:` header is literal.)
- **Credentials** override per-run: `maestro test -e EMAIL=… -e PASSWORD=… .maestro/flows/login.yaml`.
- **Text matchers use Bahasa** — it is the i18n default (`src/lib/i18n.ts`). `login.yaml`
  asserts only `testID`s (locale-proof); `login-invalid.yaml` asserts the Bahasa banner
  "Email atau kata sandi salah". Change the device locale → update those strings.
- Adding a new flow? Anchor on `testID`s, not layout/copy — copy changes break text matchers.
- **Verified green** 2026-06-19 on iPhone 17 Pro sim (iOS 26.5): both flows pass.
