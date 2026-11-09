# Folio（知微）部署与功能开启指南

这是一份面向第一次部署的完整指南：从一台空服务器开始，到 HTTPS 上线、开启注册/邮件/七牛/AI/备份/监控。  
文中会尽量写清楚**阿里云、七牛、QQ 邮箱等控制台的具体入口**，避免“知道要配置，但找不到页面”。

> 本文以阿里云 ECS + Alibaba Cloud Linux + Caddy 自动 HTTPS 为例；其他云厂商和 Linux 发行版也可以参考，命令按发行版调整即可。

---

## 0. 你需要准备什么

| 项目 | 是否必须 | 说明 |
| --- | --- | --- |
| Linux 服务器 | 必须 | 1C2G 可跑，推荐 2C4G；磁盘 20G 以上 |
| 域名 | 必须 | 例如 `folio.example.com` |
| ICP 备案 | 大陆服务器必须 | 未备案域名在大陆 80/443 会被拦截 |
| Docker + Compose | 必须 | 部署指南会从零安装 |
| SMTP 邮箱 | 可选 | 邮箱验证码、找回密码、通知邮件 |
| 七牛云 | 可选 | 图片对象存储；默认本地磁盘也能用 |
| 阿里云 OSS | 可选 | 异地备份 |
| QQ 互联 | 可选 | QQ 登录 |

---

## 1. 服务器、域名和网络

### 1.1 安全组

阿里云控制台入口：

**云服务器 ECS → 实例 → 安全组 → 配置规则 → 入方向**

只放行这些端口：

| 端口 | 来源 | 用途 |
| --- | --- | --- |
| `22/tcp` | 你的公网 IP | SSH |
| `80/tcp` | `0.0.0.0/0` | Caddy HTTP 验证 + 跳转 |
| `443/tcp` | `0.0.0.0/0` | HTTPS |

**不要放行** `3306`、`6379`、`8080`。它们只应该在服务器本机或 Docker 内网访问。

### 1.2 DNS 解析

阿里云控制台入口：

**云解析 DNS → 域名 → 解析设置 → 添加记录**

| 字段 | 值 |
| --- | --- |
| 记录类型 | `A` |
| 主机记录 | `folio` |
| 解析线路 | 默认 |
| 记录值 | ECS 的**公网 IPv4** |
| TTL | `600` |

验证：

```bash
dig +short folio.example.com @223.5.5.5
```

如果能返回 ECS 公网 IP，说明解析生效。有稳定 IPv6 再加 AAAA。

### 1.3 ICP 备案

大陆服务器必须完成 ICP 备案。备案通过后，在 Folio 后台：

**管理后台 → 站点设置 → ICP 备案号 / 公安备案号**

填写后页脚会自动展示并链接到备案查询页。

### 1.4 连接服务器

```bash
ssh root@<ECS 公网 IP>
```

如果使用密钥登录，先把本机公钥加入服务器的 `~/.ssh/authorized_keys`（权限 600）。

---

## 2. 安装 Docker

### 2.1 Alibaba Cloud Linux / CentOS / Anolis

```bash
dnf install -y moby-engine moby-client docker-compose-plugin git

# 某些镜像安装后没有 docker 用户组，会导致 docker.socket 启动失败
getent group docker >/dev/null || groupadd -r docker

systemctl enable --now docker.socket docker.service
docker --version
docker compose version
```

如果 `docker.socket` 报：

```text
Failed to resolve group docker: No such process
```

就是缺少 `docker` 用户组，执行上面的 `groupadd -r docker` 后重新启动即可。

### 2.2 Ubuntu / Debian

```bash
apt update
apt install -y docker.io docker-compose-plugin git
systemctl enable --now docker
docker --version
docker compose version
```

或者使用 Docker 官方脚本：

```bash
curl -fsSL https://get.docker.com | sh
```

### 2.3 国内服务器配置 Docker 镜像加速

大陆服务器直接拉 Docker Hub 经常超时。编辑：

