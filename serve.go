package main

import (
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/llimllib/photos/data"
	"github.com/llimllib/photos/middleware"
	"github.com/lmittmann/tint"
	"golang.org/x/crypto/bcrypt"
)

const (
	DBFILE = "photos.db"
)

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
	salt       string
}

func NewServer(logger *slog.Logger) *server {
	salt := getenv("SALT", "")
	if salt == "" {
		panic("missing required SALT env var")
	}

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
		salt:   salt,
	}
}

func ensureUploadsDirExists() error {
	// make sure the "uploads" folder exists
	if _, err := os.Stat("./uploads"); os.IsNotExist(err) {
		// If the folder doesn't exist, create it
		// 0755 = drwxr-xr-x permissions
		err = os.Mkdir("./uploads", 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) upload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if err := render("templates/upload.tmpl", w, nil); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case "POST":
		title := r.FormValue("title")
		caption := r.FormValue("caption")

		err := r.ParseMultipartForm(100 << 20) // 100MB limit
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err = ensureUploadsDirExists(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		files := r.MultipartForm.File["images"]
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			path := filepath.Join("uploads", fileHeader.Filename)
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()

			_, err = io.Copy(f, file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			data.NewUpload(s.db, title, caption, fileHeader.Filename, data.UploadMetadata{}, r.Context())
			// TODO: kick off a background process to:
			// - create thumbnails
			// - process exif data
		}

		// TODO: flash a message that the image was successfully uploaded
		http.Redirect(w, r, "/", http.StatusFound)

	default:
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

func (s *server) loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if err := render("templates/login.tmpl", w, nil); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case "POST":
		username := r.FormValue("username")
		password := r.FormValue("password")

		var user *data.User
		var err error
		if user, err = data.GetUserByUsername(s.db, username, r.Context()); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		if err = bcrypt.CompareHashAndPassword(user.Password, []byte(fmt.Sprintf("%s%s", password, s.salt))); err != nil {
			s.logger.Error(err.Error())
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		var sess *data.Session
		if sess, err = data.NewSession(s.db, &data.SessionData{Username: username}, r.Context()); err != nil {
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
	Photos  []*data.Upload
	Session *data.Session
}

func (s *server) root(w http.ResponseWriter, r *http.Request) {
	var sess *data.Session
	cookie, err := r.Cookie("photos-login")
	if err == nil {
		s.logger.Debug("Found cookie", "cookie", cookie)
		sess, err = data.LookupSession(s.db, cookie.Value, r.Context())
		if err != nil {
			s.logger.Debug("Failed to find session", "id", cookie.Value, "err", err)
		}
	}

	photos, err := data.GetUploads(s.db, r.Context())
	if err != nil {
		s.logger.Error(err.Error())
		http.Error(w, "Error getting photos", http.StatusInternalServerError)
		return
	}

	if err := render("templates/index.tmpl", w, RootPageData{photos, sess}); err != nil {
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

	recover := middleware.NewPanicMiddleware(logger)
	log := middleware.NewLoggingMiddleware(logger)
	authRequired := middleware.NewAuthRequiredMiddleware(logger, server.db)

	uploadFS := http.FileServer(http.Dir("uploads"))

	// Authorized routes
	http.Handle("/upload", log(recover(authRequired(server.upload))))

	// Open routes
	http.Handle("GET /uploads/", log(recover(http.StripPrefix("/uploads", uploadFS).ServeHTTP)))
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
