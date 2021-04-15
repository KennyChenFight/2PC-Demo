package main

import (
	"github.com/KennyChenFight/golib/loglib"
	"github.com/KennyChenFight/golib/uuidlib"
	"github.com/gin-gonic/gin"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"sync"
)

type GinConfig struct {
	Port string `long:"port" description:"port" env:"PORT" default:"8081"`
	Mode string `long:"mode" description:"mode" env:"MODE" default:"debug"`
}

type Environment struct {
	GinConfig GinConfig `group:"gin" namespace:"gin" env-namespace:"GIN"`
}

type Charge struct {
	ID      string `json:"id"`
	UserID  int    `json:"userId"`
	Money   int    `json:"money"`
	Capture bool   `json:"capture"`
}

var MemoryStore = sync.Map{}

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
			MemoryStore.Store(charge.ID, charge)
			logger.Info("charges", zap.Any("charge", charge))
			c.JSON(http.StatusCreated, gin.H{"id": charge.ID})
		})

		v1Group.PATCH("/charges/:id", func(c *gin.Context) {
			id := c.Param("id")
			obj, ok := MemoryStore.Load(id)
			if ok {
				charge := obj.(Charge)
				if !charge.Capture {
					charge.Capture = true
					MemoryStore.Store(id, charge)
					logger.Info("charges", zap.Any("charge", charge))
					c.JSON(http.StatusNoContent, nil)
				} else {
					c.JSON(http.StatusConflict, gin.H{"message": "already charge this user"})
				}
			} else {
				c.JSON(http.StatusNotFound, gin.H{"message": "can not find charge"})
			}
		})
	}
	router.Run(":" + env.GinConfig.Port)
}
