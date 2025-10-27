package websocket

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Client représente un client WebSocket.
type Client struct {
	Conn *websocket.Conn
}

// Manager gère l'enregistrement et la désinscription des clients, ainsi que la diffusion des messages.
type Manager struct {
	sync.RWMutex
	clients  map[int]map[*Client]bool // map de conversationID -> map de client
	upgrader websocket.Upgrader
}

// Manager de notifications
type NotificationManager struct {
	sync.RWMutex
	clients  map[int]*Client
	upgrader websocket.Upgrader
}

// NewManager crée un nouveau gestionnaire de WebSocket.
func NewManager() *Manager {
	log.Println("Création d'un nouveau gestionnaire WebSocket...")
	return &Manager{
		clients: make(map[int]map[*Client]bool),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				log.Printf("Vérification de l'origine de la requête : %s", r.Header.Get("Origin"))
				// Accepter toutes les origines pour le développement
				return true
			},
		},
	}
}

// NewNotificationManager crée un nouveau gestionnaire de WebSocket pour les notifications.
func NewNotificationManager() *NotificationManager {
	log.Println("Création d'un nouveau gestionnaire de notifications WebSocket...")
	return &NotificationManager{
		clients: make(map[int]*Client),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// GetUpgrader retourne l'objet websocket.Upgrader pour la mise à niveau des connexions.
func (m *Manager) GetUpgrader() *websocket.Upgrader {
	return &m.upgrader
}

// GetUpgrader retourne l'objet websocket.Upgrader pour la mise à niveau des connexions.
func (m *NotificationManager) GetUpgrader() *websocket.Upgrader {
	return &m.upgrader
}

// Register enregistre un nouveau client.
func (m *Manager) Register(conversationID int, client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[conversationID]; !ok {
		m.clients[conversationID] = make(map[*Client]bool)
	}
	m.clients[conversationID][client] = true
	log.Printf("Nouveau client enregistré pour la conversation %d.", conversationID)
}

// Register enregistre un nouveau client pour les notifications.
func (m *NotificationManager) Register(userID int, client *Client) {
	m.Lock()
	defer m.Unlock()
	m.clients[userID] = client
	log.Printf("Client de notification enregistré pour l'utilisateur %d.", userID)
}

// Unregister désenregistre un client.
func (m *Manager) Unregister(conversationID int, client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[conversationID]; ok {
		delete(m.clients[conversationID], client)
		log.Printf("Client désenregistré pour la conversation %d.", conversationID)
		if len(m.clients[conversationID]) == 0 {
			delete(m.clients, conversationID)
			log.Printf("Dernier client désenregistré. La map de la conversation %d a été supprimée.", conversationID)
		}
	}
	log.Printf("Nombre total de conversations actives après désenregistrement: %d", len(m.clients))
}

// Unregister désenregistre un client de notification.
func (m *NotificationManager) Unregister(userID int) {
	m.Lock()
	defer m.Unlock()
	delete(m.clients, userID)
	log.Printf("Client de notification désenregistré pour l'utilisateur %d.", userID)
}

// Broadcast envoie un message à tous les clients d'une conversation.
func (m *Manager) Broadcast(conversationID int, message []byte) {
	m.RLock()
	defer m.RUnlock()

	log.Printf("Diffusion d'un message pour la conversation %d. Message: %s", conversationID, string(message))

	if clients, ok := m.clients[conversationID]; ok {
		log.Printf("Nombre de clients à diffuser pour la conversation %d: %d", conversationID, len(clients))
		for client := range clients {
			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Erreur d'envoi du message à un client de la conversation %d: %v", conversationID, err)
				client.Conn.Close()
				delete(clients, client)
			}
		}
	}
}

// Notify envoie une notification à un utilisateur spécifique.
func (m *NotificationManager) Notify(userID int, message []byte) {
	m.RLock()
	defer m.RUnlock()

	if client, ok := m.clients[userID]; ok {
		if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("Erreur d'envoi de la notification à l'utilisateur %d: %v", userID, err)
			client.Conn.Close()
			delete(m.clients, userID)
		} else {
			log.Printf("Notification envoyée à l'utilisateur %d.", userID)
		}
	}
}
