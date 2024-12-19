package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
)

var (
	db       *sql.DB
)

type todo struct {
	ID        int    `json:"id"`
	Item      string `json:"item"`
	Completed bool   `json:"completed"`
}


func parseValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var result string
		for _, fieldError := range validationErrors {
			result += fmt.Sprintf(
				"Field validation for '%s' failed: '%s' (condition: %s)\n",
				fieldError.Field(),
				fieldError.ActualTag(),
				fieldError.Param(),
			)
		}
		return result
	}
	return "an unknown validation error occurred"
}

func parseIDParam(ginContext *gin.Context) (int64, error) {
	idParam := ginContext.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id format")
	}
	return id, nil
}

type todoPayload struct {
	Item      string `json:"item" binding:"required,max=100,min=2"`
	Completed bool   `json:"completed"`
}

func createTodo(ginContext *gin.Context) {
	var payload todoPayload

	if err := ginContext.ShouldBindJSON(&payload); err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": parseValidationError(err)})
		return
	}

	result, err := db.Exec("INSERT INTO todos (item, completed) VALUES (?, ?)", payload.Item, payload.Completed)
	if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	ginContext.JSON(http.StatusCreated, gin.H{"id": id, "item": payload.Item, "completed": payload.Completed})
}

func getTodos(ginContext *gin.Context) {
rows, err := db.Query("SELECT id, item, completed FROM todos")
	if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var todos = []todo{}
	for rows.Next() {
		var t todo
		if err := rows.Scan(&t.ID, &t.Item, &t.Completed); err != nil {
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		todos = append(todos, t)
	}

	ginContext.JSON(http.StatusOK, todos)
}

func getTodo(ginContext *gin.Context) {
	id, err := parseIDParam(ginContext)
	if err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var todo todo
	err = db.QueryRow("SELECT id, item, completed FROM todos WHERE id = ?", id).Scan(
		&todo.ID, &todo.Item, &todo.Completed,
	)

	if err == sql.ErrNoRows {
		ginContext.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		return
	} else if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ginContext.JSON(http.StatusOK, todo)
}

func toggleTodoStatus(ginContext *gin.Context) {
	id, err := parseIDParam(ginContext)
	if err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var todo todo
	err = db.QueryRow("SELECT id, item, completed FROM todos WHERE id = ?", id).Scan(
		&todo.ID, &todo.Item, &todo.Completed,
	)

	if err == sql.ErrNoRows {
		ginContext.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		return
	} else if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newStatus := !todo.Completed
	_, err = db.Exec("UPDATE todos SET completed = ? WHERE id = ?", newStatus, id)
	if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"id": todo.ID, "item": todo.Item, "completed": newStatus})
}

func updateTodo(ginContext *gin.Context) {
	id, err := parseIDParam(ginContext)
	if err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var payload todoPayload
	if err := ginContext.ShouldBindJSON(&payload); err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": parseValidationError(err)})
		return
	}

	result, err := db.Exec("UPDATE todos SET item = ?, completed = ? WHERE id = ?", payload.Item, payload.Completed, id)
	if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		ginContext.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"id": id, "item": payload.Item, "completed": payload.Completed})
}

func deleteTodo(ginContext *gin.Context) {
	id, err := parseIDParam(ginContext)
	if err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var deletedTodo todo
	err = db.QueryRow("SELECT id, item, completed FROM todos WHERE id = ?", id).Scan(
		&deletedTodo.ID, &deletedTodo.Item, &deletedTodo.Completed,
	)
	if err == sql.ErrNoRows {
		ginContext.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		return
	} else if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = db.Exec("DELETE FROM todos WHERE id = ?", id)
	if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ginContext.IndentedJSON(http.StatusOK, deletedTodo)
}

func main() {
	var err error
	db, err = sql.Open("mysql", "admin:adminpassword@tcp(localhost:3306)/app_db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		panic(err)
	}

	fmt.Println("Connected to MySQL")

	router := gin.Default()
	router.GET("/todos", getTodos)
	router.POST("/todos", createTodo)
	router.GET("/todos/:id", getTodo)
	router.PATCH("/todos/:id", toggleTodoStatus)
	router.PUT("/todos/:id", updateTodo)
	router.DELETE("/todos/:id", deleteTodo)
	router.Run("localhost:9191")
}