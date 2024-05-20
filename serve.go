package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/google/uuid"
	"github.com/lmittmann/tint"
	"golang.org/x/crypto/bcrypt"
)

// TODO: look up proper salt procedures
const (
	SALT   = "0f6f68577436a6b466b36b59504191b6"
	DBFILE = "photos.db"
)

type User struct {
	ID       string
	Username string
	Password []byte
}

func getUserByUsername(db *sqlitex.Pool, username string, ctx context.Context) (*User, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	var user *User
	fn := func(stmt *sqlite.Stmt) error {
		user = &User{
			ID:       stmt.ColumnText(0),
			Username: stmt.ColumnText(1),
			Password: []byte(stmt.ColumnText(2)),
		}
		return nil
	}
	if err := sqlitex.Execute(conn,
		"SELECT id, username, password FROM users WHERE username = ?;", &sqlitex.ExecOptions{
			Args:       []any{username},
			ResultFunc: fn,
		}); err != nil {
		err = fmt.Errorf("unable to find user %s: %w", username, err)
		return nil, err
	}

	return user, nil
}

type Session struct {
	ID        string
	Data      *SessionData
	CreatedAt time.Time
}

type SessionData struct {
	Username string `json:"string"`
}

func NewSession(db *sqlitex.Pool, data *SessionData, ctx context.Context) (*Session, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	createdAt := time.Now()
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	encData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	if err := sqlitex.Execute(conn,
		"INSERT INTO sessions (id, data, created_at) VALUES (?, ?, ?);",
		&sqlitex.ExecOptions{
			Args: []any{id, encData, createdAt.Format(time.RFC3339)},
		}); err != nil {
		return nil, err
	}

	return &Session{
		ID:        id.String(),
		Data:      data,
		CreatedAt: createdAt,
	}, nil
}

func lookupSession(db *sqlitex.Pool, id string, ctx context.Context) (*Session, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	var session *Session
	fn := func(stmt *sqlite.Stmt) error {
		t, err := time.Parse(time.RFC3339, stmt.ColumnText(2))
		if err != nil {
			return err
		}

		var data SessionData
		err = json.Unmarshal([]byte(stmt.ColumnText(1)), &data)
		if err != nil {
			return err
		}

		session = &Session{
			ID:        stmt.ColumnText(0),
			Data:      &data,
			CreatedAt: t,
		}
		return nil
	}
	if err := sqlitex.Execute(conn,
		"SELECT id, data, created_at FROM sessions WHERE id = ?;",
		&sqlitex.ExecOptions{
			ResultFunc: fn,
			Args:       []any{id},
		}); err != nil {
		return nil, fmt.Errorf("unable to find session %s: %w", id, err)
	}

	return session, nil
}

func NewLoggingMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrappedWriter := newResponseWriter(w)
			t1 := time.Now()
			next.ServeHTTP(wrappedWriter, r)
			t2 := time.Now()

			statusCode := wrappedWriter.statusCode
			path := r.URL.Path

			if statusCode < 500 {
				logger.InfoContext(r.Context(),
					"request",
					"status", statusCode,
					"path", path,
					"duration", t2.Sub(t1),
				)
			} else {
				logger.ErrorContext(r.Context(),
					"error",
					"status", statusCode,
					"path", path,
					"duration", t2.Sub(t1),
				)
			}
		})
	}
}

// NewPanicMiddleware returns a middleware function that closes over a given
// logger. The returned middleware function recovers from a panic, logs it, and
// writes a 500 to the http stream
func NewPanicMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic handled", "error:", r)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func render(templateFile string, w io.Writer, data any) error {
	t, err := template.ParseFiles(templateFile)
	if err != nil {
		return err
	}

	if err := t.Execute(w, data); err != nil {
		return err
	}

	return nil
}

type server struct {
	logger     *slog.Logger
	db         *sqlitex.Pool
	sessionKey string
}

func NewServer(logger *slog.Logger) *server {
	sqlite.Logger = func(code sqlite.ErrorCode, msg []byte) {
		logger.Debug(string(msg), "code", code, "source", "sqlite")
	}
	dbpool, err := sqlitex.Open(DBFILE, 0, 10)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	return &server{
		logger: logger,
		db:     dbpool,
	}
}

func (s *server) loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if err := render("templates/login.html", w, nil); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case "POST":
		username := r.FormValue("username")
		password := r.FormValue("password")

		var user *User
		var err error
		if user, err = getUserByUsername(s.db, username, r.Context()); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		}

		if err = bcrypt.CompareHashAndPassword(user.Password, []byte(fmt.Sprintf("%s%s", password, SALT))); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		var sess *Session
		if sess, err = NewSession(s.db, &SessionData{username}, r.Context()); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		sessionCookie := &http.Cookie{
			Name:     "photos-login",
			Value:    sess.ID,
			Expires:  time.Now().Add(24 * time.Hour), // Expires in 24 hours
			HttpOnly: true,                           // Prevent client-side script access
			Secure:   true,                           // Only send over HTTPS
		}

		http.SetCookie(w, sessionCookie)
		http.Redirect(w, r, "/", http.StatusFound)
	default:
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

type RootPageData struct {
	Session *Session
}

func (s *server) root(w http.ResponseWriter, r *http.Request) {
	var sess *Session
	cookie, err := r.Cookie("photos-login")
	if err == nil {
		s.logger.Debug("Found cookie", "cookie", cookie)
		sess, err = lookupSession(s.db, cookie.Value, r.Context())
		if err != nil {
			s.logger.Debug("Failed to find session", "id", cookie.Value, "err", err)
		}
	}
	if err := render("templates/index.html", w, RootPageData{sess}); err != nil {
		s.logger.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func initLogger() *slog.Logger {
	w := os.Stderr
	level := new(slog.LevelVar)

	switch getenv("LOG_LEVEL", "Info") {
	case "Warn":
		level.Set(slog.LevelWarn)
	case "Debug":
		level.Set(slog.LevelDebug)
	case "Info":
		level.Set(slog.LevelInfo)
	default:
		panic("Invalid log level " + getenv("LOG_LEVEL", "Info"))
	}

	// If PRETTY_LOGGER is present, create a nice-looking local logger.
	// Otherwise, log JSON output.
	if os.Getenv("PRETTY_LOGGER") == "true" {
		return slog.New(tint.NewHandler(w, &tint.Options{Level: level}))
	} else {
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
	}
}

func getenv(key string, deflaut string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return deflaut
}

func main() {
	logger := initLogger()
	server := NewServer(logger)

	recover := NewPanicMiddleware(logger)
	log := NewLoggingMiddleware(logger)

	http.Handle("/login", log(recover(server.loginHandler)))
	http.Handle("/", log(recover(server.root)))

	host := getenv("HOST", "localhost")
	port := getenv("PORT", "8080")
	addr := fmt.Sprintf("%s:%s", host, port)
	logger.Info("Starting server on", "host", host, "port", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
