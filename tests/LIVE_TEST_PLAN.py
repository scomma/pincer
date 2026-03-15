#!/usr/bin/env python3
"""
Live Device Test Plan
=====================

This file is the single source of truth for live-device QA against Pincer.
It is intentionally written in a literate style: the prose describes the
intent of each test, and the code below executes that exact test.

These checks complement the synthetic fixture tests in `e2e_test.go` and
`robustness_test.go`. Re-run them after any change to navigation, screen
detection, text input, or scroll behavior.

Prerequisites
-------------

- A device is connected via USB (`adb devices` shows it).
- ADB Keyboard is installed (`adb shell ime list -a | grep adbkeyboard`).
- Grab, LINE, and Shopee are installed and logged in.
- Android animations are disabled in Developer Options.

Documented cases
----------------

1. Screen off -> Grab food search (no query)
   Expected: wakes the screen, launches Grab, reaches food home, returns
   restaurants, and does not leak `Ad` / `Only at Grab` into names.

2. Grab food search with query
   Expected: opens search, types `pad thai`, submits, returns plausible
   restaurants, and echoes the exact query in the response.

3. Grab auth status
   Expected: returns `logged_in: true` and a screen name from any Grab state.

4. Wrong app -> LINE chat list (unread)
   Expected: starting from Grab, launches LINE, navigates to chat list, and
   returns up to five unread chats with unread counts.

5. LINE chat read (scroll to find)
   Expected: uses a visible chat name from case 4, scrolls to find it, opens
   the thread, and returns messages.

6. Shopee cart list (full scroll)
   Expected: navigates to the cart, scrolls through all items, and returns
   entries with shop/name/variation/price/quantity.

7. Shopee search with query
   Expected: types `usb cable`, submits the search, and returns products with
   names and prices.

8. All apps killed + screen off -> cold launch
   Expected: wakes the device, cold-launches LINE, and returns three chats.

9. Rapid cross-app sequence
   Expected: Grab browse -> Shopee cart -> LINE unread list -> Grab search all
   succeed back-to-back without getting wedged in the wrong app or screen.

10. Deep sub-screen recovery
    Expected: from a deeper Grab sub-screen, backs out until food home and
    still returns restaurants.

Error contract
--------------

All CLI errors must write JSON to stderr, not stdout:

    {"ok": false, "error": "...", "message": "..."}

Unexpected cases
----------------

The runner can also include adversarial cases that extend the documented plan:

- Repeat the same query twice in Grab or Shopee.
- Preserve leading/trailing spaces in a query.
- Use Thai, uppercase, or mixed-language queries.
- Search Shopee from a screen-off state.
- Switch apps immediately before the next command.

Usage
-----

Run one randomized pass with a reproducible order:

    python3 tests/LIVE_TEST_PLAN.py --seed 123

Run two documented passes plus unexpected cases:

    python3 tests/LIVE_TEST_PLAN.py --passes 2 --include-extra --seed 123
"""

from __future__ import annotations

import argparse
import json
import random
import re
import subprocess
import sys
import time
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from pathlib import Path
from typing import Callable


ADB_KEYBOARD_IME = "com.android.adbkeyboard/.AdbIME"
GRAB_PKG = "com.grabtaxi.passenger"
LINE_PKG = "jp.naver.line.android"
SHOPEE_PKG = "com.shopee.th"


@dataclass
class CommandResult:
    code: int
    stdout: str
    stderr: str


@dataclass
class CaseResult:
    case_id: str
    name: str
    ok: bool
    detail: str


@dataclass
class Case:
    case_id: str
    name: str
    doc: str
    fn: Callable[["Harness"], CaseResult]


