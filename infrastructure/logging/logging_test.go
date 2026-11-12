package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/log/v2"

	"github.com/DaanV2/go-tool-template/infrastructure/logging"
)

func TestSetupWriter(t *testing.T) {
	tests := []struct {
		name    string
		opts    logging.Options
		wantErr bool
	}{
		{name: "defaults", opts: logging.Options{Level: "info"}},
		{name: "json", opts: logging.Options{Level: "debug", Format: "json"}},
		{name: "logfmt", opts: logging.Options{Level: "warn", Format: "logfmt"}},
		{name: "bad level", opts: logging.Options{Level: "loud"}, wantErr: true},
		{name: "bad format", opts: logging.Options{Level: "info", Format: "xml"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			err := logging.SetupWriter(&buf, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetupWriter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetupWriterJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := logging.SetupWriter(&buf, logging.Options{Level: "info", Format: "json"}); err != nil {
		t.Fatal(err)
	}

	log.Info("hello", "key", "value")

	if !strings.Contains(buf.String(), `"msg":"hello"`) {
		t.Fatalf("expected JSON output, got %q", buf.String())
	}
}
