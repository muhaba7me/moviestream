package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func Connect() *mongo.Client {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("Warning: Unable to find .env file")
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI not set in environment variables")
	}

	fmt.Println("MongoDB URI:", mongoURI)
	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}

	fmt.Println("Connected to MongoDB successfully!")
	return client
}


func OpenCollection(collectionName string, client *mongo.Client) *mongo.Collection{
	err :=godotenv.Load(".env")

	if err!=nil{
		log.Println("Warning: Unable to find .env file")
	}
	databaseName := os.Getenv("DATABASE_NAME")

	fmt.Println("DATABASE_NAME",databaseName)

	collection := client.Database(databaseName).Collection(collectionName)

	if collection == nil{
		return nil
	}
  return  collection
}