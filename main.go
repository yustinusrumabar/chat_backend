package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User adalah data user untuk login
type User struct {
	Username string `json:"username"`
}

// ChatMessage adalah struktur data pesan
type ChatMessage struct {
	Username string    `json:"username"`
	Message  string    `json:"message"`
	SentAt   time.Time `json:"sent_at"`
}

var (
	client            *mongo.Client
	userCollection    *mongo.Collection
	messageCollection *mongo.Collection
)

func main() {
	ctx := context.Background()

	// Ambil URI MongoDB dari environment variable
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("Missing MONGO_URI environment variable")
	}

	// Koneksi ke MongoDB
	var err error
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("Mongo Connect Error:", err)
	}
	defer client.Disconnect(ctx)

	// Pilih database dan collection
	db := client.Database("chatdb")
	userCollection = db.Collection("users")
	messageCollection = db.Collection("messages")

	// Routing
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/send", sendHandler)
	http.HandleFunc("/messages", getMessagesHandler)

	// Ambil port dari env atau default ke 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// loginHandler memeriksa apakah username terdaftar
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil || user.Username == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	filter := bson.M{"username": user.Username}
	count, err := userCollection.CountDocuments(context.Background(), filter)
	if err != nil || count == 0 {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	log.Println("Login sukses:", user.Username)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// sendHandler menyimpan pesan ke database
func sendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var message ChatMessage
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil || message.Username == "" || message.Message == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	message.SentAt = time.Now()
	_, err := messageCollection.InsertOne(context.Background(), message)
	if err != nil {
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	log.Printf("Pesan disimpan dari %s: %s\n", message.Username, message.Message)
	w.WriteHeader(http.StatusOK)
}

// getMessagesHandler mengirim semua pesan ke frontend
func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opts := options.Find().SetSort(bson.M{"sent_at": 1})
	cursor, err := messageCollection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		http.Error(w, "Failed to get messages", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var messages []ChatMessage
	if err := cursor.All(context.Background(), &messages); err != nil {
		http.Error(w, "Error decoding messages", http.StatusInternalServerError)
		return
	}

	log.Println("Jumlah pesan dikirim ke frontend:", len(messages))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}
