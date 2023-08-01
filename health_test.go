package foundation

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	app := &Application{
		Config: &Config{},
		Logger: initLogger("test"),
	}

	// Actual test function
	assertExample := func(app *Application, expectedStatusCode int) {
		t.Helper()

		req, err := http.NewRequest(http.MethodGet, "/", nil)
		if err != nil {
			t.Fatalf("could not create request: %v", err)
		}

		w := httptest.NewRecorder()
		app.healthHandler(w, req)

		if w.Code != expectedStatusCode {
			t.Errorf("Expected status code %d, but got %d", expectedStatusCode, w.Code)
		}
	}

	// When no dependencies are enabled
	assertExample(app, http.StatusOK)

	// When database is enabled, but not connected
	app.Config = &Config{DatabaseEnabled: true}
	assertExample(app, http.StatusInternalServerError)

	// When Kafka consumer is enabled, but not connected
	app.Config = &Config{KafkaConsumerEnabled: true}
	assertExample(app, http.StatusInternalServerError)

	// When Kafka producer is enabled, but not connected
	app.Config = &Config{KafkaProducerEnabled: true}
	assertExample(app, http.StatusInternalServerError)
}