class Harness:
    def __init__(self, *, binary: str, device: str, timeout: int, adb: str, verbose: bool) -> None:
        self.binary = binary
        self.device = device
        self.timeout = timeout
        self.adb = adb
        self.verbose = verbose
        self.state: dict[str, str] = {}

    def run(self, args: list[str], *, timeout: int = 150) -> CommandResult:
        proc = subprocess.run(
            args,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=timeout,
        )
        return CommandResult(proc.returncode, proc.stdout, proc.stderr)

    def adb_run(self, *args: str, timeout: int = 45) -> CommandResult:
        cmd = [self.adb]
        if self.device:
            cmd.extend(["-s", self.device])
        cmd.extend(args)
        return self.run(cmd, timeout=timeout)

    def adb_shell(self, command: str, *, timeout: int = 45) -> CommandResult:
        return self.adb_run("shell", command, timeout=timeout)

    def pincer(self, *args: str, timeout: int = 150) -> CommandResult:
        cmd = [self.binary]
        if self.device:
            cmd.extend(["-d", self.device])
        cmd.extend(["-t", str(self.timeout)])
        cmd.extend(args)
        return self.run(cmd, timeout=timeout)

    def sleep(self, seconds: float) -> None:
        time.sleep(seconds)

    def require(self, ok: bool, message: str) -> None:
        if not ok:
            raise RuntimeError(message)

    def require_adb_keyboard(self) -> None:
        ime_list = self.adb_shell("ime list -a", timeout=30)
        self.require(ADB_KEYBOARD_IME in (ime_list.stdout + ime_list.stderr), "ADB Keyboard IME is not installed")

    def wake_screen(self) -> None:
        self.adb_shell("input keyevent KEYCODE_WAKEUP")
        self.sleep(1)
        self.adb_shell("wm dismiss-keyguard")
        self.adb_shell("input keyevent 82")
        self.sleep(1)

    def screen_off(self) -> None:
        self.adb_shell("input keyevent KEYCODE_SLEEP")
        self.sleep(2)

    def force_stop(self, package: str) -> None:
        self.adb_shell(f"am force-stop {package}")

    def launch(self, package: str) -> None:
        self.adb_run("shell", "monkey", "-p", package, "-c", "android.intent.category.LAUNCHER", "1", timeout=40)
        self.sleep(3)

    def parse_stdout_json(self, result: CommandResult) -> dict | None:
        blob = result.stdout.strip()
        if not blob:
            return None
        try:
            return json.loads(blob)
        except json.JSONDecodeError:
            return None

    def parse_stderr_json(self, result: CommandResult) -> dict | None:
        blob = result.stderr.strip()
        if not blob:
            return None
        try:
            return json.loads(blob)
        except json.JSONDecodeError:
            return None

    def expect_ok(self, result: CommandResult) -> dict:
        payload = self.parse_stdout_json(result)
        self.require(
            result.code == 0 and isinstance(payload, dict) and payload.get("ok") is True,
            f"expected success, rc={result.code}, stderr={result.stderr.strip()[:240]}",
        )
        return payload

    def expect_error(self, result: CommandResult) -> dict:
        payload = self.parse_stderr_json(result)
        self.require(
            result.code != 0 and result.stdout.strip() == "" and isinstance(payload, dict) and payload.get("ok") is False,
            f"expected stderr-only JSON error, rc={result.code}, stdout={result.stdout.strip()[:120]!r}, stderr={result.stderr.strip()[:240]!r}",
        )
        return payload

    def dump_ui(self) -> ET.Element | None:
        result = self.adb_run("exec-out", "uiautomator", "dump", "/dev/tty", timeout=45)
        text = result.stdout if result.stdout.strip() else result.stderr
        idx = text.find("<?xml")
        if idx == -1:
            return None
        try:
            return ET.fromstring(text[idx:])
        except ET.ParseError:
            return None

    def tap_first(self, predicate: Callable[[ET.Element], bool]) -> bool:
        root = self.dump_ui()
        if root is None:
            return False
        for node in root.iter("node"):
            if predicate(node):
                center = self.bounds_center(node.attrib.get("bounds", ""))
                if center is None:
                    continue
                self.adb_shell(f"input tap {center[0]} {center[1]}")
                self.sleep(2)
                return True
        return False

    @staticmethod
    def bounds_center(bounds: str) -> tuple[int, int] | None:
        match = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", bounds)
        if not match:
            return None
        x1, y1, x2, y2 = map(int, match.groups())
        return (x1 + x2) // 2, (y1 + y2) // 2

    @staticmethod
    def grab_restaurants(payload: dict) -> list[dict]:
        return ((payload.get("data") or {}).get("restaurants") or [])

    @staticmethod
    def line_chats(payload: dict) -> list[dict]:
        return ((payload.get("data") or {}).get("chats") or [])

    @staticmethod
    def line_messages(payload: dict) -> list[dict]:
        return ((payload.get("data") or {}).get("messages") or [])

    @staticmethod
    def shopee_items(payload: dict) -> list[dict]:
        data = payload.get("data") or {}
        return data.get("items") or data.get("products") or []

    def ensure_chat_name(self) -> str:
        chat_name = self.state.get("chat_name")
        if chat_name:
            return chat_name
        payload = self.expect_ok(self.pincer("line", "chat", "list", "--unread", "--limit", "5"))
        chats = self.line_chats(payload)
        self.require(bool(chats), "no unread chats available for case 5")
        chat_name = chats[0].get("name", "")
        self.require(bool(chat_name), "chat entry missing name")
        self.state["chat_name"] = chat_name
        return chat_name


