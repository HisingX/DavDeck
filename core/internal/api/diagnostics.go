package api

import (
	"context"
	"net/http"

	"davdeck.dev/davdeck/core/internal/diagnostics"
)

type diagnosticsService interface {
	Run(context.Context) diagnostics.Report
	Latest() (diagnostics.Report, bool)
}

func (s *Server) handleDiagnostics(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	report, available := s.diagnostics.Latest()
	writeSuccess(writer, http.StatusOK, map[string]any{"available": available, "report": optionalReport(report, available)})
}

func (s *Server) handleDiagnosticsRun(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	writeSuccess(writer, http.StatusOK, s.diagnostics.Run(request.Context()))
}

func (s *Server) handleDiagnosticsReport(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	if mode := request.URL.Query().Get("mode"); mode != "" && mode != "redacted" {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Only redacted diagnostic reports are supported", nil)
		return
	}
	report, available := s.diagnostics.Latest()
	if !available {
		writeError(writer, http.StatusNotFound, ErrorCode("DIAGNOSTICS_NOT_RUN"), "Diagnostics have not been run", nil)
		return
	}
	writeSuccess(writer, http.StatusOK, report)
}

func optionalReport(report diagnostics.Report, available bool) any {
	if !available {
		return nil
	}
	return report
}
