package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"

	"crawshaw.io/sqlite/sqlitex"
	"github.com/lmittmann/tint"
	"golang.org/x/crypto/bcrypt"
)

// TODO: look up proper salt procedures
var SALT = "0f6f68577436a6b466b36b59504191b6"

// User struct to represent user data
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

	stmt := conn.Prep("SELECT id, username, password FROM users WHERE username = $user")
	stmt.SetText("$user", username)
	if hasRow, err := stmt.Step(); err != nil {
		slog.Warn("Unable to find user", "username", username)
		err = fmt.Errorf("unable to find user %s: %w", username, err)
		return nil, err
	} else if !hasRow {
		err = fmt.Errorf("unable to find user %s: %w", username, err)
		return nil, err
	}

	return &User{
		ID:       stmt.GetText("id"),
		Username: stmt.GetText("username"),
		Password: []byte(stmt.GetText("password")),
	}, nil
}

func NewLoggingMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrappedWriter := newResponseWriter(w)
			next.ServeHTTP(wrappedWriter, r)

			statusCode := wrappedWriter.statusCode
			path := r.URL.Path

			if statusCode < 500 {
				logger.InfoContext(r.Context(),
					"request",
					"status", statusCode,
					"path", path,
				)
			} else {
				logger.ErrorContext(r.Context(),
					"error",
					"status", statusCode,
					"path", path,
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
	logger *slog.Logger
	db     *sqlitex.Pool
}

func NewServer(logger *slog.Logger) *server {
	dbpool, err := sqlitex.Open("file:memory:?mode=memory", 0, 10)
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

		// Check if the password is correct
		if err = bcrypt.CompareHashAndPassword(user.Password, []byte(fmt.Sprintf("%s%s", SALT, password))); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		// User is authenticated, set a session cookie or take further actions
		http.Redirect(w, r, "/", http.StatusFound)

	default:
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

func (s *server) root(w http.ResponseWriter, r *http.Request) {
	if err := render("templates/index.html", w, "Hello, World!"); err != nil {
		s.logger.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	w := os.Stderr
	logger := slog.New(tint.NewHandler(w, nil))

	server := NewServer(logger)

	recover := NewPanicMiddleware(logger)
	logging := NewLoggingMiddleware(logger)

	http.Handle("/login", logging(recover(server.loginHandler)))
	http.Handle("/", logging(recover(server.root)))

	logger.Info("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
