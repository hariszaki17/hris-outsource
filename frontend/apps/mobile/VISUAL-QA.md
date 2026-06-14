# Mobile Visual QA — run the app on an iOS simulator + screenshot screens

Runbook for visually auditing mobile screens against `docs/design/brainstorm.pen`.
Copy-paste friendly. Written from a real session that screenshotted all 8 E1 auth states.

> TL;DR: **Use the iOS Simulator, not web.** Web is broken for this app (see Gotchas).
> Boot sim → start Expo in **Go** mode → install Expo Go → open screens via deep links →
> `simctl io booted screenshot`. A one-time dev-menu overlay must be clicked away with a
> Swift CGEvent helper (no `idb`/`xdotool` here).

---

## 0. Why not web / why not `expo run:ios`

- **`expo start --web` does NOT work.** Two hard stops:
  1. NativeWind defaults `darkMode` to `media` → throws *"Cannot manually set color scheme"*.
     (Worked around with `darkMode: 'class'` in `tailwind.config.ts`, but…)
  2. `expo-secure-store` has **no web implementation** → session bootstrap
     (`src/lib/auth.ts`) crashes with *"getValueWithKeyAsync is not a function"*.
  So web layout never renders. Don't burn time there.
- **`expo run:ios` works but is slow** (CocoaPods + xcodebuild, several minutes, first run).
  The project has `expo-dev-client` + native `ios/` dirs, so this is the "correct" runtime —
  but for **screenshotting JS/layout** the auth/profile screens only use Expo-Go-compatible
  natives (fonts, secure-store, router), so **Expo Go is the fast path**. Use it.

---

## 1. Boot the iOS simulator

```bash
# List available iPhones, pick one
xcrun simctl list devices available | grep -iE "iphone (16|17)"

# Boot it (example: iPhone 17) + open the Simulator window
xcrun simctl boot "iPhone 17" 2>/dev/null; open -a Simulator
# wait until booted
until xcrun simctl list devices booted | grep -q iPhone; do sleep 1; done
```

## 2. Start Metro in Expo Go mode (background)

```bash
cd frontend/apps/mobile
# --go forces Expo Go (project has expo-dev-client which would otherwise want a dev build)
# --ios runs the same "press i" logic: auto-open + try to install Expo Go
npx expo start --go --ios --port 8082 > /tmp/expo-go.log 2>&1 &
# wait for Metro
until grep -q "Waiting on http" /tmp/expo-go.log; do sleep 2; done
```

(Port 8082 is arbitrary; default is 8081. Stick to one and reuse it in deep links below.)

## 3. Install Expo Go into the sim

`expo start --ios` tries to fetch Expo Go automatically, but the auto-fetch **often stalls**
("- Fetching Expo Go" with no progress). If it hasn't installed within ~1 min, install it
manually (much more reliable):

```bash
# Find the Expo Go iOS-simulator build URL for THIS project's SDK (check app SDK first)
curl -s https://api.expo.dev/v2/versions/latest \
 | node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>{const v=JSON.parse(s).sdkVersions;const k=Object.keys(v).find(x=>x.startsWith('56'));console.log(v[k].iosClientUrl)})"
# → e.g. https://github.com/expo/expo-go-releases/releases/download/Expo-Go-56.0.3/Expo-Go-56.0.3.tar.gz

# Download (it's ~77 MB and GitHub throttles to ~5 MB/min — be patient), extract, install
cd /tmp && curl -L "<iosClientUrl from above>" -o expogo.tar.gz
gzip -t expogo.tar.gz && echo OK    # verify complete before extracting
mkdir -p eg && tar xzf expogo.tar.gz -C eg
xcrun simctl install booted "/tmp/eg/Expo Go.app"   # note the SPACE in the name — quote it

# verify
xcrun simctl listapps booted | grep -iq host.exp.Exponent && echo "Expo Go installed"
```

## 4. Open a screen via deep link

Routes are `expo-router` paths under the app. Deep-link form: `exp://127.0.0.1:<port>/--/<route>`.

```bash
xcrun simctl openurl booted "exp://127.0.0.1:8082/--/login"
# wait for "iOS Bundled" in the log on first open (~10s), then it's instant
```

Query params work too (e.g. driving states — see §7):
`exp://127.0.0.1:8082/--/login?err=locked`

## 5. Dismiss the one-time Expo Go dev-menu overlay

