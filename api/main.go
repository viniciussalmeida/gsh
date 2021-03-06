package main

import (
	"os"
	"strconv"

	"github.com/globocom/gsh/api/config"
	"github.com/globocom/gsh/api/permissions"
	"github.com/globocom/gsh/api/storage"
	"github.com/globocom/gsh/types"

	"github.com/globocom/gsh/api/handlers"
	"github.com/globocom/gsh/api/workers"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func main() {
	// Reading configuration
	configuration := config.Init()
	err := config.Check(configuration)
	if err != nil {
		panic(err)
	}

	// Configuring storage
	db, err := storage.Init(configuration)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Configuring Casbin
	permEnforcer, err := permissions.Init(configuration)
	if err != nil {
		panic(err)
	}

	// Configuring channels
	var defaultChannelSize, _ = strconv.Atoi(os.Getenv("channel_size"))
	var auditChannel = make(chan types.AuditRecord, defaultChannelSize)
	var logChannel = make(chan map[string]interface{}, defaultChannelSize)
	var stopChannel = make(chan bool)
	workers.InitWorkers(configuration, &auditChannel, &logChannel, &stopChannel, db)
	defer workers.StopWorkers(&stopChannel)

	// Init echo framework
	e := echo.New()

	// Creating handler with pointers to persistent data
	appHandler := handlers.NewAppHandler(configuration, auditChannel, logChannel, db, permEnforcer)

	// Middlewares
	e.Use(middleware.Logger())

	// Routes (live test if application crash, ready test backend services)
	e.GET("/status/live", handlers.StatusLive)
	e.GET("/status/ready", handlers.StatusReady)
	e.GET("/status/config", appHandler.StatusConfig)
	e.GET("/publickey", appHandler.PublicKey)
	e.GET("/certificates/*", appHandler.CertInfo)
	e.POST("/certificates", appHandler.CertCreate)

	e.GET("/authz/roles/me", appHandler.GetRolesForMe)
	e.GET("/authz/roles", appHandler.GetRoles)
	e.GET("/authz/roles/:role", appHandler.GetUsersWithRole)
	e.POST("/authz/roles", appHandler.AddRoles)
	e.DELETE("/authz/roles/:role", appHandler.RemoveRole)
	e.GET("/authz/user/:user", appHandler.GetRolesByUser)
	e.POST("/authz/roles/:role/:user", appHandler.AssociateRoleToUser)
	e.DELETE("/authz/roles/:role/:user", appHandler.DisassociateRoleToUser)

	e.Logger.Fatal(e.Start(":" + os.Getenv("PORT")))
}
