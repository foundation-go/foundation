package foundation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	fkafka "github.com/foundation-go/foundation/kafka"
	fpg "github.com/foundation-go/foundation/postgresql"
)

func TestHealthHandler(t *testing.T) {
	app := &Service{
		Config: &Config{},
		Logger: initLogger("test"),
	}

	// Actual test function
	assertExample := func(s *Service, expectedStatusCode int) {
		t.Helper()

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Fatalf("could not create request: %v", err)
		}

		w := httptest.NewRecorder()
		s.healthHandler(w, req)

		if w.Code != expectedStatusCode {
			t.Errorf("Expected status code %d, but got %d", expectedStatusCode, w.Code)
		}
	}

	// When no dependencies are enabled
	assertExample(app, http.StatusOK)

	// When database is enabled, but not connected
	app.Components = append(app.Components, &fpg.Component{})
	assertExample(app, http.StatusInternalServerError)

	// When Kafka consumer is enabled, but not connected
	app.Components = []Component{&fkafka.ConsumerComponent{}}
	assertExample(app, http.StatusInternalServerError)

	// When Kafka producer is enabled, but not connected
	app.Components = []Component{&fkafka.ProducerComponent{}}
	assertExample(app, http.StatusInternalServerError)
}
