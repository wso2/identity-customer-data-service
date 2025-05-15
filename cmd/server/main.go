package main

import (
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
	"github.com/wso2/identity-customer-data-service/internal/system/managers"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

func initDatabaseFromConfig(config *config.Config) {
	host := config.DatabaseConfig.Host
	port := config.DatabaseConfig.Port
	user := config.DatabaseConfig.User
	password := config.DatabaseConfig.Password
	dbname := config.DatabaseConfig.DbName

	if host == "" || port == "" || user == "" || password == "" || dbname == "" {
		log.Fatal("One or more PostgreSQL configuration values are missing")
	}

	log.Println("PostgreSQL database initialized successfully from configuration")
}

func main() {
	cdsHome := getCDSHome()
	const configFile = "/repository/conf/deployment.yaml"

	envFiles, err := filepath.Glob("config/*.env")
	if err != nil || len(envFiles) == 0 {
		logger.Error(err, "No .env files found in config directory")
	}
	_ = godotenv.Load(envFiles...)

	// Load the configuration file
	cdsConfig, err := config.LoadConfig(cdsHome, configFile)
	if err != nil {
		log.Fatalf("Failed to load cdsConfig: %v", err)
	}

	// Initialize runtime configurations.
	if err := config.InitializeCDSRuntime(cdsHome, cdsConfig); err != nil {
		log.Fatalf("Failed to initialize thunder runtime: %v", err)
	}

	// Initialize logger
	logger.Init(cdsConfig.Log.DebugEnabled)

	// Initialize database
	initDatabaseFromConfig(cdsConfig)

	// Initialize Event queue
	workers.StartProfileWorker()

	serverAddr := fmt.Sprintf("%s:%d", cdsConfig.Addr.Host, cdsConfig.Addr.Port)
	mux := enableCORS(initMultiplexer())
	logger.Info("WSO2 CDS starting in: %v", serverAddr)
	ln, err := net.Listen("tcp", serverAddr)
	if err != nil {
		logger.Error(err, "Failed to start TLS listener")
	}

	logger.Info("WSO2 CDS started in: %v", serverAddr)

	server1 := &http.Server{Handler: mux}

	if err := server1.Serve(ln); err != nil {
		logger.Error(err, "\"Failed to serve requests.")
	}

	logger.Info("identity-customer-data-service component has started.")

}

// initMultiplexer initializes the HTTP multiplexer and registers the services.
func initMultiplexer() *http.ServeMux {

	mux := http.NewServeMux()
	serviceManager := managers.NewServiceManager(mux)

	// Register the services.
	err := serviceManager.RegisterServices(constants.ApiBasePath)
	if err != nil {
		logger.Error(err, "Failed to register the services.")
	}

	return mux
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
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
