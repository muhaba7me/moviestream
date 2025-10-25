package routes

import (
	"github.com/gin-gonic/gin"
	controller "github.com/muhaba7me/moviestream/controllers"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func SetupUnProtectedRoutes(router *gin.Engine, client *mongo.Client) {
	router.POST("/register", controller.RegisterUser(client))
	router.POST("/login", controller.LoginUser(client))
	router.POST("/logout", controller.LogoutHandler(client))
	router.GET("/movies", controller.GetMovies(client))
	router.POST("/refresh", controller.RefreshTokenHandler(client))
	router.GET("/genres", controller.GetGenres(client))

}
