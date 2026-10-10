#!/usr/bin/env python3
"""为本地测试补全商品详情，并生成一批商品数据。

用途：
  1. 给现有商品补上 HTML 详情（content 字段此前为空）；
  2. 生成 50 个测试商品，随机分配到各个商品分类，同样带 HTML 详情。

约定（与线上数据保持一致，改动前请先读这段）：
  - content 是富文本 HTML（后台上架用 TipTap 富文本编辑器，前台用 v-html + prose 渲染），
    不是 Markdown。这里只用编辑器支持的标准标签：h3/h4/p/ul/ol/li/strong/blockquote。
    特别注意不要用 <table>：后台编辑器没有装 table 扩展，前台虽能显示，
    但在后台打开该商品再保存时表格会被静默丢弃。
  - products 与 product_skus 里的 JSON 列（title_json 等）在 SQLite 中是 BLOB。
    写入必须传 bytes，写 str 会被 GORM 的 Scan 静默读成 nil。
  - 每个商品至少配一个 DEFAULT SKU，否则前台无规格可选。
  - 库存用 fulfillment_type=manual + SKU 手工库存：auto 的商品若没有上游映射，
    前台会一律算作 0 库存并显示"售罄"，不适合做可购买的测试数据。
  - 脚本可重复执行：补详情只处理"没有详情"的商品；生成新商品按分类只补差额
    （以 tag=本地数据 计数），不会重复写入或越写越多。

用法：
  python3 scripts/seed-local-products.py            # 预览将要写入的内容
  python3 scripts/seed-local-products.py --apply    # 真正写入
"""

import argparse
import json
import random
import sqlite3
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

DB_PATH = Path(__file__).resolve().parent.parent / "db" / "dujiao.db"

# 分类 ID 与 slug 的对应关系来自 categories 表（本地环境固定 5 个一级分类）。
CATEGORY_SLUGS = {
    1: "game-topup",
    2: "software-key",
    3: "digital-card",
    4: "vip-service",
    5: "virtual-goods",
}

