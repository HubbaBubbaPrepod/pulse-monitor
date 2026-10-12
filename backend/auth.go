package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

type owner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Hash  string `json:"passwordHash"`
}
type authSession struct {
	CSRF    string
	Expires time.Time
}
type attemptWindow struct {
	Count int
	Until time.Time
}
type authState struct {
	mu         sync.Mutex
	registerMu sync.Mutex
	account    *owner
	sessions   map[[32]byte]authSession
	attempts   map[string]attemptWindow
	slots      chan struct{}
}

func loadAuth(dir string) (*authState, error) {
	a := &authState{sessions: make(map[[32]byte]authSession), attempts: make(map[string]attemptWindow), slots: make(chan struct{}, 2)}
	raw, err := os.ReadFile(filepath.Join(dir, "account.json"))
	if errors.Is(err, os.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	var account owner
	if err = json.Unmarshal(raw, &account); err != nil {
		return nil, err
	}
	if account.ID == "" || account.Email == "" {
		return nil, errors.New("invalid saved account")
	}
	if _, err = bcrypt.Cost([]byte(account.Hash)); err != nil {
		return nil, err
	}
	a.account = &account
	return a, nil
}
func randomToken() (string, error) {
	var b [32]byte
	_, err := rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:]), err
}
func (a *app) currentSession(r *http.Request) (authSession, bool) {
	c, err := r.Cookie("pulse_session")
	if err != nil {
		return authSession{}, false
	}
	key := sha256.Sum256([]byte(c.Value))
	a.auth.mu.Lock()
	defer a.auth.mu.Unlock()
	s, ok := a.auth.sessions[key]
	if ok && !time.Now().Before(s.Expires) {
		delete(a.auth.sessions, key)
		ok = false
	}
	return s, ok
}
func (a *app) authStatus(w http.ResponseWriter, r *http.Request) {
	s, ok := a.currentSession(r)
	a.auth.mu.Lock()
	account := a.auth.account
	a.auth.mu.Unlock()
	result := map[string]any{"authenticated": ok, "setupRequired": account == nil}
	if ok && account != nil {
		result["user"] = map[string]string{"id": account.ID, "name": account.Name, "email": account.Email}
		result["csrfToken"] = s.CSRF
	}
	writeJSON(w, 200, result)
}
func (a *app) issueSession(w http.ResponseWriter, r *http.Request, status int) {
	token, err := randomToken()
	if err != nil {
		apiError(w, 500, "Could not create session")
		return
	}
	csrf, err := randomToken()
	if err != nil {
		apiError(w, 500, "Could not create session")
		return
	}
	a.auth.mu.Lock()
	for key, s := range a.auth.sessions {
		if time.Now().After(s.Expires) {
			delete(a.auth.sessions, key)
		}
	}
	if len(a.auth.sessions) >= 16 {
		var oldest [32]byte
		var date time.Time
		for key, s := range a.auth.sessions {
			if date.IsZero() || s.Expires.Before(date) {
				oldest = key
				date = s.Expires
			}
		}
		delete(a.auth.sessions, oldest)
	}
	a.auth.sessions[sha256.Sum256([]byte(token))] = authSession{csrf, time.Now().Add(8 * time.Hour)}
	account := a.auth.account
	a.auth.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "pulse_session", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: 28800})
	writeJSON(w, status, map[string]any{"authenticated": true, "setupRequired": false, "csrfToken": csrf, "user": map[string]string{"id": account.ID, "name": account.Name, "email": account.Email}})
}
func (a *app) credentials(w http.ResponseWriter, r *http.Request, register bool) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	a.auth.mu.Lock()
	now := time.Now()
	for key, v := range a.auth.attempts {
		if now.After(v.Until) {
			delete(a.auth.attempts, key)
		}
	}
	v, known := a.auth.attempts[ip]
	if v.Until.IsZero() {
		v.Until = now.Add(time.Minute)
	}
	v.Count++
	allowed := v.Count <= 10 && (known || len(a.auth.attempts) < 256)
	if known || len(a.auth.attempts) < 256 {
		a.auth.attempts[ip] = v
	}
	a.auth.mu.Unlock()
	if !allowed {
		apiError(w, 429, "Слишком много попыток. Повторите через минуту.")
		return
	}
	select {
	case a.auth.slots <- struct{}{}:
		defer func() { <-a.auth.slots }()
	default:
		apiError(w, 429, "Повторите попытку через несколько секунд.")
		return
	}
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		apiError(w, 400, "Некорректные данные формы")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if register {
		a.auth.registerMu.Lock()
		defer a.auth.registerMu.Unlock()
		a.auth.mu.Lock()
		exists := a.auth.account != nil
		a.auth.mu.Unlock()
		if exists {
			apiError(w, 409, "Владелец панели уже зарегистрирован")
			return
		}
		email, err := mail.ParseAddress(input.Email)
		if err != nil || email.Address != input.Email || len(input.Email) > 254 || input.Name == "" || utf8.RuneCountInString(input.Name) > 64 || utf8.RuneCountInString(input.Password) < 12 || len(input.Password) > 72 {
			apiError(w, 400, "Укажите имя, email и пароль от 12 символов, до 72 байт")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
		if err != nil {
			apiError(w, 500, "Could not protect password")
			return
		}
		id, err := randomToken()
		if err != nil {
			apiError(w, 500, "Could not create account")
			return
		}
		account := &owner{id, input.Name, input.Email, string(hash)}
		raw, err := json.Marshal(account)
		if err != nil {
			apiError(w, 500, "Could not save account")
			return
		}
		f, err := os.CreateTemp(a.dir, "account-*.tmp")
		if err != nil {
			apiError(w, 500, "Could not save account")
			return
		}
		defer os.Remove(f.Name())
		_, err = f.Write(raw)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(f.Name(), filepath.Join(a.dir, "account.json"))
		}
		if err != nil {
			apiError(w, 500, "Could not save account")
			return
		}
		a.auth.mu.Lock()
		a.auth.account = account
		a.auth.mu.Unlock()
		a.issueSession(w, r, 201)
		return
	}
	a.auth.mu.Lock()
	account := a.auth.account
	a.auth.mu.Unlock()
	if account == nil {
		apiError(w, 401, "Неверный email или пароль")
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(account.Hash), []byte(input.Password))
	if err != nil || input.Email != account.Email {
		apiError(w, 401, "Неверный email или пароль")
		return
	}
	a.issueSession(w, r, 200)
}
func (a *app) authRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/status", a.authStatus)
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) { a.credentials(w, r, true) })
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) { a.credentials(w, r, false) })
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("pulse_session")
		if err == nil {
			a.auth.mu.Lock()
			delete(a.auth.sessions, sha256.Sum256([]byte(c.Value)))
			a.auth.mu.Unlock()
		}
		http.SetCookie(w, &http.Cookie{Name: "pulse_session", Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		writeJSON(w, 200, map[string]bool{"authenticated": false})
	})
}
func validCSRF(r *http.Request, s authSession) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.CSRF)) == 1
}
