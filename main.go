package main

import (
	"telerealm/handlers"
	"telerealm/initializers"
	"telerealm/middleware"
	"telerealm/repositories"
	"telerealm/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	h := initializeHandlers()

	// Serve static files
	r.Static("/static", "./static")

	// Public endpoints
	r.GET("/ping", h.Ping)
	r.GET("/drive/:key", h.DownloadFile)
	r.GET("/multipart/download/:key", h.DownloadMultipartFile)
	r.GET("/", func(c *gin.Context) {
		c.File("static/index.html")
	})
	r.GET("/upload", func(c *gin.Context) {
		c.File("static/upload.html")
	})
	r.GET("/public-api", func(c *gin.Context) {
		c.File("static/public-api.html")
	})
	r.GET("/demo", func(c *gin.Context) {
		c.File("static/demo.html")
	})
	r.GET("/docs", func(c *gin.Context) {
		c.File("static/docs.html")
	})
	r.GET("/robots.txt", func(c *gin.Context) {
		c.File("static/robots.txt")
	})
	r.GET("/sitemap.xml", func(c *gin.Context) {
		c.File("static/sitemap.xml")
	})

	publicLink := r.Group("/link/:botToken/:chatID")
	{
		publicLink.POST("", h.PublicCreateLinkRecord)
		publicLink.GET("", h.PublicListLinkRecords)
		publicLink.GET("/:id", h.PublicGetLinkRecord)
		publicLink.PATCH("/:id", h.PublicUpdateLinkRecord)
		publicLink.DELETE("/:id", h.PublicDeleteLinkRecord)
	}

	// Protected endpoints
	auth := r.Group("/")
	auth.Use(middleware.AuthRequired())
	{
		auth.POST("/send", h.SendFile)
		auth.POST("/multipart/init", h.InitMultipartUpload)
		auth.POST("/multipart/:uploadID/part", h.UploadMultipartPart)
		auth.POST("/multipart/:uploadID/complete", h.CompleteMultipartUpload)
		auth.POST("/files", h.CreateFileRecord)
		auth.GET("/files", h.ListFileRecords)
		auth.GET("/files/:id", h.GetFileRecord)
		auth.PATCH("/files/:id", h.UpdateFileRecord)
		auth.DELETE("/files/:id", h.DeleteFileRecord)
		auth.GET("/url", h.GetFileURL)
		auth.GET("/info", h.GetFileInfo)
		auth.GET("/verify", h.CheckBotAndChat)
	}

	apiAuth := r.Group("/api")
	apiAuth.Use(middleware.AuthRequired())
	{
		apiAuth.POST("/multipart/init", h.InitMultipartUpload)
		apiAuth.POST("/multipart/:uploadID/part", h.UploadMultipartPart)
		apiAuth.POST("/multipart/:uploadID/complete", h.CompleteMultipartUpload)
	}

	r.Run(":7777")
}

func initializeHandlers() *handlers.Handlers {
	initializers.LoadEnvironment()

	repo := initializeRepositories()
	service := services.NewFileService(repo)
	return handlers.NewHandlers(service)
}

func initializeRepositories() repositories.FileRepository {
	return repositories.NewFileRepository()
}
