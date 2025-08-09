// Package middleware provides reusable HTTP middleware components for the Beacon API server.
// This includes method validation, CORS handling, JSON response helpers, and error handling.
package middleware

import (
	"encoding/json"
	"log"
	"net/http"
)

// MethodValidator creates middleware that validates HTTP methods for handlers
func MethodValidator(allowedMethods ...string) func(http.Handler) http.Handler {
	methodSet := make(map[string]bool)
	for _, method := range allowedMethods {
		methodSet[method] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !methodSet[r.Method] {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORSHandler adds CORS headers to all responses
func CORSHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// JSONContentType sets the content type to application/json
func JSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// ResponseHelper provides helper methods for consistent JSON responses
type ResponseHelper struct {
	w http.ResponseWriter
}

// NewResponseHelper creates a new response helper
func NewResponseHelper(w http.ResponseWriter) *ResponseHelper {
	return &ResponseHelper{w: w}
}

// JSON encodes and sends a JSON response with proper error handling
func (rh *ResponseHelper) JSON(data interface{}) error {
	if err := json.NewEncoder(rh.w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(rh.w, "Internal server error", http.StatusInternalServerError)
		return err
	}
	return nil
}

// Error sends a JSON error response
func (rh *ResponseHelper) Error(message string, statusCode int) {
	rh.w.WriteHeader(statusCode)
	errorResponse := map[string]interface{}{
		"error":  message,
		"status": statusCode,
	}

	if err := json.NewEncoder(rh.w).Encode(errorResponse); err != nil {
		log.Printf("Error encoding error response: %v", err)
		// Fallback to plain text error
		http.Error(rh.w, message, statusCode)
	}
}

// Success sends a JSON success response
func (rh *ResponseHelper) Success(data interface{}) {
	successResponse := map[string]interface{}{
		"success": true,
		"data":    data,
	}
	rh.JSON(successResponse)
}

// Chain combines multiple middleware functions
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
