// hello is the demo app used throughout this tutorial.
//
//   - "/"        prints the MESSAGE env var, its own HOSTNAME (Pod name),
//     and its build-time version — used for the load-balancing demo in
//     section 5-17 min, and to tell v1/v2 apart by eye in section 47-57 min.
//   - "/count"   increments and returns a counter. With no DB_HOST set it
//     lives in-memory (one counter per Pod — the "stateless" demo). With
//     DB_HOST set it's stored in Postgres instead, shared by every Pod —
//     the "shared DB" / PVC demo in section 35-47 min.
//   - "/healthz" always 200 — readiness/liveness probe target.
//
// version and crashOnStart are baked in at build time via -ldflags -X (see
// ../Dockerfile and ../05-lifecycle/build-images.sh) — that's how the same
// source produces the hello:v1 / hello:v2 / hello:v3-broken images used in
// section 47-57 min without maintaining three copies of this file.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var (
	version      = "v1"    // overridden via -ldflags -X main.version=...
	crashOnStart = "false" // -ldflags -X main.crashOnStart=true simulates a broken release

	db       *sql.DB // nil unless DB_HOST is set
	memMu    sync.Mutex
	memCount int
)

func main() {
	if crashOnStart == "true" {
		// Simulates a bad deploy: the process dies before it ever binds a
		// port, so kubelet never sees a Ready Pod and Kubernetes cycles it
		// into CrashLoopBackOff. No env var or Helm value controls this —
		// it's baked into the hello:v3-broken image itself, the same way a
		// real bad build would be.
		log.Fatal("simulated crash: this build is broken and exits on start")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	log.Printf("version %s, log level: %s", version, logLevel)

	if host := os.Getenv("DB_HOST"); host != "" {
		conn, err := connectDB(host)
		if err != nil {
			log.Fatalf("connect db at %s: %v", host, err)
		}
		db = conn
		log.Printf("using postgres at %s for /count", host)
	} else {
		log.Printf("no DB_HOST set, /count uses an in-memory (per-pod) counter")
	}

	http.HandleFunc("/", handleHello)
	http.HandleFunc("/count", handleCount)
	http.HandleFunc("/healthz", handleHealthz)

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// connectDB opens a connection to Postgres and makes sure the counter table
// exists. It retries the initial ping for a while since the DB Pod is very
// likely still starting when this app Pod does.
func connectDB(host string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host,
		getenv("DB_PORT", "5432"),
		getenv("DB_USER", "postgres"),
		os.Getenv("DB_PASSWORD"),
		getenv("DB_NAME", "postgres"),
	)

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	var pingErr error
	for i := 0; i < 30; i++ {
		if pingErr = conn.Ping(); pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		return nil, pingErr
	}

	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS counter (
			id    INT PRIMARY KEY DEFAULT 1,
			value INT NOT NULL DEFAULT 0
		)`)
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(`INSERT INTO counter (id, value) VALUES (1, 0) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	message := os.Getenv("MESSAGE")
	if message == "" {
		message = "Hello"
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	fmt.Fprintf(w, "%s from %s (version %s)\n", message, hostname, version)
}

func handleCount(w http.ResponseWriter, r *http.Request) {
	var n int
	if db != nil {
		err := db.QueryRow(`UPDATE counter SET value = value + 1 WHERE id = 1 RETURNING value`).Scan(&n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		memMu.Lock()
		memCount++
		n = memCount
		memMu.Unlock()
	}
	fmt.Fprintln(w, strconv.Itoa(n))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
