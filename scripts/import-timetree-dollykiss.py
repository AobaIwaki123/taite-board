#!/usr/bin/env python3
import json
import re
import os
import sys

def parse_day_of_week(date_str):
    from datetime import datetime
    dt = datetime.strptime(date_str, "%Y-%m-%d")
    days = ["月", "火", "水", "木", "金", "土", "日"]
    # Check 11/3 (祝)
    if date_str == "2026-11-03":
        return "火祝"
    if date_str == "2026-11-23":
        return "月祝"
    return days[dt.weekday()]

def clean_title(title):
    t = title.strip()
    t = re.sub(r'^[❤️‍🔥\s]+', '', t)
    t = re.sub(r'[｢｣『』]', '', t)
    return t.strip()

def parse_event(ev):
    desc = ev.get("description", "")
    date_str = ev.get("startAt", "")[:10]
    if not date_str:
        return None

    raw_title = ev.get("title", "ライブ予定")
    title = clean_title(raw_title)

    # 1. Venue
    venue_name = "告知待ち"
    m_venue = re.search(r'📍([^\n\r]+)', desc)
    if m_venue:
        venue_name = m_venue.group(1).strip()
    elif "新橋" in title:
        venue_name = "ZEAL THEATER 新橋"
    elif "Spotify O-" in title:
        m = re.search(r'Spotify O-[a-zA-Z]+', title)
        if m:
            venue_name = m.group(0)

    # 2. Live time
    live_start = "--:--"
    live_end = "--:--"
    m_live = re.search(r'🎤\s*(\d{1,2}:\d{2})\s*[-~〜]\s*(\d{1,2}:\d{2})', desc)
    if m_live:
        live_start = m_live.group(1)
        live_end = m_live.group(2)

    # 3. Benefit time & booth
    benefit_start = "--:--"
    benefit_end = "--:--"
    benefit_type = "未定" if live_start == "--:--" else "並行"
    benefit_booth = "告知待ち" if live_start == "--:--" else "通常"

    m_benefit = re.search(r'📸\s*(\d{1,2}:\d{2})\s*[-~〜]\s*(\d{1,2}:\d{2})(?:\s+([^\n\r]+))?', desc)
    if m_benefit:
        benefit_start = m_benefit.group(1)
        benefit_end = m_benefit.group(2)
        if m_benefit.group(3):
            benefit_booth = m_benefit.group(3).strip()

    # 4. Admission bonus
    bonus = ""
    m_bonus = re.search(r'🎁\s*([^\n\r]+)', desc)
    if m_bonus:
        raw_b = m_bonus.group(1).strip()
        raw_b = raw_b.replace("\u2795", "+")
        raw_b = re.sub(r'[\U00010000-\U0010ffff\u2600-\u27ff\ufe0f]', '', raw_b).strip()
        bonus = f"指名入場特典：{raw_b}"

    # 2本回し特典 check
    if "2本回し特典" in desc:
        m_2hon = re.search(r'2本回し特典[^\n\r]*\n[^\n\r]*\n*([^\n\r]+)', desc)
        if m_2hon and "券" in m_2hon.group(1):
            raw_2hon = m_2hon.group(1).strip().replace("\u2795", "+")
            raw_2hon = re.sub(r'[\U00010000-\U0010ffff\u2600-\u27ff\ufe0f]', '', raw_2hon).strip()
            bonus += f"（2本回し特典：{raw_2hon}）"

    # 5. Ticket URL and platform
    ticket_url = ev.get("url") or ""
    m_ticket = re.search(r'🎫\s*([^\s\n\r]+)', desc)
    if m_ticket:
        raw_turl = m_ticket.group(1).strip()
        if not raw_turl.startswith("http"):
            raw_turl = "https://" + raw_turl
        if not ticket_url or "timetree" in ticket_url:
            ticket_url = raw_turl

    platform = "公式アナウンス待ち"
    if "ticketdive" in ticket_url or "t-dv.com" in ticket_url:
        platform = "TicketDive"
    elif "livepocket" in ticket_url:
        platform = "LivePocket"
    elif "eplus" in ticket_url:
        platform = "イープラス"
    elif "atjam" in ticket_url:
        platform = "atjam"
    elif "ticketvillage" in ticket_url:
        platform = "TicketVillage"

    # 6. Ticket prices
    prices = []
    m_price = re.search(r'料金[：:]\s*([^\n\r]+)', desc)
    if m_price:
        line = m_price.group(1).strip()
        parts = re.findall(r'([^/／]+(?:¥|円|\d+円|\d+00円)[^/／]*)', line)
        for part in parts:
            p_clean = part.strip().rstrip("（別途1D）").strip()
            prices.append({"tier": p_clean, "price": p_clean})

    # 7. Image
    image_url = ""
    images = ev.get("imageUrls", [])
    if images and len(images) > 0:
        image_url = images[0]

    return {
        "title": title,
        "date": date_str,
        "day_of_week": parse_day_of_week(date_str),
        "venue": {
            "name": venue_name,
            "map_query": venue_name,
            "map_url": f"https://maps.google.com/?q={venue_name}" if venue_name != "告知待ち" else None
        },
        "live": {
            "start_time": live_start,
            "end_time": live_end
        },
        "benefit": {
            "type": benefit_type,
            "start_time": benefit_start,
            "end_time": benefit_end,
            "booth": benefit_booth
        },
        "admission_bonus": bonus if bonus else None,
        "ticket": {
            "platform": platform,
            "url": ticket_url if ticket_url else None,
            "prices": prices if prices else None
        },
        "image_url": image_url if image_url else None,
        "source_timetree_id": ev.get("id")
    }

