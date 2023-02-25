package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

type apiFunc func(w http.ResponseWriter, r *http.Request) error

type ApiError struct {
	Error string
}

func makeHttpHandleFunc(f apiFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := f(writer, request); err != nil {
			// handle the error
			WriteJSON(writer, http.StatusBadRequest, ApiError{Error: err.Error()})
		}
	}
}

type APIServer struct {
	listenAddr string
}

func (s *APIServer) Run() {
	router := mux.NewRouter()
	router.HandleFunc("/account", makeHttpHandleFunc(s.handleAccount))
	router.HandleFunc("/account/{id}", makeHttpHandleFunc(s.handleGetAccount))
	err := http.ListenAndServe(s.listenAddr, router)
	if err != nil {
		log.Fatalf("error while running server %v", err)
	}

	log.Println("API server running on port:", s.listenAddr)
}

func NewAPIServer(listenAddr string) *APIServer {
	return &APIServer{listenAddr: listenAddr}
}

func (s *APIServer) handleAccount(writer http.ResponseWriter, request *http.Request) error {
	if request.Method == http.MethodGet {
		return s.handleGetAccount(writer, request)
	}
	if request.Method == http.MethodPost {
		return s.handleCreateAccount(writer, request)
	}
	if request.Method == http.MethodDelete {
		return s.handleDeleteAccount(writer, request)
	}

	return fmt.Errorf("method not allowed %v", request.Method)
}

func (s *APIServer) handleGetAccount(writer http.ResponseWriter, request *http.Request) error {
	vars := mux.Vars(request)["id"]
	return WriteJSON(writer, http.StatusOK, vars)
}

func (s *APIServer) handleCreateAccount(writer http.ResponseWriter, request *http.Request) error {
	return nil
}

func (s *APIServer) handleDeleteAccount(writer http.ResponseWriter, request *http.Request) error {
	return nil
}

func (s *APIServer) handleTransfer(writer http.ResponseWriter, request *http.Request) error {
	return nil
}
