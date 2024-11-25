package main

import (
	"context"
	aws "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"log"
	"log/slog"
	"os"
)

func main() {
	ctx := context.Background()
	traceProvider, loggerProvider, err := NewOtelProviders(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer traceProvider.Shutdown(ctx)
	defer loggerProvider.Shutdown(ctx)

	// keep this one if you dont want to use slogmulti
	//slog.SetDefault(otelslog.NewLogger("api.cloudificando.com", otelslog.WithLoggerProvider(loggerProvider)))
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
	otelHandler := otelslog.NewHandler("api.cloudificando.com", otelslog.WithLoggerProvider(loggerProvider))
	slog.SetDefault(slog.New(slogmulti.Fanout(consoleHandler, otelHandler)))

	awsConfig, err := aws.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}
	otelaws.AppendMiddlewares(&awsConfig.APIOptions)
	db := dynamodb.NewFromConfig(awsConfig)

	slog.Info("Cold start")
	router := gin.New()
	router.Use(otelgin.Middleware("api.cloudificando.com"))

	router.GET("/tables", func(c *gin.Context) {
		ctx := c.Request.Context()
		listOutput, err := db.ListTables(ctx, &dynamodb.ListTablesInput{})
		if err != nil {
			slog.ErrorContext(ctx, "Failed to list tables", err)
			c.AbortWithStatusJSON(500, gin.H{
				"error": "Unexpected error",
			})
			return
		}
		slog.InfoContext(ctx, "Tables", slog.Any("tables", listOutput.TableNames))
		c.JSON(200, gin.H{
			"tables": listOutput.TableNames,
		})
	})

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
		slog.InfoContext(c.Request.Context(), "Ping endpoint accessed")
	})

	router.GET("/ping/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
			"id":      c.Param("id"),
		})
		slog.InfoContext(c.Request.Context(), "Ping endpoint accessed with id", slog.String("id", c.Param("id")))
	})

	router.Run()
}