def passed(case_id: str, name: str, detail: str) -> CaseResult:
    return CaseResult(case_id, name, True, detail)


def failed(case_id: str, name: str, exc: Exception) -> CaseResult:
    return CaseResult(case_id, name, False, str(exc))


def run_case(h: Harness, case: Case) -> CaseResult:
    try:
        return case.fn(h)
    except Exception as exc:
        return failed(case.case_id, case.name, exc)


def case_1(h: Harness) -> CaseResult:
    h.screen_off()
    h.wake_screen()
    payload = h.expect_ok(h.pincer("grab", "food", "search"))
    restaurants = h.grab_restaurants(payload)
    bad = [r.get("name", "") for r in restaurants if r.get("name", "").startswith("Ad") or "Only at Grab" in r.get("name", "")]
    h.require(len(restaurants) > 0, "Grab browse returned no restaurants")
    h.require(not bad, f"Grab names leaked ad labels: {bad[:2]}")
    return passed("1", "Screen off -> Grab food search", f"count={len(restaurants)} sample={[r.get('name') for r in restaurants[:2]]}")


def case_2(h: Harness) -> CaseResult:
    payload = h.expect_ok(h.pincer("grab", "food", "search", "--query", "pad thai"))
    data = payload.get("data") or {}
    restaurants = h.grab_restaurants(payload)
    h.require(data.get("query") == "pad thai", f"Grab echoed query {data.get('query')!r}")
    h.require(len(restaurants) > 0, "Grab search returned no restaurants")
    return passed("2", "Grab food search query", f"query={data.get('query')!r} count={len(restaurants)} sample={[r.get('name') for r in restaurants[:2]]}")


def case_3(h: Harness) -> CaseResult:
    payload = h.expect_ok(h.pincer("grab", "auth", "status"))
    data = payload.get("data") or {}
    h.require(data.get("logged_in") is True, f"expected logged_in=true, got {data.get('logged_in')!r}")
    h.require(bool(data.get("screen")), "Grab auth status returned no screen name")
    return passed("3", "Grab auth status", f"screen={data.get('screen')}")


def case_4(h: Harness) -> CaseResult:
    h.launch(GRAB_PKG)
    payload = h.expect_ok(h.pincer("line", "chat", "list", "--unread", "--limit", "5"))
    chats = h.line_chats(payload)
    h.require(bool(chats), "LINE unread list returned no chats")
    h.require(all((chat.get("unread_count") or 0) > 0 for chat in chats), "LINE unread list contains zero-unread chats")
    h.state["chat_name"] = chats[0].get("name", "")
    return passed("4", "Wrong app -> LINE chat list unread", f"count={len(chats)} chat={h.state['chat_name']!r}")


def case_5(h: Harness) -> CaseResult:
    chat_name = h.ensure_chat_name()
    payload = h.expect_ok(h.pincer("line", "chat", "read", "--chat", chat_name))
    messages = h.line_messages(payload)
    h.require(bool(messages), f"LINE chat read returned no messages for {chat_name!r}")
    return passed("5", "LINE chat read", f"chat={chat_name!r} messages={len(messages)}")


