package main

import (
	"context"
	"log"
	"os"
	
	"github.com/jackc/pgx/v5"
)

func main() {
	connStr := "host=localhost user=postgres password=postgres dbname=pos sslmode=disable"
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(context.Background())

	sql, err := os.ReadFile("scripts/dummy_data.sql")
	if err != nil {
		log.Fatalf("Unable to read file: %v\n", err)
	}

	_, err = conn.Exec(context.Background(), string(sql))
	if err != nil {
		log.Fatalf("Unable to execute query: %v\n", err)
	}
	log.Println("Successfully inserted dummy data!")
}
