package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/wso2/identity-customer-data-service/internal/constants"
	"github.com/wso2/identity-customer-data-service/internal/database"
	"github.com/wso2/identity-customer-data-service/internal/handlers"
	"github.com/wso2/identity-customer-data-service/internal/logger"
	"github.com/wso2/identity-customer-data-service/internal/service"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

func initPostgresDatabaseFromConfig(config *config.Config) {
	host := config.DatabaseConfig.Host
	port := config.DatabaseConfig.Port
	user := config.DatabaseConfig.User
	password := config.DatabaseConfig.Password
	dbname := config.DatabaseConfig.DbName

	if host == "" || port == "" || user == "" || password == "" || dbname == "" {
		log.Fatal("One or more PostgreSQL configuration values are missing")
	}

	database.ConnectPostgres(host, port, user, password, dbname)
	log.Println("PostgreSQL database initialized successfully from configuration")
}

func main() {
	cdsHome := getCDSHome()
	const configFile = "/config/repository/conf/deployment.yaml"

	envFiles, err := filepath.Glob("config/*.env")
	if err != nil || len(envFiles) == 0 {
		logger.Error(err, "No .env files found in config directory")
	}
	err = godotenv.Load(envFiles...)

	// Load the configuration file
	cdsConfig, err := config.LoadConfig(cdsHome, configFile)
	if err != nil {
		log.Fatalf("Failed to load cdsConfig: %v", err)
	}

	// Initialize logger
	logger.Init(cdsConfig.Log.DebugEnabled)
	router := gin.Default()
	server := handlers.NewServer()

	// Apply CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     cdsConfig.Auth.CORSAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"}, // Or specify "filter" if needed
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Initialize MongoDB
	mongoDB := database.ConnectMongoDB(cdsConfig.MongoDB.URI, cdsConfig.MongoDB.Database)

	database.InitLocks(mongoDB.Database)

	// Initialize Event queue
	service.StartProfileWorker()

	// Initialize PostgreSQL database
	initPostgresDatabaseFromConfig(cdsConfig)

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

func getCDSHome() string {

	// Parse project directory from command line arguments.
	projectHome := ""
	projectHomeFlag := flag.String("cdsHome", "", "Path to customer data service home directory")
	flag.Parse()

	if *projectHomeFlag != "" {
		logger.Info(fmt.Sprintf("Using %s from command line argument", *projectHomeFlag))
		projectHome = *projectHomeFlag
	} else {
		// If no command line argument is provided, use the current working directory.
		dir, dirErr := os.Getwd()
		if dirErr != nil {
			logger.Error(dirErr, "Failed to get current working directory")
		}
		projectHome = dir
	}

	return projectHome
}
