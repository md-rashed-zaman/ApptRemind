package main

import (
	"context"
	"net/http"
	"time"

	"github.com/md-rashed-zaman/apptremind/libs/config"
	"github.com/md-rashed-zaman/apptremind/libs/db"
	"github.com/md-rashed-zaman/apptremind/libs/httpx"
	otelx "github.com/md-rashed-zaman/apptremind/libs/otel"
	"github.com/md-rashed-zaman/apptremind/libs/runtime"
	"github.com/md-rashed-zaman/apptremind/services/business-service/internal/handlers"
	"github.com/md-rashed-zaman/apptremind/services/business-service/internal/storage"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	service := config.String("SERVICE_NAME", "business-service")
	port, err := config.Port("PORT", "8082")
	if err != nil {
		panic(err)
	}
	logger := runtime.NewLogger(service)

	ctx, stop := runtime.SignalContext()
	defer stop()

	otelShutdown, err := otelx.Setup(ctx, otelx.ConfigFromEnv(service))
	if err != nil {
		logger.Error("otel setup failed", "err", err)
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = otelShutdown(shutdownCtx)
		}()
	}

	dbURL, err := config.RequiredString("DATABASE_URL")
	if err != nil {
		panic(err)
	}
	pool, err := db.Open(ctx, dbURL)
	if err != nil {
		logger.Error("db connection failed", "err", err)
		panic(err)
	}
	defer pool.Close()

	repo := storage.NewRepository(pool)
	httpHandler := handlers.New(repo)

	mux := runtime.NewBaseMuxWithReady(
		runtime.ReadyCheck{Name: "db", Check: db.ReadyCheck(pool)},
	)
	mux.HandleFunc("/api/v1/business/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			httpHandler.GetProfile(w, r)
			return
		}
		if r.Method == http.MethodPut {
			httpHandler.UpdateProfile(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/business/services", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			httpHandler.CreateService(w, r)
			return
		}
		if r.Method == http.MethodGet {
			httpHandler.ListServices(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/business/staff", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			httpHandler.CreateStaff(w, r)
			return
		}
		if r.Method == http.MethodGet {
			httpHandler.ListStaff(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/business/staff/working-hours", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			httpHandler.ListWorkingHours(w, r)
			return
		}
		if r.Method == http.MethodPut {
			httpHandler.UpsertWorkingHours(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v1/business/staff/time-off", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			httpHandler.CreateTimeOff(w, r)
			return
		}
		if r.Method == http.MethodGet {
			httpHandler.ListTimeOff(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			httpHandler.DeleteTimeOff(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	handler := httpx.Chain(mux,
		httpx.WithRequestID,
		httpx.WithAccessLog(logger),
	)
	handler = otelhttp.NewHandler(handler, "business")
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("http server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", "err", err)
		}
	}()

	if err := startGrpcServer(ctx, logger, pool, repo); err != nil {
		logger.Error("grpc server failed to start", "err", err)
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "err", err)
	}
	logger.Info("http server stopped")
}
