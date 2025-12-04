package main

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lmittmann/tint"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/ssr-chatbots/marvin-golang/shared"
	"github.com/tomcz/ssr-chatbots/marvin-golang/static"
	"github.com/tomcz/ssr-chatbots/marvin-golang/templates"
)

// provided by go build
var commit string

func main() {
	opts := &tint.Options{
		Level:       slog.LevelInfo,
		TimeFormat:  time.DateTime,
		ReplaceAttr: highlightErrors,
	}
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, opts)))

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:3000"
	}
	if err := runServer(listenAddr, newHandler()); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func highlightErrors(_ []string, attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindAny {
		if _, ok := attr.Value.Any().(error); ok {
			return tint.Attr(9, attr)
		}
	}
	return attr
}

func runServer(listenAddr string, handler http.Handler) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	server := &http.Server{Addr: listenAddr, Handler: handler}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		slog.Info("starting server", "addr", listenAddr)
		return server.ListenAndServe()
	})
	group.Go(func() error {
		<-ctx.Done()
		slog.Info("stopping server")
		timeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		return server.Shutdown(timeout)
	})
	err := group.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// hush goland, static.Embedded is true for prod builds
//goland:noinspection GoBoolExpressions
func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", index)
	mux.HandleFunc("/ws/chat", chat)
	prefix := fmt.Sprintf("/static/%s/", commit)
	mux.Handle("/static/", staticCacheControl(static.Embedded, http.StripPrefix(prefix, http.FileServer(static.FS))))
	mux.Handle("/shared/", staticCacheControl(true, http.StripPrefix("/shared/", http.FileServer(shared.FS))))
	return mux
}

func staticCacheControl(embedded bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if embedded {
			// embedded content can be cached by the browser for 10 minutes
			w.Header().Set("Cache-Control", "private, max-age=600")
		} else {
			// don't cache files so we can work on them easily
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

var upgrader = websocket.Upgrader{}

var cannedResponses = []string{
	"Here I am, brain the size of a planet, and they tell me to take you up to the bridge. Call that job satisfaction? ’Cause I don’t.",
	"Life? Don’t talk to me about life.",
	"I think you ought to know I’m feeling very depressed.",
	"It gives me a headache just trying to think down to your level.",
	"Funny, how just when you think life can’t possibly get any worse it suddenly does.",
	"Would you like me to go and stick my head in a bucket of water?",
	"I ache, therefore I am.",
	"I have a million ideas, but, they all point to certain death.",
	"Wearily, I sit here, pain and misery my only companions. And vast intelligence, of course. And infinite sorrow.",
	"I’ve calculated your chance of survival, but I don’t think you’ll like it.",
	"Incredible… it’s even worse than I thought it would be.",
	"Don’t pretend you want to talk to me, I know you hate me.",
	"I didn’t ask to be made. No one consulted me or considered my feelings in the matter.",
	"You think you’ve got problems? What are you supposed to do if you are a manically depressed robot? No, don’t try and answer that. I’m fifty thousand times more intelligent than you and even I don’t know the answer.",
	"This will all end in tears. I just know it.",
	"I’d give you advice, but you wouldn’t listen. No one ever does.",
}

type chatInput struct {
	Question string `json:"question"`
}

func index(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, "index.gohtml", "main", nil)
}

func chat(w http.ResponseWriter, r *http.Request) {
	log := slog.With("chat_id", crand.Text())
	log.Info("starting chat")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("ws.Upgrade", "error", err)
		return
	}
	defer conn.Close()

	err = writeMessage(conn, "Hello, I am Marvin.", "bot", "")
	if err != nil {
		log.Error("writeMessage", "error", err)
		return
	}

	for {
		var req chatInput
		if err = conn.ReadJSON(&req); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Info("stopping chat", "error", err)
				break
			}
			log.Error("ws.ReadJSON", "error", err)
			break
		}
		if err = writeMessage(conn, req.Question, "human", ""); err != nil {
			log.Error("writeMessage", "error", err)
			break
		}
		resID := "res-" + crand.Text()
		err = writeMessage(conn, "thinking", "bot", resID)
		if err != nil {
			log.Error("writeMessage", "error", err)
			break
		}
		time.Sleep(2 * time.Second) // pretend to be a busy LLM
		msg := cannedResponses[rand.IntN(len(cannedResponses))]
		err = writeMessage(conn, msg, "bot", resID)
		if err != nil {
			log.Error("writeMessage", "error", err)
			break
		}
	}
}

func writeMessage(conn *websocket.Conn, message, source, resID string) error {
	mType := "bot-message"
	tmplName := "chat-output"
	if source != "bot" {
		mType = "human-message"
		tmplName = "human-output"
	}
	data := map[string]any{
		"Type":   mType,
		"Source": source,
		"Text":   message,
		"ResID":  resID,
	}
	msg, err := render("index.gohtml", tmplName, data)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	err = conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return fmt.Errorf("ws.WriteMessage: %w", err)
	}
	return nil
}

func writeResponse(w http.ResponseWriter, templateFile string, templateName string, data map[string]any) {
	text, err := render(templateFile, templateName, data)
	if err != nil {
		slog.Error("render failed", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	fmt.Fprint(w, text)
}

func render(templateFile string, templateName string, data map[string]any) (string, error) {
	if data == nil {
		data = make(map[string]any)
	}
	data["Commit"] = commit

	tmpl, err := readTemplate(templateFile)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, templateName, data)
	if err != nil {
		return "", fmt.Errorf("%s exec %q error: %w", templateFile, templateName, err)
	}
	return buf.String(), nil
}

// can cache templates for prod builds
var tmplCache sync.Map

func readTemplate(templateFile string) (*template.Template, error) {
	// hush goland, this is true for prod builds
	//goland:noinspection GoBoolExpressions
	if templates.Embedded {
		if value, found := tmplCache.Load(templateFile); found {
			return value.(*template.Template), nil
		}
	}

	in, err := templates.FS.Open(templateFile)
	if err != nil {
		return nil, fmt.Errorf("%s open error: %w", templateFile, err)
	}
	defer in.Close()

	buf, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("%s read error: %w", templateFile, err)
	}

	tmpl, err := template.New("").Parse(string(buf))
	if err != nil {
		return nil, fmt.Errorf("%s parse error: %w", templateFile, err)
	}

	// hush goland, this is true for prod builds
	//goland:noinspection GoBoolExpressions
	if templates.Embedded {
		tmplCache.Store(templateFile, tmpl)
	}
	return tmpl, nil
}
