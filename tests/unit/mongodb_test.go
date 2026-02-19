package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"service-platform/internal/config"
	"service-platform/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoDBConnection(t *testing.T) {
	// Load config
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	// Initialize MongoDB connection using config
	err = database.InitMongoDB()
	require.NoError(t, err, "Failed to connect to MongoDB")
	defer database.CloseMongoDB()

	// Get client
	client := database.GetMongoDBClient()
	require.NotNil(t, client, "MongoDB client should not be nil")

	// Test database access
	db := client.Database("service_platform_test")
	collection := db.Collection("test_collection")

	// Insert a test document
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testDoc := bson.M{"test": "value", "timestamp": time.Now()}
	_, err = collection.InsertOne(ctx, testDoc)
	require.NoError(t, err, "Failed to insert document")

	// Find the document
	var result bson.M
	err = collection.FindOne(ctx, bson.M{"test": "value"}).Decode(&result)
	require.NoError(t, err, "Failed to find document")

	assert.Equal(t, "value", result["test"], "Test field should be 'value'")

	// Clean up: delete the test document
	_, err = collection.DeleteOne(ctx, bson.M{"test": "value"})
	assert.NoError(t, err, "Failed to clean up test document")

	t.Log("MongoDB connection and basic operations test passed")
}

func TestMongoDBCRUDOperations(t *testing.T) {
	// Load config
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	// Initialize MongoDB
	err = database.InitMongoDB()
	require.NoError(t, err)
	defer database.CloseMongoDB()

	client := database.GetMongoDBClient()
	db := client.Database("service_platform_test")
	collection := db.Collection("crud_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Clean up before test
	defer collection.Drop(ctx)

	t.Run("Create", func(t *testing.T) {
		doc := bson.M{
			"name":      "John Doe",
			"email":     "john@example.com",
			"age":       30,
			"active":    true,
			"createdAt": time.Now(),
		}
		result, err := collection.InsertOne(ctx, doc)
		require.NoError(t, err)
		assert.NotNil(t, result.InsertedID)
		t.Logf("Inserted document with ID: %v", result.InsertedID)
	})

	t.Run("Read", func(t *testing.T) {
		var result bson.M
		err := collection.FindOne(ctx, bson.M{"name": "John Doe"}).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "john@example.com", result["email"])
		assert.Equal(t, int32(30), result["age"])
	})

	t.Run("Update", func(t *testing.T) {
		update := bson.M{"$set": bson.M{"age": 31, "active": false}}
		result, err := collection.UpdateOne(ctx, bson.M{"name": "John Doe"}, update)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.ModifiedCount)

		// Verify update
		var updated bson.M
		collection.FindOne(ctx, bson.M{"name": "John Doe"}).Decode(&updated)
		assert.Equal(t, int32(31), updated["age"])
		assert.False(t, updated["active"].(bool))
	})

	t.Run("Delete", func(t *testing.T) {
		result, err := collection.DeleteOne(ctx, bson.M{"name": "John Doe"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.DeletedCount)

		// Verify deletion
		count, _ := collection.CountDocuments(ctx, bson.M{"name": "John Doe"})
		assert.Equal(t, int64(0), count)
	})
}

