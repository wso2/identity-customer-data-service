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
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/managers"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

func initDatabaseFromConfig(config *config.Config) {

	logger := log.GetLogger()
	host := config.DatabaseConfig.Host
	port := config.DatabaseConfig.Port
	user := config.DatabaseConfig.User
	password := config.DatabaseConfig.Password
	dbname := config.DatabaseConfig.DbName

	if host == "" || port == "" || user == "" || password == "" || dbname == "" {
		logger.Error("One or more Database configuration values are missing.")
	}

	logger.Info(fmt.Sprintf("Database initialized successfully for configurations - db name:%s, db host:%s, "+
		"db port:%s", dbname, host, port))
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
	}

	// Initialize runtime configurations.
	if err := config.InitializeCDSRuntime(cdsHome, cdsConfig); err != nil {
		fmt.Println("Failed to initialize cds runtime.", err)
	}

	// Initialize logger
	if err := log.Init(cdsConfig.Log.LogLevel); err != nil {
		fmt.Println("Failed to initialize cds runtime.", err)
	}

	// Initialize database
	initDatabaseFromConfig(cdsConfig)

	// Initialize Event queue
	workers.StartProfileWorker()

	serverAddr := fmt.Sprintf("%s:%d", cdsConfig.Addr.Host, cdsConfig.Addr.Port)
	mux := enableCORS(initMultiplexer())

	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("WSO2 CDS starting in server address: %s", serverAddr))
	ln, err := net.Listen("tcp", serverAddr)
	if err != nil {
		logger.Error("Error when starting the CDS.", log.Error(err))
	}

	logger.Info(fmt.Sprintf("WSO2 CDS started in server address: %s", serverAddr))
	server1 := &http.Server{Handler: mux}

	if err := server1.Serve(ln); err != nil {
		logger.Error("Failed to serve requests. ", log.Error(err))
	}

	logger.Info("identity-customer-data-service component has started.")

}

// initMultiplexer initializes the HTTP multiplexer and registers the services.
func initMultiplexer() *http.ServeMux {

	mux := http.NewServeMux()
	serviceManager := managers.NewServiceManager(mux)
	logger := log.GetLogger()
	// Register the services.
	err := serviceManager.RegisterServices(constants.ApiBasePath)
	if err != nil {
		logger.Error("Failed to register the services. ", log.Error(err))
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
		fmt.Printf("Using %s from command line argument", *projectHomeFlag)
		projectHome = *projectHomeFlag
	} else {
		// If no command line argument is provided, use the current working directory.
		dir, dirErr := os.Getwd()
		if dirErr != nil {
			fmt.Println("Failed to get current working directory", dirErr)
		}
		projectHome = dir
	}

	return projectHome
}
