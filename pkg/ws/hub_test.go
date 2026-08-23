package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketHubPingPong(t *testing.T) {
	hub := NewHub(func(ctx context.Context, client *Client, msg InboundMessage) {
		// Echo for test
		client.Send(OutboundMessage{
			Type:    TypeDone,
			Payload: "echo:" + msg.Content,
		})
	})
	go hub.Run()
	defer hub.Close()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer wsConn.Close()

	// Send Ping
	pingMsg := InboundMessage{Type: TypePing}
	if err := wsConn.WriteJSON(pingMsg); err != nil {
		t.Fatalf("failed to send ping: %v", err)
	}

	_ = wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp OutboundMessage
	if err := wsConn.ReadJSON(&resp); err != nil {
		t.Fatalf("failed to read pong response: %v", err)
	}

	if resp.Type != TypePong {
		t.Errorf("expected type %s, got %s", TypePong, resp.Type)
	}

	// Send Chat Message
	chatMsg := InboundMessage{
		Type:    TypeChatMessage,
		Content: "hello agent",
	}
	if err := wsConn.WriteJSON(chatMsg); err != nil {
		t.Fatalf("failed to send chat message: %v", err)
	}

	if err := wsConn.ReadJSON(&resp); err != nil {
		t.Fatalf("failed to read chat response: %v", err)
	}

	if resp.Type != TypeDone {
		t.Errorf("expected type %s, got %s", TypeDone, resp.Type)
	}
	if resp.Payload != "echo:hello agent" {
		t.Errorf("expected payload 'echo:hello agent', got %v", resp.Payload)
	}
}
