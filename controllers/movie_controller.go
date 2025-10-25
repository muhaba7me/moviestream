package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/muhaba7me/moviestream/database"
	"github.com/muhaba7me/moviestream/models"
	"github.com/muhaba7me/moviestream/utils"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var validate = validator.New()

func GetMovies(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c, 100*time.Second)
		defer cancel()

		var movies []models.Movie
		var movieCollection *mongo.Collection = database.OpenCollection("movies", client)
		cursor, err := movieCollection.Find(ctx, bson.M{})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error to fetch movies"})
		}
		defer cursor.Close(ctx)
		if err = cursor.All(ctx, &movies); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed dcecode the movies "})
		}
		c.JSON(http.StatusOK, movies)
	}
}
func GetMovie(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c, 100*time.Second)
		defer cancel()

		movieID := c.Param("imdb_id")

		if movieID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Movie ID is required"})
			return
		}

		var movieCollection *mongo.Collection = database.OpenCollection("movies", client)

		var movie models.Movie

		err := movieCollection.FindOne(ctx, bson.D{{Key: "imdb_id", Value: movieID}}).Decode(&movie)

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
			return
		}

		c.JSON(http.StatusOK, movie)

	}
}
func AddMovie(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c, 100*time.Second)
		defer cancel()

		var movie models.Movie
		if err := c.ShouldBindJSON(&movie); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		if err := validate.Struct(movie); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
			return
		}
		var movieCollection *mongo.Collection = database.OpenCollection("movies", client)

		result, err := movieCollection.InsertOne(ctx, movie)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add movie"})
			return
		}

		c.JSON(http.StatusCreated, result)

	}
}

