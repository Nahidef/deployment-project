package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Metrics struct {
	mu                sync.RWMutex
	TotalRequests     int64
	SuccessfulRequests int64
	FailedRequests    int64
	AverageLatency    float64
	Version           string
	Healthy           bool
	StartTime         time.Time
}

type ApiHandler struct {
	dbWrite *pgxpool.Pool
	dbRead  *pgxpool.Pool
	metrics *Metrics
}

type User struct {
	ID   int    `+"`json:"id"`"+`
	Name string `+"`json:"name"`"+`
}

type CreateUserRequest struct {
	Name string `+"`json:"name" binding:"required"`"+`
}

type DeploymentInfo struct {
	Version        string    `+"`json:"version"`"+`
	Environment    string    `+"`json:"environment"`"+`
	Healthy        bool      `+"`json:"healthy"`"+`
	Uptime         string    `+"`json:"uptime"`"+`
	TotalRequests  int64     `+"`json:"total_requests"`"+`
	SuccessRate    float64   `+"`json:"success_rate"`"+`
	AverageLatency float64   `+"`json:"average_latency_ms"`"+`
	Timestamp      time.Time `+"`json:"timestamp"`"+`
}

func main() {
	log.Println("🚀 API sunucusu başlıyor...")

	writeURL := os.Getenv("PG_URL_WRITE")
	readURL := os.Getenv("PG_URL_READ")
	version := os.Getenv("APP_VERSION")
	environment := os.Getenv("ENVIRONMENT")

	if writeURL == "" || readURL == "" {
		log.Fatal("HATA: PG_URL_WRITE ve PG_URL_READ ortam değişkenleri set edilmeli.")
	}
	if version == "" {
		version = "v1"
	}
	if environment == "" {
		environment = "development"
	}

	dbWrite, err := connectToDB(writeURL, "Master (Yazma)")
	if err != nil {
		log.Fatalf("Master veritabanına bağlanılamadı: %v", err)
	}
	defer dbWrite.Close()

	dbRead, err := connectToDB(readURL, "Replica (Okuma)")
	if err != nil {
		log.Fatalf("Replica veritabanına bağlanılamadı: %v", err)
	}
	defer dbRead.Close()

	initSchema(dbWrite)

	metrics := &Metrics{
		Version:   version,
		Healthy:   true,
		StartTime: time.Now(),
	}

	handler := &ApiHandler{
		dbWrite: dbWrite,
		dbRead:  dbRead,
		metrics: metrics,
	}

	router := gin.Default()

	router.POST("/users", handler.addUser)
	router.GET("/users", handler.listUsers)
	router.GET("/health", handler.healthCheck)
	router.GET("/ready", handler.readinessCheck)
	router.GET("/metrics", handler.getMetrics)
	router.GET("/deployment-info", handler.getDeploymentInfo)
	router.GET("/version", handler.getVersion)

	log.Printf("✅ Sunucu :8080 portunda dinlemede... [Version: %s]", version)
	router.Run(":8080")
}

func connectToDB(url, name string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}
	log.Printf("✅ PostgreSQL bağlantısı başarılı: %s", name)
	return pool, nil
}

func initSchema(db *pgxpool.Pool) {
	schema := `+"`"+`
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`+"`"+`
	_, err := db.Exec(context.Background(), schema)
	if err != nil {
		log.Fatalf("Tablo oluşturulamadı: %v", err)
	}
	log.Println("✅ 'users' tablosu hazır.")
}

func (h *ApiHandler) healthCheck(c *gin.Context) {
	if !h.metrics.Healthy {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "Service sağlıksız",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"version": h.metrics.Version,
		"uptime":  time.Since(h.metrics.StartTime).String(),
	})
}

func (h *ApiHandler) readinessCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.dbWrite.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready": false,
			"error": "Master DB bağlantısı yok",
		})
		return
	}

	if err := h.dbRead.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready": false,
			"error": "Replica DB bağlantısı yok",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ready":        true,
		"version":      h.metrics.Version,
		"db_write":     "connected",
		"db_read":      "connected",
	})
}