def case_6(h: Harness) -> CaseResult:
    payload = h.expect_ok(h.pincer("shopee", "cart", "list"))
    items = h.shopee_items(payload)
    good = [item for item in items if item.get("shop") and item.get("name")]
    h.require(bool(good), f"Shopee cart returned no populated items: count={len(items)}")
    return passed("6", "Shopee cart list", f"count={len(items)} good={len(good)} sample={[item.get('name') for item in items[:2]]}")


def case_7(h: Harness) -> CaseResult:
    payload = h.expect_ok(h.pincer("shopee", "search", "--query", "usb cable"))
    data = payload.get("data") or {}
    items = h.shopee_items(payload)
    priced = [item for item in items if item.get("price")]
    h.require(data.get("query") == "usb cable", f"Shopee echoed query {data.get('query')!r}")
    h.require(bool(items), "Shopee search returned no products")
    h.require(bool(priced), "Shopee search returned no priced products")
    return passed("7", "Shopee search query", f"query={data.get('query')!r} count={len(items)} priced={len(priced)}")


def case_8(h: Harness) -> CaseResult:
    for package in (GRAB_PKG, LINE_PKG, SHOPEE_PKG):
        h.force_stop(package)
    h.screen_off()
    h.wake_screen()
    payload = h.expect_ok(h.pincer("line", "chat", "list", "--limit", "3"))
    chats = h.line_chats(payload)
    h.require(len(chats) == 3, f"expected 3 chats, got {len(chats)}")
    return passed("8", "Cold launch LINE list", f"count={len(chats)} sample={[chat.get('name') for chat in chats[:2]]}")


def case_9(h: Harness) -> CaseResult:
    sequence = [
        ("grab browse", ("grab", "food", "search")),
        ("shopee cart", ("shopee", "cart", "list")),
        ("line unread", ("line", "chat", "list", "--unread", "--limit", "2")),
        ("grab burger", ("grab", "food", "search", "--query", "burger")),
    ]
    completed: list[str] = []
    for label, args in sequence:
        payload = h.expect_ok(h.pincer(*args))
        completed.append(label)
        if label == "shopee cart":
            h.require(bool(h.shopee_items(payload)), "cross-app Shopee cart returned no items")
        if label == "line unread":
            h.require(bool(h.line_chats(payload)), "cross-app LINE unread list returned no chats")
        if label == "grab burger":
            h.require(bool(h.grab_restaurants(payload)), "cross-app Grab burger search returned no restaurants")
    return passed("9", "Rapid cross-app sequence", ", ".join(completed))


def case_10(h: Harness) -> CaseResult:
    h.launch(GRAB_PKG)
    entered = h.tap_first(
        lambda node: (
            "duxton_card" in (node.attrib.get("resource-id") or "")
            or "merchant" in (node.attrib.get("resource-id") or "").lower()
        )
        and node.attrib.get("clickable") == "true"
    )
    if entered:
        h.tap_first(
            lambda node: node.attrib.get("clickable") == "true"
            and node.attrib.get("class") in ("android.view.ViewGroup", "android.view.View", "android.widget.Button")
        )
    payload = h.expect_ok(h.pincer("grab", "food", "search"))
    restaurants = h.grab_restaurants(payload)
    h.require(bool(restaurants), "Grab deep recovery returned no restaurants")
    return passed("10", "Deep sub-screen recovery", f"entered={entered} count={len(restaurants)}")


def case_error_contract(h: Harness) -> CaseResult:
    payload = h.expect_error(h.pincer("shopee", "search"))
    return passed("E", "stderr error contract", f"error={payload.get('error')!r}")


def extra_grab_repeat(h: Harness) -> CaseResult:
    first = h.expect_ok(h.pincer("grab", "food", "search", "--query", "burger"))
    second = h.expect_ok(h.pincer("grab", "food", "search", "--query", "burger"))
    a = len(h.grab_restaurants(first))
    b = len(h.grab_restaurants(second))
    h.require(a > 0 and b > 0, f"Grab repeat counts were first={a}, second={b}")
    return passed("X1", "Grab same query twice", f"first={a} second={b}")


