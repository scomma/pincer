# Shopee Driver

Package: `com.shopee.th`

Automates the Shopee e-commerce app (Thai locale). Currently implements read-only commands for viewing the shopping cart and searching for products.

## Commands

| Command | Description |
|---------|-------------|
| `pincer shopee cart list` | List cart items with shop, price, variation, quantity. |
| `pincer shopee search --query TEXT` | Search for products. |

## Screens

| Screen | Detection | Key indicators |
|--------|-----------|----------------|
| `HOME` | Main feed visible | `com.shopee.th:id/homepage_main_recycler` ID |
| `CART` | Cart page | "Shopping Cart" text |
| `ME` | Profile page | "Edit Profile" text |
| `SEARCH` | (declared, not yet detected) | — |
| `ORDERS` | (declared, not yet detected) | — |

## Element ID quirks

**Shopee uses bare resource IDs** — no package prefix. This is different from Grab and LINE.

```
Grab:   com.grabtaxi.passenger:id/duxton_card
LINE:   jp.naver.line.android:id/name
Shopee: labelItemName                          ← no prefix
```

When writing element queries for Shopee, use `core.HasID("labelItemName")`, not `core.HasID("com.shopee.th:id/labelItemName")`.

The one exception is `homepage_main_recycler`, which does use the full prefix: `com.shopee.th:id/homepage_main_recycler`.

Key cart element IDs (all bare):
- `labelShopName` — shop name header
- `labelItemName` — product name
- `labelVariation` — selected variant (e.g., "GRAPHITE", "Dock only")
- `labelPriceBeforeDiscount` — original price (strikethrough)
- `labelShopVoucherDiscountText` — voucher info
- `buttonEdit` — edit button per shop group

## Cart parsing logic

Cart items are associated to shops by **vertical position** — for each item, the parser finds the nearest `labelShopName` element above it (lower Y coordinate). This works because Shopee renders shop headers above their items in the DOM. If shop and item Y-ranges overlap during scroll animations, items could be mis-assigned.

Price detection: the first `฿`-prefixed text child that is not `labelPriceBeforeDiscount` is treated as the current price. Quantity is read from `EditText` elements.

## Fixture notes

- `home.xml` — Shopee home screen with product feed, flash sale banners, live streams, bottom nav bar. Contains synthetic product cards with prices and discount badges.
- `cart.xml` — Shopping cart with 3 items across 3 synthetic shops. Includes variations, discounted prices, and shop vouchers.
- `orders.xml` — Despite the filename, this is actually a **profile/edit screen** (captured from the Me tab). It now contains synthetic placeholder profile data. Not an orders list.

## Output examples

```bash
$ pincer shopee cart list
```
```json
{
  "ok": true,
  "data": {
    "items": [
      {
        "shop": "Northstar Supply",
        "name": "Northstar Travel Adapter 45W Multi-Port Charger",
        "variation": "GRAPHITE",
        "price": "฿1,290",
        "quantity": "1"
      },
      {
        "shop": "Orbit Labs",
        "name": "Orbit Magnetic Charging Dock 15W for Phone and Watch",
        "variation": "Dock only",
        "price": "฿490",
        "old_price": "฿748",
        "quantity": "1"
      }
    ],
    "count": 3
  }
}
```