On first launch (and after each cold relaunch) Expo Go shows a **"This is the developer menu …
Continue"** sheet covering the app. There's no `idb`/tap CLI here, so click it with a Swift
CGEvent helper.

Create the helper once:

```bash
cat > /tmp/click.swift <<'EOF'
import CoreGraphics
import Foundation
let x = Double(CommandLine.arguments[1])!, y = Double(CommandLine.arguments[2])!
let p = CGPoint(x: x, y: y)
let src = CGEventSource(stateID: .hidSystemState)
CGEvent(mouseEventSource: src, mouseType: .leftMouseDown, mouseCursorPosition: p, mouseButton: .left)?.post(tap: .cghidEventTap)
usleep(80000)
CGEvent(mouseEventSource: src, mouseType: .leftMouseUp, mouseCursorPosition: p, mouseButton: .left)?.post(tap: .cghidEventTap)
EOF
```

Get the Simulator window rect, then click "Continue" (~50% width, ~93% height), then the
dev-menu's **X** (~93% width, ~51% height) if the full menu opened:

```bash
# window: "x, y, w, h"
osascript -e 'tell application "Simulator" to activate' -e 'delay 0.4' \
  -e 'tell application "System Events" to tell process "Simulator" to get position of window 1 & size of window 1'
# Example output: 565, 47, 396, 852  → click Continue:
osascript -e 'tell application "Simulator" to activate' >/dev/null; sleep 0.3
swift /tmp/click.swift 763 839    # x=565+0.5*396, y=47+0.93*852
# if the full Reload/Tools menu appears, close via its X:
swift /tmp/click.swift 933 481    # x=565+0.93*396, y=47+0.51*852
```

Once dismissed, the app screen is clean (a small floating **"Tools" gear** stays top-right —
that's Expo Go chrome, ignore it; it's not part of the app).

## 6. Screenshot

```bash
xcrun simctl io booted screenshot /tmp/shot.png
# iPhone 17 captures at 1206x2622 px (3x → 402x874 pt). Then Read /tmp/shot.png in-session.
```

Helper to navigate + shoot in one go:

```bash
cap(){ xcrun simctl openurl booted "exp://127.0.0.1:8082/--/$1" >/dev/null 2>&1; sleep 3; \
       xcrun simctl io booted screenshot "/tmp/ios-$2.png" >/dev/null 2>&1 && echo "$2 ok"; }
cap "login"            "1-login"
cap "forgot-password"  "2-forgot"
cap "reset-sent"       "3-sent"
# …etc
```

## 7. Gotcha: component state sticks across deep-link param changes

Deep-linking to a route that's **already mounted** reuses the same component instance —
`useState` initializers do **not** re-run. So `/login` after `/login?err=disabled` still shows
the old error. Two fixes:

- **Route through a neutral screen** between captures to force unmount/remount:
  `cap forgot-password …; cap login …`. (Sometimes still reuses the stack entry — see below.)
- **More reliable for param-driven states:** temporarily seed state from the search param AND
  re-sync it with `useEffect(() => setState(fromParam), [param])`, screenshot, then **revert
  the scaffold**. (That's how the 3 login error states were captured. Remember to delete it.)

## 8. Cleanup

```bash
lsof -tiTCP:8082 -sTCP:LISTEN | xargs kill 2>/dev/null   # stop Metro
xcrun simctl shutdown booted                             # stop the sim (optional)
```

---

### Quick reference — the deep-link routes that exist today

Auth (`app/(auth)/`): `login`, `forgot-password`, `reset-sent`, `reset-success`, `reset-password`
App (`app/(app)/`): `index`, `schedule`, `attendance`, `leave`, `more`, `notifications`,
`profile`, `leader-beranda`, `sl-verifikasi`
Modals/details (`app/`): `attendance-history`, `correction`, `correction-detail`,
`correction-tracker`, `leave-new`, `overtime`, `overtime-new`, `payslip`,
`profile-change-request`, `profile-status`

> Source of truth for design = `docs/design/brainstorm.pen` (Pencil MCP only — never Read/Grep
> the `.pen`). Mobile screens live under the `PLATFORM · MOBILE` board (`yPwPD`), in role lanes
> (Agent `AikTF`, Shift Leader `Iavxr`, Auth/Shared `R8uJX`). See `docs/design/DESIGN-SYSTEM.md`.
