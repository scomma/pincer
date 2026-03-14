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
| `FOOD_RESULTS` | Search bar + restaurant cards | `search_bar_clickable_area` + `duxton_card` IDs |
| `LOGIN_PHONE` | Phone input | "Continue With Mobile Number" text |
| `LOGIN_OTP` | OTP input | "Enter the 6-digit code" text |
| `LOGIN_PIN` | PIN input | "Enter your PIN" text |

## Element ID quirks

- Resource IDs use the full package prefix: `com.grabtaxi.passenger:id/duxton_card`
- The food tile is most reliably found via `content-desc="Food, double tap to select"` rather than matching the "Food" text, because the text element is not clickable — the parent `LinearLayout` with the content-desc is.
- Restaurant cards use `duxton_card` as the resource ID (Grab's internal component name). This is not a meaningful name — it's what Grab ships.
- Restaurant names and promos are child TextViews inside `duxton_card` with no distinguishing resource IDs. The parser identifies them by position: first text = name, promo matched by regex (`off|free|deal|%|฿`).

## Fixture notes

All four Grab fixtures (`home.xml`, `food_home.xml`, `food_results.xml`, `food_search.xml`) were captured while the food home screen was visible. They all contain both the service tiles (Food/Transport/Mart) and the food search bar + restaurant cards. The "pure" Grab home screen (before tapping Food) is not yet captured as a fixture.

The screen detection logic differentiates them by checking for `search_bar_clickable_area` first (which rules out a pure home screen), then `duxton_card` to distinguish FOOD_HOME from FOOD_RESULTS.

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
    "logged_in": true,
    "screen": "FOOD_RESULTS"
  }
}
```