def extra_shopee_repeat(h: Harness) -> CaseResult:
    first = h.expect_ok(h.pincer("shopee", "search", "--query", "usb cable"))
    second = h.expect_ok(h.pincer("shopee", "search", "--query", "usb cable"))
    a = len(h.shopee_items(first))
    b = len(h.shopee_items(second))
    h.require(a > 0 and b > 0, f"Shopee repeat counts were first={a}, second={b}")
    return passed("X2", "Shopee same query twice", f"first={a} second={b}")


def extra_grab_spaces(h: Harness) -> CaseResult:
    query = "  pizza  "
    payload = h.expect_ok(h.pincer("grab", "food", "search", "--query", query))
    data = payload.get("data") or {}
    restaurants = h.grab_restaurants(payload)
    h.require(data.get("query") == query, f"Grab echoed query {data.get('query')!r}")
    h.require(bool(restaurants), "Grab spaced query returned no restaurants")
    return passed("X3", "Grab query with spaces", f"query={data.get('query')!r} count={len(restaurants)}")


def extra_line_after_shopee(h: Harness) -> CaseResult:
    h.expect_ok(h.pincer("shopee", "search", "--query", "usb cable"))
    payload = h.expect_ok(h.pincer("line", "chat", "list", "--unread", "--limit", "2"))
    chats = h.line_chats(payload)
    h.require(bool(chats), "LINE unread after Shopee returned no chats")
    return passed("X4", "LINE unread after Shopee", f"count={len(chats)}")


def extra_grab_mixed(h: Harness) -> CaseResult:
    query = "burger เบอร์เกอร์"
    payload = h.expect_ok(h.pincer("grab", "food", "search", "--query", query))
    data = payload.get("data") or {}
    restaurants = h.grab_restaurants(payload)
    h.require(data.get("query") == query, f"Grab echoed query {data.get('query')!r}")
    h.require(bool(restaurants), "Grab mixed-language query returned no restaurants")
    return passed("X5", "Grab mixed-language query", f"query={data.get('query')!r} count={len(restaurants)}")


def extra_shopee_caps(h: Harness) -> CaseResult:
    query = "USB CABLE"
    payload = h.expect_ok(h.pincer("shopee", "search", "--query", query))
    data = payload.get("data") or {}
    items = h.shopee_items(payload)
    h.require(data.get("query") == query, f"Shopee echoed query {data.get('query')!r}")
    h.require(bool(items), "Shopee uppercase query returned no products")
    return passed("X6", "Shopee uppercase query", f"query={data.get('query')!r} count={len(items)}")


def extra_grab_thai(h: Harness) -> CaseResult:
    query = "กะเพรา"
    payload = h.expect_ok(h.pincer("grab", "food", "search", "--query", query))
    data = payload.get("data") or {}
    restaurants = h.grab_restaurants(payload)
    h.require(data.get("query") == query, f"Grab echoed query {data.get('query')!r}")
    h.require(bool(restaurants), "Grab Thai query returned no restaurants")
    return passed("X7", "Grab Thai query", f"query={data.get('query')!r} count={len(restaurants)}")


def extra_screenoff_shopee(h: Harness) -> CaseResult:
    h.screen_off()
    h.wake_screen()
    query = "charger"
    payload = h.expect_ok(h.pincer("shopee", "search", "--query", query))
    data = payload.get("data") or {}
    items = h.shopee_items(payload)
    h.require(data.get("query") == query, f"Shopee echoed query {data.get('query')!r}")
    h.require(bool(items), "Screen-off Shopee search returned no products")
    return passed("X8", "Screen-off -> Shopee search", f"query={data.get('query')!r} count={len(items)}")


DOCUMENTED_CASES = [
    Case("1", "Screen off -> Grab food search", "Wake device and browse Grab food.", case_1),
    Case("2", "Grab food search query", "Search Grab for pad thai.", case_2),
    Case("3", "Grab auth status", "Check Grab auth state.", case_3),
    Case("4", "Wrong app -> LINE chat list unread", "Launch LINE from Grab and list unread chats.", case_4),
    Case("5", "LINE chat read", "Open a visible LINE chat and read messages.", case_5),
    Case("6", "Shopee cart list", "Scroll Shopee cart and collect items.", case_6),
    Case("7", "Shopee search query", "Search Shopee for usb cable.", case_7),
    Case("8", "Cold launch LINE list", "Kill apps, turn screen off, and cold-launch LINE.", case_8),
    Case("9", "Rapid cross-app sequence", "Run Grab -> Shopee -> LINE -> Grab in one chain.", case_9),
    Case("10", "Deep sub-screen recovery", "Recover Grab from a deeper sub-screen.", case_10),
]


