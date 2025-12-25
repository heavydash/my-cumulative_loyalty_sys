package handler

import (
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/mailru/easyjson"
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

type creds struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type UserHandler struct {
	userStorage storage.UserStorage
	signingKey  []byte
}

func NewUserHandler(userStorage *storage.UserStorage, signingKey []byte) *UserHandler {
	return &UserHandler{
		userStorage: *userStorage,
		signingKey:  signingKey,
	}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var c creds

	if err := easyjson.UnmarshalFromReader(r.Body, &c); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if c.Login == "" || c.Password == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user, err := h.userStorage.Create(r.Context(), c.Login, c.Password)
	if err != nil {
		http.Error(w, "Conflict", http.StatusConflict)
		return
	}

	token, err := auth.GenerateToken(user.ID, h.signingKey)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
	})
	w.WriteHeader(http.StatusOK)
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var c creds

	if err := easyjson.UnmarshalFromReader(r.Body, &c); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user, err := h.userStorage.GetByLogin(r.Context(), c.Login)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(c.Password)) != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(user.ID, h.signingKey)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
	})

	w.WriteHeader(http.StatusOK)
}