# 每个分类的详情模板。{name} 会替换成商品名，其余为固定文案。
# 纯 HTML，不使用任何 Markdown 语法。
DETAIL_TEMPLATES = {
    "game-topup": {
        "zh-CN": (
            "<h3>商品介绍</h3>"
            "<p><strong>{name}</strong> 为官方渠道直充，下单后由系统自动提交，无需等待人工处理。</p>"
            "<h4>购买流程</h4>"
            "<ol>"
            "<li>选择需要的面额并完成支付</li>"
            "<li>在订单页填写游戏账号与区服信息</li>"
            "<li>系统自动充值，通常 1-5 分钟到账</li>"
            "</ol>"
            "<h4>注意事项</h4>"
            "<ul>"
            "<li>请确认账号与区服填写正确，充值成功后不支持退款</li>"
            "<li>部分平台对单日充值次数有限制，建议分批下单</li>"
            "<li>如遇到账延迟，请联系在线客服并提供订单号</li>"
            "</ul>"
            "<blockquote><p>请勿将账号密码提供给任何人，包括自称客服的人员。</p></blockquote>"
        ),
        "zh-TW": (
            "<h3>商品介紹</h3>"
            "<p><strong>{name}</strong> 為官方渠道直充，下單後由系統自動提交，無需等待人工處理。</p>"
            "<h4>購買流程</h4>"
            "<ol>"
            "<li>選擇需要的面額並完成付款</li>"
            "<li>在訂單頁填寫遊戲帳號與伺服器資訊</li>"
            "<li>系統自動儲值，通常 1-5 分鐘到帳</li>"
            "</ol>"
            "<h4>注意事項</h4>"
            "<ul>"
            "<li>請確認帳號與伺服器填寫正確，儲值成功後不支援退款</li>"
            "<li>部分平台對單日儲值次數有限制，建議分批下單</li>"
            "<li>如遇到帳延遲，請聯繫線上客服並提供訂單編號</li>"
            "</ul>"
            "<blockquote><p>請勿將帳號密碼提供給任何人，包括自稱客服的人員。</p></blockquote>"
        ),
        "en-US": (
            "<h3>About this item</h3>"
            "<p><strong>{name}</strong> is topped up through official channels. "
            "The order is submitted automatically right after payment.</p>"
            "<h4>How it works</h4>"
            "<ol>"
            "<li>Pick the amount you need and complete the payment</li>"
            "<li>Fill in your game account and server on the order page</li>"
            "<li>We submit the top-up automatically; delivery usually takes 1-5 minutes</li>"
            "</ol>"
            "<h4>Before you buy</h4>"
            "<ul>"
            "<li>Double-check your account and server. Top-ups cannot be refunded once delivered</li>"
            "<li>Some platforms limit daily top-up count, so split large orders if needed</li>"
            "<li>If delivery is delayed, contact support with your order number</li>"
            "</ul>"
            "<blockquote><p>Never share your account password with anyone, "
            "including people claiming to be support staff.</p></blockquote>"
        ),
    },
    "software-key": {
        "zh-CN": (
            "<h3>商品介绍</h3>"
            "<p>本商品为 <strong>{name}</strong> 正版授权，一机一码，激活后长期有效。</p>"
            "<h4>交付方式</h4>"
            "<ul>"
            "<li>支付成功后自动发放激活码，可在订单详情查看</li>"
            "<li>支持官方渠道在线验证，非共享账号</li>"
            "</ul>"
            "<h4>激活步骤</h4>"
            "<ol>"
            "<li>打开软件，进入「激活 / 输入许可证」页面</li>"
            "<li>粘贴收到的激活码并确认</li>"
            "<li>激活成功后建议截图保存凭证</li>"
            "</ol>"
            "<h4>说明</h4>"
            "<ul>"
            "<li>激活码仅限单台设备使用，绑定后不支持更换设备</li>"
            "<li>建议在激活前完成系统重装，以免影响绑定</li>"
            "</ul>"
        ),
        "zh-TW": (
            "<h3>商品介紹</h3>"
            "<p>本商品為 <strong>{name}</strong> 正版授權，一機一碼，啟用後長期有效。</p>"
            "<h4>交付方式</h4>"
            "<ul>"
            "<li>付款成功後自動發放啟用碼，可在訂單詳情查看</li>"
            "<li>支援官方渠道線上驗證，非共享帳號</li>"
            "</ul>"
            "<h4>啟用步驟</h4>"
            "<ol>"
            "<li>開啟軟體，進入「啟用 / 輸入授權碼」頁面</li>"
            "<li>貼上收到的啟用碼並確認</li>"
            "<li>啟用成功後建議截圖保存憑證</li>"
            "</ol>"
            "<h4>說明</h4>"
            "<ul>"
            "<li>啟用碼僅限單台裝置使用，綁定後不支援更換裝置</li>"
            "<li>建議在啟用前完成系統重裝，以免影響綁定</li>"
            "</ul>"
        ),
        "en-US": (
            "<h3>About this item</h3>"
            "<p><strong>{name}</strong> is a genuine license key: one key per machine, "
            "valid permanently after activation.</p>"
            "<h4>Delivery</h4>"
            "<ul>"
            "<li>The key is issued automatically after payment and shown on your order page</li>"
            "<li>Verifiable through the official channel; this is not a shared account</li>"
            "</ul>"
            "<h4>Activation</h4>"
            "<ol>"
            "<li>Open the app and go to the activation or license entry screen</li>"
            "<li>Paste the key you received and confirm</li>"
            "<li>Take a screenshot of the confirmation for your records</li>"
            "</ol>"
            "<h4>Notes</h4>"
            "<ul>"
            "<li>Each key is bound to a single device and cannot be transferred afterwards</li>"
            "<li>Reinstall your system before activating if you plan to do so</li>"
            "</ul>"
        ),
    },
    "digital-card": {
        "zh-CN": (
            "<h3>商品介绍</h3>"
            "<p><strong>{name}</strong> 为正规渠道卡券，以卡密形式发放，可在官方渠道查验余额。</p>"
            "<h4>使用方式</h4>"
            "<ol>"
            "<li>支付完成后在订单详情页查看卡号与密码</li>"
            "<li>前往对应平台的「兑换 / 充值」入口输入卡密</li>"
            "<li>确认余额到账后即可使用</li>"
            "</ol>"
            "<h4>注意事项</h4>"
            "<ul>"
            "<li>卡密一经发放不支持退换，请确认平台后再购买</li>"
            "<li>卡片为虚拟商品，请勿将卡密泄露给他人</li>"
            "<li>部分卡片存在地区限制，请留意适用范围</li>"
            "</ul>"
        ),
        "zh-TW": (
            "<h3>商品介紹</h3>"
            "<p><strong>{name}</strong> 為正規渠道卡券，以卡密形式發放，可在官方渠道查驗餘額。</p>"
            "<h4>使用方式</h4>"
            "<ol>"
            "<li>付款完成後在訂單詳情頁查看卡號與密碼</li>"
            "<li>前往對應平台的「兌換 / 儲值」入口輸入卡密</li>"
            "<li>確認餘額到帳後即可使用</li>"
            "</ol>"
            "<h4>注意事項</h4>"
            "<ul>"
            "<li>卡密一經發放不支援退換，請確認平台後再購買</li>"
            "<li>卡片為虛擬商品，請勿將卡密洩露給他人</li>"
            "<li>部分卡片存在地區限制，請留意適用範圍</li>"
            "</ul>"
        ),
        "en-US": (
            "<h3>About this item</h3>"
            "<p><strong>{name}</strong> is sourced through official channels and delivered "
            "as a code. The balance can be verified on the official platform.</p>"
            "<h4>How to redeem</h4>"
            "<ol>"
            "<li>Find the card number and PIN on your order page after payment</li>"
            "<li>Go to the redeem or top-up page on the matching platform</li>"
            "<li>Enter the code and confirm the balance before use</li>"
            "</ol>"
            "<h4>Notes</h4>"
            "<ul>"
            "<li>Codes cannot be refunded or exchanged once issued; check the platform first</li>"
            "<li>This is a virtual item, so never share the code with others</li>"
            "<li>Some cards are region-restricted; check the scope of use</li>"
            "</ul>"
        ),
    },
    "vip-service": {
        "zh-CN": (
            "<h3>服务说明</h3>"
            "<p><strong>{name}</strong> 为官方渠道代充服务，开通后即可享受对应会员权益。</p>"
            "<h4>服务流程</h4>"
            "<ol>"
            "<li>下单后提交需要开通的账号</li>"
            "<li>工作人员核对账号并提交开通</li>"
            "<li>开通完成后会通过站内信通知</li>"
            "</ol>"
            "<h4>权益说明</h4>"
            "<ul>"
            "<li>会员有效期内可享受对应特权，到期自动失效</li>"
            "<li>支持续费叠加，剩余时长累计不丢失</li>"
            "</ul>"
            "<h4>温馨提示</h4>"
            "<ul>"
            "<li>请确保账号可正常登录，否则可能影响开通</li>"
            "<li>开通期间请勿修改密码或退出登录</li>"
            "</ul>"
        ),
        "zh-TW": (
            "<h3>服務說明</h3>"
            "<p><strong>{name}</strong> 為官方渠道代充服務，開通後即可享有對應會員權益。</p>"
            "<h4>服務流程</h4>"
            "<ol>"
            "<li>下單後提交需要開通的帳號</li>"
            "<li>工作人員核對帳號並提交開通</li>"
            "<li>開通完成後會透過站內信通知</li>"
            "</ol>"
            "<h4>權益說明</h4>"
            "<ul>"
            "<li>會員有效期內可享有對應特權，到期自動失效</li>"
            "<li>支援續費疊加，剩餘時長累計不遺失</li>"
            "</ul>"
            "<h4>溫馨提示</h4>"
            "<ul>"
            "<li>請確保帳號可正常登入，否則可能影響開通</li>"
            "<li>開通期間請勿修改密碼或登出</li>"
            "</ul>"
        ),
        "en-US": (
            "<h3>What this service is</h3>"
            "<p><strong>{name}</strong> is activated through the official channel. "
            "Your membership benefits start as soon as activation completes.</p>"
            "<h4>How it works</h4>"
            "<ol>"
            "<li>Place the order and submit the account to be activated</li>"
            "<li>Our staff verify the account and submit the activation</li>"
            "<li>You are notified in the message center once it is done</li>"
            "</ol>"
            "<h4>Benefits</h4>"
            "<ul>"
            "<li>Benefits apply for the whole membership period and expire automatically</li>"
            "<li>Renewals stack, and remaining time is never lost</li>"
            "</ul>"
            "<h4>Please note</h4>"
            "<ul>"
            "<li>Make sure the account can sign in normally, or activation may fail</li>"
            "<li>Do not change the password or sign out while activation is in progress</li>"
            "</ul>"
        ),
    },
    "virtual-goods": {
        "zh-CN": (
            "<h3>商品说明</h3>"
            "<p><strong>{name}</strong> 为虚拟服务，下单后按下单信息完成开通与配置。</p>"
            "<h4>开通流程</h4>"
            "<ol>"
            "<li>下单并补充所需的配置信息</li>"
            "<li>工作人员在 30 分钟内完成开通</li>"
            "<li>开通结果与使用方式会发送到订单详情</li>"
            "</ol>"
            "<h4>服务条款</h4>"
            "<ul>"
            "<li>虚拟服务开通后不支持退款</li>"
            "<li>请在使用前阅读随附的使用说明</li>"
            "<li>如遇故障可提交工单，我们将在工作时间响应</li>"
            "</ul>"
            "<h4>常见问题</h4>"
            "<ul>"
            "<li><strong>多久开通？</strong>一般 30 分钟内，高峰期可能延后。</li>"
            "<li><strong>能否更换？</strong>开通后不支持更换，请下单前确认。</li>"
            "</ul>"
        ),
        "zh-TW": (
            "<h3>商品說明</h3>"
            "<p><strong>{name}</strong> 為虛擬服務，下單後依訂單資訊完成開通與設定。</p>"
            "<h4>開通流程</h4>"
            "<ol>"
            "<li>下單並補充所需的設定資訊</li>"
            "<li>工作人員在 30 分鐘內完成開通</li>"
            "<li>開通結果與使用方式會發送到訂單詳情</li>"
            "</ol>"
            "<h4>服務條款</h4>"
            "<ul>"
            "<li>虛擬服務開通後不支援退款</li>"
            "<li>請在使用前閱讀隨附的使用說明</li>"
            "<li>如遇故障可提交工單，我們將在工作時間回應</li>"
            "</ul>"
            "<h4>常見問題</h4>"
            "<ul>"
            "<li><strong>多久開通？</strong>一般 30 分鐘內，高峰期可能延後。</li>"
            "<li><strong>能否更換？</strong>開通後不支援更換，請下單前確認。</li>"
            "</ul>"
        ),
        "en-US": (
            "<h3>About this item</h3>"
            "<p><strong>{name}</strong> is a virtual service. Provisioning is done from the "
            "details you submit with the order.</p>"
            "<h4>Provisioning</h4>"
            "<ol>"
            "<li>Place the order and fill in the required configuration</li>"
            "<li>Our staff provision the service within 30 minutes</li>"
            "<li>The result and usage instructions appear on your order page</li>"
            "</ol>"
            "<h4>Terms</h4>"
            "<ul>"
            "<li>Virtual services cannot be refunded once provisioned</li>"
            "<li>Please read the usage instructions before you start</li>"
            "<li>If something fails, open a ticket and we will respond during business hours</li>"
            "</ul>"
            "<h4>FAQ</h4>"
            "<ul>"
            "<li><strong>How long does it take?</strong> Usually within 30 minutes; longer at peak times.</li>"
            "<li><strong>Can it be changed?</strong> No, so please confirm before ordering.</li>"
            "</ul>"
        ),
    },
}

