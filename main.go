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


//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