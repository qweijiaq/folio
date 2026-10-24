package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) writeJSON(value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(value)
}

type wsHub struct {
	mu      sync.RWMutex
	clients map[uint]map[*wsClient]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{clients: map[uint]map[*wsClient]struct{}{}}
}

func (h *wsHub) add(userID uint, client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = map[*wsClient]struct{}{}
	}
	h.clients[userID][client] = struct{}{}
}

func (h *wsHub) remove(userID uint, client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.clients[userID]
	delete(set, client)
	if len(set) == 0 {
		delete(h.clients, userID)
	}
}

// send 把同一条事件推送给多个用户的所有在线连接。写失败只关闭连接，不重试。
func (h *wsHub) send(userIDs []uint, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	seen := map[uint]bool{}
	targets := make([]*wsClient, 0, len(userIDs))
	h.mu.RLock()
	for _, id := range userIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		for client := range h.clients[id] {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range targets {
		client.mu.Lock()
		_ = client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := client.conn.WriteMessage(websocket.TextMessage, data)
		client.mu.Unlock()
		if err != nil {
			_ = client.conn.Close()
		}
	}
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// WebSocket 建立实时连接。浏览器无法设置 Authorization 头，因此 token 通过查询参数传入。
func (a *API) WebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		header := c.GetHeader("Authorization")
		token = header
		if len(header) > 7 && header[:7] == "Bearer " {
			token = header[7:]
		}
	}
	user, _ := a.userFromToken(token, c.Request.Context())
	if user == nil {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &wsClient{conn: conn}
	a.Hub.add(user.ID, client)
	defer func() {
		a.Hub.remove(user.ID, client)
		_ = conn.Close()
	}()

	if err := client.writeJSON(gin.H{"type": "ready", "userID": user.ID}); err != nil {
		return
	}
	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	})

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					_ = conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	close(done)
}
