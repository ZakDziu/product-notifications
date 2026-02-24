package config

import (
	"os"
	"testing"
)

func TestLoadProducts(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing DATABASE_URL",
			env:     map[string]string{"RABBITMQ_URL": "amqp://localhost"},
			wantErr: "DATABASE_URL is required",
		},
		{
			name:    "missing RABBITMQ_URL",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost"},
			wantErr: "RABBITMQ_URL is required",
		},
		{
			name: "valid config with defaults",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/db",
				"RABBITMQ_URL": "amqp://localhost",
			},
		},
		{
			name: "custom HTTP_ADDR overrides default",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/db",
				"RABBITMQ_URL": "amqp://localhost",
				"HTTP_ADDR":    ":9090",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := LoadProducts()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("want error %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.DatabaseURL != tt.env["DATABASE_URL"] {
				t.Fatalf("want DatabaseURL %q, got %q", tt.env["DATABASE_URL"], cfg.DatabaseURL)
			}
			if cfg.RabbitMQURL != tt.env["RABBITMQ_URL"] {
				t.Fatalf("want RabbitMQURL %q, got %q", tt.env["RABBITMQ_URL"], cfg.RabbitMQURL)
			}
			if addr, ok := tt.env["HTTP_ADDR"]; ok && cfg.HTTPAddr != addr {
				t.Fatalf("want HTTPAddr %q, got %q", addr, cfg.HTTPAddr)
			}
			if _, ok := tt.env["HTTP_ADDR"]; !ok && cfg.HTTPAddr != defaultHTTPAddr {
				t.Fatalf("want default HTTPAddr %q, got %q", defaultHTTPAddr, cfg.HTTPAddr)
			}
			if cfg.DBMaxOpenConns != defaultDBMaxOpenConns {
				t.Fatalf("want DBMaxOpenConns %d, got %d", defaultDBMaxOpenConns, cfg.DBMaxOpenConns)
			}
			if cfg.ShutdownTimeout != defaultShutdownTimeout {
				t.Fatalf("want ShutdownTimeout %v, got %v", defaultShutdownTimeout, cfg.ShutdownTimeout)
			}
		})
	}
}

func TestLoadNotifications(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing RABBITMQ_URL",
			env:     map[string]string{},
			wantErr: "RABBITMQ_URL is required",
		},
		{
			name: "valid config",
			env:  map[string]string{"RABBITMQ_URL": "amqp://localhost"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := LoadNotifications()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("want error %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.RabbitMQURL != tt.env["RABBITMQ_URL"] {
				t.Fatalf("want RabbitMQURL %q, got %q", tt.env["RABBITMQ_URL"], cfg.RabbitMQURL)
			}
			if cfg.ShutdownTimeout != defaultShutdownTimeout {
				t.Fatalf("want ShutdownTimeout %v, got %v", defaultShutdownTimeout, cfg.ShutdownTimeout)
			}
		})
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"DATABASE_URL", "RABBITMQ_URL", "HTTP_ADDR", "MIGRATIONS_PATH"} {
		if val, ok := os.LookupEnv(key); ok {
			t.Setenv(key, val)
		}
		os.Unsetenv(key)
	}
}