func AdminReviewUpdate(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		movieId := c.Param("imdb_id")

		role, err := utils.GetRoleFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Role not found in context"})
			return
		}

		if role != "ADMIN" {
			c.JSON(http.StatusForbidden, gin.H{"error": "user must be part of the admin role"})
			return
		}

		var req struct {
			AdminReview string `json:"admin_review"`
		}

		var resp struct {
			RankingName string `json:"ranking_name"`
			AdminReview string `json:"admin_review"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		sentiment, rankVal, err := GetReviewRanking(req.AdminReview, client, c)
		if err != nil {
			log.Printf("Error getting review ranking: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting review ranking"})
			return
		}

		filter := bson.M{"imdb_id": movieId}
		update := bson.M{
			"$set": bson.M{
				"admin_review":          req.AdminReview,
				"ranking.ranking_name":  sentiment,
				"ranking.ranking_value": rankVal,
			},
		}

		ctx, cancel := context.WithTimeout(c, 100*time.Second)
		defer cancel()
		var movieCollection *mongo.Collection = database.OpenCollection("movies", client)
		result, err := movieCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			log.Printf("Failed to update movie %s: %v", movieId, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update movie"})
			return
		}

		if result.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
			return
		}

		resp.AdminReview = req.AdminReview
		resp.RankingName = sentiment

		c.JSON(http.StatusOK, resp)
	}
}

// Simple keyword-based sentiment analysis (no API needed)
func GetReviewRanking(admin_review string, client *mongo.Client, c *gin.Context) (string, int, error) {
	log.Println("=== Starting GetReviewRanking ===")
	log.Printf("Admin review: %s", admin_review)

	rankings, err := GetRanking(client, c)
	if err != nil {
		log.Printf("ERROR: GetRanking failed: %v", err)
		return "", 0, err
	}
	log.Printf("Retrieved %d rankings", len(rankings))

	// Analyze sentiment using keywords
	sentiment := analyzeSentiment(admin_review)
	log.Printf("Detected sentiment: %s", sentiment)

	// Find matching ranking value
	rankVal := 0
	for _, ranking := range rankings {
		if strings.EqualFold(ranking.RankingName, sentiment) {
			rankVal = ranking.RankingValue
			log.Printf("Matched ranking: %s = %d", ranking.RankingName, rankVal)
			break
		}
	}

	if rankVal == 0 {
		log.Printf("ERROR: Sentiment '%s' not found in rankings", sentiment)
		return "", 0, errors.New("sentiment not found in rankings")
	}

	log.Println("=== GetReviewRanking completed successfully ===")
	return sentiment, rankVal, nil
}

// Simple keyword-based sentiment analysis
func analyzeSentiment(review string) string {
	reviewLower := strings.ToLower(review)

	// Define keyword sets for each sentiment
	excellentKeywords := []string{
		"excellent", "outstanding", "masterpiece", "brilliant", "amazing",
		"fantastic", "superb", "phenomenal", "incredible", "perfect",
		"magnificent", "extraordinary", "exceptional", "stunning", "wonderful",
	}

	goodKeywords := []string{
		"good", "great", "nice", "enjoyable", "solid", "decent", "fine",
		"satisfying", "pleasant", "impressive", "entertaining", "compelling",
	}

	badKeywords := []string{
		"bad", "poor", "weak", "disappointing", "mediocre", "boring",
		"dull", "lackluster", "uninspired", "forgettable", "waste",
	}

	terribleKeywords := []string{
		"terrible", "awful", "horrible", "worst", "pathetic", "garbage",
		"trash", "atrocious", "abysmal", "dreadful", "unwatchable",
	}

	// Count keyword matches
	excellentScore := countKeywords(reviewLower, excellentKeywords)
	goodScore := countKeywords(reviewLower, goodKeywords)
	badScore := countKeywords(reviewLower, badKeywords)
	terribleScore := countKeywords(reviewLower, terribleKeywords)

	log.Printf("Scores - Excellent: %d, Good: %d, Bad: %d, Terrible: %d",
		excellentScore, goodScore, badScore, terribleScore)

	// Calculate net sentiment scores
	positiveScore := excellentScore + goodScore
	negativeScore := badScore + terribleScore

	// Determine sentiment in order: Excellent > Good > Okay > Bad > Terrible
	if positiveScore > negativeScore {
		// More positive keywords found
		if excellentScore >= goodScore && excellentScore > 0 {
			return "Excellent"
		}
		if goodScore > 0 {
			return "Good"
		}
	} else if negativeScore > positiveScore {
		// More negative keywords found
		if terribleScore >= badScore && terribleScore > 0 {
			return "Terrible"
		}
		if badScore > 0 {
			return "Bad"
		}
	}

	// Default to "Okay" if scores are equal or no keywords found
	return "Okay"
}

// Helper function to count keyword matches
func countKeywords(text string, keywords []string) int {
	count := 0
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			count++
		}
	}
	return count
}
func GetRanking(client *mongo.Client, c *gin.Context) ([]models.Ranking, error) {
	var rankings []models.Ranking
	var ctx, cancel = context.WithTimeout(c, 100*time.Second)
	defer cancel()
	var rankingsCollection *mongo.Collection = database.OpenCollection("rankings", client)
	cursor, err := rankingsCollection.Find(ctx, bson.M{})

	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &rankings); err != nil {
		return nil, err
	}
	return rankings, nil
}

func GetRecommendedMovies(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := utils.GetUserIdFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User Id not found in context"})
			return
		}
		favourite_genres, err := GetUsersFavouriteGenres(userId,client, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Get limit from environment variable (loaded once at startup)
		var recommendedMovieLimitVal int64 = 5
		if recommendedMovieLimitStr := os.Getenv("RECOMMENDED_MOVIE_LIMIT"); recommendedMovieLimitStr != "" {
			if val, err := strconv.ParseInt(recommendedMovieLimitStr, 10, 64); err == nil {
				recommendedMovieLimitVal = val
			} else {
				log.Printf("Invalid RECOMMENDED_MOVIE_LIMIT value: %v", err)
			}
		}

		// Set up query options
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{Key: "ranking.ranking_value", Value: 1}}) // -1 for highest first
		findOptions.SetLimit(recommendedMovieLimitVal)
		filter := bson.M{"genre.genre_name": bson.M{"$in": favourite_genres}}

		ctx, cancel := context.WithTimeout(c, 100*time.Second)
		defer cancel()
		var movieCollection *mongo.Collection = database.OpenCollection("movies", client)

		cursor, err := movieCollection.Find(ctx, filter, findOptions)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching recommended movies"})
			return
		}
		defer cursor.Close(ctx)

		var recommendedMovies []models.Movie
		if err := cursor.All(ctx, &recommendedMovies); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, recommendedMovies)
	}
}

func GetUsersFavouriteGenres(userId string, client *mongo.Client, c *gin.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(c , 100*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userId}

	projection := bson.M{
		"favourite_genres.genre_name": 1,
		"_id":                         0,
	}
	opts := options.FindOne().SetProjection(projection)
	var result bson.M
	var userCollection *mongo.Collection = database.OpenCollection("users", client)
	err := userCollection.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return []string{}, nil
		}
		return nil, err
	}
	favGenresArray, ok := result["favourite_genres"].(bson.A)

	if !ok {
		return []string{}, errors.New("unable to retrieve favourite genres for user")
	}
	var genreNames []string
	for _, item := range favGenresArray {
		if genreMap, ok := item.(bson.D); ok {
			for _, elem := range genreMap {
				if elem.Key == "genre_name" {
					if name, ok := elem.Value.(string); ok {
						genreNames = append(genreNames, name)

					}
				}
			}

		}
	}
	return genreNames, nil
}

func GetGenres(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(c, 100*time.Second)
		defer cancel()

		var genreCollection *mongo.Collection = database.OpenCollection("genres", client)

		cursor, err := genreCollection.Find(ctx, bson.D{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching movie genres"})
			return
		}
		defer cursor.Close(ctx)

		var genres []models.Genre
		if err := cursor.All(ctx, &genres); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, genres)

	}
}
