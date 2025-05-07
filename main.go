package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	 "encoding/json"
)

// User model
type User struct {
	gorm.Model
	Username string `gorm:"unique"`
	Password string
}

// NewsArticle represents a single news item
type NewsArticle struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	URLToImage  string `json:"urlToImage"`
	PublishedAt string `json:"publishedAt"`
}

// NewsResponse represents the API response
type NewsResponse struct {
	Status       string        `json:"status"`
	TotalResults int           `json:"totalResults"`
	Articles     []NewsArticle `json:"articles"`
}

var db *gorm.DB
var newsAPIKey string

func initDB() {
	var err error
	db, err = gorm.Open(sqlite.Open("newsapp.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&User{})
}

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	newsAPIKey = os.Getenv("NEWS_API_KEY")
	if newsAPIKey == "" {
		log.Fatal("NEWS_API_KEY not set in .env file")
	}
}

func main() {
	loadEnv()
	initDB()

	router := gin.Default()

	// Set up sessions
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.Sessions("newsapp", store))

	// Load HTML templates
	router.LoadHTMLGlob("templates/*")

	// Routes
	router.GET("/", homeHandler)
	router.GET("/login", loginFormHandler)
	router.POST("/login", loginHandler)
	router.GET("/register", registerFormHandler)
	router.POST("/register", registerHandler)
	router.GET("/logout", logoutHandler)
	router.GET("/news", authMiddleware(), newsHandler)

	router.Run(":8080")
}

func homeHandler(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	c.HTML(http.StatusOK, "index.html", gin.H{"user": user})
}

func loginFormHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

func loginHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user User
	result := db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "Invalid credentials"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "Invalid credentials"})
		return
	}

	session := sessions.Default(c)
	session.Set("user", username)
	session.Save()

	c.Redirect(http.StatusFound, "/news")
}

func registerFormHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", nil)
}

func registerHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// Check if user already exists
	var existingUser User
	result := db.Where("username = ?", username).First(&existingUser)
	if result.Error == nil {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{"error": "Username already exists"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{"error": "Error creating user"})
		return
	}

	newUser := User{
		Username: username,
		Password: string(hashedPassword),
	}

	db.Create(&newUser)

	c.Redirect(http.StatusFound, "/login")
}

func logoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/")
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("user")
		if user == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func newsHandler(c *gin.Context) {
	// Fetch news from API
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://newsapi.org/v2/top-headlines?country=us&apiKey=%s", newsAPIKey)

	resp, err := client.Get(url)
	if err != nil {
			c.HTML(http.StatusInternalServerError, "news.html", gin.H{"error": "Failed to fetch news"})
			return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
			c.HTML(http.StatusInternalServerError, "news.html", gin.H{"error": "News API returned an error"})
			return
	}

	var newsResponse NewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&newsResponse); err != nil {
			c.HTML(http.StatusInternalServerError, "news.html", gin.H{"error": "Failed to parse news"})
			return
	}

	c.HTML(http.StatusOK, "news.html", gin.H{
			"articles": newsResponse.Articles,
			"user":    sessions.Default(c).Get("user"),
	})
}