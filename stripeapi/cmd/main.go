package main

import (
	"context"
	"errors"
	"github.com/KennyChenFight/golib/loglib"
	"github.com/KennyChenFight/golib/migrationlib"
	"github.com/KennyChenFight/golib/pglib"
	"github.com/KennyChenFight/golib/uuidlib"
	"github.com/gin-gonic/gin"
	"github.com/go-pg/pg/v10"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
)

type PostgresConfig struct {
	URL       string `long:"url" description:"the url of postgres" env:"URL" required:"true"`
	PoolSize  int    `long:"pool-size" description:"the poolSize of postgres" env:"POOL_SIZE" default:"5"`
	DebugMode bool   `long:"debug-mode" description:"the debugMode of postgres" env:"DEBUG_MODE"`
}

type MigrationConfig struct {
	SourceURL string `long:"source-url" description:"the source url of file" env:"SOURCE_URL" default:"file://stripeapi/migrations"`
}

type GinConfig struct {
	Port string `long:"port" description:"port" env:"PORT" default:"8081"`
	Mode string `long:"mode" description:"mode" env:"MODE" default:"debug"`
}

type Environment struct {
	GinConfig       GinConfig       `group:"gin" namespace:"gin" env-namespace:"GIN"`
	PostgresConfig  PostgresConfig  `group:"postgres" namespace:"postgres" env-namespace:"POSTGRES"`
	MigrationConfig MigrationConfig `group:"migration" namespace:"migration" env-namespace:"MIGRATION"`
}

type Charge struct {
	ID      string `json:"id"`
	UserID  int    `json:"userId"`
	Money   int    `json:"money"`
	Capture bool   `json:"capture"`
}

var ErrChargeNotFound = errors.New("charge not found")
var ErrChargeConflict = errors.New("already charge this user")

func main() {
	var env Environment
	parser := flags.NewParser(&env, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			log.Println("help error:", err)
			os.Exit(0)
		} else {
			log.Println("parse env error:", err)
			os.Exit(1)
		}
	}

	err := migrationlib.NewMigrateLib(migrationlib.Config{
		DatabaseDriver: migrationlib.PostgresDriver,
		DatabaseURL:    env.PostgresConfig.URL,
		SourceDriver:   migrationlib.FileDriver,
		SourceURL:      env.MigrationConfig.SourceURL,
		TableName:      "migration_versions",
	}).Up()
	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("fail to migrate:%v", err)
	}

	pgClient, err := pglib.NewDefaultGOPGClient(pglib.GOPGConfig{
		URL:       env.PostgresConfig.URL,
		DebugMode: env.PostgresConfig.DebugMode,
		PoolSize:  env.PostgresConfig.PoolSize,
	})
	if err != nil {
		log.Fatalf("fail to connect to postgres:%v", err)
	}

	logger, err := loglib.NewProductionLogger()
	if err != nil {
		log.Fatalf("fail to init logger")
	}

	router := gin.Default()
	v1Group := router.Group("/v1")
	{
		v1Group.POST("/charges", func(c *gin.Context) {
			var charge Charge
			if err := c.ShouldBindJSON(&charge); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err})
				return
			}
			charge.ID = uuidlib.NewV4().String()
			charge.Capture = false
			// do not charge, just store charge
			_, err := pgClient.Model(&charge).Insert()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
				return
			}
			logger.Info("charges", zap.Any("charge", charge))
			c.JSON(http.StatusCreated, gin.H{"id": charge.ID})
		})

		v1Group.PATCH("/charges/:id", func(c *gin.Context) {
			id := c.Param("id")
			charge := Charge{ID: id}
			err := pgClient.RunInTransaction(context.Background(), func(tx *pg.Tx) error {
				err := tx.Model(&charge).WherePK().For("UPDATE").Select()
				if err != nil {
					return err
				}
				if !charge.Capture {
					charge.Capture = true
					_, err = tx.Model(&charge).Set("capture = ?capture").WherePK().Update()
					if err != nil {
						return err
					}
				} else {
					return ErrChargeConflict
				}
				return nil
			})
			if err != nil {
				if err == pg.ErrNoRows {
					c.JSON(http.StatusNotFound, ErrChargeNotFound)
					return
				}
				if err == ErrChargeConflict {
					c.JSON(http.StatusConflict, ErrChargeConflict)
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			} else {
				logger.Info("charges", zap.Any("charge", charge))
				c.JSON(http.StatusNoContent, nil)
			}
		})
	}
	router.Run(":" + env.GinConfig.Port)
}
