package main

import (
	"encoding/json"
	"fmt"
	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"strconv"
)

type apiFunc func(w http.ResponseWriter, r *http.Request) error

type APIServer struct {
	listenAddr string
	store      Storage
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{listenAddr: listenAddr, store: store}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()
	router.HandleFunc("/login", makeHttpHandleFunc(s.HandleLogin))
	router.HandleFunc("/account", makeHttpHandleFunc(s.handleAccount))
	router.HandleFunc("/account/{id}", withJWTAuth(makeHttpHandleFunc(s.handleGetAccountById), s.store))
	router.HandleFunc("/transfer", makeHttpHandleFunc(s.handleTransfer))
	err := http.ListenAndServe(s.listenAddr, router)
	if err != nil {
		log.Fatalf("error while running server %v", err)
	}
	log.Println("API server running on port:", s.listenAddr)
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
	account, err := s.store.GetAccount()
	if err != nil {
		return err
	}
	return WriteJSON(writer, http.StatusOK, account)
}

func (s *APIServer) handleGetAccountById(writer http.ResponseWriter, request *http.Request) error {
	if request.Method == http.MethodGet {
		id, err := getID(request)
		if err != nil {
			return err
		}
		account, err := s.store.GetAccountById(id)
		if err != nil {
			return err
		}
		return WriteJSON(writer, http.StatusOK, account)
	}
	if request.Method == http.MethodDelete {
		return s.handleDeleteAccount(writer, request)
	}

	return fmt.Errorf("Method not allowed %s", request.Method)
}

func (s *APIServer) handleCreateAccount(writer http.ResponseWriter, request *http.Request) error {
	req := new(CreateAccountRequest)
	if err := json.NewDecoder(request.Body).Decode(req); err != nil {
		return err
	}
	account, err := NewAccount(req.FirstName, req.LastName, req.Password)
	if err != nil {
		return err
	}
	if err := s.store.CreateAccount(account); err != nil {
		return err
	}

	return WriteJSON(writer, http.StatusOK, account)
}

func createJWT(account *Account) (string, error) {
	claims := &jwt.MapClaims{"expiresAt": 15000, "accountNumber": account.Number}

	secret := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

func (s *APIServer) handleDeleteAccount(writer http.ResponseWriter, request *http.Request) error {
	id, err := getID(request)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAccount(id); err != nil {
		return err
	}
	return WriteJSON(writer, http.StatusOK, map[string]int{"deleted": id})
}

func (s *APIServer) handleTransfer(writer http.ResponseWriter, request *http.Request) error {
	transferReq := new(TransferAccount)
	if err := json.NewDecoder(request.Body).Decode(transferReq); err != nil {
		return err
	}
	defer request.Body.Close()
	return WriteJSON(writer, http.StatusOK, transferReq)
}

func (s *APIServer) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("method not allowed %s", r.Method)
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	acc, err := s.store.GetAccountByNumber(int(req.Number))
	if err != nil {
		return err
	}

	if !acc.ValidatePassword(req.Password) {
		return fmt.Errorf("not authenticated")
	}

	token, err := createJWT(acc)
	if err != nil {
		return err
	}

	res := LoginResponse{
		Number: acc.Number,
		Token:  token,
	}

	return WriteJSON(w, http.StatusOK, res)
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func withJWTAuth(handleFunc http.HandlerFunc, s Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		fmt.Println("calling JWT auth middleware")
		tokenString := request.Header.Get("x-jwt-token")
		token, err := validateJWT(tokenString)
		if err != nil {
			permissionDenied(w)
			return
		}
		if !token.Valid {
			permissionDenied(w)
			return
		}
		userId, err := getID(request)
		if err != nil {
			WriteJSON(w, http.StatusForbidden, ApiError{Error: "Invalid userId"})
			return
		}
		account, err := s.GetAccountById(userId)
		if err != nil {
			WriteJSON(w, http.StatusForbidden, ApiError{Error: "Invalid account Id"})
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		if account.Number != int64(claims["accountNumber"].(float64)) {
			permissionDenied(w)
			return
		}
		handleFunc(w, request)
	}
}

func permissionDenied(w http.ResponseWriter) {
	WriteJSON(w, http.StatusForbidden, ApiError{Error: "permission denied"})
}

func validateJWT(tokenString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
}

type ApiError struct {
	Error string `json:"error"`
}

func makeHttpHandleFunc(f apiFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := f(writer, request); err != nil {
			// handle the error
			WriteJSON(writer, http.StatusBadRequest, ApiError{Error: err.Error()})
		}
	}
}

func getID(request *http.Request) (int, error) {
	idStr := mux.Vars(request)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return id, fmt.Errorf("invalid id given %s", idStr)
	}
	return id, nil
}