# 每个分类的候选商品：(slug, zh-CN, zh-TW, en-US, 价格, 标签)
PRODUCT_POOL = {
    1: [
        ("hok-100-points", "王者荣耀 100点券", "王者榮耀 100點券", "Honor of Kings 100 Points", 1000, ["热销", "秒到"]),
        ("hok-500-points", "王者荣耀 500点券", "王者榮耀 500點券", "Honor of Kings 500 Points", 5000, ["热销"]),
        ("pubg-uc-100", "和平精英 100UC", "和平精英 100UC", "PUBG Mobile 100 UC", 1200, ["秒到"]),
        ("genshin-1980", "原神 1980创世结晶", "原神 1980創世結晶", "Genshin Impact 1980 Crystals", 19800, ["热销", "折扣"]),
        ("starrail-3280", "崩坏星穹铁道 3280古老梦华", "崩壞星穹鐵道 3280古老夢華", "Honkai: Star Rail 3280 Oneiric Shards", 32800, ["新品"]),
        ("naraka-1000", "永劫无间 1000金块", "永劫無間 1000金塊", "Naraka: Bladepoint 1000 Gold", 10000, ["折扣"]),
        ("lol-500-points", "英雄联盟 500点券", "英雄聯盟 500點券", "League of Legends 500 RP", 5000, ["热销"]),
        ("cf-1000-points", "穿越火线 1000CF点", "穿越火線 1000CF點", "CrossFire 1000 CF Points", 10000, []),
        ("mlbb-500-diamonds", "决胜巅峰 500钻石", "決勝巔峰 500鑽石", "Mobile Legends 500 Diamonds", 5000, ["秒到"]),
        ("valorant-1000-vp", "无畏契约 1000VP", "無畏契約 1000VP", "VALORANT 1000 VP", 10000, ["热销"]),
        ("dota2-1000-tokens", "DOTA2 1000勇士积分", "DOTA2 1000勇士積分", "DOTA2 1000 Battle Points", 10000, []),
        ("gbf-3000-crystals", "碧蓝幻想 3000宝晶石", "碧藍幻想 3000寶晶石", "Granblue Fantasy 3000 Crystals", 30000, ["限量"]),
    ],
    2: [
        ("office2021-home-key", "Office 2021 家庭版", "Office 2021 家用版", "Office 2021 Home", 19900, ["正版", "推荐"]),
        ("win10-pro-key", "Windows 10 Pro 激活码", "Windows 10 Pro 啟用碼", "Windows 10 Pro Key", 12800, ["正版"]),
        ("adobe-ai-annual", "Adobe Illustrator 年费", "Adobe Illustrator 年費", "Adobe Illustrator Annual", 98800, ["精选"]),
        ("jetbrains-idea-personal", "JetBrains IDEA 个人版", "JetBrains IDEA 個人版", "JetBrains IDEA Personal", 69900, ["开发者"]),
        ("sketch-annual", "Sketch 年度授权", "Sketch 年度授權", "Sketch Annual License", 79900, ["设计"]),
        ("cleanmymac-x-yearly", "CleanMyMac X 一年版", "CleanMyMac X 一年版", "CleanMyMac X 1 Year", 19900, ["推荐"]),
        ("idm-lifetime", "IDM 终身授权", "IDM 終身授權", "Internet Download Manager Lifetime", 12900, ["热销"]),
        ("capture-one-annual", "Capture One 年度订阅", "Capture One 年度訂閱", "Capture One Annual", 108800, ["摄影"]),
        ("davinci-studio", "DaVinci Resolve Studio", "DaVinci Resolve Studio", "DaVinci Resolve Studio", 158800, ["剪辑"]),
        ("matlab-student", "MATLAB 学生版", "MATLAB 學生版", "MATLAB Student", 49900, ["教育"]),
        ("parallels-desktop-standard", "Parallels Desktop 标准版", "Parallels Desktop 標準版", "Parallels Desktop Standard", 39900, ["推荐"]),
        ("notion-plus-yearly", "Notion Plus 年费", "Notion Plus 年費", "Notion Plus Annual", 58000, ["效率"]),
    ],
    3: [
        ("steam-gift-20", "Steam 礼品卡 $20", "Steam 禮品卡 $20", "Steam Gift Card $20", 14500, ["热销"]),
        ("steam-gift-100", "Steam 礼品卡 $100", "Steam 禮品卡 $100", "Steam Gift Card $100", 72000, ["热销"]),
        ("apple-gift-50", "Apple 充值卡 ¥50", "Apple 儲值卡 ¥50", "Apple Gift Card ¥50", 5000, ["秒到"]),
        ("google-play-25", "Google Play 礼品卡 $25", "Google Play 禮品卡 $25", "Google Play Gift Card $25", 18000, []),
        ("psn-50", "PSN 点卡 $50", "PSN 點數卡 $50", "PlayStation Network Card $50", 36000, ["热销"]),
        ("nintendo-30", "任天堂 eShop 卡 $30", "任天堂 eShop 卡 $30", "Nintendo eShop Card $30", 22000, []),
        ("jd-ecard-100", "京东 E 卡 ¥100", "京東 E 卡 ¥100", "JD.com E-Card ¥100", 9800, ["国内"]),
        ("amazon-gift-50", "亚马逊礼品卡 $50", "亞馬遜禮品卡 $50", "Amazon Gift Card $50", 36000, []),
        ("spotify-gift-60", "Spotify 礼品卡 $60", "Spotify 禮品卡 $60", "Spotify Gift Card $60", 42000, ["音乐"]),
        ("xbox-gamepass-3m", "Xbox Game Pass 3个月", "Xbox Game Pass 3個月", "Xbox Game Pass 3 Months", 45000, ["热销"]),
        ("razer-gold-20", "Razer Gold $20", "Razer Gold $20", "Razer Gold $20", 14500, ["游戏"]),
        ("itunes-25", "iTunes 礼品卡 $25", "iTunes 禮品卡 $25", "iTunes Gift Card $25", 18000, []),
    ],
    4: [
        ("iqiyi-gold-yearly", "爱奇艺黄金会员年卡", "愛奇藝黃金會員年卡", "iQIYI Gold Annual", 19800, ["热销"]),
        ("tencent-video-vip-yearly", "腾讯视频 VIP 年卡", "騰訊視頻 VIP 年卡", "Tencent Video VIP Annual", 19800, ["热销"]),
        ("youku-yearly", "优酷酷喵年卡", "優酷酷喵年卡", "Youku Annual", 18800, []),
        ("bilibili-premium-yearly", "哔哩哔哩大会员年卡", "嗶哩嗶哩大會員年卡", "Bilibili Premium Annual", 14800, ["热销"]),
        ("netflix-3month", "Netflix 季卡", "Netflix 季卡", "Netflix 3 Months", 7500, ["影视"]),
        ("spotify-premium-yearly", "Spotify 年度会员", "Spotify 年度會員", "Spotify Premium Annual", 39800, ["音乐"]),
        ("qqmusic-green-yearly", "QQ音乐绿钻年卡", "QQ音樂綠鑽年卡", "QQ Music Green Annual", 11800, []),
        ("netease-music-black-yearly", "网易云音乐黑胶年卡", "網易雲音樂黑膠年卡", "NetEase Cloud Music Annual", 10800, []),
        ("baidu-netdisk-super-yearly", "百度网盘超级会员年卡", "百度網盤超級會員年卡", "Baidu Netdisk Super Annual", 16800, ["热销"]),
        ("quark-svip-yearly", "夸克网盘 SVIP 年卡", "夸克網盤 SVIP 年卡", "Quark Drive SVIP Annual", 15800, []),
        ("wps-super-yearly", "WPS 超级会员年卡", "WPS 超級會員年卡", "WPS Super Annual", 8900, ["办公"]),
        ("adobe-cc-monthly", "Adobe CC 全家桶月费", "Adobe CC 全家桶月費", "Adobe CC All Apps Monthly", 32800, ["设计"]),
    ],
    5: [
        ("domain-net-first-year", "域名注册 .net 首年", "網域註冊 .net 首年", ".net Domain Registration", 6800, ["建站"]),
        ("ssl-dv-single-yearly", "DV 单域名 SSL 证书", "DV 單域名 SSL 憑證", "DV Single Domain SSL", 9900, ["安全"]),
        ("vps-2c4g-monthly", "云服务器 2核4G 月付", "雲端主機 2核4G 月付", "VPS 2C4G Monthly", 5900, ["热销"]),
        ("sms-package-5000", "短信包 5000条", "簡訊包 5000條", "SMS Package 5000", 22000, []),
        ("enterprise-smtp-100k", "企业邮箱 SMTP 10万封", "企業信箱 SMTP 10萬封", "Enterprise SMTP 100K", 45000, ["企业"]),
        ("cdn-traffic-500g", "CDN 流量包 500G", "CDN 流量包 500G", "CDN Traffic 500GB", 42900, ["加速"]),
        ("object-storage-1tb-yearly", "对象存储 1TB 年付", "物件儲存 1TB 年付", "Object Storage 1TB", 39900, []),
        ("api-gateway-pro-monthly", "API 网关专业版月付", "API 閘道專業版月付", "API Gateway Pro Monthly", 29900, ["开发"]),
        ("apm-pro-yearly", "应用监控专业版年付", "應用監控專業版年付", "APM Pro Annual", 128800, ["企业"]),
        ("cloud-backup-500g", "云备份服务 500G", "雲端備份服務 500G", "Cloud Backup 500GB", 19800, []),
        ("gpu-instance-a10-monthly", "GPU 实例 A10 月付", "GPU 執行個體 A10 月付", "GPU Instance A10 Monthly", 158000, ["算力"]),
        ("smart-dns-pro-yearly", "智能 DNS 专业版年付", "智慧 DNS 專業版年付", "Smart DNS Pro Annual", 8900, []),
    ],
}

