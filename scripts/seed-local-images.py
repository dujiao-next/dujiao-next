#!/usr/bin/env python3
"""为本地测试补上商品主图与商品分类图片。

用途：
  1. 分类图片：categories.icon 目前存的是 emoji（🎮💻🎫👑📦），
     但前端是用 <img :src="getImageUrl(icon)"> 渲染的 —— emoji 会被拼成
     形如 "/🎮" 的 URL，结果 5 个分类图标全是破图。这里改成真实图片地址。
  2. 商品主图：products.images 目前全为空，前台卡片与详情页只显示占位图标。
     这里给每个商品补上图片，详情页的缩略图画廊也能生效。

图片直接用外部占位图服务，不落地文件：
  - 分类图：placehold.co，纯色块 + 英文短标签（该服务不支持中文，会显示 ??）
  - 商品图：picsum.photos，按 slug 取稳定的随机真实照片

⚠️ 外链意味着离线 / 断网时图片会全部失败。本地测试够用，
   如果要长期稳定的演示环境，应把这些图下载到 uploads/ 再改成 /uploads/ 路径。

约定（改动前请先读）：
  - categories.icon 与 products.images 都是 varchar/json 列，
    products.images 是 JSON 列，在 SQLite 里是 BLOB，写入必须传 bytes。
  - icon 的既有语义是"图片地址"（前端 ProductCard 会在商品无主图时回退到它做卡片图），
    emoji 属于历史遗留。脚本只替换"看起来不是图片地址"的值，不会覆盖已配置的图片。
  - 可重复执行：已是外链图片的会被跳过。

用法：
  python3 scripts/seed-local-images.py            # 预览
  python3 scripts/seed-local-images.py --apply    # 写入
"""

import argparse
import json
import sqlite3
import sys
from pathlib import Path

DB_PATH = Path(__file__).resolve().parent.parent / "db" / "dujiao.db"

# 分类图片：分类 id -> (slug, 背景色, 标签)。
# 色相区分是为了在 20x20 的侧边栏小图标上靠颜色就能辨认。
CATEGORY_IMAGES = {
    1: ("game-topup", "7C3AED", "GAME"),
    2: ("software-key", "2563EB", "SOFT"),
    3: ("digital-card", "EA580C", "CARD"),
    4: ("vip-service", "D97706", "VIP"),
    5: ("virtual-goods", "0891B2", "VIRT"),
}

# 商品图尺寸取 4:3，与前台卡片的 aspect-[4/3] 一致。
PRODUCT_IMAGE_SIZE = (800, 600)
IMAGES_PER_PRODUCT = 2  # 第 2 张用于详情页的缩略图画廊


def category_image_url(bg: str, label: str) -> str:
    return f"https://placehold.co/128x128/{bg}/FFFFFF/png?text={label}"


def product_image_urls(slug: str) -> list:
    w, h = PRODUCT_IMAGE_SIZE
    urls = [f"https://picsum.photos/seed/{slug}/{w}/{h}"]
    if IMAGES_PER_PRODUCT > 1:
        for i in range(2, IMAGES_PER_PRODUCT + 1):
            urls.append(f"https://picsum.photos/seed/{slug}-{i}/{w}/{h}")
    return urls


def is_image_ref(value) -> bool:
    """判断一个 icon 值是否已经是可用的图片地址（而不是 emoji 之类的占位）。"""
    if not value:
        return False
    text = str(value).strip()
    return text.startswith("/uploads/") or text.startswith("http://") or text.startswith("https://")


def json_bytes(value) -> bytes:
    """JSON 列在库里是 BLOB，必须写 bytes。"""
    return json.dumps(value, ensure_ascii=False).encode("utf-8")


def decode_json(raw):
    if raw is None:
        return None
    if isinstance(raw, (bytes, bytearray)):
        raw = raw.decode("utf-8")
    try:
        return json.loads(raw)
    except (ValueError, TypeError):
        return None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="真正写入数据库（默认只预览）")
    args = parser.parse_args()

    if not DB_PATH.exists():
        print(f"数据库不存在：{DB_PATH}", file=sys.stderr)
        return 1

    conn = sqlite3.connect(DB_PATH)
    conn.execute("PRAGMA busy_timeout = 10000")
    cur = conn.cursor()

    # ---------- 1. 分类图片 ----------
    print("[1/2] 分类图片")
    cur.execute("select id, slug, icon from categories where deleted_at is null order by id")
    cat_rows = cur.fetchall()
    cat_updated = 0
    cat_skipped = 0
    for cid, slug, icon in cat_rows:
        spec = CATEGORY_IMAGES.get(cid)
        if spec is None:
            print(f"      - id={cid} {slug} 跳过：脚本里没有配置该分类的图片")
            continue
        _, bg, label = spec
        url = category_image_url(bg, label)
        if is_image_ref(icon):
            print(f"      = id={cid} {slug:14} 已是图片地址，跳过（当前 {str(icon)[:40]}）")
            cat_skipped += 1
            continue
        print(f"      + id={cid} {slug:14} {str(icon):4} → {url}")
        if args.apply:
            cur.execute("update categories set icon = ? where id = ?", (url, cid))
        cat_updated += 1

    # ---------- 2. 商品主图 ----------
    print(f"\n[2/2] 商品主图（每个 {IMAGES_PER_PRODUCT} 张，{PRODUCT_IMAGE_SIZE[0]}x{PRODUCT_IMAGE_SIZE[1]}）")
    cur.execute(
        "select id, slug, images from products where deleted_at is null order by id"
    )
    prod_rows = cur.fetchall()
    prod_updated = 0
    prod_skipped = 0
    for pid, slug, images_raw in prod_rows:
        current = decode_json(images_raw)
        if isinstance(current, list) and current and all(is_image_ref(x) for x in current):
            prod_skipped += 1
            continue
        urls = product_image_urls(slug)
        print(f"      + id={pid:>3} {slug:30} → {len(urls)} 张")
        if args.apply:
            cur.execute("update products set images = ? where id = ?", (json_bytes(urls), pid))
        prod_updated += 1

    if prod_skipped:
        print(f"      （{prod_skipped} 个商品已有图片，跳过）")

    if args.apply:
        conn.commit()
        print(f"\n已提交：分类图片 {cat_updated} 个，商品主图 {prod_updated} 个")
    else:
        print(f"\n预览模式：将更新分类图片 {cat_updated} 个、商品主图 {prod_updated} 个。加 --apply 才会写入。")

    conn.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