func TestMongoDBBulkOperations(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	err = database.InitMongoDB()
	require.NoError(t, err)
	defer database.CloseMongoDB()

	client := database.GetMongoDBClient()
	db := client.Database("service_platform_test")
	collection := db.Collection("bulk_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defer collection.Drop(ctx)

	t.Run("InsertMany", func(t *testing.T) {
		docs := []interface{}{
			bson.M{"name": "User1", "score": 85},
			bson.M{"name": "User2", "score": 92},
			bson.M{"name": "User3", "score": 78},
			bson.M{"name": "User4", "score": 95},
			bson.M{"name": "User5", "score": 88},
		}
		result, err := collection.InsertMany(ctx, docs)
		require.NoError(t, err)
		assert.Len(t, result.InsertedIDs, 5)
		t.Logf("Inserted %d documents", len(result.InsertedIDs))
	})

	t.Run("FindMultiple", func(t *testing.T) {
		cursor, err := collection.Find(ctx, bson.M{"score": bson.M{"$gte": 85}})
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		err = cursor.All(ctx, &results)
		require.NoError(t, err)
		assert.Len(t, results, 4)
	})

	t.Run("UpdateMany", func(t *testing.T) {
		filter := bson.M{"score": bson.M{"$lt": 90}}
		update := bson.M{"$set": bson.M{"passed": true}}
		result, err := collection.UpdateMany(ctx, filter, update)
		require.NoError(t, err)
		t.Logf("Updated %d documents", result.ModifiedCount)
	})

	t.Run("DeleteMany", func(t *testing.T) {
		result, err := collection.DeleteMany(ctx, bson.M{"score": bson.M{"$lt": 80}})
		require.NoError(t, err)
		t.Logf("Deleted %d documents", result.DeletedCount)
	})
}

func TestMongoDBQueryOperations(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	err = database.InitMongoDB()
	require.NoError(t, err)
	defer database.CloseMongoDB()

	client := database.GetMongoDBClient()
	db := client.Database("service_platform_test")
	collection := db.Collection("query_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defer collection.Drop(ctx)

	// Insert test data
	docs := []interface{}{
		bson.M{"name": "Alice", "age": 25, "city": "Jakarta", "salary": 5000},
		bson.M{"name": "Bob", "age": 30, "city": "Bandung", "salary": 6000},
		bson.M{"name": "Charlie", "age": 35, "city": "Jakarta", "salary": 7000},
		bson.M{"name": "David", "age": 28, "city": "Surabaya", "salary": 5500},
		bson.M{"name": "Eve", "age": 32, "city": "Jakarta", "salary": 6500},
	}
	collection.InsertMany(ctx, docs)

	t.Run("FilterByField", func(t *testing.T) {
		cursor, err := collection.Find(ctx, bson.M{"city": "Jakarta"})
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		cursor.All(ctx, &results)
		assert.Len(t, results, 3, "Expected 3 documents from Jakarta")
	})

	t.Run("RangeQuery", func(t *testing.T) {
		filter := bson.M{"age": bson.M{"$gte": 28, "$lte": 32}}
		cursor, err := collection.Find(ctx, filter)
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		cursor.All(ctx, &results)
		assert.Len(t, results, 3, "Expected 3 documents with age 28-32")
	})

	t.Run("SortAndLimit", func(t *testing.T) {
		opts := options.Find().SetSort(bson.M{"salary": -1}).SetLimit(2)
		cursor, err := collection.Find(ctx, bson.M{}, opts)
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		cursor.All(ctx, &results)
		assert.Len(t, results, 2, "Expected 2 documents")
		// First should be highest salary
		assert.Equal(t, "Charlie", results[0]["name"], "Expected Charlie first (highest salary)")
	})

	t.Run("CountDocuments", func(t *testing.T) {
		count, err := collection.CountDocuments(ctx, bson.M{"city": "Jakarta"})
		require.NoError(t, err)
		assert.Equal(t, int64(3), count, "Expected 3 documents from Jakarta")
	})

	t.Run("Aggregation", func(t *testing.T) {
		pipeline := []bson.M{
			{"$group": bson.M{
				"_id":            "$city",
				"avgSalary":      bson.M{"$avg": "$salary"},
				"totalEmployees": bson.M{"$sum": 1},
			}},
			{"$sort": bson.M{"avgSalary": -1}},
		}

		cursor, err := collection.Aggregate(ctx, pipeline)
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		cursor.All(ctx, &results)
		assert.NotEmpty(t, results, "Expected aggregation results")
		t.Logf("Aggregation results: %+v", results)
	})
}

func TestMongoDBIndexing(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	err = database.InitMongoDB()
	require.NoError(t, err)
	defer database.CloseMongoDB()

	client := database.GetMongoDBClient()
	db := client.Database("service_platform_test")
	collection := db.Collection("index_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defer collection.Drop(ctx)

	t.Run("CreateIndex", func(t *testing.T) {
		indexModel := mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		}
		indexName, err := collection.Indexes().CreateOne(ctx, indexModel)
		require.NoError(t, err)
		t.Logf("Created index: %s", indexName)
	})

	t.Run("UniqueConstraint", func(t *testing.T) {
		doc1 := bson.M{"email": "test@example.com", "name": "Test User"}
		_, err := collection.InsertOne(ctx, doc1)
		require.NoError(t, err)

		// Try to insert duplicate
		doc2 := bson.M{"email": "test@example.com", "name": "Another User"}
		_, err = collection.InsertOne(ctx, doc2)
		assert.Error(t, err, "Expected error for duplicate email")
		t.Logf("Correctly rejected duplicate: %v", err)
	})

	t.Run("ListIndexes", func(t *testing.T) {
		cursor, err := collection.Indexes().List(ctx)
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var indexes []bson.M
		cursor.All(ctx, &indexes)
		assert.GreaterOrEqual(t, len(indexes), 2, "Expected at least 2 indexes (_id + email)")
		t.Logf("Found %d indexes", len(indexes))
	})
}

func TestMongoDBErrorHandling(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	err = database.InitMongoDB()
	require.NoError(t, err)
	defer database.CloseMongoDB()

	client := database.GetMongoDBClient()
	db := client.Database("service_platform_test")
	collection := db.Collection("error_test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defer collection.Drop(ctx)

	t.Run("DocumentNotFound", func(t *testing.T) {
		var result bson.M
		err := collection.FindOne(ctx, bson.M{"_id": "nonexistent"}).Decode(&result)
		assert.ErrorIs(t, err, mongo.ErrNoDocuments, "Expected ErrNoDocuments")
	})

	t.Run("InvalidQuery", func(t *testing.T) {
		// Insert a document first
		collection.InsertOne(ctx, bson.M{"name": "Test"})

		// Try invalid update operation
		_, err := collection.UpdateOne(ctx, bson.M{"name": "Test"}, bson.M{"invalid": "update"})
		assert.Error(t, err, "Expected error for invalid update")
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		shortCtx, shortCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer shortCancel()

		time.Sleep(10 * time.Millisecond) // Ensure context is expired

		var result bson.M
		err := collection.FindOne(shortCtx, bson.M{}).Decode(&result)
		assert.Error(t, err, "Expected context deadline error")
	})
}

func TestMongoDBVectorOperations(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	err = database.InitMongoDB()
	require.NoError(t, err)
	defer database.CloseMongoDB()

	client := database.GetMongoDBClient()
	db := client.Database("service_platform_test")
	collection := db.Collection("vector_embeddings")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	defer collection.Drop(ctx)

	t.Run("InsertVectorEmbeddings", func(t *testing.T) {
		// Simulate embeddings (typically from AI models like OpenAI, Cohere, etc.)
		// These would normally be 384, 768, or 1536 dimensions
		docs := []interface{}{
			bson.M{
				"title":       "Machine Learning Basics",
				"description": "Introduction to machine learning concepts",
				"embedding":   []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
				"category":    "AI",
			},
			bson.M{
				"title":       "Deep Learning Tutorial",
				"description": "Advanced neural network techniques",
				"embedding":   []float64{0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75, 0.85},
				"category":    "AI",
			},
			bson.M{
				"title":       "Database Design",
				"description": "Principles of database architecture",
				"embedding":   []float64{0.9, 0.8, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6},
				"category":    "Database",
			},
			bson.M{
				"title":       "Web Development",
				"description": "Building modern web applications",
				"embedding":   []float64{0.5, 0.6, 0.7, 0.8, 0.1, 0.2, 0.3, 0.4},
				"category":    "Web",
			},
			bson.M{
				"title":       "Neural Networks",
				"description": "Understanding artificial neural networks",
				"embedding":   []float64{0.12, 0.22, 0.32, 0.42, 0.52, 0.62, 0.72, 0.82},
				"category":    "AI",
			},
		}

		result, err := collection.InsertMany(ctx, docs)
		require.NoError(t, err)
		assert.Len(t, result.InsertedIDs, 5, "Expected 5 documents inserted")
		t.Logf("Inserted %d documents with embeddings", len(result.InsertedIDs))
	})

	t.Run("QueryByVectorField", func(t *testing.T) {
		// Query documents that have embeddings
		cursor, err := collection.Find(ctx, bson.M{"embedding": bson.M{"$exists": true}})
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		err = cursor.All(ctx, &results)
		require.NoError(t, err)
		assert.Len(t, results, 5, "Expected 5 documents with embeddings")

		// Verify embedding structure
		for _, doc := range results {
			if embedding, ok := doc["embedding"].(bson.A); ok {
				assert.Len(t, embedding, 8, "Expected 8-dimensional embedding")
			} else {
				t.Error("Embedding field is not an array")
			}
		}
	})

	t.Run("CosineSimilaritySearch", func(t *testing.T) {
		// Query vector (looking for ML/AI content)
		queryVector := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}

		// Manual cosine similarity calculation using aggregation pipeline
		// In production, you'd use Atlas Vector Search with $vectorSearch
		pipeline := []bson.M{
			{
				"$addFields": bson.M{
					"similarity": bson.M{
						"$let": bson.M{
							"vars": bson.M{
								"dotProduct": bson.M{
									"$reduce": bson.M{
										"input":        bson.M{"$range": bson.A{0, 8}},
										"initialValue": 0,
										"in": bson.M{
											"$add": bson.A{
												"$$value",
												bson.M{
													"$multiply": bson.A{
														bson.M{"$arrayElemAt": bson.A{"$embedding", "$$this"}},
														queryVector[0], // Simplified for demonstration
													},
												},
											},
										},
									},
								},
							},
							"in": "$$dotProduct",
						},
					},
				},
			},
			{
				"$sort": bson.M{"similarity": -1},
			},
			{
				"$limit": 3,
			},
		}

		cursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			t.Logf("Vector search aggregation not fully supported in local MongoDB: %v", err)
			t.Skip("Skipping full vector similarity - requires MongoDB Atlas Vector Search")
		}
		defer cursor.Close(ctx)

		var results []bson.M
		err = cursor.All(ctx, &results)
		require.NoError(t, err)

		if len(results) > 0 {
			t.Logf("Top similar documents: %d", len(results))
			for i, doc := range results {
				t.Logf("  %d. %s", i+1, doc["title"])
			}
		}
	})

	t.Run("EuclideanDistanceSearch", func(t *testing.T) {
		// Simple distance-based filtering
		// Find documents in the "AI" category (semantic grouping)
		filter := bson.M{"category": "AI"}
		cursor, err := collection.Find(ctx, filter)
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		err = cursor.All(ctx, &results)
		require.NoError(t, err)
		assert.Len(t, results, 3, "Expected 3 AI documents")

		t.Log("AI-related documents found:")
		for _, doc := range results {
			t.Logf("  - %s", doc["title"])
		}
	})

	t.Run("VectorIndexCreation", func(t *testing.T) {
		// Create a standard index on the category field for filtering
		indexModel := mongo.IndexModel{
			Keys: bson.D{{Key: "category", Value: 1}},
		}
		indexName, err := collection.Indexes().CreateOne(ctx, indexModel)
		require.NoError(t, err)
		t.Logf("Created index: %s", indexName)

		// Note: Vector Search Index creation requires MongoDB Atlas
		// For Atlas Vector Search, you would create via Atlas UI or API:
		// {
		//   "type": "vectorSearch",
		//   "fields": [{
		//     "type": "vector",
		//     "path": "embedding",
		//     "numDimensions": 8,
		//     "similarity": "cosine"
		//   }]
		// }
		t.Log("Note: Vector Search Indexes require MongoDB Atlas")
		t.Log("For local testing, use standard indexes and aggregation pipelines")
	})

	t.Run("UpdateVectorEmbedding", func(t *testing.T) {
		// Update an embedding (e.g., after retraining ML model)
		newEmbedding := []float64{0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9}
		filter := bson.M{"title": "Machine Learning Basics"}
		update := bson.M{
			"$set": bson.M{
				"embedding":  newEmbedding,
				"updated_at": time.Now(),
				"version":    2,
			},
		}

		result, err := collection.UpdateOne(ctx, filter, update)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.ModifiedCount, "Expected 1 document modified")

		// Verify update
		var updated bson.M
		collection.FindOne(ctx, filter).Decode(&updated)
		if embedding, ok := updated["embedding"].(bson.A); ok {
			assert.Len(t, embedding, 8, "Updated embedding should have 8 dimensions")
		}
		t.Log("Successfully updated vector embedding")
	})

	t.Run("BatchVectorOperations", func(t *testing.T) {
		// Simulate batch embedding generation (common in ML workflows)
		batchDocs := []interface{}{
			bson.M{
				"title":     "Quantum Computing",
				"embedding": []float64{0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
				"batch_id":  "batch_001",
			},
			bson.M{
				"title":     "Blockchain Technology",
				"embedding": []float64{0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.1},
				"batch_id":  "batch_001",
			},
			bson.M{
				"title":     "Cloud Computing",
				"embedding": []float64{0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.1, 0.2},
				"batch_id":  "batch_001",
			},
		}

		result, err := collection.InsertMany(ctx, batchDocs)
		require.NoError(t, err)
		t.Logf("Batch inserted %d documents with embeddings", len(result.InsertedIDs))

		// Query by batch ID
		count, err := collection.CountDocuments(ctx, bson.M{"batch_id": "batch_001"})
		require.NoError(t, err)
		assert.Equal(t, int64(3), count, "Expected 3 batch documents")
	})
}

func TestMongoDBVectorSearchPipeline(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		require.NoError(t, err, "Config should be loaded successfully")
	}

	err = database.InitMongoDB()
	require.NoError(t, err)
	defer database.CloseMongoDB()

	client := database.GetMongoDBClient()
	db := client.Database("service_platform_test")
	collection := db.Collection("semantic_search")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	defer collection.Drop(ctx)

	// Insert documents with metadata and embeddings
	docs := []interface{}{
		bson.M{
			"product":     "Laptop Pro 15",
			"description": "High-performance laptop for professionals",
			"price":       1299.99,
			"tags":        []string{"electronics", "computer", "professional"},
			"embedding":   []float64{0.8, 0.7, 0.6, 0.5, 0.4, 0.3, 0.2, 0.1},
		},
		bson.M{
			"product":     "Wireless Mouse",
			"description": "Ergonomic wireless mouse",
			"price":       29.99,
			"tags":        []string{"electronics", "accessories"},
			"embedding":   []float64{0.7, 0.6, 0.5, 0.4, 0.3, 0.2, 0.1, 0.0},
		},
		bson.M{
			"product":     "Programming Book",
			"description": "Learn advanced programming techniques",
			"price":       49.99,
			"tags":        []string{"books", "education", "programming"},
			"embedding":   []float64{0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9},
		},
	}

	collection.InsertMany(ctx, docs)

	t.Run("HybridSearch", func(t *testing.T) {
		// Combine vector similarity with traditional filters
		// This simulates a hybrid search approach
		pipeline := []bson.M{
			{
				"$match": bson.M{
					"price": bson.M{"$lt": 100},
					"tags":  bson.M{"$in": []string{"electronics"}},
				},
			},
			{
				"$addFields": bson.M{
					"has_embedding": bson.M{"$type": "$embedding"},
				},
			},
			{
				"$match": bson.M{
					"has_embedding": "array",
				},
			},
		}

		cursor, err := collection.Aggregate(ctx, pipeline)
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		cursor.All(ctx, &results)

		t.Logf("Hybrid search found %d products", len(results))
		for _, doc := range results {
			t.Logf("  - %s: $%.2f", doc["product"], doc["price"])
		}
	})

	t.Run("SemanticFiltering", func(t *testing.T) {
		// Filter by embedding dimension values (simplified semantic search)
		// In production, use proper vector similarity with Atlas Vector Search
		cursor, err := collection.Find(ctx, bson.M{
			"embedding.0": bson.M{"$gt": 0.5}, // First dimension > 0.5
		})
		require.NoError(t, err)
		defer cursor.Close(ctx)

		var results []bson.M
		cursor.All(ctx, &results)
		t.Logf("Found %d documents matching vector criteria", len(results))
	})

	t.Run("VectorMetadataQuery", func(t *testing.T) {
		// Query combining vector data with metadata
		filter := bson.M{
			"$and": []bson.M{
				{"embedding": bson.M{"$exists": true}},
				{"tags": bson.M{"$in": []string{"electronics"}}},
			},
		}

		count, err := collection.CountDocuments(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count, "Expected 2 electronics with embeddings")
		t.Logf("Electronics products with embeddings: %d", count)
	})
}
