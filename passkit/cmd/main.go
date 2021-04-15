package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/KennyChenFight/golib/loglib"
	"github.com/KennyChenFight/golib/migrationlib"
	"github.com/KennyChenFight/golib/pglib"
	"github.com/KennyChenFight/golib/uuidlib"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"io/ioutil"
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
	SourceURL string `long:"source-url" description:"the source url of file" env:"SOURCE_URL" default:"file://passkit/migrations"`
}

type StripeClientConfig struct {
	Host string `long:"host" description:"the source url of file" env:"HOST" default:"http://localhost:8081"`
}

type GinConfig struct {
	Port string `long:"port" description:"port" env:"PORT" default:"8080"`
	Mode string `long:"mode" description:"mode" env:"MODE" default:"debug"`
}

type Environment struct {
	Crash              bool               `long:"crash" description:"crash" env:"CRASH"`
	GinConfig          GinConfig          `group:"gin" namespace:"gin" env-namespace:"GIN"`
	PostgresConfig     PostgresConfig     `group:"postgres" namespace:"postgres" env-namespace:"POSTGRES"`
	MigrationConfig    MigrationConfig    `group:"migration" namespace:"migration" env-namespace:"MIGRATION"`
	StripeClientConfig StripeClientConfig `group:"stripe" namespace:"stripe" env-namespace:"STRIPE"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Payment struct {
	ID       string `json:"id"`
	UserID   int    `json:"userId"`
	Money    int    `json:"money"`
	Status   string `json:"status"`
	ChargeID string `json:"chargeId"`
}

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
		v1Group.POST("/users", func(c *gin.Context) {
			var user User
			if err := c.ShouldBindJSON(&user); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err})
				return
			}
			_, err := pgClient.Model(&user).Insert()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"id": user.ID})
		})
		v1Group.POST("/payments", func(c *gin.Context) {
			var payment Payment
			if err := c.ShouldBindJSON(&payment); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err})
				return
			}
			payment.ID = uuidlib.NewV4().String()
			payment.Status = "initial"
			_, err := pgClient.Model(&payment).Insert()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err})
				return
			}

			// call stripe API to create charge object
			chargeID, err := stripeAPICreateCharge(env.StripeClientConfig.Host, payment.UserID, payment.Money)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
				return
			}
			logger.Info("charge success", zap.Any("chargeID", chargeID))

			payment.ChargeID = chargeID
			_, err = pgClient.Model(&payment).WherePK().Update()
			if err != nil {
				logger.Error("fail to update charge id in payment", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
				return
			}

			// call stripe API to update charge and charge user money
			err = stripeAPIUpdateCharge(env.StripeClientConfig.Host, chargeID)
			if err != nil && err != ErrAlreadyCharge {
				logger.Error("fail to update charge in stripeAPI", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
				return
			}

			payment.Status = "success"
			payment.ChargeID = chargeID
			_, err = pgClient.Model(&payment).WherePK().Update()
			if err != nil {
				logger.Error("fail to update payment to success status", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
				return
			}

			logger.Info("success pay!")
			c.JSON(http.StatusCreated, gin.H{"id": payment.ID})
		})
	}
	router.Run()
}

type ChargeRequest struct {
	UserID int `json:"userId"`
	Money  int `json:"money"`
}

func stripeAPICreateCharge(host string, userID int, money int) (string, error) {
	body := ChargeRequest{
		UserID: userID,
		Money:  money,
	}
	b, err := json.Marshal(&body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/charges", host), bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return "", errors.New("fail")
	}

	responseBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	var responseBody map[string]string
	err = json.Unmarshal(responseBodyBytes, &responseBody)
	if err != nil {
		return "", err
	}

	return responseBody["id"], nil
}

var ErrAlreadyCharge = errors.New("already charge this user")

func stripeAPIUpdateCharge(host string, chargeID string) error {
	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/charges/%s", host, chargeID), nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		if res.StatusCode == http.StatusConflict {
			return ErrAlreadyCharge
		} else if res.StatusCode == http.StatusBadRequest {
			return errors.New("can not find this charge")
		}
		return errors.New("unknown fail")
	}
	return nil
}