NEW_PRODUCT_COUNT = 50
NEW_TAG = "本地数据"  # 标记新生成的商品，便于日后清理


def json_bytes(value) -> bytes:
    """JSON 列在库里是 BLOB，必须写 bytes；写 str 会被 Scan 读成 nil。"""
    return json.dumps(value, ensure_ascii=False).encode("utf-8")


def is_blank_content(raw) -> bool:
    """判断 content_json 是否"没有详情"。

    后台保存过一次空富文本的商品，content_json 会是
    {"en-US":"","zh-CN":"","zh-TW":""} 这样的占位对象——它既不是 NULL 也不是空串，
    但同样没有内容，需要一并补上。
    """
    if raw is None:
        return True
    if isinstance(raw, (bytes, bytearray)):
        try:
            raw = raw.decode("utf-8")
        except UnicodeDecodeError:
            return False
    text = raw.strip()
    if text in ("", "{}", "[]"):
        return True
    try:
        obj = json.loads(text)
    except (ValueError, TypeError):
        return False
    if isinstance(obj, dict):
        return not any(str(value or "").strip() for value in obj.values())
    if isinstance(obj, list):
        return len(obj) == 0
    if isinstance(obj, str):
        return not obj.strip()
    return False


def localized(zh_cn: str, zh_tw: str, en_us: str) -> dict:
    return {"zh-CN": zh_cn, "zh-TW": zh_tw, "en-US": en_us}