func (h *ApiHandler) getVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":   h.metrics.Version,
		"timestamp": time.Now(),
	})
}

func (h *ApiHandler) getMetrics(c *gin.Context) {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	successRate := 0.0
	if h.metrics.TotalRequests > 0 {
		successRate = float64(h.metrics.SuccessfulRequests) / float64(h.metrics.TotalRequests) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_requests":      h.metrics.TotalRequests,
		"successful_requests": h.metrics.SuccessfulRequests,
		"failed_requests":     h.metrics.FailedRequests,
		"success_rate":        successRate,
		"average_latency_ms":  h.metrics.AverageLatency,
		"healthy":             h.metrics.Healthy,
		"uptime":              time.Since(h.metrics.StartTime).String(),
	})
}

func (h *ApiHandler) getDeploymentInfo(c *gin.Context) {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	successRate := 0.0
	if h.metrics.TotalRequests > 0 {
		successRate = float64(h.metrics.SuccessfulRequests) / float64(h.metrics.TotalRequests) * 100
	}

	info := DeploymentInfo{
		Version:        h.metrics.Version,
		Environment:    os.Getenv("ENVIRONMENT"),
		Healthy:        h.metrics.Healthy,
		Uptime:         time.Since(h.metrics.StartTime).String(),
		TotalRequests:  h.metrics.TotalRequests,
		SuccessRate:    successRate,
		AverageLatency: h.metrics.AverageLatency,
		Timestamp:      time.Now(),
	}

	c.JSON(http.StatusOK, info)
}

func (h *ApiHandler) addUser(c *gin.Context) {
	start := time.Now()
	h.metrics.mu.Lock()
	h.metrics.TotalRequests++
	h.metrics.mu.Unlock()

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordFailure()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz istek. 'name' alanı gerekiyor."})
		return
	}

	if os.Getenv("FAULT_MODE") == "true" {
		log.Printf("⚠️  FAULT_MODE aktif: POST istekleri 500 dönüyor")
		h.recordFailure()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Kasıtlı test hatası (FAULT_MODE)."})
		return
	}

	var newUser User
	err := h.dbWrite.QueryRow(context.Background(),
		"INSERT INTO users (name) VALUES ($1) RETURNING id, name",
		req.Name).Scan(&newUser.ID, &newUser.Name)

	if err != nil {
		log.Printf("❌ Veritabanına eklenemedi: %v", err)
		h.recordFailure()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Kullanıcı oluşturulamadı"})
		return
	}

	h.recordSuccess(time.Since(start))
	c.JSON(http.StatusCreated, newUser)
}

func (h *ApiHandler) listUsers(c *gin.Context) {
	start := time.Now()
	h.metrics.mu.Lock()
	h.metrics.TotalRequests++
	h.metrics.mu.Unlock()

	var users []User
	rows, err := h.dbRead.Query(context.Background(), "SELECT id, name FROM users ORDER BY id ASC")
	if err != nil {
		log.Printf("❌ Veritabanı sorgu hatası: %v", err)
		h.recordFailure()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Kullanıcılar listelenemedi"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			log.Printf("❌ Veri okuma hatası: %v", err)
			h.recordFailure()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Veri okunurken hata oluştu"})
			return
		}
		users = append(users, u)
	}

	if users == nil {
		users = make([]User, 0)
	}

	h.recordSuccess(time.Since(start))
	c.JSON(http.StatusOK, users)
}

func (h *ApiHandler) recordSuccess(duration time.Duration) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	h.metrics.SuccessfulRequests++
	h.metrics.AverageLatency = (h.metrics.AverageLatency + float64(duration.Milliseconds())) / 2
}

func (h *ApiHandler) recordFailure() {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	h.metrics.FailedRequests++
}