```bash
mkdir -p /etc/docker
cat >/etc/docker/daemon.json <<'EOF'
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run",
    "https://docker.1panel.live",
    "https://docker.xuanyuan.me"
  ],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "20m",
    "max-file": "3"
  }
}
EOF

systemctl restart docker
docker info --format '{{json .RegistryConfig.Mirrors}}'
```

镜像站可能会变化；如果某个不可用，换一个能访问的即可。  
`log-opts` 同时配置了 Docker 日志轮转，避免日志把磁盘写满。

验证：

```bash
docker run --rm hello-world
```

---

## 3. 获取代码和基础配置

### 3.1 克隆代码

```bash
git clone https://github.com/qweijiaq/folio.git /opt/folio
cd /opt/folio
```

### 3.2 创建 `.env`

```bash
cp .env.example .env
chmod 600 .env
```

必须修改的项：

| 变量 | 说明 |
| --- | --- |
| `FOLIO_SECRET` | **至少 32 个字符的随机串**；非演示模式弱密钥会拒绝启动 |
| `FOLIO_DEMO` | 正式部署必须是 `0` |
| `FOLIO_SITE_URL` | 例如 `https://folio.example.com` |
| `FOLIO_DOMAIN` | 例如 `folio.example.com`（Caddy 用） |
| `ACME_EMAIL` | 证书通知邮箱 |
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码 |
| `MYSQL_PASSWORD` | 应用数据库密码 |
| `FOLIO_MYSQL_DSN` | 要和 `MYSQL_PASSWORD` 一致 |
| `FOLIO_ADMIN_USERNAME` | 首次启动创建的管理员用户名 |
| `FOLIO_ADMIN_PASSWORD` | 管理员密码，8-64 位 |
| `TZ` | 建议 `Asia/Shanghai` |
| `NPM_REGISTRY` | 国内建议 `https://registry.npmmirror.com` |
| `GOPROXY` | 国内建议 `https://goproxy.cn,direct` |

生成随机密钥：

```bash
openssl rand -hex 32
```

MySQL 密码建议只用字母和数字，避免 DSN 里的 `@ : / ? &` 转义问题。

示例：

```bash
FOLIO_SECRET=<64位随机串>
FOLIO_DEMO=0
FOLIO_SITE_URL=https://folio.example.com
FOLIO_DOMAIN=folio.example.com
ACME_EMAIL=you@example.com
MYSQL_ROOT_PASSWORD=<随机串>
MYSQL_PASSWORD=<随机串>
FOLIO_MYSQL_DSN=folio:<MYSQL_PASSWORD>@tcp(db:3306)/folio?charset=utf8mb4&parseTime=True&loc=Local
FOLIO_ADMIN_USERNAME=admin
FOLIO_ADMIN_PASSWORD=<强密码>
TZ=Asia/Shanghai
NPM_REGISTRY=https://registry.npmmirror.com
GOPROXY=https://goproxy.cn,direct
```

### 3.3 关于第一个管理员

空库启动时：

- 如果设置了 `FOLIO_ADMIN_USERNAME` / `FOLIO_ADMIN_PASSWORD`，会自动创建超级管理员；
- 如果没有设置，**第一位注册用户会成为超级管理员**。

所以推荐两种方式：

1. 启动前在 `.env` 设置 `FOLIO_ADMIN_*`，创建后从 `.env` 删除密码；
2. 或者先只在本机端口启动，完成注册并关闭公开注册，再开放 Caddy。

---

## 4. 第一次启动（先不开 Caddy）

先启动数据库、Redis 和应用：

```bash
cd /opt/folio
docker compose up -d db redis app
```

如果第一次构建比较慢，是正常现象。查看状态：

```bash
docker compose ps
docker compose logs -f app
```

健康检查：

```bash
curl -s http://127.0.0.1:8080/api/health
```

正常应返回：

```json
{"db":"mysql","dbStatus":"up","ok":true,"redis":"up"}
```

此时可以：

