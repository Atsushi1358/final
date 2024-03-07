package main

import (
	"os"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"github.com/gorilla/mux"
)


// PredictionRequest represents the structure of the incoming JSON request
type PredictionRequest struct {
	Date string `json:"date"`
}

// PredictionResponse represents the structure of the response JSON
type PredictionResponse struct {
	PredictedSales float64 `json:"predicted_sales"`
}

// PredictSales retrieves predictions from BigQuery ML model
func PredictSales(date string) (float64, error) {
	ctx := context.Background()

	// Initialize BigQuery client
	client, err := bigquery.NewClient(ctx, "final-412411")
	if err != nil {
		return 0, fmt.Errorf("failed to access bigquery: %v", err)
	}
	defer client.Close()

	// Prepare and execute SQL query
	query := fmt.Sprintf(`
		SELECT
			*
		FROM
			ML.EXPLAIN_FORECAST(MODEL `+"`final-412411.final_2024.total_sales_arima_model`"+`,
					 STRUCT(30 AS horizon, 0.8 AS confidence_level))
		WHERE
			date(time_series_timestamp) = '%s'
	`, date)

	// Run the query
	q := client.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query: %v", err)
	}

	// Parse the result
	// Parse the result
	var predictedSales float64
	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to parse query result: %v", err)
		}
		// Check if the "time_series_data" field exists in the row
		val, ok := row["time_series_data"]
		if !ok {
			return 0, fmt.Errorf("time_series_data field not found in query result")
		}
		// Convert the value to float64
		predictedSales, ok = val.(float64)
		if !ok {
			return 0, fmt.Errorf("failed to parse predicted_sales as float64")
		}
		// Break loop since we've found and parsed the predicted sales
		break
	}
	fmt.Print(predictedSales)
	return predictedSales, nil
}

func predictHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var request PredictionRequest
	err := decoder.Decode(&request)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	date := request.Date

	predictedSales, err := PredictSales(date)
	if err != nil {
		http.Error(w, "Failed to predict sales", http.StatusInternalServerError)
		return
	}

	response := PredictionResponse{
		PredictedSales: predictedSales,
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	log.Print("Starting server...")

	r := mux.NewRouter()
	r.HandleFunc("/predict", predictHandler).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	serverAddr := ":" + port
	srv := &http.Server{
		Handler:      r,
		Addr:         serverAddr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Listening on port %s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}