def pick_counts(total: int) -> list:
    """把总数分到 5 个分类，每个 8-12 个，且带随机抖动。"""
    counts = [total // 5] * 5
    remainder = total - sum(counts)
    counts[0] += remainder
    for _ in range(random.randint(4, 10)):
        i, j = random.sample(range(5), 2)
        if counts[i] > 8 and counts[j] < 12:
            counts[i] -= 1
            counts[j] += 1
    return counts


def random_created_at(days_back: int = 45) -> str:
    delta = timedelta(
        days=random.randint(0, days_back),
        hours=random.randint(0, 23),
        minutes=random.randint(0, 59),
    )
    return (datetime.now(timezone.utc) - delta).strftime("%Y-%m-%d %H:%M:%S.%f+00:00")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="真正写入数据库（默认只预览）")
    parser.add_argument("--seed", type=int, default=None, help="随机种子，便于复现同一批数据")
    args = parser.parse_args()

    if args.seed is not None:
        random.seed(args.seed)

    if not DB_PATH.exists():
        print(f"数据库不存在：{DB_PATH}", file=sys.stderr)
        return 1

    conn = sqlite3.connect(DB_PATH)
    conn.execute("PRAGMA busy_timeout = 10000")
    cur = conn.cursor()

    # ---------- 1. 现有商品补详情 ----------
    # content_json 为 NULL、空串，或三语言全空的占位对象，都算"没有详情"。
    cur.execute(
        "select id, slug, category_id, title_json, content_json from products "
        "where deleted_at is null order by id"
    )
    pending = [row for row in cur.fetchall() if is_blank_content(row[4])]
    print(f"[1/2] 现有商品补详情：待处理 {len(pending)} 个")
    existing_filled = 0
    for pid, slug, category_id, title_raw, _ in pending:
        cat_slug = CATEGORY_SLUGS.get(category_id)
        if cat_slug is None:
            print(f"      - id={pid} {slug} 跳过：未知分类 {category_id}")
            continue
        title = json.loads(title_raw.decode("utf-8")) if isinstance(title_raw, (bytes, bytearray)) else {}
        name_cn = title.get("zh-CN") or slug
        name_tw = title.get("zh-TW") or name_cn
        name_en = title.get("en-US") or name_cn

        tpl = DETAIL_TEMPLATES[cat_slug]
        content = localized(
            tpl["zh-CN"].format(name=name_cn),
            tpl["zh-TW"].format(name=name_tw),
            tpl["en-US"].format(name=name_en),
        )
        print(f"      + id={pid:3} {slug:26} → {cat_slug}（{len(content['zh-CN'])} 字符 HTML）")
        if args.apply:
            cur.execute(
                "update products set content_json = ?, updated_at = ? where id = ?",
                (json_bytes(content), datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S.%f+00:00"), pid),
            )
        existing_filled += 1

    # ---------- 2. 生成新商品 ----------
    counts = pick_counts(NEW_PRODUCT_COUNT)

    # 统计每个分类已经生成过的测试商品（带 NEW_TAG），
    # 这样脚本重复执行时只补差额，不会越写越多。
    seeded_per_cat = {cid: 0 for cid in CATEGORY_SLUGS}
    cur.execute("select category_id, tags from products where deleted_at is null")
    for cat_id, tags_raw in cur.fetchall():
        if cat_id not in seeded_per_cat or not tags_raw:
            continue
        try:
            tags = json.loads(tags_raw.decode("utf-8") if isinstance(tags_raw, (bytes, bytearray)) else tags_raw)
        except (ValueError, TypeError):
            continue
        if isinstance(tags, list) and NEW_TAG in tags:
            seeded_per_cat[cat_id] += 1

    print(f"\n[2/2] 生成新商品：目标 {NEW_PRODUCT_COUNT} 个，已存在 {sum(seeded_per_cat.values())} 个")
    print("      分类分配：" + "，".join(
        f"{CATEGORY_SLUGS[cid]}={counts[cid - 1]}（已有 {seeded_per_cat[cid]}）" for cid in sorted(CATEGORY_SLUGS)
    ))

    cur.execute("select slug from products")
    taken = {row[0] for row in cur.fetchall()}

    created = 0
    for cid in sorted(CATEGORY_SLUGS):
        want = max(0, counts[cid - 1] - seeded_per_cat[cid])
        if want == 0:
            continue
        pool = PRODUCT_POOL[cid][:]
        random.shuffle(pool)

        selected = []
        for item in pool:
            slug = item[0]
            if slug in taken:
                continue
            selected.append(item)
            taken.add(slug)
            if len(selected) >= want:
                break
        if len(selected) < want:
            print(f"      ! {CATEGORY_SLUGS[cid]} 候选不足，实际 {len(selected)}/{want}")

        cat_slug = CATEGORY_SLUGS[cid]
        tpl = DETAIL_TEMPLATES[cat_slug]
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S.%f+00:00")

        for slug, zh, tw, en, price, tags in selected:
            title_json = localized(zh, tw, en)
            description_json = localized(f"{zh}，官方渠道，下单后快速交付。",
                                         f"{tw}，官方渠道，下單後快速交付。",
                                         f"{en} — delivered fast from official channels.")
            content_json = localized(
                tpl["zh-CN"].format(name=zh),
                tpl["zh-TW"].format(name=tw),
                tpl["en-US"].format(name=en),
            )
            cost = round(price * 0.7, 2)
            created_at = random_created_at()
            # 手工库存是前台可购买的库存来源；用 auto 但没有上游映射的商品会被判为售罄，
            # 所以测试商品统一用 manual + 一个正库存。
            sku_stock = random.randint(20, 500)

            if not args.apply:
                print(f"      + {slug:30} → {cat_slug:14} ¥{price / 100:>8.2f}  库存{sku_stock:>4}  {zh}")
                created += 1
                continue

            cur.execute(
                """
                insert into products
                  (category_id, slug, title_json, description_json, content_json,
                   price_amount, cost_price_amount, images, tags,
                   purchase_type, min_purchase_quantity, max_purchase_quantity,
                   stock_display_mode, fulfillment_type, manual_stock_total,
                   manual_stock_locked, manual_stock_sold, is_affiliate_enabled,
                   is_mapped, is_active, sort_order, created_at, updated_at, deleted_at)
                values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,null)
                """,
                (
                    cid, slug, json_bytes(title_json), json_bytes(description_json), json_bytes(content_json),
                    price, cost, json_bytes([]), json_bytes(tags + [NEW_TAG]),
                    "member", 0, 0,
                    "exact", "manual", 0,
                    0, 0, 0,
                    0, 1, 0, created_at, created_at,
                ),
            )
            product_id = cur.lastrowid
            cur.execute(
                """
                insert into product_skus
                  (product_id, sku_code, spec_values_json, price_amount, cost_price_amount,
                   manual_stock_total, manual_stock_locked, manual_stock_sold,
                   is_active, sort_order, created_at, updated_at, deleted_at)
                values (?,?,?,?,?,?,?,?,?,?,?,?,null)
                """,
                (
                    product_id, "DEFAULT", json_bytes(title_json), price, cost,
                    sku_stock, 0, 0,
                    1, 0, created_at, created_at,
                ),
            )
            created += 1
            print(f"      + {slug:30} → {cat_slug:14} ¥{price / 100:>8.2f}  库存{sku_stock:>4}  {zh}")

    if args.apply:
        conn.commit()
        print(f"\n已提交：现有商品补详情 {existing_filled} 个，新增商品 {created} 个")
    else:
        print(f"\n预览模式：将补详情 {existing_filled} 个、新增 {created} 个。加 --apply 才会写入。")

    conn.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
