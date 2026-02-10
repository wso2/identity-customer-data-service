package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"

	// your mcp/init.go package
	profileSvc "github.com/wso2/identity-customer-data-service/internal/profile/service"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/mcp"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

func main() {
	mux := http.NewServeMux()

	// Wire CDS services needed by MCP tools
	profilesSvc := profileSvc.GetProfilesService()
	cdsHome := resolveCDSHome()
	utils.SetCDSHome(cdsHome)
	const configFile = "/config/repository/conf/deployment.yaml"

	envFiles, err := filepath.Glob("config/*.env")
	if err != nil || len(envFiles) == 0 {
		fmt.Println("No .env files found in config directory. ", err)
	}
	_ = godotenv.Load(envFiles...)

	// Load the configuration file
	cdsConfig, err := config.LoadConfig(cdsHome, configFile)
	if err != nil {
		fmt.Println("Failed to load cdsConfig. ", err)
		os.Exit(1)
	}
	// Initialize runtime configurations
	if err := config.InitializeCDSRuntime(cdsHome, cdsConfig); err != nil {
		fmt.Println("Failed to initialize cds runtime.", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := log.Init(cdsConfig.Log.LogLevel); err != nil {
		fmt.Println("Failed to initialize logger.", err)
		os.Exit(1)
	}

	// Initialize database
	initDatabaseFromConfig(cdsConfig)

	// Initialize Profile worker
	workers.StartProfileWorker()

	// Initialize Schema Sync worker
	workers.StartSchemaSyncWorker()

	// Register MCP routes (/mcp)
	mcp.Initialize(mux, profilesSvc)

	addr := ":8081"
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("CDS MCP server listening on %s", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("Failed to start HTTPS server.", log.Error(err))
		os.Exit(1)
	}
}

// resolveCDSHome parses flags and determines the CDS home directory.
func resolveCDSHome() string {
	// Define the flag locally
	cdsHomeFlag := flag.String("cdsHome", "", "Path to customer data service home directory")

	// Parse flags once (only if not already parsed)
	if !flag.Parsed() {
		flag.Parse()
	}

	// Determine the directory
	if *cdsHomeFlag != "" {
		fmt.Printf("Using %s from command line argument\n", *cdsHomeFlag)
		return *cdsHomeFlag
	}

	// Fallback to environment variable
	if envHome := os.Getenv("CDS_HOME"); envHome != "" {
		fmt.Printf("Using CDS_HOME from environment: %s\n", envHome)
		return envHome
	}

	// Fallback to working directory
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Failed to get current working directory", err)
		os.Exit(1)
	}
	return dir
}

func initDatabaseFromConfig(config *config.Config) {

	logger := log.GetLogger()
	host := config.DataSource.Hostname
	port := config.DataSource.Port
	user := config.DataSource.Username
	password := config.DataSource.Password
	dbname := config.DataSource.Name

	if host == "" || user == "" || password == "" || dbname == "" {
		logger.Error("One or more Database configuration values are missing.")
	}

	logger.Info(fmt.Sprintf("Database initialized successfully for configurations - db name:%s, db host:%s, "+
		"db port:%d", dbname, host, port))
}
