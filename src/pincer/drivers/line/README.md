# LINE Driver

Package: `jp.naver.line.android`

Automates the LINE messaging app. Currently implements read-only commands for listing chats and reading messages.

## Commands

| Command | Description |
|---------|-------------|
| `pincer line chat list [--unread] [--limit N]` | List chats with last message, unread count, member count. |
| `pincer line chat read --chat NAME [--limit N]` | Open a chat and read messages. Exact name match required. |

## Screens

| Screen | Detection | Key indicators |
|--------|-----------|----------------|
| `CHATS` | Chat list visible | `chat_list_recycler_view` ID |
| `CHAT_DETAIL` | Message composer or history visible | `chathistory_message_edit_text`, `chat_ui_message_edit`, or `chathistory_message_list` IDs |

## Element ID quirks

- Resource IDs use the full package prefix: `jp.naver.line.android:id/name`
- Chat list items are direct children of the `chat_list_recycler_view` RecyclerView.
- Each chat item has well-labeled children with stable IDs:
  - `name` — chat/group name
  - `last_message` — message preview
  - `date` — timestamp
  - `unread_message_count` — unread badge (regular chats)
  - `square_chat_unread_message_count` — unread badge (OpenChat groups, different ID)
  - `member_count` — group size, formatted as `(N)`
  - `notification_off` — present (as an element) when the chat is muted, even with no text content
- Unread counts can show `"999+"`. The `+` is stripped before `Atoi` parsing, so `999+` becomes `999`.

## Chat read behavior

- Chat read uses **exact name match only**. The `--chat` flag must match the chat name as it appears in `pincer line chat list`. No fuzzy or substring matching, to prevent opening the wrong conversation.
- After tapping a chat, the driver waits 2 seconds for the detail screen to load, then parses message elements. The `Message` struct supports `sender`, `text`, and `time` fields, though only `text` is reliably populated in the current implementation.

## Fixture notes

- `chats.xml` — The chat list tab with 11 visible chats, mix of 1:1 and group chats, Thai and English text. Includes muted chats, OpenChat groups, and chats with `999+` unread.
- `chat_detail.xml` — A chat detail view. Large file (~27k tokens). Contains message bubbles but LINE's message resource IDs vary across versions.

## Output examples

```bash
$ pincer line chat list --unread --limit 3
```
```json
{
  "ok": true,
  "data": {
    "chats": [
      {
        "name": "Project Atlas",
        "last_message": "Most devices here use the standard adapter setup.",
        "time": "6:51 PM",
        "unread_count": 154,
        "member_count": 239,
        "muted": true
      },
      {
        "name": "Building Notices",
        "last_message": "Notice: the shuttle route will use a temporary vehicle...",
        "time": "6:08 PM",
        "unread_count": 4,
        "member_count": 62,
        "muted": true
      },
      {
        "name": "Parents Circle July",
        "last_message": "We saw the same thing earlier, but it cleared up quickly...",
        "time": "5:01 PM",
        "unread_count": 775,
        "member_count": 721,
        "muted": true
      }
    ]
  }
}
```

```bash
$ pincer line chat read --chat "Project Atlas" --limit 2
```
```json
{
  "ok": true,
  "data": {
    "chat_name": "Project Atlas",
    "messages": [
      {
        "text": "Can you send the adapter model number again?"
      },
      {
        "text": "Most devices here use the standard adapter setup."
      }
    ]
  }
}
```