def main():
    json_path = "data/candidate/dollykiss_timetree_202611.json"
    if not os.path.exists(json_path):
        print(f"File not found: {json_path}")
        sys.exit(1)

    with open(json_path, "r", encoding="utf-8") as f:
        data = json.load(f)

    # Read existing schedule to preserve verified confirmed sessions
    existing_sched_path = "data/groups/dollykiss/schedule.json"
    existing_sessions = {}
    if os.path.exists(existing_sched_path):
        with open(existing_sched_path, "r", encoding="utf-8") as f:
            ex_data = json.load(f)
            for s in ex_data.get("sessions", []):
                key = (s.get("date"), clean_title(s.get("title", "")))
                existing_sessions[key] = s

    sessions = []
    # Sort events by date
    raw_events = data.get("events", [])
    raw_events.sort(key=lambda x: (x.get("startAt", ""), x.get("title", "")))

    date_counters = {}
    for ev in raw_events:
        parsed = parse_event(ev)
        if not parsed:
            continue
        # Skip events without meaningful title
        if parsed["title"] in ["ライブ予定", "ライブ"]:
            # Check if there is details in desc
            if parsed["venue"]["name"] == "告知待ち" and parsed["admission_bonus"] is None:
                continue

        key = (parsed["date"], parsed["title"])
        if key in existing_sessions:
            # Keep confirmed session with its rich data
            sessions.append(existing_sessions[key])
        else:
            d = parsed["date"].replace("-", "")
            date_counters[d] = date_counters.get(d, 0) + 1
            parsed["id"] = f"dollykiss-{d}-{date_counters[d]}"
            sessions.append(parsed)

    # Assign IDs to existing if missing
    for idx, s in enumerate(sessions):
        if "id" not in s:
            d = s["date"].replace("-", "")
            s["id"] = f"dollykiss-{d}-{idx+1}"

    # Strip any emojis from admission_bonus across all sessions
    def strip_emoji(text):
        if not text:
            return text
        t = text.replace("\u2795", "+").replace("➕", "+")
        t = re.sub(r'[\U00010000-\U0010ffff\u2600-\u27bf\ufe0f]', '', t)
        return t.strip()

    for s in sessions:
        if s.get("admission_bonus"):
            s["admission_bonus"] = strip_emoji(s["admission_bonus"])
        if s.get("special_notes"):
            s["special_notes"] = strip_emoji(s["special_notes"])

    from datetime import datetime, timezone as dt_tz
    doc = {
        "schema_version": 1,
        "generated_at": datetime.now(dt_tz.utc).isoformat(),
        "timezone": "Asia/Tokyo",
        "target_group": {
            "name": "Dolly Kiss",
            "short_name": "ドリキス",
            "official_x": "https://x.com/DollyKiss_staff",
            "canonical_id": "group:dollykiss"
        },
        "sessions": sessions
    }

    # Write back formatted JSON
    for p in [
        "data/groups/dollykiss/schedule.json",
        "apps/web/public/data/groups/dollykiss/schedule.json",
        "apps/web/dist/data/groups/dollykiss/schedule.json"
    ]:
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as f:
            json.dump(doc, f, ensure_ascii=False, indent=2)
        print(f"Wrote {len(sessions)} sessions to {p}")

if __name__ == "__main__":
    main()
