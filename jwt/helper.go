package jwtSerivce

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)


var secretKey = []byte("secret-key")


func CreateToken(username string,email string,userId string) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, 
        jwt.MapClaims{ 
        "username": username, 
		"email" : email,
		"id":userId,
        "exp": time.Now().Add(time.Hour * 24).Unix(), 
        })

    tokenString, err := token.SignedString(secretKey)
    if err != nil {
    return "", err
    }

 return tokenString, nil
}


func VerifyToken(tokenString string) error {
   token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
      return secretKey, nil
   })

   if err != nil {
      return err
   }

   if !token.Valid {
      return fmt.Errorf("invalid token")
   }

   return nil
}


func GetUserIDFromToken(tokenString string) (string, error) {
   token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
      return secretKey, nil
   })

   if err != nil {
      return "", err
   }

   if !token.Valid {
      return "", fmt.Errorf("invalid token")
   }

   claims, ok := token.Claims.(jwt.MapClaims)
   if !ok {
      return "", fmt.Errorf("invalid token claims")
   }

   userId, ok := claims["id"].(string)
   if !ok {
      return "", fmt.Errorf("user id not found in token")
   }

   return userId, nil
}