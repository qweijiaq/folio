#!/usr/bin/env python3
"""对运行中的知微实例做一轮端到端冒烟测试。

默认连接 http://localhost:8080，使用演示管理员 weijia，并临时注册一个
测试用户；测试结束后会删除该用户和测试内容。

用法：
    python3 scripts/smoke.py
    BASE_URL=http://localhost:8080 python3 scripts/smoke.py
"""

import base64
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

BASE = os.getenv("BASE_URL", "http://localhost:8080").rstrip("/")
ADMIN = os.getenv("SMOKE_ADMIN", "weijia")
ADMIN_PASSWORD = os.getenv("SMOKE_PASSWORD", "folio1234")
SUFFIX = str(int(time.time()))[-8:]
PEER = f"smoke_{SUFFIX}"
PEER_PASSWORD = "smoke1234"

passed = 0


def check(name, condition, detail=""):
    global passed
    if condition:
        passed += 1
        print(f"  PASS  {name}")
    else:
        print(f"  FAIL  {name} {detail}")
        raise SystemExit(1)


def request(method, path, token=None, body=None, expect=None):
    url = BASE + path
    data = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as res:
            status = res.status
            raw = res.read()
    except urllib.error.HTTPError as exc:
        status = exc.code
        raw = exc.read()
    except urllib.error.URLError as exc:
        print(f"无法连接 {BASE}: {exc.reason}")
        raise SystemExit(1)
    payload = json.loads(raw) if raw else {}
    if expect is not None:
        check(f"{method} {path} -> {expect}", status == expect, f"(实际 {status}: {payload})")
    return status, payload


def login(username, password):
    _, data = request("POST", "/api/auth/login", body={"username": username, "password": password}, expect=200)
    return data["token"]


