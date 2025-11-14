/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package main

import (
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/managers"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"net/http"
	"os"
	"path/filepath"
)

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

func main() {

	cdsHome := getCDSHome()
	const configFile = "/repository/conf/deployment.yaml"

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

	serverAddr := fmt.Sprintf("%s:%d", cdsConfig.Addr.Host, cdsConfig.Addr.Port)
	mux := enableCORS(initMultiplexer())

	logger := log.GetLogger()
	logger.Info(fmt.Sprintf(" WSO2 CDS starting securely on: https://%s", serverAddr))

	certDir := cdsConfig.TLS.CertDir
	if certDir == "" {
		certDir = "./etc/certs"
	}

	serverCertPath := filepath.Join(certDir, "server.crt")
	serverKeyPath := filepath.Join(certDir, "server.key")

	// Check cert files before starting
	if _, err := os.Stat(serverCertPath); os.IsNotExist(err) {
		logger.Error(fmt.Sprintf("Server certificate not found at %s", serverCertPath))
		os.Exit(1)
	}
	if _, err := os.Stat(serverKeyPath); os.IsNotExist(err) {
		logger.Error(fmt.Sprintf("Server key not found at %s", serverKeyPath))
		os.Exit(1)
	}

	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	logger.Info(fmt.Sprintf("HTTPS server listening on %s", serverAddr))
	if err := server.ListenAndServeTLS(serverCertPath, serverKeyPath); err != nil {
		logger.Error("Failed to start HTTPS server.", log.Error(err))
		os.Exit(1)
	}
}

// initMultiplexer initializes the HTTP multiplexer and registers the services.
func initMultiplexer() *http.ServeMux {

	mux := http.NewServeMux()
	serviceManager := managers.NewServiceManager(mux)
	logger := log.GetLogger()

	// Register the services.
	if err := serviceManager.RegisterServices(); err != nil {
		logger.Error("Failed to register the services. ", log.Error(err))
	}

	return mux
}

// enableCORS allows cross-origin requests for UI/SDK clients.
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

// getCDSHome detects the CDS project root for config resolution.
func getCDSHome() string {

	projectHomeFlag := flag.String("cdsHome", "", "Path to customer data service home directory")
	flag.Parse()

	if *projectHomeFlag != "" {
		fmt.Printf("Using %s from command line argument", *projectHomeFlag)
		return *projectHomeFlag
	}

	dir, dirErr := os.Getwd()
	if dirErr != nil {
		fmt.Println("Failed to get current working directory", dirErr)
		return ""
	}

	return dir
}
