package api

import "net/http"

func (s *Server) handleDaemonShutdown(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	writeSuccess(writer, http.StatusOK, map[string]string{"result": "shutting_down"})
	select {
	case s.shutdown <- struct{}{}:
	default:
	}
}
