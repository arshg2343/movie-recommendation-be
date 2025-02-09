package main

import (
	"example/movie-search/handler"

	"github.com/gin-gonic/gin"
)

func main() {
    server:=gin.Default()

    server.GET("/", handler.UserPromptHandle)

    server.Run(":8080")

}


