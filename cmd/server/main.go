package main

import (
	"github.com/joho/godotenv"
	"github.com/wso2/identity-customer-data-service/config"
	"github.com/wso2/identity-customer-data-service/pkg/constants"
	"github.com/wso2/identity-customer-data-service/pkg/handlers"
	"github.com/wso2/identity-customer-data-service/pkg/locks"
	"github.com/wso2/identity-customer-data-service/pkg/logger"
	"github.com/wso2/identity-customer-data-service/pkg/service"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	const configFile = "config/config.yaml"

	envFiles, err := filepath.Glob("config/*.env")
	if err != nil || len(envFiles) == 0 {
		logger.Error(err, "No .env files found in config directory")
	}
	err = godotenv.Load(envFiles...)

	// Load the configuration file
	cdsConfig, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load cdsConfig: %v", err)
	}

	// Initialize logger
	logger.Init(cdsConfig.Log.DebugEnabled)
	router := gin.Default()
	server := handlers.NewServer()

	// Apply CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins: cdsConfig.Auth.CORSAllowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		//AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowHeaders:     []string{"*"}, // Or specify "filter" if needed
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Initialize MongoDB
	mongoDB := locks.ConnectMongoDB(cdsConfig.MongoDB.URI, cdsConfig.MongoDB.Database)

	locks.InitLocks(mongoDB.Database)

	// Initialize Event queue
	service.StartProfileWorker()

	api := router.Group(constants.ApiBasePath)
	handlers.RegisterHandlers(api, server)
	s := &http.Server{
		Handler: router,
		Addr:    cdsConfig.Addr.Host + ":" + cdsConfig.Addr.Port,
	}

	router.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(200)
	})

	// Close MongoDB connection on exit
	defer mongoDB.Client.Disconnect(nil)

	logger.Info("identity-customer-data-service component has started.")

	// And we serve HTTP until the world ends.
	log.Fatal(s.ListenAndServe())

}