1. 通过 `http://127.0.0.1:8080` 或 SSH 隧道登录；
2. 确认管理员账号；
3. 在后台关闭公开注册；
4. 开启登录图形验证码；
5. 填写站点名称、描述、ICP/公安备案号。

> 不要在这个阶段把 8080 暴露到公网。

---

## 5. 开启 HTTPS（Caddy 自动证书）

### 5.1 确认前置条件

- 域名 A 记录指向 ECS 公网 IP；
- 安全组放行 80/443；
- 大陆服务器已完成 ICP 备案；
- `.env` 已设置 `FOLIO_DOMAIN`、`ACME_EMAIL`、`FOLIO_SITE_URL`。

### 5.2 启动 Caddy

```bash
docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d
```

Caddy 会自动申请 Let's Encrypt 证书。查看日志：

```bash
docker compose -f docker-compose.yml -f docker-compose.edge.yml logs -f caddy
```

看到下面这行就说明证书签发成功：

```text
certificate obtained successfully
```

验证：

```bash
curl -I https://folio.example.com
echo | openssl s_client -servername folio.example.com -connect folio.example.com:443 2>/dev/null \
  | openssl x509 -noout -issuer -dates
```

### 5.3 证书申请失败怎么办

按顺序检查：

1. `dig +short your-domain.com` 是否指向 ECS；
2. 公网 80 端口是否可达；
3. 安全组是否放行 80/443；
4. 大陆服务器是否完成 ICP 备案；
5. 是否有 AAAA 记录指向了错误地址；
6. Caddy 是否频繁重启触发 Let's Encrypt 频率限制。

证书保存在 `caddy-data` 卷里，重启 Caddy 不会重复申请。

---

## 6. 功能开启

### 6.1 站点设置

后台入口：

**管理后台 → 站点设置**

可以修改：

- 站点名称、描述、维护者；
- ICP / 公安备案号；
- 是否开放注册；
- 是否开启登录图形验证码。

> `.env` 里的 `FOLIO_LOGIN_CAPTCHA` 只在首次建库时作为初始值；之后以数据库设置为准。

### 6.2 邮件 / SMTP（邮箱验证码、找回密码、通知）

#### QQ 邮箱获取授权码

1. 登录 QQ 邮箱网页版；
2. **设置 → 账户**；
3. 找到 **POP3/IMAP/SMTP/Exchange/CardDAV/CalDAV 服务**；
4. 开启 **POP3/SMTP 服务**；
5. 按提示生成 **授权码**（不是 QQ 密码）。

#### `.env` 配置

```bash
FOLIO_SMTP_HOST=smtp.qq.com
FOLIO_SMTP_PORT=465
FOLIO_SMTP_USER=you@qq.com
FOLIO_SMTP_PASSWORD=你的授权码
FOLIO_SMTP_FROM=you@qq.com
FOLIO_SMTP_FROM_NAME=知微
FOLIO_SMTP_TLS=1
FOLIO_MAIL_DEBUG=0
```

- 465 端口：`FOLIO_SMTP_TLS=1`（隐式 TLS）；
- 587 端口：`FOLIO_SMTP_TLS=0`（STARTTLS）。

修改后重启应用：

```bash
docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d app
```

#### 测试

登录后：

**我的空间 → 账号安全 → 绑定邮箱**

能收到验证码就说明 SMTP 正常。也可以测试「忘记密码」和邮箱验证码登录。

> 未配置 SMTP 时，邮箱验证码、邮箱登录、找回密码会自动关闭；用户名密码登录不受影响。

### 6.3 邀请注册

后台入口：

**管理后台 → 注册与邀请**

推荐流程：

1. 关闭「允许公开注册」；
2. 生成一次性邀请链接；
3. 设置 1 / 7 / 30 / 90 天有效期；
4. 把链接发给朋友。

邀请码使用后立即失效，未使用的可以撤销。

> 本项目已修复“关闭公开注册 + 配置 SMTP 后邀请注册收不到验证码”的问题；如果部署的是旧版本，请先升级。

### 6.4 QQ 登录（可选）