def upload_image(path, token, filename, content, content_type="image/png"):
    boundary = "----folio" + uuid.uuid4().hex
    head = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'
        f"Content-Type: {content_type}\r\n\r\n"
    ).encode()
    body = head + content + f"\r\n--{boundary}--\r\n".encode()
    req = urllib.request.Request(
        BASE + path,
        data=body,
        headers={
            "Accept": "application/json",
            "Authorization": "Bearer " + token,
            "Content-Type": f"multipart/form-data; boundary={boundary}",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as res:
            status = res.status
            raw = res.read()
    except urllib.error.HTTPError as exc:
        status = exc.code
        raw = exc.read()
    return status, json.loads(raw) if raw else {}


def fetch_url(path):
    req = urllib.request.Request(BASE + path)
    try:
        with urllib.request.urlopen(req, timeout=10) as res:
            return res.status, res.headers.get("Content-Type", ""), res.read()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.headers.get("Content-Type", ""), exc.read()


def main():
    print(f"冒烟测试 {BASE}")
    _, health = request("GET", "/api/health", expect=200)
    check("健康检查", health.get("ok") is True, health)
    print(f"        数据库={health.get('db')} Redis={health.get('redis')}")

    admin_token = login(ADMIN, ADMIN_PASSWORD)
    check("管理员登录", bool(admin_token))

    _, registered = request(
        "POST",
        "/api/auth/register",
        body={
            "username": PEER,
            "nickname": "冒烟测试用户",
            "email": f"{PEER}@example.com",
            "password": PEER_PASSWORD,
        },
        expect=201,
    )
    peer_id = registered["user"]["id"]
    peer_token = login(PEER, PEER_PASSWORD)

    # 角色权限：普通用户不能发布，超管可以提升为管理员
    request(
        "POST",
        "/api/docs",
        peer_token,
        {"kind": "article", "title": "普通用户不能写", "content": "x", "status": "draft"},
        expect=403,
    )
    request(
        "PUT",
        f"/api/admin/users/{peer_id}/role",
        admin_token,
        {"role": "admin"},
        expect=200,
    )
    _, peer_doc = request(
        "POST",
        "/api/docs",
        peer_token,
        {"kind": "article", "title": "管理员测试稿", "content": "## 测试\n\n管理员可以写。", "status": "published"},
        expect=201,
    )
    peer_doc_id = peer_doc["doc"]["id"]
    request("GET", "/api/admin/users", peer_token, expect=403)
    request("PUT", f"/api/admin/users/{peer_id}/role", peer_token, {"role": "user"}, expect=403)
    request("DELETE", f"/api/docs/{peer_doc_id}", peer_token, expect=200)

    _, category_data = request(
        "POST",
        "/api/categories",
        admin_token,
        {"name": f"冒烟分类{SUFFIX}", "sort": 99},
        expect=201,
    )
    category_id = category_data["category"]["id"]

    title = f"冒烟测试文章 {SUFFIX}"
    _, created = request(
        "POST",
        "/api/docs",
        admin_token,
        {
            "kind": "article",
            "title": title,
            "content": "## 冒烟测试\n\n这是一段自动生成的测试正文。\n\n~~~go\nfmt.Println(\"hello\")\n~~~\n",
            "tags": ["测试", "冒烟"],
            "categoryID": category_id,
            "status": "published",
        },
        expect=201,
    )
    doc_id = created["doc"]["id"]

    _, related_doc = request(
        "POST",
        "/api/docs",
        admin_token,
        {
            "kind": "article",
            "title": f"相关文章 {SUFFIX}",
            "content": "共享标签的相关文章。",
            "tags": ["冒烟"],
            "status": "published",
        },
        expect=201,
    )
    related_doc_id = related_doc["doc"]["id"]

    png = base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lK3Q6wAAAABJRU5ErkJggg=="
    )
    image_status, image_data = upload_image("/api/images", admin_token, "smoke.png", png)
    check("上传图片", image_status in (200, 201) and bool(image_data.get("url")), f"{image_status} {image_data}")
    image_id = image_data["image"]["id"]
    image_url = image_data["url"]
    image_fetch_status, image_content_type, _ = fetch_url(image_url)
    check("图片静态访问", image_fetch_status == 200 and image_content_type.startswith("image/png"))
    _, image_list = request("GET", "/api/images", admin_token, expect=200)
    check("图片库列表", any(item["id"] == image_id for item in image_list["items"]))
    request("DELETE", f"/api/images/{image_id}", admin_token, expect=200)
    image_id = None

    try:
        _, detail = request("GET", f"/api/docs/{doc_id}", peer_token, expect=200)
        check("读取文章", detail["doc"]["title"] == title)
        check("文章分类", detail["doc"]["category"]["name"] == f"冒烟分类{SUFFIX}")
        first_views = detail["doc"]["viewCount"]

        _, history = request("GET", "/api/me/history", peer_token, expect=200)
        check("浏览历史", any(item["doc"]["id"] == doc_id for item in history["items"]))

        _, by_category = request(
            "GET", f"/api/docs?kind=article&categoryID={category_id}", expect=200
        )
        check("分类筛选", any(item["id"] == doc_id for item in by_category["items"]))

        _, related = request("GET", f"/api/docs/{doc_id}/related", expect=200)
        check("相关文章", any(item["id"] == related_doc_id for item in related["items"]))

        if health.get("redis") == "up":
            _, again = request("GET", f"/api/docs/{doc_id}", peer_token, expect=200)
            check("Redis 浏览量去重", again["doc"]["viewCount"] == first_views, f"{first_views} -> {again['doc']['viewCount']}")

        _, liked = request("POST", f"/api/docs/{doc_id}/like", peer_token, expect=200)
        check("点赞", liked["liked"] is True and liked["likeCount"] == 1)
        _, faved = request("POST", f"/api/docs/{doc_id}/favorite", peer_token, expect=200)
        check("收藏", faved["favorited"] is True and faved["favoriteCount"] == 1)

        _, comment = request(
            "POST", f"/api/docs/{doc_id}/comments", peer_token, {"content": "冒烟测试评论"}, expect=201
        )
        comment_id = comment["comment"]["id"]
        request(
            "POST",
            f"/api/docs/{doc_id}/comments",
            admin_token,
            {"content": "冒烟测试回复", "parentID": comment_id},
            expect=201,
        )
        _, tree = request("GET", f"/api/docs/{doc_id}/comments", expect=200)
        roots = [c for c in tree["items"] if c["id"] == comment_id]
        check("两级评论", len(roots) == 1 and len(roots[0]["replies"]) == 1)

        _, follow = request("POST", f"/api/users/{ADMIN}/follow", peer_token, expect=200)
        check("关注", follow["following"] is True)
        request(
            "POST", f"/api/messages/with/{ADMIN}", peer_token, {"content": "冒烟测试私信"}, expect=201
        )
        _, conversations = request("GET", "/api/messages/conversations", admin_token, expect=200)
        check("私信会话", any(item["user"]["username"] == PEER for item in conversations["items"]))
        _, inbox = request("GET", "/api/inbox", admin_token, expect=200)
        check("收件箱计数", inbox["messages"] >= 1, inbox)
        _, notifications = request("GET", "/api/notifications", admin_token, expect=200)
        check("通知列表", isinstance(notifications["items"], list))

        _, found = request("GET", "/api/docs?q=" + urllib.parse.quote(title, safe=""), expect=200)
        check("搜索新文章", any(doc["id"] == doc_id for doc in found["items"]))
        _, tagged = request("GET", "/api/docs?tag=" + urllib.parse.quote("冒烟", safe=""), expect=200)
        check("标签筛选", any(doc["id"] == doc_id for doc in tagged["items"]))

        request("PUT", f"/api/docs/{doc_id}", peer_token, {"title": "越权", "content": "x", "status": "published"}, expect=403)

        if health.get("redis") == "up":
            request("POST", "/api/auth/logout", peer_token, expect=200)
            request("GET", "/api/me", peer_token, expect=401)
            peer_token = None
    finally:
        request("DELETE", f"/api/docs/{doc_id}", admin_token)
        request("GET", f"/api/docs/{doc_id}")
        if related_doc_id:
            request("DELETE", f"/api/docs/{related_doc_id}", admin_token)
        if image_id:
            request("DELETE", f"/api/images/{image_id}", admin_token)
        if category_id:
            request("DELETE", f"/api/categories/{category_id}", admin_token)
        # 清理临时用户（同时清理其评论、私信、关注、通知、浏览历史和图片）
        _, me = request("GET", f"/api/users/{PEER}", admin_token)
        peer_id = me.get("user", {}).get("id")
        if peer_id:
            request("DELETE", f"/api/admin/users/{peer_id}", admin_token)

    print(f"\n全部通过：{passed} 项检查")
    print("提示：如果 Redis 为 off，登出黑名单与浏览量去重的两项会跳过。")


if __name__ == "__main__":
    main()
