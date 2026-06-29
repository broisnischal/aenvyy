package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/server"
	"github.com/nees/envvar/internal/sqlitestore"
)

func newServerCmd() *cobra.Command {
	var addr string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the self-hostable web UI + API server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), nil))

			store, err := sqlitestore.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			srv := server.New(log, store)
			httpSrv := &http.Server{
				Addr:              addr,
				Handler:           srv.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() {
				fmt.Fprintf(cmd.ErrOrStderr(), "envvar server listening on %s (db %s)\n", addr, dbPath)
				errCh <- httpSrv.ListenAndServe()
			}()

			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return httpSrv.Shutdown(shutdownCtx)
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			}
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	cmd.Flags().StringVar(&dbPath, "db", "envvar.db", "path to the SQLite database file")
	return cmd
}