1. 打开 [QQ 互联](https://connect.qq.com/)；
2. 创建网站应用；
3. 回调地址填写：
   ```text
   https://folio.example.com/api/auth/qq/callback
   ```
4. `.env` 配置：
   ```bash
   FOLIO_QQ_APP_ID=你的AppID
   FOLIO_QQ_APP_KEY=你的AppKey
   FOLIO_QQ_REDIRECT=https://folio.example.com/api/auth/qq/callback
   # 同源部署留空；本地 Vite 开发填 http://localhost:5173
   FOLIO_WEB_URL=
   ```
5. 重启应用；
6. 后台 **注册与邀请** 开启 QQ 登录。

### 6.5 AI 摘要（可选）

后台入口：

**管理后台 → AI 摘要**

- 选择服务商：Anthropic、DeepSeek、智谱 GLM、Kimi、通义千问；
- 填写 API Key（使用 `FOLIO_SECRET` 加密保存，接口不会回显）；
- 选择或填写模型；
- 开启后，编辑器会出现「AI 摘要」按钮。

AI 摘要只会补空字段，不会覆盖已经写好的内容。

### 6.6 AI 问答（用户自带 Key）

入口：`/ask`

- 用户必须自己填写所选服务商的 API Key；
- 站点不会用站长的 Key 代替；
- 支持 Anthropic、DeepSeek、智谱 GLM、Kimi、通义千问；
- 默认只在当前标签页记住 Key；勾选「记住 Key」才会加密保存到账号；
- 回答会检索站内文章、文档和站点帮助，支持严格站内 / 通用知识、多轮追问、流式输出和引用来源；
- 回答下方可以标记「有帮助 / 没帮助」。

### 6.7 图片存储：本地或七牛云

默认使用本地磁盘，图片在 `FOLIO_UPLOAD_DIR`，通过 `/uploads/*` 访问。

#### 七牛云配置步骤

**1. 创建 Bucket**

七牛控制台：**对象存储 Kodo → 空间管理 → 新建空间**

- 空间名称：例如 `folio-images`；
- 存储区域：选离服务器最近的；
- 访问控制：**公开**；
- 存储类型：标准存储。

> Folio 目前不支持私有 Bucket 的签名访问，必须公开读。

**2. 绑定图片域名**

空间 → **域名管理 → 绑定域名**，例如 `img.example.com`。  
七牛会给出一个 CNAME 地址，去阿里云 DNS 添加 CNAME 记录：

- 主机记录：`img`
- 记录值：七牛提供的 CNAME

等待七牛显示「已配置 / 正常」。

**3. 开启 HTTPS**

空间 → 域名管理 → **HTTPS 配置**，申请七牛免费证书或上传自己的证书。  
站点是 HTTPS，图片域名也必须是 HTTPS，否则会被浏览器拦截。

**4. 获取 AccessKey / SecretKey**

七牛控制台：**个人中心 → 密钥管理**。  
建议使用只授权 Kodo 的子账号；用主账号 AK/SK 也可以，但权限更大。

**5. 在 Folio 后台配置**

**管理后台 → 存储设置**

| 字段 | 填写 |
| --- | --- |
| 存储方式 | 七牛云 |
| AccessKey | 七牛 AK |
| SecretKey | 七牛 SK |
| 空间名 | Bucket 名 |
| 绑定域名 | `https://img.example.com`（带 https，不要带路径） |
| 区域 | 见下表；不确定可留空自动查询 |
| 对象前缀 | 默认 `folio` |

区域代码：

| 七牛区域 | 代码 |
| --- | --- |
| 华东 | `z0` |
| 华东-浙江 | `cn-east-2` |
| 华北 | `z1` |
| 华南 | `z2` |
| 北美 | `na0` |
| 新加坡 | `as0` |

点击 **测试连接**：会上传一个临时文件并立即删除。成功后保存，新图片就会走七牛。

> 如果报 `incorrect region, please use up-z0.qiniup.com`，说明 Bucket 实际在 `z0`，把区域改成 `z0` 或留空即可。

**6. 注意备份范围**

切换到七牛后，新图片不再落在本地 `uploads/`，每日备份和 OSS 异地备份不会包含七牛图片。  
建议在七牛侧单独做备份、跨区域复制或生命周期管理。

### 6.8 全站公告、邀请码、用户角色

后台入口：

- **全站公告**：管理后台 → 全站通知；
- **邀请码**：管理后台 → 注册与邀请；
- **用户与权限**：管理后台 → 用户与权限（仅超级管理员）；
- **操作日志**：管理后台 → 操作日志（仅超级管理员）。

### 6.9 RSS、sitemap、SEO

确保 `.env` 设置了：

```bash
FOLIO_SITE_URL=https://folio.example.com
```

然后验证：

```bash
curl -s https://folio.example.com/feed.xml
curl -s https://folio.example.com/sitemap.xml
curl -s https://folio.example.com/robots.txt
```

服务端会给文章/文档页注入 title、description、canonical、OG、Twitter Card 和 JSON-LD，爬虫不执行 JS 也能读到。

---

## 7. 备份与恢复

### 7.1 本地备份

```bash
cd /opt/folio
./scripts/backup.sh
```

备份包包含：

- `folio.sql`（MySQL）
- `uploads/`（本地上传图片）
- `env.backup`（`.env`，含 `FOLIO_SECRET`）

默认保留 14 天。安装每天 03:30 的 systemd 定时器：

```bash
sudo install -m 644 deploy/folio-backup.service /etc/systemd/system/
sudo install -m 644 deploy/folio-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now folio-backup.timer
```

### 7.2 异地备份到阿里云 OSS

**1. 创建私有 Bucket**

阿里云控制台：**对象存储 OSS → Bucket 列表 → 创建 Bucket**

- 区域：与 ECS 同区域，例如华东1 杭州；
- 读写权限：**私有**；
- 建议开启服务端加密；
- 建议设置生命周期（例如 90 天自动删除旧备份）。

**2. 创建 RAM 角色**

RAM 控制台：**访问控制 RAM → 角色 → 创建角色**

- 可信实体类型：**云服务 → ECS**（如果只能选阿里云账号，就把信任策略改成下面的 JSON）；
- 角色名：例如 `folio-oss-backup`；
- 权限：给该角色授权 OSS 写入权限（最小权限只授权这个 Bucket）。

ECS 实例 RAM 角色的信任策略：

```json
{
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "ecs.aliyuncs.com"
        ]
      }
    }
  ],
  "Version": "1"
}
```

**3. 授予 ECS**

ECS 控制台：**实例 → 更多 → 实例 RAM 角色 → 授予/替换 RAM 角色**，选择 `folio-oss-backup`。

等待 1 分钟，验证：

```bash
curl -s http://100.100.100.200/latest/meta-data/ram/security-credentials/
```

应该返回角色名。

**4. 配置阿里云 CLI**

```bash
aliyun configure set --profile default \
  --mode EcsRamRole \
  --ram-role-name folio-oss-backup \
  --region cn-hangzhou
aliyun sts GetCallerIdentity
aliyun oss ls
```

**5. 配置 `.env`**

```bash
OSS_BUCKET=your-bucket
OSS_REGION=cn-hangzhou
OSS_PREFIX=folio-backups
OSS_BACKUP_PASSPHRASE_FILE=/root/.folio-backup-pass
```

**6. 生成加密口令**

```bash
openssl rand -base64 32 > /root/.folio-backup-pass
chmod 600 /root/.folio-backup-pass
cat /root/.folio-backup-pass
```

把这串口令保存到密码管理器。服务器丢失后，没有它就无法解密 OSS 上的备份。

**7. 上传并验证**

```bash
./scripts/offsite-backup.sh
aliyun oss ls oss://your-bucket/folio-backups/
```

**8. 定时器**

```bash
sudo install -m 644 deploy/folio-offsite-backup.service /etc/systemd/system/
sudo install -m 644 deploy/folio-offsite-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now folio-offsite-backup.timer
```

### 7.3 恢复

本地备份恢复：

```bash
tar -xzf backups/folio-YYYYMMDD-HHMMSS.tar.gz -C /tmp
docker compose exec -T db sh -c 'exec mysql -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE"' < /tmp/folio.sql
docker compose cp /tmp/uploads/. app:/app/data/uploads/
```

OSS 备份恢复：

```bash
aliyun oss cp oss://your-bucket/folio-backups/xxx.tar.gz.enc /tmp/backup.enc
openssl enc -d -aes-256-cbc -pbkdf2 -in /tmp/backup.enc -out /tmp/backup.tar.gz \
  -pass file:/root/.folio-backup-pass
tar -xzf /tmp/backup.tar.gz -C /tmp
```

### 7.4 密钥备份

`FOLIO_SECRET` 同时用于：

- JWT 签名；
- 加密七牛密钥；
- 加密用户选择保存的 AI Key。

丢失 `FOLIO_SECRET` 会导致：

- 所有登录态失效；
- 已保存的七牛/AI Key 无法解密，需要重新填写。

所以 `.env` 必须和数据库一起备份，并妥善保管。

---

## 8. 监控与告警

项目自带 `scripts/monitor.py`，每 5 分钟检查：

- 应用健康检查；
- 公网 HTTPS 健康检查；
- app / db / redis / caddy 容器；
- 磁盘、内存；
- 本地备份和异地备份是否超时；
- HTTPS 证书是否即将过期。

异常通过 SMTP 发邮件到 `FOLIO_ALERT_EMAIL`。

安装：

```bash
sudo install -m 644 deploy/folio-monitor.service /etc/systemd/system/
sudo install -m 644 deploy/folio-monitor.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now folio-monitor.timer
```

测试邮件：

```bash
python3 scripts/monitor.py --test-email
```

手动执行一次：

```bash
python3 scripts/monitor.py
```

---

## 9. 系统维护

### 9.1 自动安全更新

Alibaba Cloud Linux / CentOS / Anolis：

```bash
dnf install -y dnf-automatic
sed -i 's/^upgrade_type = .*/upgrade_type = security/; s/^apply_updates = .*/apply_updates = yes/' /etc/dnf/automatic.conf
systemctl enable --now dnf-automatic.timer
```

默认 `reboot = never`，不会自动重启。内核更新需要你找时间手动重启。

### 9.2 防火墙

阿里云安全组已经是最外层防火墙。  
`firewalld` 可以和 Docker 端口映射冲突，**不建议在不了解的情况下直接开启**。

如果一定要启用，先预配置：

```bash
firewall-offline-cmd --zone=public --add-service=ssh
firewall-offline-cmd --zone=public --add-service=http
firewall-offline-cmd --zone=public --add-service=https
systemctl enable --now firewalld
```

启用后测试网站、SSH 和容器端口是否正常。

### 9.3 Git 更新部署

```bash
cd /opt/folio
./scripts/deploy.sh
```

脚本会：

1. `git pull --ff-only`
2. 重新构建应用镜像
3. 用 edge compose 更新容器
4. 清理旧镜像

如果服务器访问 GitHub 超时，稍后重试，或者用 `git bundle` 手动同步。

### 9.4 常用命令

```bash
cd /opt/folio

# 状态
docker compose -f docker-compose.yml -f docker-compose.edge.yml ps

# 日志
docker compose -f docker-compose.yml -f docker-compose.edge.yml logs -f app
docker compose -f docker-compose.yml -f docker-compose.edge.yml logs -f caddy

# 健康
curl -s https://folio.example.com/api/health

# 本地备份
./scripts/backup.sh

# 异地备份
./scripts/offsite-backup.sh

# 监控
python3 scripts/monitor.py

# 定时器
systemctl list-timers folio-monitor.timer folio-backup.timer folio-offsite-backup.timer dnf-automatic.timer
```

---

## 10. 常见问题

### 10.1 `docker.socket` 启动失败：`Failed to resolve group docker`

```bash
getent group docker || groupadd -r docker
systemctl daemon-reload
systemctl restart docker.socket docker.service
```

### 10.2 Docker Hub 拉取超时

配置 `/etc/docker/daemon.json` 的 `registry-mirrors`，参考第 2.3 节。

### 10.3 `proxy.golang.org` 超时

在 `.env` 设置：

```bash
GOPROXY=https://goproxy.cn,direct
NPM_REGISTRY=https://registry.npmmirror.com
```

然后重新构建：

```bash
docker compose build app
```

### 10.4 七牛报 `incorrect region`

错误信息里的 `up-z0.qiniup.com` 就是正确区域：

- `up-z0` → `z0`
- `up-z1` → `z1`
- `up-z2` → `z2`
- `up-cn-east-2` → `cn-east-2`

把 Folio 存储设置里的区域改成对应代码，或留空自动查询。

### 10.5 Caddy 刚启动时 HTTPS 报 `tlsv1 alert internal error`

通常是因为证书还在申请中。等 10-60 秒再试；如果一直失败，检查第 5.3 节。

### 10.6 公开注册关闭后邀请注册收不到验证码

这是旧版本的一个 bug，已在当前版本修复：  
关闭公开注册时，带有效邀请码仍可以发送注册验证码。请升级到最新代码。

### 10.7 登录时提示需要图形验证码，但后台显示关闭

数据库设置优先于 `.env`。  
在 **管理后台 → 站点设置 → 登录图形验证码** 里修改；`.env` 的 `FOLIO_LOGIN_CAPTCHA` 只作为首次建库初始值。

### 10.8 修改 MySQL 密码后应用连不上

MySQL 容器只在首次初始化数据卷时使用 `MYSQL_PASSWORD`。  
如果已经初始化过，修改 `.env` 不会自动改数据库里的密码。需要：

- 进入 MySQL 修改用户密码，或
- 删除 MySQL 数据卷重新初始化（会丢数据，谨慎）。

### 10.9 备份里没有图片

- 本地存储：检查 `FOLIO_UPLOAD_DIR` 和 `uploads-data` 卷；
- 七牛存储：七牛图片不在本地备份里，需要单独做七牛备份/跨区域复制。

### 10.10 服务器访问 GitHub 超时

`deploy.sh` 的 `git pull` 会失败。可以：

- 稍后重试；
- 或本地 `git bundle create ...`，scp 到服务器后 `git fetch`。

---

## 11. 上线检查清单

- [ ] 域名 A/AAAA 解析正确
- [ ] 安全组只放行 22/80/443
- [ ] 大陆服务器已完成 ICP 备案
- [ ] `.env` 权限 600，`FOLIO_SECRET` 至少 32 位并已备份
- [ ] `FOLIO_DEMO=0`
- [ ] 管理员已创建，临时密码已修改
- [ ] 公开注册/图形验证码策略已确认
- [ ] ICP/公安备案号已填写
- [ ] SMTP 已测试（绑定邮箱或找回密码）
- [ ] 七牛存储已测试（如使用）
- [ ] HTTPS 证书有效，HTTP 自动跳转
- [ ] `/feed.xml`、`/sitemap.xml`、`/robots.txt` 正常
- [ ] 本地备份已执行
- [ ] OSS 异地备份已上传并验证解密
- [ ] 监控告警测试邮件已收到
- [ ] Docker 日志轮转已配置
- [ ] 时区为 `Asia/Shanghai`
- [ ] 已发布第一篇文章

---

## 12. 一句话总结

如果你是第一次部署：

1. 配好 DNS 和安全组；
2. 装 Docker；
3. `cp .env.example .env` 并填好密钥；
4. `docker compose up -d db redis app`；
5. 创建管理员、关闭注册；
6. `docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d`；
7. 配置 SMTP、七牛、OSS、监控；
8. 备份并开始写作。

遇到问题时，优先看容器日志和 `/api/health`，大部分问题都能从这两处定位。
