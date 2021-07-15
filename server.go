package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gofiber/fiber"
)

// MongoInstance contains the Mongo client and database objects
type MongoInstance struct {
	Client *mongo.Client
	Db     *mongo.Database
}

var mg MongoInstance

// Database settings (insert your own database name and connection URI)
const dbName = "go_todos"
const mongoURI = "mongodb://localhost:27017/" + dbName

// Todo struct
type Todo struct {
	ID        string `json:"id,omitempty" bson:"_id,omitempty"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

// Connect configures the MongoDB client and initializes the database connection.
// Source: https://www.mongodb.com/blog/post/quick-start-golang--mongodb--starting-and-setup
func Connect() error {
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	db := client.Database(dbName)

	if err != nil {
		return err
	}

	mg = MongoInstance{
		Client: client,
		Db:     db,
	}

	return nil
}

func main() {
	// Connect to the database
	if err := Connect(); err != nil {
		log.Fatal(err)
	}

	// Create a Fiber app
	app := fiber.New()

	// Get all todos records from MongoDB
	// Docs: https://docs.mongodb.com/manual/reference/command/find/
	app.Get("/", func(c *fiber.Ctx) error {
		// get all records as a cursor
		query := bson.D{{}}
		cursor, err := mg.Db.Collection("todos").Find(c.Context(), query)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		var todos []Todo = make([]Todo, 0)

		// iterate the cursor and decode each item into an Employee
		if err := cursor.All(c.Context(), &todos); err != nil {
			return c.Status(500).SendString(err.Error())

		}
		// return employees list in JSON format
		return c.JSON(todos)
	})

	// Insert a new employee into MongoDB
	// Docs: https://docs.mongodb.com/manual/reference/command/insert/
	app.Post("/", func(c *fiber.Ctx) error {
		collection := mg.Db.Collection("todos")

		// New Todo struct
		todo := new(Todo)
		// Parse body into struct
		if err := c.BodyParser(todo); err != nil {
			return c.Status(400).SendString(err.Error())
		}

		// force MongoDB to always set its own generated ObjectIDs
		todo.ID = ""

		// insert the record
		insertionResult, err := collection.InsertOne(c.Context(), todo)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		// get the just inserted record in order to return it as response
		filter := bson.D{{Key: "_id", Value: insertionResult.InsertedID}}
		createdRecord := collection.FindOne(c.Context(), filter)

		// decode the Mongo record into Todo
		createdTodo := &Todo{}
		createdRecord.Decode(createdTodo)

		// return the created Todo in JSON format
		return c.Status(201).JSON(createdTodo)
	})

	// Find one Todo record by ID
	// Docs: https://docs.mongodb.com/manual/reference/command/findOne/
	app.Get("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		todoId, err := primitive.ObjectIDFromHex(id)
		// the provided ID might be invalid ObjectID
		if err != nil {
			return c.SendStatus(400)
		}

		filter := bson.D{{Key: "_id", Value: todoId}}
		record := mg.Db.Collection("todos").FindOne(c.Context(), filter)
		if record == nil {
			return c.Status(404).SendString("Not found")
		}
		// decode the Mongo record into Todo
		todo := &Todo{}
		record.Decode(todo)
		return c.JSON(todo)
	})

	// Update an todo record in MongoDB
	// Docs: https://docs.mongodb.com/manual/reference/command/findAndModify/
	app.Put("/:id", func(c *fiber.Ctx) error {
		idParam := c.Params("id")
		todoID, err := primitive.ObjectIDFromHex(idParam)

		// the provided ID might be invalid ObjectID
		if err != nil {
			return c.SendStatus(400)
		}

		todo := new(Todo)
		// Parse body into struct
		if err := c.BodyParser(todo); err != nil {
			return c.Status(400).SendString(err.Error())
		}

		// Find the todo and update its data
		query := bson.D{{Key: "_id", Value: todoID}}
		update := bson.D{
			{Key: "$set",
				Value: bson.D{
					{Key: "text", Value: todo.Text},
					{Key: "completed", Value: todo.Completed},
				},
			},
		}
		err = mg.Db.Collection("todos").FindOneAndUpdate(c.Context(), query, update).Err()

		if err != nil {
			// ErrNoDocuments means that the filter did not match any documents in the collection
			if err == mongo.ErrNoDocuments {
				return c.SendStatus(404)
			}
			return c.SendStatus(500)
		}

		// return the updated todo
		todo.ID = idParam
		return c.Status(200).JSON(todo)
	})

	// Delete an Todo from MongoDB
	// Docs: https://docs.mongodb.com/manual/reference/command/delete/
	app.Delete("/:id", func(c *fiber.Ctx) error {
		todoID, err := primitive.ObjectIDFromHex(
			c.Params("id"),
		)

		// the provided ID might be invalid ObjectID
		if err != nil {
			return c.SendStatus(400)
		}

		// find and delete the employee with the given ID
		query := bson.D{{Key: "_id", Value: todoID}}
		result, err := mg.Db.Collection("todos").DeleteOne(c.Context(), &query)

		if err != nil {
			return c.SendStatus(500)
		}

		// the employee might not exist
		if result.DeletedCount < 1 {
			return c.SendStatus(404)
		}

		// the record was deleted
		return c.SendStatus(204)
	})

	log.Fatal(app.Listen(":4242"))
}
