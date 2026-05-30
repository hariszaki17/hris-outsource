# PRD · F10.1 — Notifications & Notification Center

> **Epic:** E10 Reporting & Notifications · **Feature:** F10.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

The whole system raises events people must act on — a schedule changed, an approval is pending, a shift starts soon, an attendance exception appeared. A central notification service delivers these via **push (mobile)** and an **in-app center**, so agents and leaders aren't surprised. (Email/SMS/WhatsApp are out for v1.)

## 2. Goals & non-goals

**Goals**
- A single service other epics call to notify users (push + in-app).
- In-app notification center with read/unread.
- Light per-user preferences.

**Non-goals**
- Email/SMS/WhatsApp (not in v1). The events themselves (owned by E3–E8).

## 3. Actors

System (emit), Agent / Shift Leader / HR (recipients).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent / Leader | Push + in-app center. |
| **Web console** | Leader / HR | In-app center. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| NT-1 | Channels are **push + in-app only** (INV-1). |
| NT-2 | Notifications are **scoped to the recipient**; no cross-user leakage. |
| NT-3 | Event catalog (v1): schedule published/changed (E4), shift reminder (E4), approval requested/decided (leave E6, OT E7, attendance verification E5, corrections), attendance exception/auto-close (E5), placement/leader changes (E3). |
| NT-4 | Each notification has type, payload (deep-link target), and read/unread state. |
| NT-5 | Light **preferences**: users may mute non-critical categories; critical (approvals, schedule changes) stay on (confirm in §10). |
| NT-6 | Delivery is best-effort + retried; in-app center is the durable record if push fails. |

## 6. Data model

`Notification` (id, recipient_id, type, channel, payload, read_at, created_at); optional `NotificationPreference` (user_id, category, muted).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Notifications

  Scenario: Schedule change notifies the agent
    Given the leader changes Budi's shift
    Then Budi receives a push and an in-app notification with a deep link

  Scenario: Approval routing notification
    Given Budi submits a leave request
    Then his shift leader is notified there is an approval pending

  Scenario: In-app center is durable
    Given a push fails to deliver
    Then the notification still appears in Budi's in-app center

  Scenario: Mark read
    When Budi opens a notification
    Then it is marked read

  Scenario: Scope
    Then a user only sees notifications addressed to them
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Device has no push token | In-app only; prompt to enable push. |
| C-2 | Burst of events | Batched/grouped where sensible (e.g., bulk schedule publish). |
| C-3 | User muted a category but it's critical | Critical categories override mute (NT-5). |
| C-4 | Stale deep link (target deleted) | Graceful "no longer available". |

## 9. Dependencies

E3–E8 (event sources), push provider (FCM/APNs), E1 (identity/scope).

## 10. Decisions & open questions

- ✅ Push + in-app; scoped; durable in-app center.
- **Open:** which categories are mutable vs always-on?
- **Open:** push provider setup (FCM/APNs) + reminder lead times (with E4/E5).