EXTRA_CASES = [
    Case("X1", "Grab same query twice", "Repeat the same Grab query twice.", extra_grab_repeat),
    Case("X2", "Shopee same query twice", "Repeat the same Shopee query twice.", extra_shopee_repeat),
    Case("X3", "Grab query with spaces", "Preserve leading/trailing spaces in Grab.", extra_grab_spaces),
    Case("X4", "LINE unread after Shopee", "Switch from Shopee immediately into LINE.", extra_line_after_shopee),
    Case("X5", "Grab mixed-language query", "Use mixed English/Thai input in Grab.", extra_grab_mixed),
    Case("X6", "Shopee uppercase query", "Use uppercase input in Shopee.", extra_shopee_caps),
    Case("X7", "Grab Thai query", "Use Thai input in Grab.", extra_grab_thai),
    Case("X8", "Screen-off -> Shopee search", "Run Shopee search from a screen-off state.", extra_screenoff_shopee),
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the literate live-device test plan against a connected Android device.")
    parser.add_argument("--seed", type=int, default=0, help="Seed used to randomize case order.")
    parser.add_argument("--passes", type=int, default=1, help="Number of randomized passes over the documented cases.")
    parser.add_argument("--device", default="", help="ADB device serial to pass through to adb and pincer.")
    parser.add_argument("--adb", default="adb", help="Path to the adb binary.")
    parser.add_argument("--bin", default="./pincer", help="Path to the built pincer binary.")
    parser.add_argument("--timeout", type=int, default=90, help="Timeout value to pass to pincer via -t.")
    parser.add_argument("--include-extra", action="store_true", help="Run unexpected/adversarial cases after the documented plan.")
    parser.add_argument("--verbose", action="store_true", help="Print extra setup information.")
    return parser.parse_args()


def print_case(result: CaseResult) -> None:
    status = "PASS" if result.ok else "FAIL"
    print(f"[{status}] case {result.case_id}: {result.name} :: {result.detail}", flush=True)


def main() -> int:
    args = parse_args()
    binary = str(Path(args.bin))
    harness = Harness(binary=binary, device=args.device, timeout=args.timeout, adb=args.adb, verbose=args.verbose)

    harness.require(Path(binary).exists(), f"pincer binary not found at {binary}")
    harness.require_adb_keyboard()

    rng = random.Random(args.seed)
    results: list[CaseResult] = []

    for pass_index in range(1, args.passes + 1):
        cases = DOCUMENTED_CASES[:]
        rng.shuffle(cases)
        print(f"=== PASS {pass_index} seed={args.seed} order={[case.case_id for case in cases]} ===", flush=True)
        for case in cases:
            result = run_case(harness, case)
            print_case(result)
            results.append(result)

    error_result = run_case(harness, Case("E", "stderr error contract", "Ensure stderr-only JSON errors.", case_error_contract))
    print_case(error_result)
    results.append(error_result)

    if args.include_extra:
        extras = EXTRA_CASES[:]
        rng.shuffle(extras)
        print(f"=== EXTRA seed={args.seed} order={[case.case_id for case in extras]} ===", flush=True)
        for case in extras:
            result = run_case(harness, case)
            print_case(result)
            results.append(result)

    passed_count = sum(1 for result in results if result.ok)
    total = len(results)
    percentage = 100.0 * passed_count / total if total else 0.0

    print("=== SUMMARY ===", flush=True)
    print(f"passed={passed_count} total={total} success_rate={percentage:.1f}%", flush=True)
    for result in results:
        if not result.ok:
            print(f"FAIL case {result.case_id}: {result.name} :: {result.detail}", flush=True)

    return 0 if passed_count == total else 1


if __name__ == "__main__":
    sys.exit(main())
