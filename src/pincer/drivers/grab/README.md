# Grab Driver

Package: `com.grabtaxi.passenger`

Automates the Grab app for food delivery, transport, and payments. Currently implements read-only commands for food search and authentication status.

## Commands

| Command | Description |
|---------|-------------|
| `pincer grab food search [--query TEXT]` | List nearby restaurants. Optional search query. |
| `pincer grab auth status` | Check if logged in. Returns current screen name. |

## Screens

| Screen | Detection | Key indicators |
|--------|-----------|----------------|
| `HOME` | Service tiles visible, no search bar | `Food, double tap to select` content-desc + `Transport, double tap to select` |
| `FOOD_HOME` | Search bar visible, no restaurant cards | `search_bar_clickable_area` ID present |
| `FOOD_RESULTS` | Restaurant feed visible | `duxton_card` or `feedList` IDs |
| `FOOD_SEARCH` | Search overlay visible | `duxton_search_bar` ID |
| `LOGIN_GUEST` | Guest login/signup bottom sheet | `simple_guest_login_view_login` / `simple_guest_login_view_signup` IDs or "Let's get you in!" text |
| `LOGIN_PHONE` | Phone input | "Continue With Mobile Number" text |
| `LOGIN_OTP` | OTP input | "Enter the 6-digit code" text |
| `LOGIN_PIN` | PIN input | "Enter your PIN" text |

## Element ID quirks

- Resource IDs use the full package prefix: `com.grabtaxi.passenger:id/duxton_card`
- The food tile is most reliably found via `content-desc="Food, double tap to select"` rather than matching the "Food" text, because the text element is not clickable — the parent `LinearLayout` with the content-desc is.
- Restaurant cards use `duxton_card` on the feed and `horizontal_merchant_card` on some search-result layouts. These are Grab internal component names, not semantic identifiers.
- Restaurant names and promos are child TextViews with no distinguishing resource IDs. The parser scores candidate text for likely restaurant names, skips UI labels like `Ad` / `Only at Grab`, and treats promo-like text via regex (`off|free|deal|promo|discount|%|฿`).

## Fixture notes

- `home.xml` — Grab home with the service tiles visible.
- `food_home.xml` — Grab Food landing screen with the search bar visible before opening search.
- `food_results.xml` — Restaurant feed with `duxton_card` results.
- `food_search.xml` — Search overlay with `duxton_search_bar` visible.

The search command scrolls and de-duplicates restaurants by name, so a single run can return more than the initially visible page of results.

## Output examples

```bash
$ pincer grab food search
```
```json
{
  "ok": true,
  "data": {
    "restaurants": [
      {
        "name": "Harbor Wok - Market Pier",
        "promo": "Up to 40% off"
      },
      {
        "name": "Noodle Project - Central Hub",
        "promo": "20% off"
      }
    ]
  }
}
```

```bash
$ pincer grab auth status
```
```json
{
  "ok": true,
  "data": {
    "logged_in": false,
    "screen": "LOGIN_GUEST"
  }
}
```
