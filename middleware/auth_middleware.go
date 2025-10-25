package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/muhaba7me/moviestream/utils"
)


func AuthMiddleware() gin.HandlerFunc{
return  func(c *gin.Context) {
	token, err := utils.GetAccessToken(c)
	if err !=nil{
		c.JSON(http.StatusUnauthorized, gin.H{"error":err.Error()})
		c.Abort() 
		return 
	}
	if token ==""{
		c.JSON(http.StatusUnauthorized, gin.H{"error":"Not token provided"})
		return 
	}
	claims, err := utils.ValidateToken(token)
	if err!=nil{
		c.JSON(http.StatusUnauthorized, gin.H{"error":"Invaild token"})
		c.Abort()
		return 
	}
	c.Set("userId", claims.UserId)
	c.Set("role", claims.Role)
	c.Next()
}


}